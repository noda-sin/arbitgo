package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) runAnalyzer() {
	for {
		depthes := trader.relationalDepthes(trader.newDepth().Symbol)
		balance := trader.GetBalance(trader.MainAsset).Free
		seq := trader.bestOfSequence(trader.MainAsset, trader.MainAsset, depthes, balance)

		if seq == nil {
			continue
		}

		*trader.tradeChan <- seq
	}
}

func (trader *Trader) bestOfSequence(from models.Asset, to models.Asset, depthes []*models.Depth, targetQuantity float64) *models.Sequence {
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

func newSequences(from models.Asset, to models.Asset, depthes []*models.Depth) []*models.Sequence {
	sequences := []*models.Sequence{}
	for i, depth := range depthes {
		if depth.QuoteAsset.Equal(from) && depth.BaseAsset.Equal(to) {
			sequences = append(sequences, &models.Sequence{
				Symbol:   depth.Symbol,
				Side:     models.SideBuy,
				From:     from,
				To:       to,
				Price:    depth.AskPrice,
				Quantity: depth.AskQty,
				Src:      depth,
			})
		} else if depth.QuoteAsset.Equal(to) && depth.BaseAsset.Equal(from) {
			sequences = append(sequences, &models.Sequence{
				Symbol:   depth.Symbol,
				Side:     models.SideSell,
				From:     from,
				To:       to,
				Price:    depth.BidPrice,
				Quantity: depth.BidQty,
				Src:      depth,
			})
		} else if depth.QuoteAsset.Equal(from) {
			for _, next := range newSequences(depth.BaseAsset, to, util.Delete(depthes, i)) {
				if depth.Symbol.Equal(next.Symbol) {
					continue
				}
				sequences = append(sequences, &models.Sequence{
					Symbol:   depth.Symbol,
					Side:     models.SideBuy,
					From:     from,
					To:       to,
					Price:    depth.AskPrice,
					Quantity: depth.AskQty,
					Src:      depth,
					Next:     next,
				})
			}
		} else if depth.QuoteAsset.Equal(to) {
			seq := &models.Sequence{
				Symbol:   depth.Symbol,
				Side:     models.SideSell,
				From:     from,
				To:       to,
				Price:    depth.BidPrice,
				Quantity: depth.BidQty,
				Src:      depth,
			}
			for _, previous := range newSequences(from, depth.BaseAsset, util.Delete(depthes, i)) {
				if depth.Symbol.Equal(previous.Symbol) {
					continue
				}
				p := previous
				for {
					if p.Next != nil {
						p = p.Next
						continue
					}
					p.Next = seq
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
