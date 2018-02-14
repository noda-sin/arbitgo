package infrastructure

import (
	"fmt"

	models "github.com/OopsMouse/arbitgo/models"
)

type ExchangeStub struct {
	Exchange
	Balances map[models.Asset]*models.Balance
}

func NewExchangeStub(apikey string, secret string, initialBalances map[models.Asset]*models.Balance) ExchangeStub {
	ex := NewExchange(apikey, secret)
	return ExchangeStub{
		Exchange: ex,
		Balances: initialBalances,
	}
}

func (ex ExchangeStub) GetBalance(asset models.Asset) (*models.Balance, error) {
	for k, b := range ex.Balances {
		if k == asset {
			return b, nil
		}
	}
	return nil, fmt.Errorf("Not found balance for %s", asset)
}

func (ex ExchangeStub) NewBalance(asset models.Asset) {
	balance := ex.Balances[asset]
	if balance == nil {
		ex.Balances[asset] = &models.Balance{
			Asset: asset,
			Free:  0.0,
			Total: 0.0,
		}
	}
}

func (ex ExchangeStub) AddBalance(asset models.Asset, qty float64) {
	ex.NewBalance(asset)
	balance := ex.Balances[asset]
	ex.Balances[asset] = &models.Balance{
		Asset: asset,
		Free:  balance.Free + qty,
		Total: balance.Total + qty,
	}
}

func (ex ExchangeStub) SubBalance(asset models.Asset, qty float64) {
	ex.NewBalance(asset)
	balance := ex.Balances[asset]
	ex.Balances[asset] = &models.Balance{
		Asset: asset,
		Free:  balance.Free - qty,
		Total: balance.Total - qty,
	}
}

func (ex ExchangeStub) GetBalances() ([]*models.Balance, error) {
	bs := []*models.Balance{}
	for _, v := range ex.Balances {
		bs = append(bs, v)
	}
	return bs, nil
}

func (ex ExchangeStub) GetSymbols() []models.Symbol {
	return ex.Exchange.GetSymbols()
}

func (ex ExchangeStub) GetDepthList() ([]*models.Depth, error) {
	return ex.Exchange.GetDepthList()
}

func (ex ExchangeStub) OnUpdateDepthList(recv chan []*models.Depth) error {
	return ex.Exchange.OnUpdateDepthList(recv)
}

func (ex ExchangeStub) SendOrder(order *models.Order) error {
	if order.Side == models.SideBuy {
		balance, err := ex.GetBalance(order.QuoteAsset)
		if err != nil {
			return err
		}
		if balance.Free < order.Qty*order.Price {
			return fmt.Errorf("Insufficent balance: %s, %f, %f\n", balance.Asset, balance.Free, order.Qty*order.Price)
		}
		ex.SubBalance(order.QuoteAsset, order.Qty*order.Price)
		ex.AddBalance(order.BaseAsset, order.Qty)
	} else {
		balance, err := ex.GetBalance(order.BaseAsset)
		if err != nil {
			return err
		}
		if balance.Free < order.Qty {
			return fmt.Errorf("Insufficent balance: %s, %f, %f\n", balance.Asset, balance.Free, order.Qty)
		}
		ex.AddBalance(order.QuoteAsset, order.Qty*order.Price)
		ex.SubBalance(order.BaseAsset, order.Qty)
	}
	return nil
}
