package usecase

import (
	"os"
	"os/signal"
	"syscall"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

type TradeStatus string

const (
	TradeWaiting = TradeStatus("Waiting")
	TradeRunning = TradeStatus("Running")
)

type Trader struct {
	Exchange   Exchange
	MainAsset  models.Asset
	cache      *util.DepthCache
	balances   []*models.Balance
	serverHost *string
	tradeChan  *chan *models.Sequence
	depthChan  *chan *models.Depth
}

func NewTrader(ex Exchange, mainAsset models.Asset, serverHost *string) *Trader {
	return &Trader{
		Exchange:   ex,
		MainAsset:  mainAsset,
		cache:      util.NewDepthCache(),
		balances:   []*models.Balance{},
		serverHost: serverHost,
	}
}

func (trader *Trader) Run() {
	log.Info("Starting Trader ....")

	trader.LoadBalances()

	log.Info("----------------- params -----------------")
	log.Info(" Main asset         : ", trader.MainAsset)
	log.Info(" Main asset balance : ", trader.GetBalance(trader.MainAsset).Free)
	log.Info(" Exchange fee       : ", trader.Exchange.GetFee())
	log.Info("------------------------------------------")

	go trader.runTrader()
	go trader.runAnalyzer()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)
	for {
		select {
		case kill := <-interrupt:
			log.Info("Got signal : ", kill)
			log.Info("Stopping trader")
			return
		}
	}
}
