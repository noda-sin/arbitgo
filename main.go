package main

import (
	"os"

	"github.com/OopsMouse/arbitgo/database"
	"github.com/OopsMouse/arbitgo/infrastructure"
	"github.com/OopsMouse/arbitgo/usecase"
)

func main() {
	exchange := infrastructure.NewExchange(
		os.Getenv("EXCHANGE_APIKEY"),
		os.Getenv("EXCHANGE_SECRET"),
	)
	anlyzr := usecase.MarketAnalyzer{}
	repo := database.NewMarketRepository(exchange)
	// ch := make(chan *models.Market)
	// _ = repo.UpdatedMarket(ch)
	// for {
	// 	fmt.Println(<-ch)
	// }
	trader := usecase.Arbitrader{
		MarketRepository: repo,
		MarketAnalyzer:   anlyzr,
	}
	trader.Run()
	// for {

	// }
}
