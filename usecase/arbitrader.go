package usecase

import (
	"fmt"
	"time"

	models "github.com/OopsMouse/arbitgo/models"
)

type Arbitrader struct {
	Exchange
	MarketAnalyzer
	MainAsset models.Asset
}

func NewArbitrader(ex Exchange, ma MarketAnalyzer, mainAsset models.Asset) *Arbitrader {
	return &Arbitrader{
		Exchange:       ex,
		MarketAnalyzer: ma,
		MainAsset:      mainAsset,
	}
}

func (arbit *Arbitrader) Run() {
	// Depthの変更通知登録
	ch := make(chan []*models.Depth)
	err := arbit.Exchange.OnUpdateDepthList(ch)
	if err != nil {
		// TODO: エラー処理
		panic(err)
	}

	for {
		// Main Assetの残高を取得
		mainAssetBalance, err := arbit.Exchange.GetBalance(arbit.MainAsset)

		if err != nil {
			fmt.Printf("get balance error. wait 5 minute, %v\n", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		for {
			depthList := <-ch
			bestOrderBook := arbit.MarketAnalyzer.GenerateBestOrderBook(
				depthList,
				mainAssetBalance.Free,
			)
			if bestOrderBook == nil {
				continue
			}

			err = arbit.Trade(bestOrderBook)
			if err != nil {
				arbit.Recovery()
			}
			break
		}
	}
}

func (arbit *Arbitrader) Trade(orderBook *models.OrderBook) error {
	for i, o := range orderBook.Orders {
		fmt.Printf("[%d] symbol => %s, side => %s, price => %f, qty => %f\n", i, o.Symbol, o.Side, o.Price, o.Qty)
		err := arbit.Exchange.SendOrder(o)
		if err != nil {
			fmt.Printf("when trading, unknown error occur, %v\n", err)
			return err
		}
	}
	return nil
}

func (arbit *Arbitrader) Recovery() {
	balances, err := arbit.Exchange.GetBalances()
	if err != nil {
		panic(err)
	}
	orderBook := arbit.MarketAnalyzer.GenerateRecoveryOrderBook(
		arbit.MainAsset,
		arbit.Exchange.GetSymbols(),
		balances,
	)
	err = arbit.Trade(orderBook)
	if err != nil {
		panic(err)
	}
}

// func (arbit *Arbitrader) PrintBalances() {
// 	balances, err := arbit.Exchange.GetBalances()
// 	if err != nil {
// 		return
// 	}

// 	fmt.Printf("balances:\n")
// 	for _, b := range balances {
// 		fmt.Printf("[%s] => %f\n", b.Asset, b.Total)
// 	}
// }
