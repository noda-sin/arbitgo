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

			log.Info("Starting trade ....")

			err = arbit.TradeOrder(orders)
			if err != nil {
				log.WithError(err).Error("Trade failed")
			}

			arbit.LogBalances()

			log.Info("Sleeping 5 second")
			time.Sleep(5 * time.Second)
			break
		}
	}
}

func (arbit *Arbitrader) TradeOrder(orders []*models.Order) error {
	order := orders[0]
	log.Info("START - send order")
	log.Info("----------------- order --------------------")
	log.Info(" OrderID  : ", order.ClientOrderID)
	log.Info(" Symbol   : ", order.Symbol.String())
	log.Info(" Side     : ", order.Side)
	log.Info(" Type     : ", order.OrderType)
	log.Info(" Price    : ", order.Price)
	log.Info(" Quantity : ", order.Qty)
	log.Info("--------------------------------------------")

	err := arbit.Exchange.SendOrder(order)
	log.Info("END - send order")

	if err != nil {
		log.WithError(err).Error("Send order failed")
		return arbit.RecoveryOrder(order)
	}

	executedTotalQty := 0.0
	waitingTotalQty := order.Qty

	for i := 0; i < 10; i++ {
		log.Info("START - confirm order")
		log.Info("OrderID: ", order.ClientOrderID)
		var executedQty float64
		executedQty, err = arbit.Exchange.ConfirmOrder(order)
		log.Info("END - confirm order")
		if err != nil {
			log.WithError(err).Error("Confirm order failed")
			continue
		}

		log.Info("------------------------------------------")
		log.Info("Executed total quantity : ", executedTotalQty, " --> ", executedTotalQty+executedQty)
		log.Info("Waiting total quantity  : ", waitingTotalQty, " --> ", waitingTotalQty-executedQty)
		log.Info("------------------------------------------")

		executedTotalQty += executedQty
		waitingTotalQty -= executedQty

		if waitingTotalQty <= 0 {
			log.Info("Success order about entire quantity")
			break
		} else if executedTotalQty > 0 && executedTotalQty > order.Symbol.MinQty {
			// executedTotalQty = 0
			// 別トレーディングとして注文
		}

		log.Info("Sleeping 10 second")
		time.Sleep(10 * time.Second)
	}

	if waitingTotalQty > 0 {
		log.Warn("Order did not end within time limit")
		log.Info("START - Cancel order")
		log.Info("----------------- order --------------------")
		log.Info(" OrderID  : ", order.ClientOrderID)
		log.Info(" Symbol   : ", order.Symbol.String())
		log.Info(" Side     : ", order.Side)
		log.Info(" Type     : ", order.OrderType)
		log.Info(" Price    : ", order.Price)
		log.Info(" Quantity : ", order.Qty)
		log.Info("--------------------------------------------")
		err := arbit.Exchange.CancelOrder(order)
		log.Info("END - Cancel order")

		if err != nil {
			log.WithError(err).Error("Cancel order failed")
			log.Error("Please confirm and manualy cancel order")
			panic(err)
		}

		return arbit.RecoveryOrder(&models.Order{
			Symbol:    order.Symbol,
			Side:      order.Side,
			OrderType: order.OrderType,
			Price:     order.Price,
			Qty:       waitingTotalQty,
		})
	}

	if len(orders) == 1 { // FIN
		return nil
	}

	return arbit.TradeOrder(orders[1:])
}

func (arbit *Arbitrader) RecoveryOrder(order *models.Order) error {
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
		return nil
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
	for i, order := range orders {
		log.Info("----------------- orders ", i, " -----------------")
		log.Info(" OrderID  : ", order.ClientOrderID)
		log.Info(" Symbol   : ", order.Symbol.String())
		log.Info(" Side     : ", order.Side)
		log.Info(" Type     : ", order.OrderType)
		log.Info(" Quantity : ", order.Qty)
		log.Info("--------------------------------------------")
	}

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
		log.Info("----------------- order --------------------")
		log.Info(" OrderID  : ", order.ClientOrderID)
		log.Info(" Symbol   : ", order.Symbol.String())
		log.Info(" Side     : ", order.Side)
		log.Info(" Type     : ", order.OrderType)
		log.Info(" Quantity : ", order.Qty)
		log.Info("--------------------------------------------")

		err = arbit.Exchange.SendOrder(order)

		log.Info("END - send recovery order")

		if err != nil {
			log.WithError(err).Error("failed to recovery")
			log.Error("please confirm and manualy recovery")

			panic(err)
		}
	}
	return nil
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
