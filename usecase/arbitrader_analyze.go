package usecase

import (
	"fmt"
	"sync"

	models "github.com/OopsMouse/arbitgo/models"
)

func (arbit *Arbitrader) Analyze(depthList []*models.Depth) {
	balance := arbit.GetBalance(arbit.MainAsset)
	tradeOrder := arbit.MarketAnalyzer.ArbitrageOrders(
		depthList,
		balance.Free,
	)

	if tradeOrder == nil {
		return
	}

	// log.Info("Found arbit orders")
	// util.LogOrders(orders)

	// orders, err := arbit.ValidateOrders(orders, balance.Free)
	// if err != nil {
	// 	return
	// }

	go arbit.StartTreding(tradeOrder)
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
	return nil, fmt.Errorf("Arbit orders destroyed")
}
