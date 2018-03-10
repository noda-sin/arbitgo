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

const Worker = 1

func (trader *Trader) Run() {
	log.Info("Starting Trader ....")

	trader.PrintBalanceOfBigAssets()

	depch := trader.depthSubscriber()
	seqch := trader.runTrader()

	for i := 0; i < Worker; i++ {
		go trader.runAnalyzer(depch, seqch)
	}

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
