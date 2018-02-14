package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
)

type MarketAnalyzer struct {
	MainAsset  models.Asset
	Charge     float64
	Threashold float64
}

func NewMarketAnalyzer(mainAsset models.Asset, charge float64, threashold float64) MarketAnalyzer {
	return MarketAnalyzer{
		MainAsset:  mainAsset,
		Charge:     charge,
		Threashold: threashold,
	}
}

func (ma *MarketAnalyzer) GenerateBestOrderBook(depthList []*models.Depth, currBalance float64) *models.OrderBook {
	var best *models.OrderBook
	for _, d := range GenerateRotationDepthList(ma.MainAsset, depthList) {
		orderBook := GenerateOrderBook(ma.MainAsset, d, currBalance, ma.Charge)
		if orderBook == nil {
			continue
		}
		if best == nil || orderBook.Score > best.Score {
			best = orderBook
		}
	}

	if best == nil || best.Score <= ma.Threashold {
		return nil
	}

	return best
}

func GenerateRotationDepthList(mainAsset models.Asset, depthList []*models.Depth) []*models.RotationDepth {
	quoteAssetSet := util.NewSet()
	for _, depth := range depthList {
		quoteAssetSet.Append(string(depth.QuoteAsset))
	}

	mainAssetDepth := map[string]*models.Depth{}
	otherAssetDepth := map[string][]*models.Depth{}
	for _, depth := range depthList {
		if depth.QuoteAsset == mainAsset {
			mainAssetDepth[string(depth.BaseAsset)] = depth
		} else {
			otherAssetDepth[string(depth.BaseAsset)] = append(otherAssetDepth[string(depth.BaseAsset)], depth)
		}
	}

	for k, _ := range mainAssetDepth {
		if quoteAssetSet.Include(k) {
			delete(mainAssetDepth, k)
		}
	}

	for k, _ := range otherAssetDepth {
		if mainAssetDepth[k] == nil {
			delete(otherAssetDepth, k)
		} else if quoteAssetSet.Include(k) {
			delete(otherAssetDepth, k)
		}
	}

	quoteAssetToQuoteAssetDepth := map[string]*models.Depth{}
	for _, depth := range depthList {
		if quoteAssetSet.Include(string(depth.QuoteAsset)) && quoteAssetSet.Include(string(depth.BaseAsset)) {
			quoteAssetToQuoteAssetDepth[string(depth.QuoteAsset)+string(depth.BaseAsset)] = depth
			quoteAssetToQuoteAssetDepth[string(depth.BaseAsset)+string(depth.QuoteAsset)] = depth
		}
	}

	rotateDepthList := []*models.RotationDepth{}
	for k, v := range mainAssetDepth {
		for _, t := range otherAssetDepth[k] {
			depth1 := v
			depth2 := t
			depth3 := quoteAssetToQuoteAssetDepth[string(mainAsset)+string(t.QuoteAsset)]
			if depth3 == nil {
				continue
			}

			depthList1 := []*models.Depth{}
			depthList1 = append(depthList1, depth1)
			depthList1 = append(depthList1, depth2)
			depthList1 = append(depthList1, depth3)

			depthList2 := []*models.Depth{}
			depthList2 = append(depthList2, depth3)
			depthList2 = append(depthList2, depth2)
			depthList2 = append(depthList2, depth1)

			rotateDepthList = append(rotateDepthList, &models.RotationDepth{
				DepthList: depthList1,
			})
			rotateDepthList = append(rotateDepthList, &models.RotationDepth{
				DepthList: depthList2,
			})
		}
	}
	return rotateDepthList
}

