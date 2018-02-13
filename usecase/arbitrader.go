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
		tr, err := arbit.MarketAnalyzer.GetBestTrade(mk, 0.0)

		if err != nil {
			continue
		}
		arbit.Trade(tr)
	}
}

func (arbit *Arbitrader) Trade(tr *models.Trade) {
	for _, or := range tr.Orders {
		fmt.Printf("Symbol: %s Side: %s Price %f MarketQty: %f BaseQty: %f QuoteQty %f\n", or.Symbol, or.Side, or.Price, or.MarketQty, or.BaseQty, or.QuoteQty)
	}
	fmt.Printf("\n")
}
