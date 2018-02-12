package usecase

import (
	"fmt"

	common "github.com/OopsMouse/arbitgo/common"
	models "github.com/OopsMouse/arbitgo/models"
)

type MarketAnalyzer struct {
}

func (ma *MarketAnalyzer) GetBestTrade(m *models.Market, threshold float64) (*models.Trade, error) {
	charge := 0.001
	var bestTrade *models.Trade
	for _, tks := range m.GetTradeTickers() {
		or := calcTradeDistortion(m.StartSymbol, tks, charge)
		if or == nil {
			continue
		}
		if bestTrade == nil || or.Score > bestTrade.Score {
			bestTrade = or
		}
	}

	if bestTrade == nil || bestTrade.Score <= threshold {
		return nil, fmt.Errorf("Best trade routes not found")
	}

	return bestTrade, nil
}

func calcTradeDistortion(startSymbol string, tickers []*models.Ticker, charge float64) *models.Trade {
	if len(tickers) == 0 {
		return nil
	}

	orders := []*models.Order{}

	// scoreの計算
	curr := 1.0
	nextSymbol := startSymbol
	priceMap := map[string]float64{}
	for _, tk := range tickers {
		if tk.QuoteSymbol == nextSymbol {
			curr = (1 - charge) * curr / tk.AskPrice
			nextSymbol = tk.BaseSymbol
			var startSymbolQty float64
			if tk.QuoteSymbol == startSymbol {
				priceMap[tk.BaseSymbol] = tk.AskPrice
				startSymbolQty = tk.AskQty * tk.AskPrice
			} else {
				if priceMap[tk.QuoteSymbol] != 0 {
					startSymbolQty = tk.AskQty * tk.AskPrice * priceMap[tk.QuoteSymbol]
				} else if priceMap[tk.BaseSymbol] != 0 {
					startSymbolQty = tk.AskQty * tk.AskPrice * (1 / priceMap[tk.BaseSymbol])
				}
			}
			order := &models.Order{
				Symbol:   tk.BaseSymbol + tk.QuoteSymbol,
				Price:    tk.AskPrice,
				Side:     common.Buy,
				BaseQty:  tk.AskQty,
				QuoteQty: startSymbolQty,
			}
			orders = append(orders, order)
		} else {
			curr = (1 - charge) * curr * tk.BidPrice
			nextSymbol = tk.QuoteSymbol
			var startSymbolQty float64
			if tk.QuoteSymbol == startSymbol {
				priceMap[tk.BaseSymbol] = tk.BidPrice
				startSymbolQty = tk.BidQty * tk.BidPrice
			} else {
				if priceMap[tk.QuoteSymbol] != 0 {
					startSymbolQty = tk.BidQty * tk.BidPrice * priceMap[tk.QuoteSymbol]
				} else if priceMap[tk.BaseSymbol] != 0 {
					startSymbolQty = tk.BidQty * tk.BidPrice * (1 / priceMap[tk.BaseSymbol])
				}
			}
			order := &models.Order{
				Symbol:   tk.BaseSymbol + tk.QuoteSymbol,
				Price:    tk.BidPrice,
				Side:     common.Sell,
				BaseQty:  tk.BidQty,
				QuoteQty: startSymbolQty,
			}
			orders = append(orders, order)
		}
	}

	// 取引可能Qtyの計算 (startSymbol換算)
	maxQty := 9999999999999.9
	for _, or := range orders {
		if maxQty > or.QuoteQty {
			maxQty = or.QuoteQty
		}
	}

	return &models.Trade{
		MaxQty: maxQty,
		Profit: (curr - 1.0) * maxQty,
		Score:  (curr - 1.0),
		Orders: orders,
	}
}
