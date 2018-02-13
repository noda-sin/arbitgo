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
	"github.com/jpillora/backoff"
	"github.com/orcaman/concurrent-map"
)

type Exchange struct {
	Api          binance.Binance
	QuoteSymbols []string
	Symbols      []string
	TickersCache cmap.ConcurrentMap
	RetryOrder   int
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
		RetryOrder:   10,
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

func (ex Exchange) GetBalance(symbol string) (*models.Balance, error) {
	balances, err := ex.GetBalances()
	if err != nil {
		return nil, err
	}
	for _, b := range balances {
		if b.Symbol == symbol {
			return b, nil
		}
	}
	return nil, fmt.Errorf("Not found balance for %s", symbol)
}

func (ex Exchange) GetBalances() ([]*models.Balance, error) {
	acr := binance.AccountRequest{
		Timestamp: time.Now(),
	}
	account, err := ex.Api.Account(acr)
	if err != nil {
		return nil, err
	}
	balances := []*models.Balance{}
	for _, b := range account.Balances {
		balance := &models.Balance{
			Symbol: b.Asset,
			Free:   b.Free,
			Total:  b.Free + b.Locked,
		}
		balances = append(balances, balance)
	}
	return balances, nil
}

func (ex Exchange) GetTicker(symbol string) (*models.Ticker, error) {
	tk, _ := ex.TickersCache.Get(symbol)
	return tk.(*models.Ticker), nil
}

func (ex Exchange) GetMarket(startSymbol string) (*models.Market, error) {
	tks := []*models.Ticker{}
	for _, v := range ex.TickersCache.Items() {
		tks = append(tks, v.(*models.Ticker))
	}
	return models.NewMarket(startSymbol, tks), nil
}

func (ex Exchange) OnUpdatedMarket(startSymbol string, recv chan *models.Market) error {
	for _, s := range ex.Symbols {
		go func(s string) {
			obr := binance.OrderBookRequest{
				Symbol: s,
			}

			b := &backoff.Backoff{
				Max: 5 * time.Minute,
			}

			var obch chan *binance.OrderBook
			for {
				ret, _, err := ex.Api.OrderBookWebsocket(obr)
				obch = ret
				if err != nil {
					d := b.Duration()
					fmt.Printf("%s, reconnecting in %s", err, d)
					time.Sleep(d)
					continue
				}
				b.Reset()
				break
			}

			for {
				orderbook := <-obch
				ticker, err := ex.ConvertOrderBook2Ticker(s, orderbook)
				if err != nil {
					fmt.Printf("%s, convert error, order book to ticker, last update id is %#v\n", err, orderbook.LastUpdateID)
					continue
				}
				ex.TickersCache.Set(s, ticker)
				mkt, err := ex.GetMarket(startSymbol)
				if err != nil {
					fmt.Printf("error get market error, %s\n", startSymbol)
					continue
				}
				recv <- mkt
			}
		}(s)
	}
	return nil
}

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
	b := &backoff.Backoff{
		Max: 5 * time.Minute,
	}
	for i := 0; i < ex.RetryOrder; i++ {
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
		time.Sleep(b.Duration())
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
