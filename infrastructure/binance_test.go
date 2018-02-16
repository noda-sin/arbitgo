package infrastructure

import (
	"os"
	"testing"

	"github.com/OopsMouse/arbitgo/models"
)

func TestOrder(t *testing.T) {
	ex := NewBinance(os.Getenv("EXCHANGE_APIKEY"), os.Getenv("EXCAHNGE_SECRET"))
	order := &models.Order{
		Symbol:    models.Symbol{Text: "ETHBTC"},
		OrderType: models.TypeLimit,
		Side:      models.SideSell,
		Price:     0.091573,
		Qty:       0.025878,
	}
	err := ex.SendOrder(order)
	if err != nil {
		t.Fatalf("failed test %v", err)
	}
}
