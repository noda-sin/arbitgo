package models

import (
	"github.com/OopsMouse/arbitgo/common"
)

type Market struct {
	StartSymbol         string
	QuoteSymbols        []string
	StartSymbolTickers  map[string]*Ticker
	OtherTickers        map[string][]*Ticker
	QuoteToQuoteTickers map[string]*Ticker
}

func NewMarket(startSymbol string, tickers []*Ticker) *Market {
	qs := common.NewSet() // 基軸通貨を格納するSET
	startTksMap := map[string]*Ticker{}
	otherTksMap := map[string][]*Ticker{}
	qqTksMap := map[string]*Ticker{}
	for _, tk := range tickers {
		qs.Append(tk.QuoteSymbol)
		if tk.QuoteSymbol == startSymbol { // ベースがBTCのものとそうでないもので分ける
			startTksMap[tk.BaseSymbol] = tk
		} else {
			otherTksMap[tk.BaseSymbol] = append(otherTksMap[tk.BaseSymbol], tk)
		}
	}
	for k, _ := range startTksMap {
		if qs.Include(k) { // 基軸通貨同士のTickerは除外する
			delete(startTksMap, k)
		}
	}
	for k, _ := range otherTksMap {
		if startTksMap[k] == nil { // BTCでの取引ができないシンボルは除外する
			delete(otherTksMap, k)
		} else if qs.Include(k) { // 基軸通貨同士のTickerは除外する
			delete(otherTksMap, k)
		}
	}
	for _, tk := range tickers {
		// 基軸通貨同士のTickerである
		if qs.Include(tk.QuoteSymbol) && qs.Include(tk.BaseSymbol) {
			qqTksMap[tk.QuoteSymbol+tk.BaseSymbol] = tk
			qqTksMap[tk.BaseSymbol+tk.QuoteSymbol] = tk
		}
	}
	return &Market{
		StartSymbol:         startSymbol,
		QuoteSymbols:        qs.ToSlice(),
		StartSymbolTickers:  startTksMap,
		OtherTickers:        otherTksMap,
		QuoteToQuoteTickers: qqTksMap,
	}
}

func (m *Market) GetTradeTickers() [][]*Ticker {
	tradeTickers := [][]*Ticker{}
	for k, v := range m.StartSymbolTickers {
		for _, t := range m.OtherTickers[k] {
			quoteToQuoteTicker := m.QuoteToQuoteTickers[m.StartSymbol+t.QuoteSymbol]
			if quoteToQuoteTicker == nil {
				continue
			}
			// 正方向
			tickerPairs1 := []*Ticker{}
			tickerPairs1 = append(tickerPairs1, v)
			tickerPairs1 = append(tickerPairs1, t)
			tickerPairs1 = append(tickerPairs1, quoteToQuoteTicker)
			tradeTickers = append(tradeTickers, tickerPairs1)

			// 逆方向
			tickerPairs2 := []*Ticker{}
			tickerPairs2 = append(tickerPairs2, quoteToQuoteTicker)
			tickerPairs2 = append(tickerPairs2, t)
			tickerPairs2 = append(tickerPairs2, v)
			tradeTickers = append(tradeTickers, tickerPairs2)
		}
	}
	return tradeTickers
}
