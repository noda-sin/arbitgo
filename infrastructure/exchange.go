package infrastructure

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	common "github.com/OopsMouse/arbitgo/common"
	models "github.com/OopsMouse/arbitgo/models"
	binance "github.com/OopsMouse/go-binance"
	"github.com/go-kit/kit/log"
	"github.com/orcaman/concurrent-map"
)

type Exchange struct {
	Api          binance.Binance
	QuoteSymbols []string
	Symbols      []string
	TickersCache cmap.ConcurrentMap
	RtryOrder    int
}

func NewExchange(apikey string, secret string) Exchange {
	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.With(logger, "time", log.DefaultTimestampUTC, "caller", log.DefaultCaller)

	hmacSigner := &binance.HmacSigner{
		Key: []byte(secret),
	}
	ctx, _ := context.WithCancel(context.Background())
	binanceService := binance.NewAPIService(
		"https://www.binance.com",
		os.Getenv("BINANCE_APIKEY"),
		hmacSigner,
		logger,
		ctx,
	)
	b := binance.NewBinance(binanceService)
	exInfo, err := b.ExchangeInfo()
	if err != nil {
		panic(err)
	}
	quoteSymbols := common.NewSet()
	symbols := []string{}
	for _, s := range exInfo.Symbols {
		if s.Symbol == "123456" { // binanceのゴミ
			continue
		}
		quoteSymbols.Append(s.QuoteAsset)
		symbols = append(symbols, s.Symbol)
	}
	ex := Exchange{
		Api:          b,
		QuoteSymbols: quoteSymbols.ToSlice(),
		Symbols:      symbols,
		TickersCache: cmap.New(),
		RtryOrder:    10,
	}
	return ex
}

func (ex Exchange) QuoteSymbol(symbol string) (*string, error) {
	for _, s := range ex.QuoteSymbols {
		if strings.HasSuffix(symbol, s) {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("Not found quote symbol for %s", symbol)
}

func (ex Exchange) ConvertOrderBook2Ticker(symbol string, book *binance.OrderBook) (*models.Ticker, error) {
	qs, err := ex.QuoteSymbol(symbol)
	if err != nil {
		return nil, err
	}
	bidPrice := book.Bids[0].Price
	bidQty := book.Bids[0].Quantity
	askPrice := book.Asks[0].Price
	askQty := book.Asks[0].Quantity
	return &models.Ticker{
		BaseSymbol:  strings.Replace(symbol, *qs, "", 1),
		QuoteSymbol: *qs,
		BidPrice:    bidPrice,
		AskPrice:    askPrice,
		BidQty:      bidQty,
		AskQty:      askQty,
	}, nil
}

func (ex Exchange) GetTicker(symbol string) (*models.Ticker, error) {
	tk, _ := ex.TickersCache.Get(symbol)
	return tk.(*models.Ticker), nil
}

func (ex Exchange) GetMarket() (*models.Market, error) {
	tks := []*models.Ticker{}
	for _, v := range ex.TickersCache.Items() {
		tks = append(tks, v.(*models.Ticker))
	}
	return models.NewMarket(tks), nil
}

func (ex Exchange) OnUpdatedMarket(recv chan *models.Market) error {
	for _, s := range ex.Symbols {
		go func(s string) {
			obr := binance.OrderBookRequest{
				Symbol: s,
			}
			obch, _, err := ex.Api.OrderBookWebsocket(obr)
			if err != nil {
				// TODO: 再接続処理
				panic(err) // 初期の接続で失敗したらpanic
			}

			for {
				orderbook := <-obch
				ticker, err := ex.ConvertOrderBook2Ticker(s, orderbook)
				if err != nil {
					fmt.Printf("Error: %#v\n", err)
					continue
				}
				ex.TickersCache.Set(s, ticker)
				mkt, err := ex.GetMarket()
				if err != nil {
					fmt.Printf("Error: %#v\n", err)
					continue
				}
				recv <- mkt
			}
		}(s)
	}
	return nil
}

// func (ex Exchange) ComfirmOrderBook(order *models.Order) bool {

// }

func (ex Exchange) SendOrder(order *models.Order) error {
	side := binance.SideBuy
	if order.Side == common.Buy {
		side = binance.SideBuy
	} else {
		side = binance.SideSell
	}
	nor := binance.NewOrderRequest{
		Symbol:    order.Symbol,
		Type:      binance.TypeLimit,
		Side:      side,
		Quantity:  order.BaseQty,
		Price:     order.Price,
		Timestamp: time.Now(),
	}
	po, err := ex.Api.NewOrder(nor)
	if err != nil {
		return err
	}
	orderID := po.OrderID
	for i := 0; i < ex.RtryOrder; i++ {
		oor := binance.OpenOrdersRequest{
			Symbol:    order.Symbol,
			Timestamp: time.Now(),
		}
		oo, err := ex.Api.OpenOrders(oor)
		if err != nil {
			return err
		}
		if len(oo) == 0 {
			return nil
		}
		time.Sleep(10 * time.Second)
	}
	cor := binance.CancelOrderRequest{
		Symbol:    order.Symbol,
		OrderID:   orderID,
		Timestamp: time.Now(),
	}
	_, err = ex.Api.CancelOrder(cor)
	if err != nil {
		return err
	}
	return nil
}
