package usecase

import (
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
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

func loginit() {
	format := &log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	}
	log.SetFormatter(format)
}

func (arbit *Arbitrader) Run() {
	loginit()
	log.Info("Starting Arbitrader ....")

	mainAssetBalance, err := arbit.Exchange.GetBalance(arbit.MainAsset)
	if err != nil {
		log.WithError(err).Error("Failed to get balance of main asset.")
		panic(err)
	}

	log.Info("----------------- params -----------------")
	log.Info(" Main asset         : ", arbit.MainAsset)
	log.Info(" Main asset balance : ", mainAssetBalance.Free)
	log.Info(" Exchange charge    : ", arbit.MarketAnalyzer.Charge)
	log.Info(" Threshold          : ", arbit.MarketAnalyzer.Threshold)
	log.Info("------------------------------------------")

	// Depthの変更通知登録
	ch := make(chan []*models.Depth)

	log.Info("Add event listener to receive updating depthes")

	err = arbit.Exchange.OnUpdateDepthList(ch)
	if err != nil {
		log.WithError(err).Error("Add event listener failed")
		panic(err)
	}

	for {
		log.Info("Get main asset balance")
		mainAssetBalance, err := arbit.Exchange.GetBalance(arbit.MainAsset)
		if err != nil {
			log.WithError(err).Error("Get main asset failed")
			log.Info("Sleeping 5 second")
			time.Sleep(5 * time.Second)
			continue
		}
		log.Info(arbit.MainAsset, " : ", mainAssetBalance.Free)

		for {
			depthList := <-ch
			orders := arbit.MarketAnalyzer.ArbitrageOrders(
				depthList,
				mainAssetBalance.Free,
			)
			if orders == nil {
				continue
			}

			log.Info("Found arbit orders")
			LogOrders(orders)

			log.Info("Starting trade ....")

			<-arbit.TradeOrder(orders)

			arbit.LogBalances()

			log.Info("Sleeping 5 second")
			time.Sleep(5 * time.Second)
			break
		}
	}
}

func (arbit *Arbitrader) TradeOrder(orders []models.Order) chan struct{} {
	done := make(chan struct{})
	currentOrders := orders
	currentOrder := currentOrders[0]

	go func() {
		defer close(done)

		if len(orders) == 0 {
			return
		}

		log.WithField("OrderID", currentOrder.ClientOrderID).Info("START - send order")
		LogOrder(currentOrder)

		err := arbit.Exchange.SendOrder(&currentOrder)
		log.WithField("OrderID", currentOrder.ClientOrderID).Info("END - send order")

		if err != nil {
			log.WithField("OrderID", currentOrder.ClientOrderID).WithError(err).Error("Send order failed")
			arbit.RecoveryOrder(currentOrder)
			return
		}

		executedTotalQty := 0.0
		waitingTotalQty := currentOrder.Qty
		childTrades := []chan struct{}{}

		for i := 0; i < 10; i++ {
			log.WithField("OrderID", currentOrder.ClientOrderID).Info("START - confirm order")
			log.Info("OrderID: ", currentOrder.ClientOrderID)
			var executedQty float64
			executedQty, err = arbit.Exchange.ConfirmOrder(&currentOrder)
			log.WithField("OrderID", currentOrder.ClientOrderID).Info("END - confirm order")
			if err != nil {
				log.WithField("OrderID", currentOrder.ClientOrderID).WithError(err).Error("Confirm order failed")
				continue
			}

			log.WithField("OrderID", currentOrder.ClientOrderID).Info("------------------------------------------")
			log.WithField("OrderID", currentOrder.ClientOrderID).Info("Executed total quantity : ", executedTotalQty, " --> ", executedTotalQty+executedQty)
			log.WithField("OrderID", currentOrder.ClientOrderID).Info("Waiting total quantity  : ", waitingTotalQty, " --> ", waitingTotalQty-executedQty)
			log.WithField("OrderID", currentOrder.ClientOrderID).Info("------------------------------------------")

			executedTotalQty += executedQty
			waitingTotalQty -= executedQty

			if waitingTotalQty <= 0 {
				log.WithField("OrderID", currentOrder.ClientOrderID).Info("Success order about entire quantity")
				break
			} else if executedTotalQty > 0 && executedTotalQty > currentOrder.Symbol.MinQty {
				// 別トレーディングとして注文
				log.WithField("OrderID", currentOrder.ClientOrderID).Info("Few quantity into order earlier succeeded")
				log.WithField("OrderID", currentOrder.ClientOrderID).Info("Create a child and trade ahead child trade")

				executedTotalQty = 0
				var childOrders []models.Order
				log.WithField("OrderID", currentOrder.ClientOrderID).Info("START - create child orders")

				currentOrders, childOrders = arbit.MarketAnalyzer.SplitOrders(orders, executedQty)
				currentOrder = currentOrders[0]

				log.WithField("OrderID", currentOrder.ClientOrderID).Info("# Parent Orders")
				LogOrders(currentOrders)
				log.WithField("OrderID", currentOrder.ClientOrderID).Info("# Child Orders")
				LogOrders(childOrders)

				log.WithField("OrderID", currentOrder.ClientOrderID).Info("END - create child orders")
				childTrade := arbit.TradeOrder(childOrders)
				childTrades = append(childTrades, childTrade)
			}

			log.WithField("OrderID", currentOrder.ClientOrderID).Info("Sleeping 10 second")
			time.Sleep(10 * time.Second)
		}

		defer func() {
			for _, c := range childTrades {
				<-c
			}
		}()

		if waitingTotalQty > 0 {
			log.WithField("OrderID", currentOrder.ClientOrderID).Warn("Order did not end within time limit")
			log.WithField("OrderID", currentOrder.ClientOrderID).Info("START - Cancel order")
			LogOrder(currentOrder)

			err := arbit.Exchange.CancelOrder(&currentOrder)
			log.WithField("OrderID", currentOrder.ClientOrderID).Info("END - Cancel order")

			if err != nil {
				log.WithField("OrderID", currentOrder.ClientOrderID).WithError(err).Error("Cancel order failed")
				log.WithField("OrderID", currentOrder.ClientOrderID).Error("Please confirm and manualy cancel order")
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

	log.Info("Current asset : ", currentAsset)

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
	LogOrders(orders)

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
		LogOrder(order)

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

func LogOrder(order models.Order) {
	log.Info("----------------- orders -------------------")
	log.Info(" OrderID  : ", order.ClientOrderID)
	log.Info(" Symbol   : ", order.Symbol.String())
	log.Info(" Side     : ", order.Side)
	log.Info(" Type     : ", order.OrderType)
	log.Info(" Price    : ", order.Price)
	log.Info(" Quantity : ", order.Qty)
	log.Info("--------------------------------------------")
}

func LogOrders(orders []models.Order) {
	for i, order := range orders {
		log.Info("----------------- orders ", i, " -----------------")
		log.Info(" OrderID  : ", order.ClientOrderID)
		log.Info(" Symbol   : ", order.Symbol.String())
		log.Info(" Side     : ", order.Side)
		log.Info(" Type     : ", order.OrderType)
		log.Info(" Price    : ", order.Price)
		log.Info(" Quantity : ", order.Qty)
		log.Info("--------------------------------------------")
	}
}

func (arbit *Arbitrader) LogBalances() {
	balances, err := arbit.Exchange.GetBalances()
	if err != nil {
		return
	}
	log.Info("----------------- Balances -----------------")

	for _, balance := range balances {
		log.Info(balance.Asset, " : ", balance.Total)
	}

	log.Info("--------------------------------------------")
}
