package usecase

import (
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) runTrader() {
	tradeChan := make(chan *models.Sequence)
	trader.tradeChan = &tradeChan

	trading := false
	for {
		seq := <-*trader.tradeChan
		if trading {
			continue
		}
		trading = true
		go func() {
			defer func() {
				trading = false
			}()

			log.Info("Start trade")
			trader.PrintBalance(trader.MainAsset)

			<-trader.doSequence(seq)

			trader.PrintBalance(trader.MainAsset)
			log.Info("End trade")
		}()
	}
}

func (trader *Trader) sendOrder(order models.Order) {
	log.Info("START - send order")
	util.LogOrder(order)

	err := trader.Exchange.SendOrder(&order)
	log.Info("END - send order")

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
	for i := 0; i < 12; i++ {
		executed, err := trader.Exchange.ConfirmOrder(&order)
		if err != nil {
			panic(err)
		}

		if executed == order.Quantity { // 全部OK
			log.Info("Executed : ", executed)
			trader.LoadBalances()
			return ALLOK
		} else if executed > 0 { // 部分的にOK
			log.Info("Executed : ", executed)
			trader.LoadBalances()
			return PARTOK
		} else { // 全部だめ
			log.Info("Executed : ", executed)
			continue
		}

		time.Sleep(5 * time.Second)
	}
	return ALLNG
}

func (trader *Trader) cancelOrder(order models.Order) {
	err := trader.Exchange.CancelOrder(&order)
	if err != nil {
		panic(err)
	}
}

func (trader *Trader) doSequence(seq *models.Sequence) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		child := []chan struct{}{}
		order := trader.newOrder(seq)
		trader.sendOrder(order)
		status := trader.confirmOrder(order)

		switch status {
		case ALLNG:
			trader.cancelOrder(order)
			child = append(child, trader.doRecovery(seq))
			return
		case PARTOK:
			trader.cancelOrder(order)
			child = append(child, trader.doRecovery(seq))
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

func (trader *Trader) doRecovery(seq *models.Sequence) chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		// リカバリする必要なし
		if seq.From == trader.MainAsset {
			return
		}

		for {
			// MainAssetまでのシーケンスを探索
			newSeq := trader.bestOfSequence(seq.From, trader.MainAsset, trader.relationalDepthes(seq.Symbol), seq.Target)
			if newSeq != nil {
				<-trader.doSequence(newSeq)
				return
			}
			time.Sleep(5 * time.Second)
		}
	}()

	return done
}

func (trader *Trader) newOrder(seq *models.Sequence) models.Order {
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
	}
	return order
}
