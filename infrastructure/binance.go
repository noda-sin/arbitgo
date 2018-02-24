package infrastructure

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	binance "github.com/OopsMouse/go-binance"
	"github.com/go-kit/kit/log"
	"github.com/orcaman/concurrent-map"
	"github.com/pkg/errors"
)

type Binance struct {
	Api          binance.Binance
	QuoteAsset   *util.Set
	Symbols      []models.Symbol
	DepthCache   cmap.ConcurrentMap
	OrderRetry   int
	UseWebsocket bool
}

func NewBinance(apikey string, secret string) Binance {
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
		symbol := models.Symbol{
			Text:           s.Symbol,
			BaseAsset:      models.Asset(s.BaseAsset),
			BasePrecision:  s.BaseAssetPrecision,
			QuoteAsset:     models.Asset(s.QuoteAsset),
			QuotePrecision: s.QuotePrecision,
		}
		for _, f := range s.Filters {
			filterType := f["filterType"].(string)
			if filterType == "PRICE_FILTER" {
				symbol.MinPrice, _ = strconv.ParseFloat(f["minPrice"].(string), 64)
				symbol.MaxPrice, _ = strconv.ParseFloat(f["maxPrice"].(string), 64)
				symbol.TickSize, _ = strconv.ParseFloat(f["tickSize"].(string), 64)
			} else if filterType == "LOT_SIZE" {
				symbol.MinQty, _ = strconv.ParseFloat(f["minQty"].(string), 64)
				symbol.MaxQty, _ = strconv.ParseFloat(f["maxQty"].(string), 64)
				symbol.StepSize, _ = strconv.ParseFloat(f["stepSize"].(string), 64)
			} else if filterType == "MIN_NOTIONAL" {
				symbol.MinNotional, _ = strconv.ParseFloat(f["minNotional"].(string), 64)
			}
		}
		symbols = append(symbols, symbol)
	}

	chans := []chan *models.Symbol{}
	for _, s := range symbols {
		ch := make(chan *models.Symbol)
		go func(s models.Symbol) {
			err := util.BackoffRetry(5, func() error {
				tkr := binance.TickerRequest{
					Symbol: s.Text,
				}
				tk24, err := b.Ticker24(tkr)
				if tk24 != nil {
					s.Volume = tk24.Volume
				}
				return err
			})

			if err != nil {
				panic(err)
			}

			ch <- &s
		}(s)
		chans = append(chans, ch)
	}

	symbols = []models.Symbol{}
	for _, ch := range chans {
		symbol := <-ch
		symbols = append(symbols, *symbol)
	}

	ex := Binance{
		Api:          b,
		QuoteAsset:   quoteAssetSet,
		Symbols:      symbols,
		DepthCache:   cmap.New(),
		OrderRetry:   10,
		UseWebsocket: true,
	}
	return ex
}

func (bi Binance) GetCharge() float64 {
	return 0.001
}

func (bi Binance) GetBalance(asset models.Asset) (*models.Balance, error) {
	balances, err := bi.GetBalances()
	if err != nil {
		return nil, err
	}
	for _, b := range balances {
		if string(b.Asset) == string(asset) {
			return b, nil
		}
	}
	return nil, fmt.Errorf("Not found balance for %s", string(asset))
}