func GenerateOrderBook(mainAsset models.Asset, rotateDepth *models.RotationDepth, currentBalance float64, charge float64) *models.OrderBook {
	if rotateDepth == nil || len(rotateDepth.DepthList) == 0 {
		return nil
	}

	currentAsset := mainAsset
	minQty := 0.0
	profit := 1.0

	// 1回目の取引
	depth1 := rotateDepth.DepthList[0]
	symbol1 := depth1.Symbol
	var side1 models.OrderSide
	var price1 float64
	// var marketQty1 float64
	var qty1 float64
	if depth1.QuoteAsset == currentAsset {
		side1 = models.SideBuy
		price1 = depth1.AskPrice
		// marketQty1 = depth1.AskQty
		qty1 = depth1.AskPrice * depth1.AskQty
		currentAsset = depth1.BaseAsset
		profit = (1 - charge) * profit / depth1.AskPrice
	} else {
		side1 = models.SideSell
		price1 = depth1.BidPrice
		// marketQty1 = depth1.BidQty
		qty1 = depth1.BidQty
		currentAsset = depth1.QuoteAsset
		profit = (1 - charge) * profit * depth1.BidPrice
	}
	minQty = qty1

	// 2回目の取引
	depth2 := rotateDepth.DepthList[1]
	symbol2 := depth2.Symbol
	var side2 models.OrderSide
	var price2 float64
	// var marketQty2 float64
	var qty2 float64
	if depth2.QuoteAsset == currentAsset {
		side2 = models.SideBuy
		price2 = depth2.AskPrice
		// marketQty2 = depth2.AskQty
		if side1 == models.SideBuy {
			qty2 = depth2.AskPrice * depth2.AskQty * price1
		} else {
			qty2 = depth2.AskPrice * depth2.AskQty * (1.0 / price1)
		}
		currentAsset = depth2.BaseAsset
		profit = (1 - charge) * profit / depth2.AskPrice
	} else {
		side2 = models.SideSell
		price2 = depth2.BidPrice
		// marketQty2 = depth2.BidQty
		if side1 == models.SideBuy {
			qty2 = depth2.BidQty * price1
		} else {
			// ありえない
			return nil
		}
		currentAsset = depth2.QuoteAsset
		profit = (1 - charge) * profit * depth2.BidPrice
	}

	if qty2 < minQty {
		minQty = qty2
	}

	// 3回目の取引
	depth3 := rotateDepth.DepthList[2]
	symbol3 := depth3.Symbol
	var side3 models.OrderSide
	var price3 float64
	// var marketQty3 float64
	var qty3 float64
	if depth3.QuoteAsset == currentAsset {
		side3 = models.SideBuy
		price3 = depth3.AskPrice
		// marketQty3 = depth3.AskQty
		qty3 = depth3.AskQty * (1.0 / depth3.AskPrice)
		profit = (1 - charge) * profit / depth3.AskPrice
	} else {
		side3 = models.SideSell
		price3 = depth3.BidPrice
		// marketQty3 = depth3.BidQty
		qty3 = depth3.BidQty * depth3.BidPrice
		profit = (1 - charge) * profit * depth3.BidPrice
	}

	if qty3 < minQty {
		minQty = qty3
	}

	if currentBalance < minQty {
		minQty = currentBalance
	}

	score := (profit - 1.0)

	// var qqty1 float64
	if side1 == models.SideBuy {
		qty1 = minQty / price1
	} else {
		qty1 = minQty
	}
	// qqty1 = minQty

	// var qqty2 float64
	if side2 == models.SideBuy {
		if side1 == models.SideBuy {
			qty2 = qty1 / price2
		} else {
			qty2 = (qty1 * price1) / price2
		}
	} else {
		if side1 == models.SideBuy {
			qty2 = qty1
		} else {
			qty2 = qty1 * price1
		}
	}
	// qqty2 = qqty1

	// var qqty3 float64
	if side3 == models.SideBuy {
		if side2 == models.SideBuy {
			qty3 = qty2 / price3
		} else {
			qty3 = (qty2 * price2) / price3
		}
	} else {
		if side2 == models.SideBuy {
			qty3 = qty2
		} else {
			qty3 = qty2 * price2
		}
	}
	// qqty3 = qqty2

	orders := []*models.Order{}
	orders = append(orders, &models.Order{
		Symbol:     symbol1,
		QuoteAsset: depth1.QuoteAsset,
		BaseAsset:  depth1.BaseAsset,
		OrderType:  models.TypeLimit,
		Price:      price1,
		Side:       side1,
		Qty:        qty1,
	})

	orders = append(orders, &models.Order{
		Symbol:     symbol2,
		QuoteAsset: depth2.QuoteAsset,
		BaseAsset:  depth2.BaseAsset,
		OrderType:  models.TypeLimit,
		Price:      price2,
		Side:       side2,
		Qty:        qty2,
	})

	orders = append(orders, &models.Order{
		Symbol:     symbol3,
		QuoteAsset: depth3.QuoteAsset,
		BaseAsset:  depth3.BaseAsset,
		OrderType:  models.TypeLimit,
		Price:      price3,
		Side:       side3,
		Qty:        qty3,
	})

	return &models.OrderBook{
		Score:  score,
		Orders: orders,
	}
}

func (ma *MarketAnalyzer) GenerateRecoveryOrderBook(mainAsset models.Asset, symbols []models.Symbol, balances []*models.Balance) *models.OrderBook {
	orders := []*models.Order{}
	for _, balance := range balances {
		if balance.Asset == mainAsset {
			continue
		}
		order := GenerateRecoveryOrder(mainAsset, symbols, balance)
		if order != nil {
			orders = append(orders, order)
		}
	}
	return &models.OrderBook{
		Orders: orders,
	}
}

func GenerateRecoveryOrder(mainAsset models.Asset, symbols []models.Symbol, balance *models.Balance) *models.Order {
	targetAsset := balance.Asset
	for _, symbol := range symbols {
		if string(symbol) == string(targetAsset)+string(mainAsset) {
			return &models.Order{
				Symbol:     symbol,
				BaseAsset:  targetAsset,
				QuoteAsset: mainAsset,
				OrderType:  models.TypeMarket,
				Side:       models.SideSell,
				Qty:        balance.Free,
			}
		}
	}
	return nil
}
