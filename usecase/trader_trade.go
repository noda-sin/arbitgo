package usecase

import (
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) StartTreding(tradeOrder *models.TradeOrder) {
	trader.StatusLock.Lock()
	if trader.Status == TradeRunning {
		trader.StatusLock.Unlock()
		return
	}

	trader.Status = TradeRunning
	trader.StatusLock.Unlock()
	go func() {
		log.Info("Starting trade ....")
		log.Info("Score : ", tradeOrder.Score)
		trader.LoadBalances()
		log.Info(trader.MainAsset, " : ", trader.GetBalance(trader.MainAsset).Free)

		<-trader.TradeOrder(tradeOrder.Orders)

		trader.StatusLock.Lock()
		trader.Status = TradeWaiting
		trader.StatusLock.Unlock()

		trader.LoadBalances()
		log.Info(trader.MainAsset, " : ", trader.GetBalance(trader.MainAsset).Free)
	}()
}

func (trader *Trader) TradeOrder(orders []models.Order) chan struct{} {
	done := make(chan struct{})
	currentOrders := orders
	currentOrder := currentOrders[0]

	go func() {
		defer close(done)

		if len(orders) == 0 {
			return
		}

		log.Info("START - send order")
		util.LogOrder(currentOrder)

		err := trader.Exchange.SendOrder(&currentOrder)
		log.Info("END - send order")

		if err != nil {
			log.WithError(err).Error("Send order failed")
			trader.RecoveryOrder(currentOrder)
			return
		}

		executedTotalQty := 0.0
		waitingTotalQty := currentOrder.Qty
		childTrades := []chan struct{}{}

		for i := 0; i < 300; i++ {
			var executedQty float64
			executedQty, err = trader.Exchange.ConfirmOrder(&currentOrder)
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

				var childOrders []models.Order
				log.Info("START - create child orders")

				currentOrders, childOrders, err = trader.MarketAnalyzer.SplitOrders(orders, executedQty)
				if err == nil {
					currentOrder = currentOrders[0]

					log.WithField("ID", currentOrder.ClientOrderID).Info("# Parent Orders")
					util.LogOrders(currentOrders)
					log.WithField("ID", currentOrder.ClientOrderID).Info("# Child Orders")
					util.LogOrders(childOrders)

					log.Info("END - create child orders")
					childTrade := trader.TradeOrder(childOrders)
					childTrades = append(childTrades, childTrade)
				}
			} else {
				// 別ルート
			}
			time.Sleep(1 * time.Second)
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

			err := trader.Exchange.CancelOrder(&currentOrder)
			log.Info("END - Cancel order")

			// TODO: すでに約定していた場合の処理

			if err != nil {
				log.WithError(err).Error("Cancel order failed")
				log.Error("Please confirm and manualy cancel order")
				panic(err)
			}

			trader.RecoveryOrder(models.Order{
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

		<-trader.TradeOrder(currentOrders[1:])
	}()

	return done
}

func (trader *Trader) RecoveryOrder(order models.Order) {
	var currentAsset models.Asset
	if order.Side == models.SideBuy {
		currentAsset = order.Symbol.QuoteAsset
	} else {
		currentAsset = order.Symbol.BaseAsset
	}

	log.Info("Current asset : ", currentAsset)

	if currentAsset == trader.MainAsset {
		log.Info("Current asset is same main asset")
		log.Info("Recovery is unnecessary")

		// nothing to do
		return
	}

	log.Info("Finding orders to recovery")

	orders, err := trader.MarketAnalyzer.ForceChangeOrders(
		trader.Exchange.GetSymbols(),
		currentAsset,
		trader.MainAsset,
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

		currentBalance, _ := trader.Exchange.GetBalance(currentAsset)

		log.Info(currentAsset, " : ", currentBalance.Free)

		order.Qty = util.Floor(currentBalance.Free, order.Symbol.StepSize)

		log.Info("START - send recovery order")
		util.LogOrder(order)

		err = trader.Exchange.SendOrder(&order)

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