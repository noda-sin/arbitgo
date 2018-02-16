package models

type Asset string

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
	Text           string
	BaseAsset      Asset
	BasePrecision  int
	QuoteAsset     Asset
	QuotePrecision int
	MaxPrice       float64
	MinPrice       float64
	TickSize       float64
	MaxQty         float64
	MinQty         float64
	StepSize       float64
	MinNotional    float64
}

func (s Symbol) String() string {
	return s.Text
}

type Order struct {
	Symbol     Symbol
	BaseAsset  Asset
	QuoteAsset Asset
	OrderType  OrderType
	Price      float64
	Side       OrderSide
	Qty        float64
}

type OrderBook struct {
	WillBeQty float64
	Score     float64
	Orders    []*Order
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
