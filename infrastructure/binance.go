package infrastructure

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	binance "github.com/OopsMouse/go-binance"
	"github.com/go-kit/kit/log"
	"github.com/orcaman/concurrent-map"
	"github.com/pkg/errors"
)

type Binance struct {
	Api            binance.Binance
	QuoteAssetList []models.Asset
	Symbols        []models.Symbol
	DepthCache     cmap.ConcurrentMap
	OrderRetry     int
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

	// chans := []chan *models.Symbol{}
	// for _, s := range symbols {
	// 	ch := make(chan *models.Symbol)
	// 	go func(s models.Symbol) {
	// 		err := util.BackoffRetry(5, func() error {
	// 			tkr := binance.TickerRequest{
	// 				Symbol: s.Text,
	// 			}
	// 			tk24, err := b.Ticker24(tkr)
	// 			if tk24 != nil {
	// 				s.Volume = tk24.Volume
	// 			}
	// 			return err
	// 		})

	// 		if err != nil {
	// 			panic(err)
	// 		}

	// 		ch <- &s
	// 	}(s)
	// 	chans = append(chans, ch)
	// }

	// symbols = []models.Symbol{}
	// for _, ch := range chans {
	// 	symbol := <-ch
	// 	symbols = append(symbols, *symbol)
	// }

	// symbols = FilterByTopVolume(symbols, 30)

	quoteAssetList := []models.Asset{}
	for _, s := range quoteAssetSet.ToSlice() {
		quoteAssetList = append(quoteAssetList, models.Asset(s))
	}

	ex := Binance{
		Api:            b,
		QuoteAssetList: quoteAssetList,
		Symbols:        symbols,
		DepthCache:     cmap.New(),
		OrderRetry:     10,
	}
	return ex
}

func FilterByTopVolume(symbols []models.Symbol, top int) []models.Symbol {
	quoteAssetList := util.NewSet()
	symbolsByQuote := map[models.Asset][]models.Symbol{}
	for _, s := range symbols {
		quoteAssetList.Append(string(s.QuoteAsset))
		symbolsByQuote[s.QuoteAsset] = append(symbolsByQuote[s.QuoteAsset], s)
	}

	filteredAssetList := util.NewSet()
	for _, v := range symbolsByQuote {
		sort.Sort(models.Symbols(v))
		for i := 0; i < len(v) && i < top; i++ {
			filteredAssetList.Append(string(v[i].BaseAsset))
		}
	}

	filteredSymbols := []models.Symbol{}
	for _, s := range symbols {
		if quoteAssetList.Include(string(s.BaseAsset)) ||
			filteredAssetList.Include(string(s.BaseAsset)) {
			filteredSymbols = append(filteredSymbols, s)
		}
	}
	return filteredSymbols
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

func (bi Binance) SetDepth(symbol models.Symbol, depth *models.Depth) {
	bi.DepthCache.Set(symbol.String(), depth)
}

func (bi Binance) GetDepthList() ([]*models.Depth, error) {
	depthList := []*models.Depth{}
	for _, v := range bi.DepthCache.Items() {
		depthList = append(depthList, v.(*models.Depth))
	}
	return depthList, nil
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
	depth, err := GetDepthInOrderBook(symbol, book, bi.QuoteAssetList)
	if err != nil {
		return nil, err
	}
	bi.SetDepth(symbol, depth)
	return depth, nil
}

func GetDepthInOrderBook(symbol models.Symbol, orderBook *binance.OrderBook, quoteAssetList []models.Asset) (*models.Depth, error) {
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
	}, nil
}

func (bi Binance) OnUpdateDepthList(recv chan []*models.Depth) error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("recover error and retry to exec func", err)
			bi.OnUpdateDepthList(recv)
		}
	}()

	for _, symbol := range bi.Symbols {
		go func(symbol models.Symbol) {
			request := binance.OrderBookRequest{
				Symbol: symbol.String(),
				Level:  20,
			}

			for {
				var obch chan *binance.OrderBook
				var done chan struct{}
				err := util.BackoffRetry(5, func() error {
					r, d, err := bi.Api.OrderBookWebsocket(request)
					obch = r
					done = d
					return err
				})

				if err != nil {
					fmt.Println("retry connect", err)
					continue
				}

				for {
					select {
					case orderbook := <-obch:
						depth, err := GetDepthInOrderBook(
							symbol,
							orderbook,
							bi.QuoteAssetList,
						)
						if err != nil {
							fmt.Printf("%s, convert error, order book to depth, last update id is %#v\n", err, orderbook.LastUpdateID)
							continue
						}
						bi.SetDepth(symbol, depth)
						depthList, err := bi.GetDepthList()
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
