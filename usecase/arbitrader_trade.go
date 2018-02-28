package usecase

import (
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

func (arbit *Arbitrader) StartTreding(orders []models.Order) {
	arbit.StatusLock.Lock()
	if arbit.Status == TradeRunning {
		arbit.StatusLock.Unlock()
		return
	}

	arbit.Status = TradeRunning
	arbit.StatusLock.Unlock()
	go func() {
		log.Info("Starting trade ....")
		arbit.LoadBalances()
		log.Info(arbit.MainAsset, " : ", arbit.GetBalance(arbit.MainAsset))

		<-arbit.TradeOrder(orders)

		arbit.StatusLock.Lock()
		arbit.Status = TradeWaiting
		arbit.StatusLock.Unlock()

		arbit.LoadBalances()
		log.Info(arbit.MainAsset, " : ", arbit.GetBalance(arbit.MainAsset))
	}()
}

func (arbit *Arbitrader) TradeOrder(orders []models.Order) chan struct{} {
	done := make(chan struct{})
	currentOrders := orders
	currentOrder := currentOrders[0]

	// var currentAsset models.Asset
	// if currentOrder.Side == models.SideBuy {
	// 	currentAsset = currentOrder.Symbol.QuoteAsset
	// } else {
	// 	currentAsset = currentOrder.Symbol.BaseAsset
	// }

	// arbit.LoadBalances()
	// currentBalance := arbit.GetBalance(currentAsset)
	// currentOrder.OrderType = models.TypeMarket
	// currentOrder.Qty = util.Floor(currentBalance.Free, currentOrder.Symbol.StepSize)

	go func() {
		defer close(done)

		if len(orders) == 0 {
			return
		}

		log.Info("START - send order")
		util.LogOrder(currentOrder)

		err := arbit.Exchange.SendOrder(&currentOrder)
		log.Info("END - send order")

		if err != nil {
			log.WithError(err).Error("Send order failed")
			arbit.RecoveryOrder(currentOrder)
			return
		}

		executedTotalQty := 0.0
		waitingTotalQty := currentOrder.Qty
		childTrades := []chan struct{}{}

		for i := 0; i < 1800; i++ {
			var executedQty float64
			executedQty, err = arbit.Exchange.ConfirmOrder(&currentOrder)
			if err != nil {
				log.WithError(err).Error("Confirm order failed")
				continue
			}

			if executedQty > 0 {
				log.Info("------------------------------------------")
				log.WithField("ID", currentOrder.ClientOrderID).Info("Executed total quantity : ", executedTotalQty, " --> ", executedTotalQty+executedQty)
				log.WithField("ID", currentOrder.ClientOrderID).Info("Waiting total quantity  : ", waitingTotalQty, " --> ", waitingTotalQty-executedQty)
				log.Info("------------------------------------------")
			}

			executedTotalQty += executedQty
			waitingTotalQty -= executedQty

			if waitingTotalQty <= 0 {
				log.WithField("ID", currentOrder.ClientOrderID).Info("Success order about entire quantity")
				break
			} else if executedTotalQty > 0 && executedTotalQty > currentOrder.Symbol.MinQty && len(currentOrders) > 1 {
				// 別トレーディングとして注文
				log.WithField("ID", currentOrder.ClientOrderID).Info("Create a child and trade ahead child trade")

				executedTotalQty = 0
				var childOrders []models.Order
				log.Info("START - create child orders")

				currentOrders, childOrders = arbit.MarketAnalyzer.SplitOrders(orders, executedQty)
				currentOrder = currentOrders[0]

				log.WithField("ID", currentOrder.ClientOrderID).Info("# Parent Orders")
				util.LogOrders(currentOrders)
				log.WithField("ID", currentOrder.ClientOrderID).Info("# Child Orders")
				util.LogOrders(childOrders)

				log.Info("END - create child orders")
				childTrade := arbit.TradeOrder(childOrders)
				childTrades = append(childTrades, childTrade)
			}
			time.Sleep(10 * time.Second)
		}

		defer func() {
			for _, c := range childTrades {
				<-c
			}
		}()

		if waitingTotalQty > 0 {
			log.WithField("ID", currentOrder.ClientOrderID).Warn("Order did not end within time limit")
			log.Info("START - Cancel order")
			util.LogOrder(currentOrder)

			err := arbit.Exchange.CancelOrder(&currentOrder)
			log.Info("END - Cancel order")

			// TODO: すでに約定していた場合の処理

			if err != nil {
				log.WithError(err).Error("Cancel order failed")
				log.Error("Please confirm and manualy cancel order")
				panic(err)
			}

			arbit.RecoveryOrder(models.Order{
				Symbol:    currentOrder.Symbol,
				Side:      currentOrder.Side,
				OrderType: currentOrder.OrderType,
				Price:     currentOrder.Price,
				Qty:       waitingTotalQty,
			})
			return
		}

		if len(currentOrders) == 1 { // FIN
			return
		}

		<-arbit.TradeOrder(currentOrders[1:])
	}()

	return done
}

func (arbit *Arbitrader) RecoveryOrder(order models.Order) {
	var currentAsset models.Asset
	if order.Side == models.SideBuy {
		currentAsset = order.Symbol.QuoteAsset
	} else {
		currentAsset = order.Symbol.BaseAsset
	}

	log.WithField("ID", order.ClientOrderID).Info("Current asset : ", currentAsset)

	if currentAsset == arbit.MainAsset {
		log.Info("Current asset is same main asset")
		log.Info("Recovery is unnecessary")

		// nothing to do
		return
	}

	log.Info("Finding orders to recovery")

	orders, err := arbit.MarketAnalyzer.ForceChangeOrders(
		arbit.Exchange.GetSymbols(),
		currentAsset,
		arbit.MainAsset,
	)

	if err != nil {
		log.WithError(err).Error("Find orders to recovery failed")
		log.Error("Please confirm and manualy recovery !!!")

		panic(err)
	}

	log.Info("Found orders to recovery")
	util.LogOrders(orders)

	log.Info("Starting recovery ....")

	for _, order := range orders {
		var currentAsset models.Asset
		if order.Side == models.SideBuy {
			currentAsset = order.Symbol.QuoteAsset
		} else {
			currentAsset = order.Symbol.BaseAsset
		}

		log.Info("Check balance of ", currentAsset)

		currentBalance, _ := arbit.Exchange.GetBalance(currentAsset)

		log.Info(currentAsset, " : ", currentBalance.Free)

		order.Qty = util.Floor(currentBalance.Free, order.Symbol.StepSize)

		log.Info("START - send recovery order")
		util.LogOrder(order)

		err = arbit.Exchange.SendOrder(&order)

		log.Info("END - send recovery order")

		if err != nil {
			log.WithError(err).Error("failed to recovery")
			log.Error("please confirm and manualy recovery")

			panic(err)
		}
	}

	log.Info("Success to recovery")

	return
}
