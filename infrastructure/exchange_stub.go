package infrastructure

import (
	"fmt"

	"github.com/OopsMouse/arbitgo/common"
	models "github.com/OopsMouse/arbitgo/models"
)

type ExchangeStub struct {
	Exchange
	Balances map[string]*models.Balance
}

func NewExchangeStub(apikey string, secret string) ExchangeStub {
	ex := NewExchange(apikey, secret)
	balances := map[string]*models.Balance{}
	balances[common.BTC] = &models.Balance{
		Symbol: common.BTC,
		Free:   1.0,
		Total:  1.0,
	}
	return ExchangeStub{
		Exchange: ex,
		Balances: balances,
	}
}

func (ex ExchangeStub) GetBalance(symbol string) (*models.Balance, error) {
	for k, b := range ex.Balances {
		if k == symbol {
			return b, nil
		}
	}
	return nil, fmt.Errorf("Not found balance for %s", symbol)
}

func (ex ExchangeStub) NewBalance(symbol string) {
	balance := ex.Balances[symbol]
	if balance == nil {
		ex.Balances[symbol] = &models.Balance{
			Symbol: symbol,
			Free:   0.0,
			Total:  0.0,
		}
	}
}

func (ex ExchangeStub) AddBalanceQty(symbol string, qty float64) {
	ex.NewBalance(symbol)
	balance := ex.Balances[symbol]
	ex.Balances[symbol] = &models.Balance{
		Symbol: symbol,
		Free:   balance.Free + qty,
		Total:  balance.Total + qty,
	}
}

func (ex ExchangeStub) SubBalanceQty(symbol string, qty float64) {
	ex.NewBalance(symbol)
	balance := ex.Balances[symbol]
	ex.Balances[symbol] = &models.Balance{
		Symbol: symbol,
		Free:   balance.Free - qty,
		Total:  balance.Total - qty,
	}
}

func (ex ExchangeStub) GetBalances() ([]*models.Balance, error) {
	bs := []*models.Balance{}
	for _, v := range ex.Balances {
		bs = append(bs, v)
	}
	return bs, nil
}

func (ex ExchangeStub) GetMarket(startSymbol string) (*models.Market, error) {
	return ex.Exchange.GetMarket(startSymbol)
}

func (ex ExchangeStub) OnUpdatedMarket(startSymbol string, recv chan *models.Market) error {
	return ex.Exchange.OnUpdatedMarket(startSymbol, recv)
}

func (ex ExchangeStub) SendOrder(order *models.Order) error {
	if order.Side == common.Buy {
		balance, err := ex.GetBalance(order.QuoteAsset)
		if err != nil {
			return err
		}
		if balance.Free < order.BaseQty*order.Price {
			return fmt.Errorf("Insufficent balance: %s, %f, %f", balance.Symbol, balance.Free, order.BaseQty*order.Price)
		}
		ex.SubBalanceQty(order.QuoteAsset, order.BaseQty*order.Price)
		ex.AddBalanceQty(order.BaseAsset, order.BaseQty)
	} else {
		balance, err := ex.GetBalance(order.BaseAsset)
		if err != nil {
			return err
		}
		if balance.Free < order.BaseQty {
			return fmt.Errorf("Insufficent balance: %s, %f, %f", balance.Symbol, balance.Free, order.BaseQty)
		}
		ex.AddBalanceQty(order.QuoteAsset, order.BaseQty*order.Price)
		ex.SubBalanceQty(order.BaseAsset, order.BaseQty)
	}
	return nil
}
