package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type MarketRepository interface {
	GetMarket() (*models.Market, error)
	UpdatedMarket(recv chan *models.Market) error
}
