package usecase

import (
	"fmt"

	models "github.com/OopsMouse/arbitgo/models"
)

type Arbitrader struct {
	MarketRepository MarketRepository
	MarketAnalyzer   MarketAnalyzer
}

func (arbit *Arbitrader) Run() {
	ch := make(chan *models.Market)
	err := arbit.MarketRepository.UpdatedMarket(ch)
	if err != nil {
		panic(err)
	}
	for {
		mk := <-ch
		tr, err := arbit.MarketAnalyzer.GetBestTradeRoutes(mk)

		if err != nil {
			continue
		}
		arbit.Trade(tr)
	}
}

func (arbit *Arbitrader) Trade(tr []*models.TradeRoute) {
	for _, t := range tr {
		fmt.Printf("Symbol: %s Side: %s Price %f\n", t.Symbol, t.Side, t.Price)
	}
	fmt.Printf("\n")
}
