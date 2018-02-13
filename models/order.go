package models

type Order struct {
	Symbol     string
	BaseAsset  string
	QuoteAsset string
	Price      float64
	Side       string
	MarketQty  float64
	BaseQty    float64
	QuoteQty   float64
}

type Trade struct {
	MaxQty float64
	Profit float64
	Score  float64
	Orders []*Order
}
