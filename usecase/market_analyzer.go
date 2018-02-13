package usecase

import (
	common "github.com/OopsMouse/arbitgo/common"
	models "github.com/OopsMouse/arbitgo/models"
)

type MarketAnalyzer struct {
}

func (ma *MarketAnalyzer) GetBestTrade(m *models.Market, balance float64, threshold float64) *models.Trade {
	charge := 0.001

	var bestTrade *models.Trade
	for _, tks := range m.GetTradeTickers() {
		or := calcTradeDistortion(m.StartSymbol, tks, balance, charge)
		if or == nil {
			continue
		}
		if bestTrade == nil || or.Score > bestTrade.Score {
			bestTrade = or
		}
	}

	if bestTrade == nil || bestTrade.Score <= threshold {
		return nil
	}

	return bestTrade
}

func calcTradeDistortion(startSymbol string, tickers []*models.Ticker, balance float64, charge float64) *models.Trade {
	if len(tickers) == 0 {
		return nil
	}

	currentSymbol := startSymbol
	minQty := 0.0
	profit := 1.0

	// 1回目の取引
	tk1 := tickers[0]
	symbol1 := tk1.BaseSymbol + tk1.QuoteSymbol
	var side1 string
	var price1 float64
	var marketQty1 float64
	var qty1 float64
	if tk1.QuoteSymbol == currentSymbol {
		side1 = common.Buy
		price1 = tk1.AskPrice
		marketQty1 = tk1.AskQty
		qty1 = tk1.AskPrice * tk1.AskQty
		currentSymbol = tk1.BaseSymbol
		profit = (1 - charge) * profit / tk1.AskPrice
	} else {
		side1 = common.Sell
		price1 = tk1.BidPrice
		marketQty1 = tk1.BidQty
		qty1 = tk1.BidQty
		currentSymbol = tk1.QuoteSymbol
		profit = (1 - charge) * profit * tk1.BidPrice
	}
	minQty = qty1

	// 2回目の取引
	tk2 := tickers[1]
	symbol2 := tk2.BaseSymbol + tk2.QuoteSymbol
	var side2 string
	var price2 float64
	var marketQty2 float64
	var qty2 float64
	if tk2.QuoteSymbol == currentSymbol {
		side2 = common.Buy
		price2 = tk2.AskPrice
		marketQty2 = tk2.AskQty
		if side1 == common.Buy {
			qty2 = tk2.AskPrice * tk2.AskQty * price1
		} else {
			qty2 = tk2.AskPrice * tk2.AskQty * (1.0 / price1)
		}
		currentSymbol = tk2.BaseSymbol
		profit = (1 - charge) * profit / tk2.AskPrice
	} else {
		side2 = common.Sell
		price2 = tk2.BidPrice
		marketQty2 = tk2.BidQty
		if side1 == common.Buy {
			qty2 = tk2.BidQty * price1
		} else {
			// ありえない
			return nil
		}
		currentSymbol = tk2.QuoteSymbol
		profit = (1 - charge) * profit * tk2.BidPrice
	}

	if qty2 < minQty {
		minQty = qty2
	}

	// 3回目の取引
	tk3 := tickers[2]
	symbol3 := tk3.BaseSymbol + tk3.QuoteSymbol
	var side3 string
	var price3 float64
	var marketQty3 float64
	var qty3 float64
	if tk3.QuoteSymbol == currentSymbol {
		side3 = common.Buy
		price3 = tk3.AskPrice
		marketQty3 = tk3.AskQty
		qty3 = tk3.AskQty * (1.0 / tk3.AskPrice)
		profit = (1 - charge) * profit / tk3.AskPrice
	} else {
		side3 = common.Sell
		price3 = tk3.BidPrice
		marketQty3 = tk3.BidQty
		qty3 = tk3.BidQty * tk3.BidPrice
		profit = (1 - charge) * profit * tk3.BidPrice
	}

	if qty3 < minQty {
		minQty = qty3
	}

	if balance < minQty {
		minQty = balance
	}

	score := (profit - 1.0)

	var qqty1 float64
	if side1 == common.Buy {
		qty1 = minQty / price1
	} else {
		qty1 = minQty
	}
	qqty1 = minQty

	var qqty2 float64
	if side2 == common.Buy {
		qty2 = qty1 / price2
	} else {
		qty2 = qty1
	}
	qqty2 = qqty1

	var qqty3 float64
	if side3 == common.Buy {
		qty3 = qty2 / price3
	} else {
		qty3 = qty2
	}
	qqty3 = qqty2

	orders := []*models.Order{}
	orders = append(orders, &models.Order{
		Symbol:     symbol1,
		QuoteAsset: tk1.QuoteSymbol,
		BaseAsset:  tk1.BaseSymbol,
		Price:      price1,
		Side:       side1,
		MarketQty:  marketQty1,
		BaseQty:    qty1,
		QuoteQty:   qqty1,
	})

	orders = append(orders, &models.Order{
		Symbol:     symbol2,
		QuoteAsset: tk2.QuoteSymbol,
		BaseAsset:  tk2.BaseSymbol,
		Price:      price2,
		Side:       side2,
		MarketQty:  marketQty2,
		BaseQty:    qty2,
		QuoteQty:   qqty2,
	})

	orders = append(orders, &models.Order{
		Symbol:     symbol3,
		QuoteAsset: tk3.QuoteSymbol,
		BaseAsset:  tk3.BaseSymbol,
		Price:      price3,
		Side:       side3,
		MarketQty:  marketQty3,
		BaseQty:    qty3,
		QuoteQty:   qqty3,
	})

	return &models.Trade{
		MaxQty: minQty,
		Profit: score * minQty,
		Score:  score,
		Orders: orders,
	}
}
