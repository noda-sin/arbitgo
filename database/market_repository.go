package database

import (
	models "github.com/OopsMouse/arbitgo/models"
)

type MarketRepository struct {
	Exchange
}

func NewMarketRepository(ex Exchange) MarketRepository {
	return MarketRepository{
		Exchange: ex,
	}
}

func (tp MarketRepository) GetMarket() (*models.Market, error) {
	tks, err := tp.Exchange.GetTickers()
	if err != nil {
		return nil, err
	}
	return models.NewMarket(tks), nil
}

func (tp MarketRepository) UpdatedMarket(recv chan *models.Market) error {
	ch := make(chan []*models.Ticker)
	err := tp.Exchange.UpdatedTickers(ch)
	if err != nil {
		return err
	}
	go func() {
		for {
			tks := <-ch
			recv <- models.NewMarket(tks)
		}
	}()
	return nil
}
