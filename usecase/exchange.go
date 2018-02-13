package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type Exchange interface {
	GetMarket() (*models.Market, error)
	OnUpdatedMarket(recv chan *models.Market) error
	SendOrder(order *models.Order) error
}
