package usecase

import (
	"fmt"

	models "github.com/OopsMouse/arbitgo/models"
)

func (arbit *Trader) Analyze(depthList []*models.Depth) {
	balance := arbit.GetBalance(arbit.MainAsset)
	tradeOrder := arbit.MarketAnalyzer.ArbitrageOrders(
		depthList,
		balance.Free,
	)

	if tradeOrder == nil {
		return
	}

	tradeOrder, err := arbit.ValidateOrders(tradeOrder, balance.Free)
	if err != nil {
		return
	}

	go arbit.StartTreding(tradeOrder)
}

func (arbit *Trader) ValidateOrders(tradeOrder *models.TradeOrder, currBalance float64) (*models.TradeOrder, error) {
	depthes := []*models.Depth{}

	for _, order := range tradeOrder.Orders {
		depth := arbit.Cache.Get(order.Symbol)
		if depth != nil {
			depthes = append(depthes, depth)
		}
	}

	if len(tradeOrder.Orders) != len(depthes) {
		return nil, fmt.Errorf("Failed to get new depthes")
	}

	// ok := arbit.MarketAnalyzer.ValidateOrders(tradeOrder.Orders, depthes)
	// if ok == true {
	// 	return tradeOrder, nil
	// }

	newTradeOrder, ok := arbit.MarketAnalyzer.ReplaceOrders(tradeOrder, depthes, currBalance)
	if ok == false {
		return nil, fmt.Errorf("Arbit orders destroyed")
	}
	return newTradeOrder, nil
}
