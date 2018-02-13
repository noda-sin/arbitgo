package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type Arbitrader struct {
	Exchange
	MarketAnalyzer
	DryRun bool
}

func (arbit *Arbitrader) Run() {
	ch := make(chan *models.Market)
	err := arbit.Exchange.OnUpdatedMarket(ch)
	if err != nil {
		panic(err)
	}
	for {
		mk := <-ch
		tr, err := arbit.MarketAnalyzer.GetBestTrade(mk, 0.0)

		if err != nil {
			// TODO: エラー処理
			continue
		}

		err = arbit.Trade(tr)
		if err != nil {
			// TODO: エラー処理
			continue
		}

		// success!!
	}
}

func (arbit *Arbitrader) Trade(tr *models.Trade) error {
	for _, or := range tr.Orders {
		if arbit.DryRun {
			continue
		}

		err := arbit.Exchange.SendOrder(or)
		if err != nil {
			return err
		}
	}
	return nil
}
