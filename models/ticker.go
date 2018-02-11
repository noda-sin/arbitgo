package models

type Ticker struct {
	BaseSymbol  string
	QuoteSymbol string
	BidPrice    float64
	AskPrice    float64
	Volume      float64
}

func NewTicker(bsSymbol string, qtSymbol string, bp float64, ap float64, v float64) *Ticker {
	return &Ticker{
		BaseSymbol:  bsSymbol,
		QuoteSymbol: qtSymbol,
		BidPrice:    bp,
		AskPrice:    ap,
		Volume:      v,
	}
}
