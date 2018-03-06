package usecase

import (
	"os"
	"os/signal"
	"syscall"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	log "github.com/sirupsen/logrus"
)

type Trader struct {
	Exchange   Exchange
	cache      *util.DepthCache
	balances   []*models.Balance
	serverHost *string
	positions  *util.Set
	seqch      *chan *models.Sequence
	depch      *chan *models.Depth
}

func NewTrader(ex Exchange, serverHost *string) *Trader {
	return &Trader{
		Exchange:   ex,
		cache:      util.NewDepthCache(),
		balances:   []*models.Balance{},
		positions:  util.NewSet(),
		serverHost: serverHost,
	}
}

func (trader *Trader) Run() {
	log.Info("Starting Trader ....")

	trader.PrintBalanceOfBigAssets()

	go trader.depthSubscriber()
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
