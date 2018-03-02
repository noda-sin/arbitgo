package usecase

import (
	"fmt"
	"strconv"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

type MarketAnalyzer struct {
	MainAsset models.Asset
	Charge    float64
}

func NewMarketAnalyzer(mainAsset models.Asset, charge float64) MarketAnalyzer {
	return MarketAnalyzer{
		MainAsset: mainAsset,
		Charge:    charge,
	}
}

func (ma *MarketAnalyzer) ArbitrageOrders(depthList []*models.Depth, currBalance float64) *models.TradeOrder {
	var bestScore float64
	var bestOrders []models.Order
	for _, d := range GenerateRotationDepthList(ma.MainAsset, depthList) {
		score, orders := ma.GenerateOrders(d, currBalance)
		if orders == nil {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestOrders = orders
		}
	}

	if bestOrders == nil ||
		bestScore <= 0 {
		return nil
	}

	return &models.TradeOrder{
		Score:  bestScore,
		Orders: bestOrders,
	}
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

func (ma *MarketAnalyzer) GenerateOrders(rotateDepth *models.RotationDepth, currentBalance float64) (float64, []models.Order) {
	mainAsset := ma.MainAsset
	charge := ma.Charge

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
	qty1sub := (1 - charge) * qty1

	if side2 == models.SideBuy {
		if side1 == models.SideBuy {
			qty2 = util.Floor(qty1sub/price2, symbol2.StepSize)
		} else {
			qty2 = util.Floor((qty1sub*price1)/price2, symbol2.StepSize)
		}
	} else {
		if side1 == models.SideBuy {
			qty2 = util.Floor(qty1sub, symbol2.StepSize)
		} else {
			qty2 = util.Floor(qty1sub*price1, symbol2.StepSize)
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
	qty2sub := (1 - charge) * qty2

	if side3 == models.SideBuy {
		if side2 == models.SideBuy {
			qty3 = util.Floor(qty2sub/price3, symbol3.StepSize)
		} else {
			qty3 = util.Floor((qty2sub*price2)/price3, symbol3.StepSize)
		}
	} else {
		if side2 == models.SideBuy {
			qty3 = util.Floor(qty2sub, symbol3.StepSize)
		} else {
			qty3 = util.Floor(qty2sub*price2, symbol3.StepSize)
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
		endMainQty = qty3 * (1 - charge)
	} else {
		endMainQty = qty3 * price3 * (1 - charge)
	}

	score := (endMainQty - beginMainQty) / beginMainQty

	// 制限チェック
	if score < 0 {
		log.Debug("Score is less than 0")
		return 0, nil
	}

	orders := []models.Order{}
	orders = append(orders, models.Order{
		Step:          len(orders) + 1,
		Symbol:        symbol1,
		OrderType:     models.TypeLimit,
		Price:         price1,
		Side:          side1,
		Qty:           qty1,
		ClientOrderID: xid.New().String(),
		SourceDepth:   depth1,
	})

	orders = append(orders, models.Order{
		Step:          len(orders) + 1,
		Symbol:        symbol2,
		OrderType:     models.TypeLimit,
		Price:         price2,
		Side:          side2,
		Qty:           qty2,
		ClientOrderID: xid.New().String(),
		SourceDepth:   depth2,
	})

	orders = append(orders, models.Order{
		Step:          len(orders) + 1,
		Symbol:        symbol3,
		OrderType:     models.TypeLimit,
		Price:         price3,
		Side:          side3,
		Qty:           qty3,
		ClientOrderID: xid.New().String(),
		SourceDepth:   depth3,
	})

	return score, orders
}

func (ma *MarketAnalyzer) ForceChangeOrders(symbols []models.Symbol, from models.Asset, to models.Asset) ([]models.Order, error) {
	orders := []models.Order{}

	// 直接的に変える手段を検索
	for _, s := range symbols {
		if s.QuoteAsset == from && s.BaseAsset == to {
			orders = append(orders, models.Order{
				Step:          len(orders) + 1,
				Symbol:        s,
				OrderType:     models.TypeMarket,
				Side:          models.SideBuy,
				ClientOrderID: xid.New().String(),
			})
			return orders, nil
		} else if s.QuoteAsset == to && s.BaseAsset == from {
			orders = append(orders, models.Order{
				Step:          len(orders) + 1,
				Symbol:        s,
				OrderType:     models.TypeMarket,
				Side:          models.SideSell,
				ClientOrderID: xid.New().String(),
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
					Step:          len(orders) + 1,
					Symbol:        i,
					OrderType:     models.TypeMarket,
					Side:          models.SideBuy,
					ClientOrderID: xid.New().String(),
				})
				orders = append(orders, models.Order{
					Step:          len(orders) + 1,
					Symbol:        j,
					OrderType:     models.TypeMarket,
					Side:          models.SideSell,
					ClientOrderID: xid.New().String(),
				})
				return orders, nil
			} else if i.QuoteAsset == from && i.BaseAsset == j.QuoteAsset {
				orders = append(orders, models.Order{
					Step:          len(orders) + 1,
					Symbol:        i,
					OrderType:     models.TypeMarket,
					Side:          models.SideBuy,
					ClientOrderID: xid.New().String(),
				})
				orders = append(orders, models.Order{
					Step:          len(orders) + 1,
					Symbol:        j,
					OrderType:     models.TypeMarket,
					Side:          models.SideBuy,
					ClientOrderID: xid.New().String(),
				})
				return orders, nil
			} else if i.BaseAsset == from && i.QuoteAsset == j.BaseAsset {
				orders = append(orders, models.Order{
					Step:          len(orders) + 1,
					Symbol:        i,
					OrderType:     models.TypeMarket,
					Side:          models.SideSell,
					ClientOrderID: xid.New().String(),
				})
				orders = append(orders, models.Order{
					Step:          len(orders) + 1,
					Symbol:        j,
					OrderType:     models.TypeMarket,
					Side:          models.SideSell,
					ClientOrderID: xid.New().String(),
				})
				return orders, nil
			} else if i.BaseAsset == from && i.QuoteAsset == j.QuoteAsset {
				orders = append(orders, models.Order{
					Step:          len(orders) + 1,
					Symbol:        i,
					OrderType:     models.TypeMarket,
					Side:          models.SideSell,
					ClientOrderID: xid.New().String(),
				})
				orders = append(orders, models.Order{
					Step:          len(orders) + 1,
					Symbol:        j,
					OrderType:     models.TypeMarket,
					Side:          models.SideBuy,
					ClientOrderID: xid.New().String(),
				})
				return orders, nil
			}
		}
	}
	return nil, errors.Errorf("Not found orders to force change")
}

func (ma *MarketAnalyzer) SplitOrders(parentOrders []models.Order, qty float64) ([]models.Order, []models.Order, error) {
	newParentOrders := []models.Order{}
	childOrders := []models.Order{}
	rate := qty / parentOrders[0].Qty

	for i, o := range parentOrders {
		if i == 0 {
			qty := util.Floor(o.Qty-qty, o.Symbol.StepSize)
			if qty == 0 {
				return nil, nil, fmt.Errorf("Qty may be 0 in the cases")
			}
			newParentOrders = append(newParentOrders, models.Order{
				Step:          o.Step,
				Symbol:        o.Symbol,
				OrderType:     o.OrderType,
				Price:         o.Price,
				Side:          o.Side,
				Qty:           qty,
				ClientOrderID: o.ClientOrderID,
				SourceDepth:   o.SourceDepth,
			})
		} else {
			qty := util.Floor((1-rate)*o.Qty, o.Symbol.StepSize)
			if qty == 0 {
				return nil, nil, fmt.Errorf("Qty may be 0 in the cases")
			}
			newParentOrders = append(newParentOrders, models.Order{
				Step:          o.Step,
				Symbol:        o.Symbol,
				OrderType:     o.OrderType,
				Price:         o.Price,
				Side:          o.Side,
				Qty:           qty,
				ClientOrderID: o.ClientOrderID,
				SourceDepth:   o.SourceDepth,
			})
			qty = util.Floor(rate*o.Qty, o.Symbol.StepSize)
			if qty == 0 {
				return nil, nil, fmt.Errorf("Qty may be 0 in the cases")
			}
			childOrders = append(childOrders, models.Order{
				Step:          o.Step,
				Symbol:        o.Symbol,
				OrderType:     o.OrderType,
				Price:         o.Price,
				Side:          o.Side,
				Qty:           qty,
				ClientOrderID: xid.New().String(),
				SourceDepth:   o.SourceDepth,
			})
		}
	}

	return newParentOrders, childOrders, nil
}

func (ma *MarketAnalyzer) ValidateOrders(orders []models.Order, depthes []*models.Depth) bool {
	// sortedDepth := []*models.Depth{}
	// for _, order := range orders {
	// 	for _, depth := range depthes {
	// 		if order.Symbol.String() == depth.Symbol.String() {
	// 			sortedDepth = append(sortedDepth, depth)
	// 		}
	// 	}
	// }

	// start := orders[0].Qty
	// currentQty := start
	// for i, depth := range sortedDepth {
	// 	order := orders[i]
	// 	if order.Side == models.SideBuy {
	// 		currentQty = util.Floor(currentQty, depth.Symbol.StepSize) / depth.AskPrice
	// 	} else {
	// 		currentQty = util.Floor(currentQty, depth.Symbol.StepSize) * depth.BidPrice
	// 	}
	// }
	// end := currentQty
	// if end > start {
	// 	return true
	// }
	// return false
	for _, order := range orders {
		for _, depth := range depthes {
			if order.Symbol.String() == depth.Symbol.String() {
				if order.Side == models.SideBuy {
					log.Info("----------------- depth #" + strconv.Itoa(order.Step) + " ----------------")
					log.Info(" Symbol   : ", order.Symbol)
					log.Info(" Side     : ", order.Side)
					log.Info(" Price    : ", order.Price, " vs ", depth.AskPrice)
					log.Info(" Quantity : ", order.Qty, " vs ", depth.AskQty)
					log.Info(" Time     : ", depth.Time.Sub(order.SourceDepth.Time))
					log.Info("------------------------------------------")

					ok := (depth.AskPrice <= order.Price) && (depth.AskQty >= order.Qty)
					if ok == false {
						return false
					}
				} else {
					log.Info("----------------- depth #" + strconv.Itoa(order.Step) + " ----------------")
					log.Info(" Symbol   : ", order.Symbol)
					log.Info(" Side     : ", order.Side)
					log.Info(" Price    : ", order.Price, " vs ", depth.BidPrice)
					log.Info(" Quantity : ", order.Qty, " vs ", depth.BidQty)
					log.Info(" Time     : ", depth.Time.Sub(order.SourceDepth.Time))
					log.Info("-------------------------------------------")

					ok := (depth.BidPrice >= order.Price) && (depth.BidQty >= order.Qty)
					if ok == false {
						return false
					}
				}
				break
			}
		}
	}
	return true
}

func (ma *MarketAnalyzer) ReplaceOrders(tradeOrder *models.TradeOrder, depthes []*models.Depth, currBalance float64) (*models.TradeOrder, bool) {
	sortedDepth := []*models.Depth{}
	for _, order := range tradeOrder.Orders {
		for _, depth := range depthes {
			if order.Symbol.String() == depth.Symbol.String() {
				sortedDepth = append(sortedDepth, depth)
			}
		}
	}

	rotateDepth := &models.RotationDepth{
		DepthList: sortedDepth,
	}

	score, newOrders := ma.GenerateOrders(rotateDepth, currBalance)
	if newOrders == nil {
		return nil, false
	}

	return &models.TradeOrder{
		Score:  score,
		Orders: newOrders,
	}, true
}