func (bi Binance) GetBalances() ([]*models.Balance, error) {
	acr := binance.AccountRequest{
		RecvWindow: 5 * time.Second,
		Timestamp:  time.Now(),
	}
	var account *binance.Account
	err := util.BackoffRetry(5, func() error {
		a, err := bi.Api.Account(acr)
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

func (bi Binance) GetSymbols() []models.Symbol {
	return bi.Symbols
}

func (bi Binance) GetDepth(symbol models.Symbol) (*models.Depth, error) {
	request := binance.OrderBookRequest{
		Symbol: symbol.String(),
		Limit:  5,
	}
	book, err := bi.Api.OrderBook(request)
	if err != nil {
		return nil, err
	}
	depth, err := getDepthInOrderBook(symbol, book)
	if err != nil {
		return nil, err
	}
	return depth, nil
}

func getDepthInOrderBook(symbol models.Symbol, orderBook *binance.OrderBook) (*models.Depth, error) {
	if len(orderBook.Bids) < 1 ||
		len(orderBook.Asks) < 1 {
		return nil, errors.Errorf("Bids or Asks length is empty")
	}

	quoteAsset := symbol.QuoteAsset
	baseAsset := symbol.BaseAsset
	bidPrice := orderBook.Bids[0].Price
	bidQty := orderBook.Bids[0].Quantity
	for i := 1; i < len(orderBook.Bids); i++ {
		if orderBook.Bids[i].Price == bidPrice {
			bidQty += orderBook.Bids[i].Quantity
		} else {
			break
		}
	}
	askPrice := orderBook.Asks[0].Price
	askQty := orderBook.Asks[0].Quantity
	for i := 1; i < len(orderBook.Asks); i++ {
		if orderBook.Asks[i].Price == askPrice {
			askQty += orderBook.Asks[i].Quantity
		} else {
			break
		}
	}
	return &models.Depth{
		Symbol:     symbol,
		BaseAsset:  models.Asset(baseAsset),
		QuoteAsset: quoteAsset,
		BidPrice:   bidPrice,
		AskPrice:   askPrice,
		BidQty:     bidQty,
		AskQty:     askQty,
		Time:       time.Now(),
	}, nil
}

func (bi Binance) connectWebsocket(symbol models.Symbol) (chan *binance.OrderBook, chan struct{}, error) {
	var obch chan *binance.OrderBook
	var done chan struct{}
	req := binance.OrderBookRequest{
		Symbol: symbol.String(),
		Level:  20,
	}
	err := util.BackoffRetry(5, func() error {
		r, d, err := bi.Api.OrderBookWebsocket(req)
		obch = r
		done = d
		return err
	})
	return obch, done, err
}

func (bi Binance) getDepthOnUpdateWebsocket(symbols []models.Symbol) (chan *models.Depth, chan bool) {
	m := new(sync.Mutex)
	stopping := false
	dch := make(chan *models.Depth)
	websockClose := make(chan struct{})

	for _, symbol := range symbols {
		go func(symbol models.Symbol) {
			for {
				for stopping {
				}

				go func(symbol models.Symbol) {
					defer func() {
						websockClose <- struct{}{}
					}()

					if stopping {
						return
					}

					obch, done, err := bi.connectWebsocket(symbol)

					if err != nil {
						return
					}

					for {
						if stopping {
							return
						}

						select {
						case orderbook := <-obch:
							depth, _ := getDepthInOrderBook(
								symbol,
								orderbook,
							)
							dch <- depth
						case <-done:
							return
						default:
						}
					}
				}(symbol)
				<-websockClose
			}
		}(symbol)
	}

	stopch := make(chan bool)

	go func() {
		defer close(stopch)
		for {
			s := <-stopch
			m.Lock()
			stopping = s
			m.Unlock()
		}
	}()

	return dch, stopch
}

func (bi Binance) getDepthOnUpdateRequest(symbols []models.Symbol) (chan *models.Depth, chan bool) {
	m := new(sync.Mutex)
	stopping := false
	stopch := make(chan bool)
	dch := make(chan *models.Depth)

	depthReqChan := make(chan models.Symbol)

	go func() {
		for {
			if stopping {
				continue
			}

			select {
			case symbol := <-depthReqChan:
				depth, err := bi.GetDepth(symbol)
				time.Sleep(1 / 1200 * time.Minute)
				if err != nil {
					continue
				}
				dch <- depth
			}
		}
	}()

	go func() {
		for {
			for _, symbol := range symbols {
				depthReqChan <- symbol
				time.Sleep(1 / 1200 * time.Minute)
			}
		}
	}()

	go func() {
		defer close(stopch)
		for {
			s := <-stopch
			m.Lock()
			stopping = s
			m.Unlock()
		}
	}()

	return dch, stopch
}

func (bi Binance) getQuoteToQuotePairSymbols(symbols []models.Symbol) []models.Symbol {
	ret := []models.Symbol{}
	for _, s := range symbols {
		if bi.QuoteAsset.Include(string(s.BaseAsset)) &&
			bi.QuoteAsset.Include(string(s.QuoteAsset)) {
			ret = append(ret, s)
		}
	}
	return ret
}

func (bi Binance) getQuoteToBasePairSymbols(symbols []models.Symbol) []models.Symbol {
	ret := []models.Symbol{}
	for _, s := range symbols {
		if !bi.QuoteAsset.Include(string(s.BaseAsset)) {
			ret = append(ret, s)
		}
	}
	return ret
}

func (bi Binance) GetDepthOnUpdate() chan *models.Depth {
	dch := make(chan *models.Depth)
	symbols := bi.GetSymbols()
	r, _ := bi.getDepthOnUpdateRequest(bi.getQuoteToQuotePairSymbols(symbols))
	w, _ := bi.getDepthOnUpdateWebsocket(bi.getQuoteToBasePairSymbols(symbols))
	go func() {
		for {
			select {
			case d := <-r:
				dch <- d
			case d := <-w:
				dch <- d
			}
		}
	}()
	return dch
}

func (bi Binance) SendOrder(order *models.Order) error {
	var side binance.OrderSide
	if order.Side == models.SideBuy {
		side = binance.SideBuy
	} else {
		side = binance.SideSell
	}
	var nor binance.NewOrderRequest
	if order.OrderType == models.TypeLimit {
		nor = binance.NewOrderRequest{
			Symbol:           order.Symbol.String(),
			Type:             binance.TypeLimit,
			TimeInForce:      binance.GTC,
			Side:             side,
			Quantity:         order.Qty,
			Price:            order.Price,
			NewClientOrderID: order.ClientOrderID,
			Timestamp:        time.Now(),
		}
	} else {
		nor = binance.NewOrderRequest{
			Symbol:           order.Symbol.String(),
			Type:             binance.TypeMarket,
			Side:             side,
			Quantity:         order.Qty,
			NewClientOrderID: order.ClientOrderID,
			Timestamp:        time.Now(),
		}
	}
	err := util.BackoffRetry(5, func() error {
		return bi.Api.NewOrderTest(nor)
	})
	if err != nil {
		return err
	}
	var po *binance.ProcessedOrder
	err = util.BackoffRetry(5, func() error {
		p, err := bi.Api.NewOrder(nor)
		po = p
		return err
	})
	if err != nil {
		return err
	}
	return nil
}

func (bi Binance) ConfirmOrder(order *models.Order) (float64, error) {
	oor := binance.OpenOrdersRequest{
		Symbol:     order.Symbol.String(),
		RecvWindow: 5 * time.Second,
		Timestamp:  time.Now(),
	}
	var openOrders []*binance.ExecutedOrder
	err := util.BackoffRetry(5, func() error {
		oo, err := bi.Api.OpenOrders(oor)
		openOrders = oo
		return err
	})
	if err != nil {
		return 0, err
	}
	for _, o := range openOrders {
		if o.ClientOrderID == order.ClientOrderID {
			return o.ExecutedQty, nil
		}
	}
	return order.Qty, nil
}

func (bi Binance) CancelOrder(order *models.Order) error {
	cor := binance.CancelOrderRequest{
		Symbol:            order.Symbol.String(),
		OrigClientOrderID: order.ClientOrderID,
		RecvWindow:        5 * time.Second,
		Timestamp:         time.Now(),
	}
	err := util.BackoffRetry(5, func() error {
		_, err := bi.Api.CancelOrder(cor)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}
