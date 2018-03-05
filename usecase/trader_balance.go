package usecase

import (
	"github.com/OopsMouse/arbitgo/models"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) LoadBalances() {
	balances, err := trader.Exchange.GetBalances()
	if err != nil {
		return
	}
	trader.balances = balances
}

func (trader *Trader) GetBalance(asset models.Asset) *models.Balance {
	for _, balance := range trader.balances {
		if string(balance.Asset) == string(asset) {
			return balance
		}
	}
	return nil
}

func (trader *Trader) PrintBalances() {
	trader.LoadBalances()

	log.Info("----------------- Balances -----------------")

	for _, balance := range trader.balances {
		log.Info(balance.Asset, " : ", balance.Total)
	}

	log.Info("--------------------------------------------")
}

func (trader *Trader) PrintBalance(asset models.Asset) {
	trader.LoadBalances()

	log.Info("----------------- Balances -----------------")

	log.Info(asset, " : ", trader.GetBalance(asset).Total)

	log.Info("--------------------------------------------")
}
