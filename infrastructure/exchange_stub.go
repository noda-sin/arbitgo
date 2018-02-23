package infrastructure

import (
	"fmt"
	"math/rand"
	"time"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
)

type ExchangeStub struct {
	Exchange
	Balances map[models.Asset]*models.Balance
}

func NewExchangeStub(ex Exchange, initialBalances map[models.Asset]*models.Balance) ExchangeStub {
	return ExchangeStub{
		Exchange: ex,
		Balances: initialBalances,
	}
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

func (ex ExchangeStub) GetBalance(asset models.Asset) (*models.Balance, error) {
	for k, b := range ex.Balances {
		if string(k) == string(asset) {
			return b, nil
		}
	}
	return nil, fmt.Errorf("Not found balance for %s", string(asset))
}

func (ex ExchangeStub) GetSymbols() []models.Symbol {
	return ex.Exchange.GetSymbols()
}

func (ex ExchangeStub) SendOrder(order *models.Order) error {
	return nil
}

func (ex ExchangeStub) ConfirmOrder(order *models.Order) (float64, error) {
	rand.Seed(time.Now().UnixNano())
	randInt := rand.Intn(15)
	if randInt > 7 {
		return 0, nil
	} else if randInt <= 7 && randInt < 5 {
		ex.CommitOrder(order, order.Qty)
		return order.Qty, nil
	} else {
		commitQty := util.Floor(order.Qty*float64(randInt)/15, order.Symbol.StepSize)
		ex.CommitOrder(order, commitQty)
		return commitQty, nil
	}
}

func (ex ExchangeStub) CommitOrder(order *models.Order, qty float64) error {
	if order.Side == models.SideBuy {
		balance, err := ex.GetBalance(order.Symbol.QuoteAsset)
		if err != nil {
			return err
		}
		var price float64
		if order.OrderType == models.TypeLimit {
			if balance.Free < order.Qty*order.Price {
				return fmt.Errorf("Insufficent balance: %s, %f < %f\n", balance.Asset, balance.Free, order.Qty*order.Price)
			}
			price = order.Price
		} else {
			price = order.Symbol.MinPrice
		}
		ex.SubBalance(order.Symbol.QuoteAsset, qty*price)
		ex.AddBalance(order.Symbol.BaseAsset, qty)
	} else {
		balance, err := ex.GetBalance(order.Symbol.BaseAsset)
		if err != nil {
			return err
		}
		var price float64
		if order.OrderType == models.TypeLimit {
			if balance.Free < order.Qty {
				return fmt.Errorf("Insufficent balance: %s, %f < %f\n", balance.Asset, balance.Free, order.Qty)
			}
			price = order.Price
		} else {
			price = order.Symbol.MinPrice
		}
		ex.AddBalance(order.Symbol.QuoteAsset, qty*price)
		ex.SubBalance(order.Symbol.BaseAsset, qty)
	}
	return nil
}

func (ex ExchangeStub) CancelOrder(order *models.Order) error {
	return nil
}
