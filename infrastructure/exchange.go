package infrastructure

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	askPrice := book.Asks[0].Price
	return &models.Ticker{
		BaseSymbol:  strings.Replace(symbol, *qs, "", 1),
		QuoteSymbol: *qs,
		BidPrice:    bidPrice,
		AskPrice:    askPrice,
	}, nil
}

func (ex Exchange) GetTicker(symbol string) (*models.Ticker, error) {
	tk, _ := ex.TickersCache.Get(symbol)
	return tk.(*models.Ticker), nil
}

func (ex Exchange) GetTickers() ([]*models.Ticker, error) {
	tks := []*models.Ticker{}
	for _, v := range ex.TickersCache.Items() {
		tks = append(tks, v.(*models.Ticker))
	}
	return tks, nil
}

func (ex Exchange) UpdatedTickers(recv chan []*models.Ticker) error {
	for _, s := range ex.Symbols {
		go func(s string) {
			obr := binance.OrderBookRequest{
				Symbol: s,
			}
			obch, _, err := ex.Api.OrderBookWebsocket(obr)
			if err != nil {
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
				tickers, err := ex.GetTickers()
				if err != nil {
					fmt.Printf("Error: %#v\n", err)
					continue
				}
				recv <- tickers
			}
		}(s)
	}
	return nil
}
