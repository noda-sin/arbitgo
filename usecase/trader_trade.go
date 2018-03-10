package usecase

import (
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) runTrader() chan *models.Sequence {
	seqch := make(chan *models.Sequence)

	go func() {
		for {
			seq := <-seqch
			if trader.isRunningPosition(seq.From) {
				continue
			}
			go func() {
				log.Info("Start trade")
				defer func() {
					log.Info("End trade")
				}()
				<-trader.doSequence(seq)
			}()
		}
	}()

	return seqch
}

func (trader *Trader) sendOrder(order models.Order) {
	log.Info("START - send order")
	log.Info("OrderID : ", order.ID)
	defer func() {
		log.Info("END - send order")
	}()

	util.LogOrder(order)

	err := trader.Exchange.SendOrder(&order)

	if err != nil {
		panic(err)
	}
}

type ConfirmStatus string

const (
	ALLOK  = ConfirmStatus("ALLOK")
	PARTOK = ConfirmStatus("PARTOK")
	ALLNG  = ConfirmStatus("ALLNG")
)

func (trader *Trader) confirmOrder(order models.Order) ConfirmStatus {
	log.Info("START - confirm order")
	log.Info("OrderID : ", order.ID)
	defer func() {
		log.Info("END - confirm order")
	}()
	for i := 0; i < 12; i++ {
		executed, err := trader.Exchange.ConfirmOrder(&order)
		if err != nil {
			panic(err)
		}

		if executed > 0 {
			trader.LoadBalances()
		}

		log.Infof("[%s] Executed : %f", order.ID, executed)

		if executed == order.Quantity { // 全部OK
			return ALLOK
		} else if executed > 0 { // 部分的にOK
			return PARTOK
		} else { // 全部だめ
			continue
		}

		time.Sleep(5 * time.Second)
	}
	return ALLNG
}

func checkQuanitiySize(order models.Order) bool {
	if order.Symbol.MaxQty < order.Quantity || order.Symbol.MinQty > order.Quantity {
		return false
	}

	if order.Quantity*order.Price < order.Symbol.MinNotional {
		return false
	}

	return true
}

func (trader *Trader) cancelOrder(order models.Order) {
	log.Info("START - cancel order")
	log.Info("OrderID : ", order.ID)
	defer func() {
		log.Info("END - cancel order")
	}()

	err := trader.Exchange.CancelOrder(&order)
	if err != nil {
		panic(err)
	}
}

func (trader *Trader) doSequence(seq *models.Sequence) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		trader.PrintSequence(seq)
		trader.addPosition(seq.From)

		child := []chan struct{}{}
		order := trader.newOrder(seq)

		if !checkQuanitiySize(order) {
			return
		}

		trader.sendOrder(order)
		status := trader.confirmOrder(order)

		log.Info("Order Result : ", status)

		switch status {
		case ALLNG:
		case PARTOK:
			trader.cancelOrder(order)
		}

		trader.delPosition(seq.From)

		switch status {
		case ALLNG:
			return
		}

		if seq.Next == nil {
			return
		}

		child = append(child, trader.doSequence(seq.Next))

		defer func() {
			for _, c := range child {
				<-c
			}
		}()
	}()

	return done
}

func (trader *Trader) newOrder(seq *models.Sequence) models.Order {
	trader.LoadBalances()
	trader.PrintBalanceOfBigAssets()
	balance := trader.GetBalance(seq.From).Free
	var quantity float64
	if seq.Side == models.SideBuy {
		quantity = util.Floor(balance/seq.Price, seq.Symbol.StepSize)
	} else {
		quantity = util.Floor(balance, seq.Symbol.StepSize)
	}

	order := models.Order{
		ID:        xid.New().String(),
		Symbol:    seq.Symbol,
		OrderType: models.TypeLimit,
		Price:     seq.Price,
		Side:      seq.Side,
		Quantity:  quantity,
		Sequence:  seq,
	}
	return order
}
