package usecase

import (
	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) runAnalyzer(depch chan *models.Depth, seqch chan *models.Sequence) {
	for {
		depth := <-depch
		bigAssets := trader.BigAssets()
		renewAsset := depth.BaseAsset

		assets := []string{}
		for _, a := range bigAssets {
			if !trader.isRunningPosition(a) {
				assets = append(assets, a)
			}
		}

		for _, a := range assets {
			depthes := trader.getDepthes(a, renewAsset)
			balance := trader.GetBalance(a).Free

			func(asset string, depthes []*models.Depth, balance float64) {
				seq := trader.bestOfSequence(asset, asset, depthes, balance)

				if seq == nil {
					return
				}

				seqch <- seq
			}(a, depthes, balance)
		}
	}
}

func (trader *Trader) bestOfSequence(from string, to string, depthes []*models.Depth, targetQuantity float64) *models.Sequence {

	symbols := []string{}
	for _, s := range depthes {
		symbols = append(symbols, s.Symbol.String())
	}

	log.Debug("Symboles : ", symbols)

	seqes := unifySequences(newSequences(from, to, depthes))

	log.Debug("Sequences Count : ", len(seqes))

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

const MAX_SEQUENCE_SIZE = 4

func newSequences(from string, to string, depthes []*models.Depth) []*models.Sequence {
	return _newSequences(from, to, depthes, 1)
}

func _newSequences(from string, to string, depthes []*models.Depth, seqDepth int) []*models.Sequence {
	sequences := []*models.Sequence{}

	if seqDepth > MAX_SEQUENCE_SIZE {
		return sequences
	}

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
		} else {
			if depth.QuoteAsset == from || depth.BaseAsset == from {
				var nextFrom string
				var side models.OrderSide
				var price float64
				var quantity float64
				if depth.QuoteAsset == from {
					nextFrom = depth.BaseAsset
					side = models.SideBuy
					price = depth.AskPrice
					quantity = depth.AskQty
				} else {
					nextFrom = depth.QuoteAsset
					side = models.SideSell
					price = depth.BidPrice
					quantity = depth.BidQty
				}
				for _, next := range _newSequences(nextFrom, to, util.Delete(depthes, i), seqDepth+1) {
					if depth.Symbol.Equal(next.Symbol) {
						continue
					}
					sequences = append(sequences, &models.Sequence{
						Symbol:   depth.Symbol,
						Side:     side,
						From:     from,
						To:       to,
						Price:    price,
						Quantity: quantity,
						Src:      depth,
						Next:     next,
					})
				}
			}
		}
	}

	return sequences
}

func unifySequences(seqes []*models.Sequence) []*models.Sequence {
	unifySeqes := []*models.Sequence{}
	seqStrings := []string{}
	for _, seq := range seqes {
		s := seq
		seqString := ""
		for {
			seqString += s.Symbol.String()
			if s.Next != nil {
				s = s.Next
				continue
			}
			break
		}
		if util.Include(seqStrings, seqString) {
			continue
		}
		seqStrings = append(seqStrings, seqString)
		unifySeqes = append(unifySeqes, seq)
	}
	return unifySeqes
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
