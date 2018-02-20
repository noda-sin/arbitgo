package usecase

import (
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	"github.com/pkg/errors"
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
			orders := arbit.MarketAnalyzer.ArbitrageOrders(
				depthList,
				mainAssetBalance.Free,
			)
			if orders == nil {
				continue
			}

			err = arbit.TradeOrder(orders, 60)
			if err != nil {
				log.WithError(err).Error("failed trade")
			}

			arbit.LogBalances()
			time.Sleep(5 * time.Second)
			break
		}
	}
}

func (arbit *Arbitrader) TradeOrder(orders []*models.Order, confirmRetry int) error {
	o := orders[0]
	log.WithFields(log.Fields{
		"symbol": o.Symbol.String(),
		"side":   o.Side,
		"type":   o.OrderType,
		"price":  o.Price,
		"qty":    o.Qty,
	}).Info("challenge to order")
	err := arbit.Exchange.SendOrder(o)
	if err != nil {
		arbit.RecoveryOrder(o)
		return err
	}
	for i := 0; i < confirmRetry; i++ {
		executedQty, err := arbit.Exchange.ConfirmOrder(o)
		if err != nil {
			arbit.RecoveryOrder(o)
			return err
		}
		log.Info("Executed Qty: ", executedQty)
		if executedQty > 0 &&
			o.Qty == executedQty {
			if len(orders) == 1 { // FIN
				return nil
			}
			return arbit.TradeOrder(orders[1:], confirmRetry)
		}
	}
	arbit.RecoveryOrder(o)
	return errors.Errorf("failed order")
}

func (arbit *Arbitrader) RecoveryOrder(order *models.Order) {
	var currentAsset models.Asset
	if order.Side == models.SideBuy {
		currentAsset = order.QuoteAsset
	} else {
		currentAsset = order.BaseAsset
	}

	orders, err := arbit.MarketAnalyzer.ForceChangeOrders(
		arbit.Exchange.GetSymbols(),
		currentAsset,
		arbit.MainAsset,
	)

	if err != nil {
		log.WithError(err).Error("failed to recovery")
		log.Error("please confirm and manualy recovery")

		panic(err)
	}

	for _, order := range orders {
		var currentAsset models.Asset
		if order.Side == models.SideBuy {
			currentAsset = order.QuoteAsset
		} else {
			currentAsset = order.BaseAsset
		}
		currentBalance, err := arbit.Exchange.GetBalance(currentAsset)
		if err != nil {
			log.WithError(err).Error("failed to recovery")
			log.Error("please confirm and manualy recovery")

			panic(err)
		}
		order.Qty = util.Floor(currentBalance.Free, order.Symbol.StepSize)
		err = arbit.Exchange.SendOrder(order)
		if err != nil {
			log.WithError(err).Error("failed to recovery")
			log.Error("please confirm and manualy recovery")

			panic(err)
		}
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
