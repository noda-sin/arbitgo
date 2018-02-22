package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	"github.com/google/uuid"
	"github.com/pkg/errors"
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

func (ma *MarketAnalyzer) ArbitrageOrders(depthList []*models.Depth, currBalance float64) []models.Order {
	var bastScore float64
	var bestOrders []models.Order
	for _, d := range GenerateRotationDepthList(ma.MainAsset, depthList) {
		score, orders := GenerateOrderBook(ma.MainAsset, d, currBalance, ma.MaxQty, ma.Charge)
		if orders == nil {
			continue
		}
		if score > bastScore {
			bestOrders = orders
		}
	}

	if bestOrders == nil ||
		bastScore <= ma.Threshold {
		return nil
	}

	return bestOrders
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

func GenerateOrderBook(mainAsset models.Asset, rotateDepth *models.RotationDepth, currentBalance float64, maxLimitQty float64, charge float64) (float64, []models.Order) {
	if rotateDepth == nil || len(rotateDepth.DepthList) == 0 {
		return 0, nil
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
			return 0, nil
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
		return 0, nil
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
		return 0, nil
	}

	if symbol1.MaxPrice < price1 || symbol1.MinPrice > price1 {
		log.Debug("Symbol1 price is not within the limit range")
		return 0, nil
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
		return 0, nil
	}

	if symbol2.MaxPrice < price2 || symbol2.MinPrice > price2 {
		log.Debug("Symbol2 price is not within the limit range")
		return 0, nil
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
		return 0, nil
	}

	if symbol3.MaxPrice < price3 || symbol3.MinPrice > price3 {
		log.Debug("Symbol3 price is not within the limit range")
		return 0, nil
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
		return 0, nil
	}

	qty1 = util.Floor(qty1, symbol1.StepSize)
	qty2 = util.Floor(qty2, symbol2.StepSize)
	qty3 = util.Floor(qty3, symbol3.StepSize)

	orders := []models.Order{}
	orders = append(orders, models.Order{
		Symbol:        symbol1,
		OrderType:     models.TypeLimit,
		Price:         price1,
		Side:          side1,
		Qty:           qty1,
		ClientOrderID: uuid.New().String(),
	})

	orders = append(orders, models.Order{
		Symbol:        symbol2,
		OrderType:     models.TypeLimit,
		Price:         price2,
		Side:          side2,
		Qty:           qty2,
		ClientOrderID: uuid.New().String(),
	})

	orders = append(orders, models.Order{
		Symbol:        symbol3,
		OrderType:     models.TypeLimit,
		Price:         price3,
		Side:          side3,
		Qty:           qty3,
		ClientOrderID: uuid.New().String(),
	})

	return score, orders
}

func (ma *MarketAnalyzer) ForceChangeOrders(symbols []models.Symbol, from models.Asset, to models.Asset) ([]models.Order, error) {
	orders := []models.Order{}

	// 直接的に変える手段を検索
	for _, s := range symbols {
		if s.QuoteAsset == from && s.BaseAsset == to {
			orders = append(orders, models.Order{
				Symbol:        s,
				OrderType:     models.TypeMarket,
				Side:          models.SideBuy,
				ClientOrderID: uuid.New().String(),
			})
			return orders, nil
		} else if s.QuoteAsset == to && s.BaseAsset == from {
			orders = append(orders, models.Order{
				Symbol:        s,
				OrderType:     models.TypeMarket,
				Side:          models.SideSell,
				ClientOrderID: uuid.New().String(),
			})
			return orders, nil
		}
	}

	// 関節的に変える手段を検索
	symbolsRelatedFrom := []models.Symbol{}
	symbolsRelatedTo := []models.Symbol{}
	for _, s := range symbols {
		if s.QuoteAsset == from || s.BaseAsset == from {
			symbolsRelatedFrom = append(symbolsRelatedFrom, s)
		} else if s.QuoteAsset == to || s.BaseAsset == to {
			symbolsRelatedTo = append(symbolsRelatedTo, s)
		}
	}

	for _, i := range symbolsRelatedFrom {
		for _, j := range symbolsRelatedTo {
			if i.QuoteAsset == from && i.BaseAsset == j.BaseAsset {
				orders = append(orders, models.Order{
					Symbol:        i,
					OrderType:     models.TypeMarket,
					Side:          models.SideBuy,
					ClientOrderID: uuid.New().String(),
				})
				orders = append(orders, models.Order{
					Symbol:        j,
					OrderType:     models.TypeMarket,
					Side:          models.SideSell,
					ClientOrderID: uuid.New().String(),
				})
				return orders, nil
			} else if i.QuoteAsset == from && i.BaseAsset == j.QuoteAsset {
				orders = append(orders, models.Order{
					Symbol:        i,
					OrderType:     models.TypeMarket,
					Side:          models.SideBuy,
					ClientOrderID: uuid.New().String(),
				})
				orders = append(orders, models.Order{
					Symbol:        j,
					OrderType:     models.TypeMarket,
					Side:          models.SideBuy,
					ClientOrderID: uuid.New().String(),
				})
				return orders, nil
			} else if i.BaseAsset == from && i.QuoteAsset == j.BaseAsset {
				orders = append(orders, models.Order{
					Symbol:        i,
					OrderType:     models.TypeMarket,
					Side:          models.SideSell,
					ClientOrderID: uuid.New().String(),
				})
				orders = append(orders, models.Order{
					Symbol:        j,
					OrderType:     models.TypeMarket,
					Side:          models.SideSell,
					ClientOrderID: uuid.New().String(),
				})
				return orders, nil
			} else if i.BaseAsset == from && i.QuoteAsset == j.QuoteAsset {
				orders = append(orders, models.Order{
					Symbol:        i,
					OrderType:     models.TypeMarket,
					Side:          models.SideSell,
					ClientOrderID: uuid.New().String(),
				})
				orders = append(orders, models.Order{
					Symbol:        j,
					OrderType:     models.TypeMarket,
					Side:          models.SideBuy,
					ClientOrderID: uuid.New().String(),
				})
				return orders, nil
			}
		}
	}
	return nil, errors.Errorf("Not found orders to force change")
}

func (ma *MarketAnalyzer) SplitOrders(parentOrders []models.Order, qty float64) ([]models.Order, []models.Order) {
	newParentOrders := []models.Order{}
	childOrders := []models.Order{}
	nextQty := qty
	var pravious *models.Order

	for i, o := range parentOrders {
		if pravious != nil {
			nextQty = (1 - ma.Charge) * nextQty
			if o.Side == models.SideBuy {
				nextQty = util.Floor(nextQty/pravious.Price, o.Symbol.StepSize)
			} else {
				nextQty = util.Floor((nextQty*pravious.Price)/o.Price, o.Symbol.StepSize)
			}
		}

		newParentOrders = append(newParentOrders, models.Order{
			Symbol:        o.Symbol,
			OrderType:     o.OrderType,
			Price:         o.Price,
			Side:          o.Side,
			Qty:           o.Qty - nextQty,
			ClientOrderID: o.ClientOrderID,
		})

		if i > 0 {
			childOrders = append(childOrders, models.Order{
				Symbol:        o.Symbol,
				OrderType:     o.OrderType,
				Price:         o.Price,
				Side:          o.Side,
				Qty:           nextQty,
				ClientOrderID: o.ClientOrderID,
			})
		}

		pravious = &o
	}

	return newParentOrders, childOrders
}
