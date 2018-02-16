package infrastructure

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type Exchange interface {
	GetCharge() float64
	GetBalance(asset models.Asset) (*models.Balance, error)
	GetBalances() ([]*models.Balance, error)
	GetSymbols() []models.Symbol
	GetDepthList() ([]*models.Depth, error)
	OnUpdateDepthList(recv chan []*models.Depth) error
	SendOrder(order *models.Order) error
}
