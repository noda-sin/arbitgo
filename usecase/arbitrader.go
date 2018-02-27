package usecase

import (
	"sync"

	models "github.com/OopsMouse/arbitgo/models"
	log "github.com/sirupsen/logrus"
)

type TradeStatus string

const (
	TradeWaiting = TradeStatus("Waiting")
	TradeRunning = TradeStatus("Running")
)

type Arbitrader struct {
	Exchange
	MarketAnalyzer
	MainAsset  models.Asset
	Status     TradeStatus
	StatusLock *sync.Mutex
	Cache      *DepthCache
	Balances   []*models.Balance
}

func NewArbitrader(ex Exchange, ma MarketAnalyzer, mainAsset models.Asset) *Arbitrader {
	return &Arbitrader{
		Exchange:       ex,
		MarketAnalyzer: ma,
		MainAsset:      mainAsset,
		Status:         TradeWaiting,
		StatusLock:     new(sync.Mutex),
		Cache:          NewDepthCache(),
		Balances:       []*models.Balance{},
	}
}

func logInit() {
	format := &log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	}
	log.SetFormatter(format)
}

func (arbit *Arbitrader) Run() {
	logInit()
	log.Info("Starting Arbitrader ....")

	arbit.LoadBalances()

	log.Info("----------------- params -----------------")
	log.Info(" Main asset         : ", arbit.MainAsset)
	log.Info(" Main asset balance : ", arbit.GetBalance(arbit.MainAsset).Free)
	log.Info(" Exchange charge    : ", arbit.MarketAnalyzer.Charge)
	log.Info(" Threshold          : ", arbit.MarketAnalyzer.Threshold)
	log.Info("------------------------------------------")

	dch := arbit.Exchange.GetDepthOnUpdate()

	for {
		depth := <-dch
		arbit.Cache.Set(depth)
		go arbit.Analyze(arbit.Cache.GetRelevantDepthes(depth))
	}
}
