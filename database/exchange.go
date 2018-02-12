package database

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type Exchange interface {
	GetTicker(symbol string) (*models.Ticker, error)
	GetTickers() ([]*models.Ticker, error)
	UpdatedTickers(recv chan []*models.Ticker) error
}
