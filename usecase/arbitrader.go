package usecase

import (
	"fmt"
	"time"

	models "github.com/OopsMouse/arbitgo/models"
)

type Arbitrader struct {
	Exchange
	MarketAnalyzer
	StartSymbol string
}

func (arbit *Arbitrader) Run() {
	ch := make(chan *models.Market)
	err := arbit.Exchange.OnUpdatedMarket(arbit.StartSymbol, ch)
	if err != nil {
		panic(err)
	}
	for {
		begin, err := arbit.Exchange.GetBalance(arbit.StartSymbol)

		if err != nil {
			fmt.Printf("%v, get balance error. wait 5 minute.\n", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		market := <-ch
		trade := arbit.MarketAnalyzer.GetBestTrade(
			market,
			arbit.Exchange.GetCharge(),
			begin.Free,
			0.0,
		)

		if trade == nil {
			continue
		}

		err = arbit.Trade(trade)
		if err != nil {
			fmt.Printf("when trading, unknown error occure %v.\n", err)
			fmt.Printf("please manual recovery. will be shutdown... \n")
			panic(err)
		}
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
