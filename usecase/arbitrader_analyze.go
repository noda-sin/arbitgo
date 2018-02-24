package usecase

import (
	"fmt"
	"sync"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

func (arbit *Arbitrader) Analyze(depthList []*models.Depth) {
	balance := arbit.GetBalance(arbit.MainAsset)
	orders := arbit.MarketAnalyzer.ArbitrageOrders(
		depthList,
		balance.Free,
	)

	if orders == nil {
		return
	}

	log.Info("Found arbit orders")
	util.LogOrders(orders)

	log.Info("Validate orders")
	orders, err := arbit.ValidateOrders(orders, balance.Free)
	if err != nil {
		log.Warn("Validate orders failed")
		return
	}
	go arbit.StartTreding(orders)
}

func (arbit *Arbitrader) ValidateOrders(orders []models.Order, currBalance float64) ([]models.Order, error) {
	depthes := []*models.Depth{}
	depch := make(chan *models.Depth)
	errch := make(chan error)

	defer close(depch)
	defer close(errch)

	wg := &sync.WaitGroup{}

	m := new(sync.Mutex)

	for _, order := range orders {
		wg.Add(1)
		go func(order models.Order) {
			defer m.Unlock()
			defer wg.Done()
			depth, _ := arbit.Exchange.GetDepth(order.Symbol)
			m.Lock()
			arbit.Cache.Set(depth)
			depthes = append(depthes, depth)
		}(order)
	}

	wg.Wait()

	ok := arbit.MarketAnalyzer.ValidateOrders(orders, depthes)
	if ok == true {
		return orders, nil
	}

	newOrders := arbit.MarketAnalyzer.ArbitrageOrders(depthes, currBalance)
	if newOrders == nil {
		return nil, fmt.Errorf("Arbit orders destroyed")
	}
	return newOrders, nil
}