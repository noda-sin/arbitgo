package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) runAnalyzer() {
	for {
		bigAssets := trader.BigAssets()

		assets := []string{}
		for _, a := range bigAssets {
			if !trader.isRunningPosition(a) {
				assets = append(assets, a)
			}
		}

		for _, a := range assets {
			depthes := trader.getDepthes(a)
			balance := trader.GetBalance(a).Free

			func(asset string, depthes []*models.Depth, balance float64) {
				seq := trader.bestOfSequence(asset, asset, depthes, balance)

				if seq == nil {
					return
				}

				*trader.seqch <- seq
			}(a, depthes, balance)
		}
	}
}

func (trader *Trader) bestOfSequence(from string, to string, depthes []*models.Depth, targetQuantity float64) *models.Sequence {
	seqes := newSequences(from, to, depthes)

	if len(seqes) == 0 {
		return nil
	}

	maxScore := 0.0
	var seqOfMaxScore *models.Sequence
	for _, seq := range seqes {
		score := trader.scoreOfSequence(seq, targetQuantity)
		if score > 0.0001 && score > maxScore {
			maxScore = score
			seqOfMaxScore = seq
		}
	}

	return seqOfMaxScore
}

func newSequences(from string, to string, depthes []*models.Depth) []*models.Sequence {
	sequences := []*models.Sequence{}
	for i, depth := range depthes {

		if depth.QuoteAsset == from && depth.BaseAsset == to {
			sequences = append(sequences, &models.Sequence{
				Symbol:   depth.Symbol,
				Side:     models.SideBuy,
				From:     from,
				To:       to,
				Price:    depth.AskPrice,
				Quantity: depth.AskQty,
				Src:      depth,
			})
		} else if depth.QuoteAsset == to && depth.BaseAsset == from {
			sequences = append(sequences, &models.Sequence{
				Symbol:   depth.Symbol,
				Side:     models.SideSell,
				From:     from,
				To:       to,
				Price:    depth.BidPrice,
				Quantity: depth.BidQty,
				Src:      depth,
			})
		} else if depth.QuoteAsset == from || depth.BaseAsset == from {
			var nextFrom string
			var side models.OrderSide
			if depth.QuoteAsset == from {
				nextFrom = depth.BaseAsset
				side = models.SideBuy
			} else {
				nextFrom = depth.QuoteAsset
				side = models.SideSell
			}
			for _, next := range newSequences(nextFrom, to, util.Delete(depthes, i)) {
				if depth.Symbol.Equal(next.Symbol) {
					continue
				}
				sequences = append(sequences, &models.Sequence{
					Symbol:   depth.Symbol,
					Side:     side,
					From:     from,
					To:       to,
					Price:    depth.AskPrice,
					Quantity: depth.AskQty,
					Src:      depth,
					Next:     next,
				})
			}
		} else if depth.QuoteAsset == to || depth.BaseAsset == to {
			var prevTo string
			var side models.OrderSide
			if depth.QuoteAsset == to {
				prevTo = depth.QuoteAsset
				side = models.SideSell
			} else {
				prevTo = depth.BaseAsset
				side = models.SideBuy
			}

			seq := &models.Sequence{
				Symbol:   depth.Symbol,
				Side:     side,
				From:     from,
				To:       to,
				Price:    depth.BidPrice,
				Quantity: depth.BidQty,
				Src:      depth,
			}
			for _, previous := range newSequences(from, prevTo, util.Delete(depthes, i)) {
				p := previous
				for {
					if p.Next != nil {
						p = p.Next
						continue
					}

					if depth.Symbol.Equal(p.Symbol) {
						break
					}

					p.Next = seq
					seq.From = p.To
					break
				}
				sequences = append(sequences, previous)
			}
		}
	}

	return sequences
}

func (trader *Trader) scoreOfSequence(sequence *models.Sequence, targetQuantity float64) float64 {
	from := sequence.From
	balance := trader.GetBalance(from).Free
	fee := trader.Exchange.GetFee()

	s := sequence
	currentQuantity := balance
	currentAsset := from

	log.Debug("--------------------------------------------")
	log.Debugf("%s:%f", currentAsset, currentQuantity)
	for {
		s.Target = targetQuantity

		currentQuantity *= (1 - fee)

		if s.Side == models.SideBuy {
			log.Debugf(" %s, BUY, %f -> ", s.Symbol, s.Price)
			currentAsset = s.Symbol.BaseAsset
			currentQuantity = util.Floor(currentQuantity/s.Price, s.Symbol.StepSize)
		} else {
			log.Debugf(" %s, SELL, %f -> ", s.Symbol, s.Price)
			currentAsset = s.Symbol.QuoteAsset
			currentQuantity = util.Floor(currentQuantity, s.Symbol.StepSize) * s.Price
		}
		log.Debugf("%s:%f", currentAsset, currentQuantity)

		if s.Next == nil {
			break
		}

		s = s.Next
	}
	log.Debugf("Rate : %f", (currentQuantity-targetQuantity)/targetQuantity)
	log.Debug("--------------------------------------------")

	return (currentQuantity - targetQuantity) / targetQuantity
}

func (trader *Trader) PrintSequence(seq *models.Sequence) {
	seqString := ""
	s := seq
	for {
		seqString += s.From
		if s.Next != nil {
			s = s.Next
			seqString += " -> "
			continue
		}
		seqString += " -> " + s.To
		break
	}
	log.Infof("Sequence : %s", seqString)
}
