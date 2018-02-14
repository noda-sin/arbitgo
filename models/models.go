package models

type Asset string

type Symbol string

type OrderSide string

const (
	AssetBTC  = Asset("BTC")
	AssetUSDT = Asset("USDT")
	AssetETH  = Asset("ETH")
	AssetBNB  = Asset("BNB")

	SideBuy  = OrderSide("BUY")
	SideSell = OrderSide("Sell")
)

type Order struct {
	Symbol     Symbol
	BaseAsset  Asset
	QuoteAsset Asset
	Price      float64
	Side       OrderSide
	Qty        float64
}

type OrderBook struct {
	Score  float64
	Orders []*Order
}

type Depth struct {
	BaseAsset  Asset
	QuoteAsset Asset
	Symbol     Symbol
	BidPrice   float64
	AskPrice   float64
	BidQty     float64
	AskQty     float64
}

type RotationDepth struct {
	DepthList []*Depth
}

type Balance struct {
	Asset Asset
	Free  float64
	Total float64
}
