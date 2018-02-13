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

func NewArbitrader(ex Exchange, ma MarketAnalyzer, s string) *Arbitrader {
	return &Arbitrader{
		Exchange:       ex,
		MarketAnalyzer: ma,
		StartSymbol:    s,
	}
}

func (arbit *Arbitrader) Run() {
	fmt.Printf("started arbit go...\n")
	fmt.Printf("start assets: %s\n", arbit.StartSymbol)
	arbit.PrintBalances()

	ch := make(chan *models.Market)
	err := arbit.Exchange.OnUpdatedMarket(arbit.StartSymbol, ch)
	if err != nil {
		panic(err)
	}
	for {
		balance, err := arbit.Exchange.GetBalance(arbit.StartSymbol)

		if err != nil {
			fmt.Printf("get balance error. wait 5 minute, %v\n", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		for {
			market := <-ch
			trade := arbit.MarketAnalyzer.GetBestTrade(
				market,
				arbit.Exchange.GetCharge(),
				balance.Free,
				0.001,
			)

			if trade == nil {
				continue
			}

			fmt.Printf("found a route that can take profits, profit => %f\n", trade.Profit)
			for i, o := range trade.Orders {
				fmt.Printf("[%d] symbol => %s, side => %s, price => %f, qty => %f\n", i, o.Symbol, o.Side, o.Price, o.BaseQty)
			}

			err = arbit.Trade(trade)
			if err != nil {
				fmt.Printf("when trading, unknown error occur, %v\n", err)
				fmt.Printf("please manual recovery. will be shutdown\n")
				panic(err)
			}

			fmt.Printf("success to arbitrage\n")
			arbit.PrintBalances()
			time.Sleep(10 * time.Second)
			break
		}

		fmt.Printf("success to arbitrage\n")
		arbit.PrintBalances()
		time.Sleep(10 * time.Second)
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

func (arbit *Arbitrader) PrintBalances() {
	balances, err := arbit.Exchange.GetBalances()
	if err != nil {
		return
	}

	fmt.Printf("balances:\n")
	for _, b := range balances {
		fmt.Printf("[%s] => %f\n", b.Symbol, b.Total)
	}
}
