package usecase

import (
	"encoding/json"
	"net/url"
	"sync"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type TradeStatus string

const (
	TradeWaiting = TradeStatus("Waiting")
	TradeRunning = TradeStatus("Running")
)

type Trader struct {
	Exchange
	MarketAnalyzer
	MainAsset  models.Asset
	Status     TradeStatus
	StatusLock *sync.Mutex
	Cache      *util.DepthCache
	Balances   []*models.Balance
	ServerHost *string
}

func NewTrader(ex Exchange, mainAsset models.Asset, serverHost *string) *Trader {
	analyzer := MarketAnalyzer{
		MainAsset: mainAsset,
		Charge:    ex.GetCharge(),
	}

	return &Trader{
		Exchange:       ex,
		MarketAnalyzer: analyzer,
		MainAsset:      mainAsset,
		Status:         TradeWaiting,
		StatusLock:     new(sync.Mutex),
		Cache:          util.NewDepthCache(),
		Balances:       []*models.Balance{},
		ServerHost:     serverHost,
	}
}

func logInit() {
	format := &log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	}
	log.SetFormatter(format)
}

func (trader *Trader) Run() {
	logInit()
	log.Info("Starting Trader ....")

	trader.LoadBalances()

	log.Info("----------------- params -----------------")
	log.Info(" Main asset         : ", trader.MainAsset)
	log.Info(" Main asset balance : ", trader.GetBalance(trader.MainAsset).Free)
	log.Info(" Exchange charge    : ", trader.MarketAnalyzer.Charge)
	log.Info("------------------------------------------")

	var dch chan *models.Depth
	if trader.ServerHost == nil || *trader.ServerHost == "" {
		dch = trader.Exchange.GetDepthOnUpdate()
	} else {
		dch = depthChannel(trader.ServerHost)
	}

	for {
		depth := <-dch
		trader.Cache.Set(depth)
		go trader.Analyze(trader.Cache.GetRelevantDepthes(depth))
	}
}

func depthChannel(host *string) chan *models.Depth {
	dch := make(chan *models.Depth)
	u := url.URL{Scheme: "ws", Host: *host, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	go func() {
		defer close(dch)
		for {
			c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				for {
					_, bytes, err := c.ReadMessage()
					if err != nil {
						return
					}
					var depth *models.Depth
					err = json.Unmarshal(bytes, &depth)
					if err != nil {
						continue
					}
					dch <- depth
				}
			}()
		}
	}()

	return dch
}
