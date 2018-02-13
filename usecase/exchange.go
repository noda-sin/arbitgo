package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type Exchange interface {
	GetCharge() float64
	GetBalance(symbol string) (*models.Balance, error)
	GetBalances() ([]*models.Balance, error)
	GetMarket(startSymbol string) (*models.Market, error)
	OnUpdatedMarket(startSymbol string, recv chan *models.Market) error
	SendOrder(order *models.Order) error
}
