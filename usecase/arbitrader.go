package usecase

import (
	"fmt"

	"github.com/OopsMouse/arbitgo/common"
	models "github.com/OopsMouse/arbitgo/models"
)

type Arbitrader struct {
	Exchange
	MarketAnalyzer
}

func (arbit *Arbitrader) Run() {
	ch := make(chan *models.Market)
	err := arbit.Exchange.OnUpdatedMarket(common.BTC, ch)
	if err != nil {
		panic(err)
	}
	for {
		begin, err := arbit.Exchange.GetBalance(common.BTC)

		if err != nil {
			fmt.Printf("Error: %v", err)
			// TODO: エラー処理
			continue
		}

		mk := <-ch
		tr, err := arbit.MarketAnalyzer.GetBestTrade(mk, begin.Free, 0.0)

		if err != nil {
			fmt.Printf("Error: %v", err)
			// TODO: エラー処理
			continue
		}

		err = arbit.Trade(tr)
		if err != nil {
			fmt.Printf("Error: %v", err)
			// TODO: エラー処理
			continue
		}

		end, err := arbit.Exchange.GetBalance(common.BTC)

		if err != nil {
			fmt.Printf("Error: %v", err)
			// TODO: エラー処理
			continue
		}

		// success!!
		fmt.Printf("Arbit rage success.\n")
		fmt.Printf("Balance updated %f => %f.\n", begin.Total, end.Total)
	}
}

func (arbit *Arbitrader) Trade(tr *models.Trade) error {
	for _, or := range tr.Orders {
		err := arbit.Exchange.SendOrder(or)
		if err != nil {
			return err
		}
	}
	return nil
}
