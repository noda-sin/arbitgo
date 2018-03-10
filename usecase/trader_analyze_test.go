package usecase

import (
	"testing"

	"github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
)

func TestDeleteDepthes(t *testing.T) {
	symbols := [][]string{
		{"XRP", "BTC"},
		{"XRP", "BNB"},
		{"BNB", "BTC"},
	}
	depthes := createDepthes(symbols)
	ret := util.Delete(depthes, 0)
	if len(ret) != 2 || len(depthes) != 3 {
		t.Fatal("test failed")
	}

	ret = util.Delete(depthes, 1)
	if len(ret) != 2 || len(depthes) != 3 {
		t.Fatal("test failed")
	}

	ret = util.Delete(depthes, 2)
	if len(ret) != 2 || len(depthes) != 3 {
		t.Fatal("test failed")
	}
}

func TestNewSequence1(t *testing.T) {
	symbols := [][]string{
		{"BNB", "BTC"},
	}
	seqes := newSequences("BNB", "BTC", createDepthes(symbols))

	if len(seqes) != 1 {
		t.Fatal("test failed")
	}

	if seqes[0].From != "BNB" || seqes[0].To != "BTC" {
		t.Fatal("test failed")
	}
}

func TestNewSequence2(t *testing.T) {
	symbols := [][]string{
		{"XRP", "BTC"},
		{"XRP", "BNB"},
		{"BNB", "BTC"},
	}
	seqes := newSequences("BTC", "BTC", createDepthes(symbols))

	if len(seqes) != 2 {
		t.Fatal("test failed")
	}
}

func description(seq *models.Sequence) string {
	desc := ""
	s := seq
	for {
		desc += s.Symbol.String()
		if s.Next != nil {
			s = s.Next
			desc += ","
			continue
		}
		break
	}
	return desc
}

func createDepthes(symbols [][]string) []*models.Depth {
	depthes := []*models.Depth{}
	for _, symbol := range symbols {
		depthes = append(depthes, &models.Depth{
			BaseAsset:  symbol[0],
			QuoteAsset: symbol[1],
			Symbol: models.Symbol{
				Text:       symbol[0] + symbol[1],
				BaseAsset:  symbol[0],
				QuoteAsset: symbol[1],
			},
		})
	}
	return depthes
}
