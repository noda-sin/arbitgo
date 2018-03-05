package models

import (
	"time"
)

type Asset string

func (a Asset) Equal(b Asset) bool {
	return string(a) == string(b)
}

type OrderSide string

type OrderType string

const (
	AssetBTC  = Asset("BTC")
	AssetUSDT = Asset("USDT")
	AssetETH  = Asset("ETH")
	AssetBNB  = Asset("BNB")

	SideBuy  = OrderSide("BUY")
	SideSell = OrderSide("Sell")

	TypeLimit  = OrderType("LIMIT")
	TypeMarket = OrderType("MARKET")
)

type Symbol struct {
	Text           string  `json:"text"`
	BaseAsset      Asset   `json:"base_asset"`
	BasePrecision  int     `json:"base_precision"`
	QuoteAsset     Asset   `json:"quote_asset"`
	QuotePrecision int     `json:"quote_precision"`
	MaxPrice       float64 `json:"max_price"`
	MinPrice       float64 `json:"min_price"`
	TickSize       float64 `json:"tick_size"`
	MaxQty         float64 `json:"max_qty"`
	MinQty         float64 `json:"min_qty"`
	StepSize       float64 `json:"stepsize"`
	MinNotional    float64 `json:"min_notional"`
	Volume         float64 `json:"volume"`
}

func (s Symbol) Equal(k Symbol) bool {
	return s.String() == k.String()
}

func (s Symbol) String() string {
	return s.Text
}

type Symbols []Symbol

func (symbs Symbols) Len() int {
	return len(symbs)
}

func (symbs Symbols) Less(i, j int) bool {
	return symbs[i].Volume > symbs[j].Volume
}

func (symbs Symbols) Swap(i, j int) {
	symbs[i], symbs[j] = symbs[j], symbs[i]
}

type Sequence struct {
	Symbol   Symbol
	Side     OrderSide
	From     Asset
	To       Asset
	Price    float64
	Quantity float64
	Target   float64
	Src      *Depth
	Next     *Sequence
}

type Order struct {
	ID        string
	Symbol    Symbol
	OrderType OrderType
	Price     float64
	Side      OrderSide
	Quantity  float64
}

type Depth struct {
	BaseAsset  Asset     `json:"base_asset"`
	QuoteAsset Asset     `json:"quote_asset"`
	Symbol     Symbol    `json:"symbol"`
	BidPrice   float64   `json:"bid_price"`
	AskPrice   float64   `json:"ask_price"`
	BidQty     float64   `json:"bid_qty"`
	AskQty     float64   `json:"ask_qty"`
	Time       time.Time `json:"time"`
}

type Balance struct {
	Asset Asset
	Free  float64
	Total float64
}
