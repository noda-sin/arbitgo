package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type Exchange interface {
	GetCharge() float64
	GetBalance(asset models.Asset) (*models.Balance, error)
	GetBalances() ([]*models.Balance, error)
	GetSymbols() []models.Symbol
	GetDepth(symbol models.Symbol) (*models.Depth, error)
	GetDepthOnUpdate() chan *models.Depth
	SendOrder(order *models.Order) error
	ConfirmOrder(order *models.Order) (float64, error)
	CancelOrder(order *models.Order) error
}
