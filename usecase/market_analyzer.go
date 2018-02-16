package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

type MarketAnalyzer struct {
	MainAsset models.Asset
	Charge    float64
	MaxQty    float64
	Threshold float64
}

func NewMarketAnalyzer(mainAsset models.Asset, charge float64, maxqty float64, threshold float64) MarketAnalyzer {
	return MarketAnalyzer{
		MainAsset: mainAsset,
		Charge:    charge,
		MaxQty:    maxqty,
		Threshold: threshold,
	}
}

func (ma *MarketAnalyzer) GenerateBestOrderBook(depthList []*models.Depth, currBalance float64) *models.OrderBook {
	var best *models.OrderBook
	var depth *models.RotationDepth
	for _, d := range GenerateRotationDepthList(ma.MainAsset, depthList) {
		orderBook := GenerateOrderBook(ma.MainAsset, d, currBalance, ma.MaxQty, ma.Charge)
		if orderBook == nil {
			continue
		}
		if best == nil || orderBook.Score > best.Score {
			best = orderBook
			depth = d

			log.Info("found a transaction with profit", best.Score)
		}
	}

	if best == nil || best.Score <= ma.Threshold {
		return nil
	}

	for _, d := range depth.DepthList {
		log.WithFields(log.Fields{
			"Symbol":      d.Symbol,
			"BidPrice":    d.BidPrice,
			"BidQty":      d.BidQty,
			"AskPrice":    d.AskPrice,
			"AskQty":      d.AskQty,
			"MaxQty":      d.Symbol.MaxQty,
			"MinQty":      d.Symbol.MinQty,
			"MinNotional": d.Symbol.MinNotional,
		}).Debug("best order depth")
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

func GenerateOrderBook(mainAsset models.Asset, rotateDepth *models.RotationDepth, currentBalance float64, maxLimitQty float64, charge float64) *models.OrderBook {
	if rotateDepth == nil || len(rotateDepth.DepthList) == 0 {
		return nil
	}

	currentAsset := mainAsset
	minMainQty := 0.0 // MainAsset建で最小のQtyを計算

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
	} else {
		side1 = models.SideSell
		price1 = depth1.BidPrice
		// marketQty1 = depth1.BidQty
		qty1 = depth1.BidQty
		currentAsset = depth1.QuoteAsset
	}
	minMainQty = qty1

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
	}

	if qty2 < minMainQty {
		minMainQty = qty2
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
	} else {
		side3 = models.SideSell
		price3 = depth3.BidPrice
		// marketQty3 = depth3.BidQty
		qty3 = depth3.BidQty * depth3.BidPrice
	}

	if qty3 < minMainQty {
		minMainQty = qty3
	}

	// お財布内のMainAsset量の方が少ない場合は、お財布内全部を利用値とする
	if currentBalance < minMainQty {
		minMainQty = currentBalance
	}

	// 利用制限上限に触れている場合は、制限値を利用値とする
	if maxLimitQty != 0 && maxLimitQty < minMainQty {
		minMainQty = maxLimitQty
	}

	if symbol1.MinNotional > minMainQty {
		log.Debug("Qty is less than min notional")
		return nil
	}

	beginMainQty := minMainQty
	// チャージ計算
	minMainQty = (1 - charge) * minMainQty

	if side1 == models.SideBuy {
		qty1 = util.Floor(minMainQty/price1, symbol1.StepSize)
	} else {
		qty1 = util.Floor(minMainQty, symbol1.StepSize)
	}

	// 制限チェック
	if symbol1.MaxQty < qty1 || symbol1.MinQty > qty1 || symbol2.MinNotional > qty1 {
		log.Debug("Symbol1 qty is not within the limit range")
		return nil
	}

	if symbol1.MaxPrice < price1 || symbol1.MinPrice > price1 {
		log.Debug("Symbol1 price is not within the limit range")
		return nil
	}

	// チャージ計算
	qty1 = (1 - charge) * qty1

	if side2 == models.SideBuy {
		if side1 == models.SideBuy {
			qty2 = util.Floor(qty1/price2, symbol2.StepSize)
		} else {
			qty2 = util.Floor((qty1*price1)/price2, symbol2.StepSize)
		}
	} else {
		if side1 == models.SideBuy {
			qty2 = util.Floor(qty1, symbol2.StepSize)
		} else {
			qty2 = util.Floor(qty1*price1, symbol2.StepSize)
		}
	}

	// 制限チェック
	if symbol2.MaxQty < qty2 || symbol2.MinQty > qty2 || symbol3.MinNotional > qty2 {
		log.Debug("Symbol2 qty is not within the limit range")
		return nil
	}

	if symbol2.MaxPrice < price2 || symbol2.MinPrice > price2 {
		log.Debug("Symbol2 price is not within the limit range")
		return nil
	}

	// チャージ計算
	qty2 = (1 - charge) * qty2

	if side3 == models.SideBuy {
		if side2 == models.SideBuy {
			qty3 = util.Floor(qty2/price3, symbol3.StepSize)
		} else {
			qty3 = util.Floor((qty2*price2)/price3, symbol3.StepSize)
		}
	} else {
		if side2 == models.SideBuy {
			qty3 = util.Floor(qty2, symbol3.StepSize)
		} else {
			qty3 = util.Floor(qty2*price2, symbol3.StepSize)
		}
	}

	// 制限チェック
	if symbol3.MaxQty < qty3 || symbol3.MinQty > qty3 {
		log.Debug("Symbol3 qty is not within the limit range")
		return nil
	}

	if symbol3.MaxPrice < price3 || symbol3.MinPrice > price3 {
		log.Debug("Symbol3 price is not within the limit range")
		return nil
	}

	var endMainQty float64
	if side3 == models.SideBuy {
		endMainQty = qty3
	} else {
		endMainQty = qty3 * price3
	}

	score := (endMainQty - beginMainQty) / beginMainQty

	// 制限チェック
	if score < 0 {
		log.Debug("Score is less than 0")
		return nil
	}

	qty1 = util.Floor(qty1, symbol1.StepSize)
	qty2 = util.Floor(qty2, symbol2.StepSize)
	qty3 = util.Floor(qty3, symbol3.StepSize)

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
		WillBeQty: endMainQty,
		Score:     score,
		Orders:    orders,
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
		if symbol.String() == string(targetAsset)+string(mainAsset) {
			return &models.Order{
				Symbol:     symbol,
				BaseAsset:  targetAsset,
				QuoteAsset: mainAsset,
				OrderType:  models.TypeMarket,
				Side:       models.SideSell,
				Qty:        balance.Free,
			}
		} else if symbol.String() == string(mainAsset)+string(targetAsset) {
			return &models.Order{
				Symbol:     symbol,
				BaseAsset:  mainAsset,
				QuoteAsset: targetAsset,
				OrderType:  models.TypeMarket,
				Side:       models.SideBuy,
				Qty:        balance.Free,
			}
		}
	}
	return nil
}
