package infrastructure

import (
	"fmt"

	models "github.com/OopsMouse/arbitgo/models"
	log "github.com/sirupsen/logrus"
)

type executingOrder struct {
	uncommit float64
	order    *models.Order
}

type ExchangeStub struct {
	Exchange
	Balances        map[models.Asset]*models.Balance
	ExecutingOrders map[string]*executingOrder
}

func NewExchangeStub(ex Exchange, initialBalances map[models.Asset]*models.Balance) ExchangeStub {
	return ExchangeStub{
		Exchange:        ex,
		Balances:        initialBalances,
		ExecutingOrders: map[string]*executingOrder{},
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
	ex.ExecutingOrders[order.ID] = &executingOrder{
		uncommit: order.Quantity,
		order:    order,
	}
	return nil
}

func (ex ExchangeStub) ConfirmOrder(order *models.Order) (float64, error) {
	executingOrder := ex.ExecutingOrders[order.ID]

	depth, err := ex.Exchange.GetDepth(order.Symbol)
	if err != nil {
		return 0, err
	}

	if order.OrderType == models.TypeMarket {
		err := ex.CommitOrder(order, depth, order.Quantity)
		if err != nil {
			return 0, err
		}
		return order.Quantity, nil
	}

	var commitQty = 0.0
	if order.Side == models.SideBuy {
		log.Debugf("Symbol : %s, Price : %f, Quantity : %f", order.Symbol, depth.AskPrice, depth.AskQty)
		if order.Price >= depth.AskPrice {
			if executingOrder.uncommit <= depth.AskQty {
				commitQty = executingOrder.uncommit
			} else {
				commitQty = executingOrder.uncommit - depth.AskQty
			}
		} else {
			commitQty = 0
		}
	} else {
		log.Debugf("Symbol : %s, Price : %f, Quantity : %f", order.Symbol, depth.BidPrice, depth.BidQty)
		if order.Price <= depth.BidPrice {
			if executingOrder.uncommit <= depth.BidQty {
				commitQty = executingOrder.uncommit
			} else {
				commitQty = executingOrder.uncommit - depth.BidQty
			}
		} else {
			commitQty = 0
		}
	}

	executingOrder.uncommit -= commitQty

	err = ex.CommitOrder(order, depth, commitQty)
	if err != nil {
		return 0, err
	}

	if executingOrder.uncommit <= 0 {
		delete(ex.ExecutingOrders, order.ID)
	}

	return executingOrder.order.Quantity - executingOrder.uncommit, nil
}

func (ex ExchangeStub) CommitOrder(order *models.Order, depth *models.Depth, qty float64) error {

	if order.Side == models.SideBuy {
		balance, err := ex.GetBalance(order.Symbol.QuoteAsset)
		if err != nil {
			return err
		}
		var price float64
		if order.OrderType == models.TypeLimit {
			if balance.Free < qty*order.Price {
				return fmt.Errorf("Insufficent balance: %s, %f < %f", balance.Asset, balance.Free, qty*order.Price)
			}
			price = order.Price
		} else {
			price = depth.AskPrice
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
			if balance.Free < qty {
				return fmt.Errorf("Insufficent balance: %s, %f < %f", balance.Asset, balance.Free, qty)
			}
			price = order.Price
		} else {
			price = depth.BidPrice
		}

		ex.AddBalance(order.Symbol.QuoteAsset, qty*price)
		ex.SubBalance(order.Symbol.BaseAsset, qty)
	}
	return nil
}

func (ex ExchangeStub) CancelOrder(order *models.Order) error {
	delete(ex.ExecutingOrders, order.ID)

	return nil
}
