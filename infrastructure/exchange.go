package infrastructure

import (
	"context"
	"os"
	"strings"

	common "github.com/OopsMouse/arbitgo/common"
	models "github.com/OopsMouse/arbitgo/models"
	binance "github.com/OopsMouse/go-binance"
	"github.com/go-kit/kit/log"
)

type Exchange struct {
	Api          binance.Binance
	QuoteSymbols []string
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
	return Exchange{
		Api: b,
		QuoteSymbols: []string{
			"BTC",
			"BNB",
			"ETH",
			"USDT",
		},
	}
}

func (ex Exchange) tickers24ToTickers(ts24 []*binance.Ticker24) []*models.Ticker {
	tickers := []*models.Ticker{}
	for _, t24 := range ts24 {
		symbl := t24.Symbol
		qss := common.Filter(ex.QuoteSymbols, func(q string) bool {
			return strings.HasSuffix(symbl, q)
		})
		if len(qss) == 0 {
			continue
		}
		qs := qss[0]
		ticker := models.NewTicker(
			strings.Replace(symbl, qs, "", 1),
			qs,
			t24.BidPrice,
			t24.AskPrice,
			t24.Volume,
		)
		tickers = append(tickers, ticker)
	}
	return tickers
}

func (ex Exchange) GetTickers() ([]*models.Ticker, error) {
	ts24, err := ex.Api.Tickers24()
	if err != nil {
		return nil, err
	}
	return ex.tickers24ToTickers(ts24), nil
}

func (ex Exchange) UpdatedTickers(recv chan []*models.Ticker) error {
	ech, done, err := ex.Api.Tickers24Websocket()
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case ts24event := <-ech:
				recv <- ex.tickers24ToTickers(ts24event.Tickers24)
			case <-done:
				break
			}
		}
	}()
	return nil
}
