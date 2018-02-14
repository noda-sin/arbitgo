package usecase

import (
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	log "github.com/sirupsen/logrus"
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
	log.WithField("main asset", arbit.MainAsset).Info("start arbitgo")

	// Depthの変更通知登録
	ch := make(chan []*models.Depth)
	err := arbit.Exchange.OnUpdateDepthList(ch)
	if err != nil {
		log.WithError(err).Error("failed to add listener on update depth")
		panic(err)
	}

	for {
		// Main Assetの残高を取得
		mainAssetBalance, err := arbit.Exchange.GetBalance(arbit.MainAsset)

		if err != nil {
			log.WithError(err).Error("failed to get balances")
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

			log.WithField("score", bestOrderBook.Score).Info("found best order book")

			err = arbit.Trade(bestOrderBook)
			if err != nil {
				log.WithError(err).Error("failed to arbit trade")
				log.Error("begin to recovery balances")

				arbit.Recovery()
			}

			arbit.LogBalances()
			time.Sleep(5 * time.Second)
			break
		}
	}
}

func (arbit *Arbitrader) Trade(orderBook *models.OrderBook) error {
	for i, o := range orderBook.Orders {
		log.WithFields(log.Fields{
			"number": i,
			"symbol": string(o.Symbol),
			"side":   o.Side,
			"type":   o.OrderType,
			"price":  o.Price,
			"qty":    o.Qty,
		}).Info("challenge to order")
		err := arbit.Exchange.SendOrder(o)
		if err != nil {
			return err
		}
	}
	return nil
}

func (arbit *Arbitrader) Recovery() {
	balances, err := arbit.Exchange.GetBalances()
	if err != nil {
		log.WithError(err).Error("failed to recovery")
		log.Error("please confirm and manualy recovery")

		panic(err)
	}

	orderBook := arbit.MarketAnalyzer.GenerateRecoveryOrderBook(
		arbit.MainAsset,
		arbit.Exchange.GetSymbols(),
		balances,
	)

	err = arbit.Trade(orderBook)
	if err != nil {
		log.WithError(err).Error("failed to recovery")
		log.Error("please confirm and manualy recovery")

		panic(err)
	}
}

func (arbit *Arbitrader) LogBalances() {
	balances, err := arbit.Exchange.GetBalances()
	if err != nil {
		return
	}
	for _, balance := range balances {
		log.WithField(string(balance.Asset), balance.Total).Info("report balance")
	}
}
