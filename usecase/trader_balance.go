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

func (trader *Trader) BigAssets() []string {
	symbols := trader.Exchange.GetSymbols()
	bigAssets := []string{}
	for _, balance := range trader.balances {
		for _, symbol := range symbols {
			if symbol.BaseAsset == balance.Asset &&
				balance.Free > symbol.MinQty {
				bigAssets = append(bigAssets, balance.Asset)
				break
			}
		}
	}
	return bigAssets
}

func (trader *Trader) GetBalance(asset string) *models.Balance {
	for _, balance := range trader.balances {
		if balance.Asset == asset {
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

func (trader *Trader) PrintBalance(asset string) {
	trader.LoadBalances()

	log.Info("----------------- Balances -----------------")

	log.Info(asset, " : ", trader.GetBalance(asset).Total)

	log.Info("--------------------------------------------")
}

func (trader *Trader) PrintBalanceOfBigAssets() {
	trader.LoadBalances()

	bigAssets := trader.BigAssets()

	log.Info("----------------- Balances -----------------")

	for _, bigAsset := range bigAssets {
		log.Info(bigAsset, " : ", trader.GetBalance(bigAsset).Total)
	}

	log.Info("--------------------------------------------")
}
