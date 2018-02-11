package usecase

import (
	"fmt"

	common "github.com/OopsMouse/arbitgo/common"
	models "github.com/OopsMouse/arbitgo/models"
)

type MarketAnalyzer struct {
}

func (ma *MarketAnalyzer) GetBestTradeRoutes(m *models.Market) ([]*models.TradeRoute, error) {
	bestScore := 0.0
	var bestTradeRoutes []*models.TradeRoute
	for _, tks := range m.GetTradeTickers() {
		score, tr := calcDistortion(m.StartSymbol, tks)
		if tr == nil {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestTradeRoutes = tr
		}
	}

	if bestScore <= 0 {
		return nil, fmt.Errorf("Best trade routes not found")
	}

	fmt.Printf("Best score: %f\n", bestScore)
	return bestTradeRoutes, nil
}

func calcDistortion(startSymbol string, tickers []*models.Ticker) (float64, []*models.TradeRoute) {
	if len(tickers) == 0 {
		return 0.0, nil
	}

	tradeRoutes := []*models.TradeRoute{}
	curr := 1.0
	nextSymbol := startSymbol
	for _, tk := range tickers {
		if tk.QuoteSymbol == nextSymbol {
			curr /= tk.AskPrice
			nextSymbol = tk.BaseSymbol
			tradeRoute := &models.TradeRoute{
				Symbol: tk.BaseSymbol + tk.QuoteSymbol,
				Price:  tk.AskPrice,
				Side:   common.Buy,
			}
			tradeRoutes = append(tradeRoutes, tradeRoute)
		} else {
			curr *= tk.BidPrice
			nextSymbol = tk.QuoteSymbol
			tradeRoute := &models.TradeRoute{
				Symbol: tk.BaseSymbol + tk.QuoteSymbol,
				Price:  tk.BidPrice,
				Side:   common.Sell,
			}
			tradeRoutes = append(tradeRoutes, tradeRoute)
		}
	}

	return (curr - 1.0), tradeRoutes
}
