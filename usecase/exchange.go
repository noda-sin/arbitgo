package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type Exchange interface {
	GetFee() float64
	GetBalances() ([]*models.Balance, error)
	GetQuotes() []string
	GetSymbols() []models.Symbol
	GetDepth(symbol models.Symbol) (*models.Depth, error)
	GetDepthOnUpdate() chan *models.Depth
	SendOrder(order *models.Order) error
	ConfirmOrder(order *models.Order) (float64, error)
	CancelOrder(order *models.Order) error
}
