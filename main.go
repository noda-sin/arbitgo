package main

import (
	"os"

	"github.com/OopsMouse/arbitgo/infrastructure"
	"github.com/OopsMouse/arbitgo/usecase"
)

func main() {
	exchange := infrastructure.NewExchange(
		os.Getenv("EXCHANGE_APIKEY"),
		os.Getenv("EXCHANGE_SECRET"),
	)
	anlyzr := usecase.MarketAnalyzer{}
	trader := usecase.Arbitrader{
		Exchange:       exchange,
		MarketAnalyzer: anlyzr,
		DryRun:         true,
	}
	trader.Run()
}
