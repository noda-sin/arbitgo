package infrastructure

import (
	"os"
	"testing"

	"github.com/OopsMouse/arbitgo/models"
)

func TestOrder(t *testing.T) {
	ex := NewBinance(os.Getenv("EXCHANGE_APIKEY"), os.Getenv("EXCHANGE_SECRET"))
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

func TestBalances(t *testing.T) {
	ex := NewBinance(os.Getenv("EXCHANGE_APIKEY"), os.Getenv("EXCHANGE_SECRET"))
	balances, err := ex.GetBalances()
	if err != nil {
		t.Fatalf("failed test %v", err)
	}
	for _, b := range balances {
		t.Logf("%s = %f\n", string(b.Asset), b.Total)
	}
}

func TestBalance(t *testing.T) {
	ex := NewBinance(os.Getenv("EXCHANGE_APIKEY"), os.Getenv("EXCHANGE_SECRET"))
	b, err := ex.GetBalance(models.Asset("YOYO"))
	if err != nil {
		t.Fatalf("failed test %v", err)
	}
	t.Logf("%s = %f\n", string(b.Asset), b.Total)
}
