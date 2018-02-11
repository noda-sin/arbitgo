package database

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type Exchange interface {
	GetTickers() ([]*models.Ticker, error)
	UpdatedTickers(recv chan []*models.Ticker) error
}
