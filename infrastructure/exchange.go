package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	binance "github.com/OopsMouse/go-binance"
	"github.com/go-kit/kit/log"
	"github.com/orcaman/concurrent-map"
)

type Exchange struct {
	Api            binance.Binance
	QuoteAssetList []models.Asset
	Symbols        []models.Symbol
	DepthCache     cmap.ConcurrentMap
	OrderRetry     int
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
		apikey,
		hmacSigner,
		logger,
		ctx,
	)

	b := binance.NewBinance(binanceService)

	var exInfo *binance.ExchangeInfo
	err := util.BackoffRetry(5, func() error {
		e, err := b.ExchangeInfo()
		exInfo = e
		return err
	})

	if err != nil {
		panic(err)
	}

	quoteAssetSet := util.NewSet()
	symbols := []models.Symbol{}
	for _, s := range exInfo.Symbols {
		if s.Symbol == "123456" { // binanceのゴミ
			continue
		}
		quoteAssetSet.Append(s.QuoteAsset)
		symbols = append(symbols, models.Symbol(s.Symbol))
	}

	quoteAssetList := []models.Asset{}
	for _, s := range quoteAssetSet.ToSlice() {
		quoteAssetList = append(quoteAssetList, models.Asset(s))
	}

	ex := Exchange{
		Api:            b,
		QuoteAssetList: quoteAssetList,
		Symbols:        symbols,
		DepthCache:     cmap.New(),
		OrderRetry:     10,
	}
	return ex
}

func (ex Exchange) GetCharge() float64 {
	return 0.001
}

func (ex Exchange) GetBalance(asset models.Asset) (*models.Balance, error) {
	balances, err := ex.GetBalances()
	if err != nil {
		return nil, err
	}
	for _, b := range balances {
		if b.Asset == asset {
			return b, nil
		}
	}
	return nil, fmt.Errorf("Not found balance for %s", asset)
}

func (ex Exchange) GetBalances() ([]*models.Balance, error) {
	acr := binance.AccountRequest{
		RecvWindow: 5 * time.Second,
		Timestamp:  time.Now(),
	}
	var account *binance.Account
	err := util.BackoffRetry(5, func() error {
		a, err := ex.Api.Account(acr)
		account = a
		return err
	})
	if err != nil {
		return nil, err
	}
	balances := []*models.Balance{}
	for _, b := range account.Balances {
		balance := &models.Balance{
			Asset: models.Asset(b.Asset),
			Free:  b.Free,
			Total: b.Free + b.Locked,
		}
		balances = append(balances, balance)
	}
	return balances, nil
}

func (ex Exchange) GetSymbols() []models.Symbol {
	return ex.Symbols
}

func (ex Exchange) SetDepth(symbol models.Symbol, depth *models.Depth) {
	ex.DepthCache.Set(string(symbol), depth)
}

func (ex Exchange) GetDepthList() ([]*models.Depth, error) {
	depthList := []*models.Depth{}
	for _, v := range ex.DepthCache.Items() {
		depthList = append(depthList, v.(*models.Depth))
	}
	return depthList, nil
}

func GetQuoteAsset(symbol models.Symbol, quoteAssetList []models.Asset) (*models.Asset, error) {
	for _, quoteAsset := range quoteAssetList {
		if strings.HasSuffix(string(symbol), string(quoteAsset)) {
			return &quoteAsset, nil
		}
	}
	return nil, errors.New("not found quote asset: " + string(symbol))
}

func GetDepthInOrderBook(symbol models.Symbol, orderBook *binance.OrderBook, quoteAssetList []models.Asset) (*models.Depth, error) {
	quoteAsset, err := GetQuoteAsset(symbol, quoteAssetList)
	if err != nil {
		return nil, err
	}
	baseAsset := strings.Replace(string(symbol), string(*quoteAsset), "", 1)
	bidPrice := orderBook.Bids[0].Price
	bidQty := orderBook.Bids[0].Quantity
	askPrice := orderBook.Asks[0].Price
	askQty := orderBook.Asks[0].Quantity

	return &models.Depth{
		Symbol:     symbol,
		BaseAsset:  models.Asset(baseAsset),
		QuoteAsset: *quoteAsset,
		BidPrice:   bidPrice,
		AskPrice:   askPrice,
		BidQty:     bidQty,
		AskQty:     askQty,
	}, nil
}

func (ex Exchange) OnUpdateDepthList(recv chan []*models.Depth) error {
	for _, symbol := range ex.Symbols {
		go func(symbol models.Symbol) {
			request := binance.OrderBookRequest{
				Symbol: string(symbol),
			}

			for {
				var obch chan *binance.OrderBook
				var done chan struct{}
				err := util.BackoffRetry(5, func() error {
					r, d, err := ex.Api.OrderBookWebsocket(request)
					obch = r
					done = d
					return err
				})

				if err != nil {
					continue
				}

				for {
					select {
					case orderbook := <-obch:
						depth, err := GetDepthInOrderBook(
							symbol,
							orderbook,
							ex.QuoteAssetList,
						)
						if err != nil {
							fmt.Printf("%s, convert error, order book to depth, last update id is %#v\n", err, orderbook.LastUpdateID)
							continue
						}
						ex.SetDepth(symbol, depth)
						depthList, err := ex.GetDepthList()
						if err != nil {
							fmt.Printf("get market error")
							continue
						}
						recv <- depthList
					case <-done:
						break
					}
				}
			}
		}(symbol)
	}
	return nil
}

func (ex Exchange) SendOrder(order *models.Order) error {
	var side binance.OrderSide
	if order.Side == models.SideBuy {
		side = binance.SideBuy
	} else {
		side = binance.SideSell
	}
	var nor binance.NewOrderRequest
	if order.OrderType == models.TypeLimit {
		nor = binance.NewOrderRequest{
			Symbol:      string(order.Symbol),
			Type:        binance.TypeLimit,
			TimeInForce: binance.GTC,
			Side:        side,
			Quantity:    order.Qty,
			Price:       order.Price,
			Timestamp:   time.Now(),
		}
	} else {
		nor = binance.NewOrderRequest{
			Symbol:    string(order.Symbol),
			Type:      binance.TypeMarket,
			Side:      side,
			Quantity:  order.Qty,
			Timestamp: time.Now(),
		}
	}
	err := util.BackoffRetry(5, func() error {
		return ex.Api.NewOrderTest(nor)
	})
	if err != nil {
		return err
	}
	var po *binance.ProcessedOrder
	err = util.BackoffRetry(5, func() error {
		p, err := ex.Api.NewOrder(nor)
		po = p
		return err
	})
	if err != nil {
		return err
	}
	orderID := po.OrderID
	for i := 0; i < ex.OrderRetry; i++ {
		oor := binance.OpenOrdersRequest{
			Symbol:     string(order.Symbol),
			RecvWindow: 5 * time.Second,
			Timestamp:  time.Now(),
		}
		var oo []*binance.ExecutedOrder
		err := util.BackoffRetry(5, func() error {
			o, err := ex.Api.OpenOrders(oor)
			oo = o
			return err
		})
		if err != nil {
			return err
		}
		if len(oo) == 0 {
			return nil
		}
		time.Sleep(10 * time.Second)
	}

	cor := binance.CancelOrderRequest{
		Symbol:     string(order.Symbol),
		OrderID:    orderID,
		RecvWindow: 5 * time.Second,
		Timestamp:  time.Now(),
	}
	err = util.BackoffRetry(5, func() error {
		_, err := ex.Api.CancelOrder(cor)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}
