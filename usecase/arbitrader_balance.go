package usecase

import (
	"github.com/OopsMouse/arbitgo/models"
	log "github.com/sirupsen/logrus"
)

func (arbit *Arbitrader) LoadBalances() {
	balances, err := arbit.Exchange.GetBalances()
	if err != nil {
		return
	}
	arbit.Balances = balances
}

func (arbit *Arbitrader) GetBalance(asset models.Asset) *models.Balance {
	for _, balance := range arbit.Balances {
		if string(balance.Asset) == string(asset) {
			return balance
		}
	}
	return nil
}

func (arbit *Arbitrader) LogBalances() {
	log.Info("----------------- Balances -----------------")

	for _, balance := range arbit.Balances {
		log.Info(balance.Asset, " : ", balance.Total)
	}

	log.Info("--------------------------------------------")
}
