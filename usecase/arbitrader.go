package usecase

import (
	"encoding/json"
	"net/url"
	"sync"

	models "github.com/OopsMouse/arbitgo/models"
	"github.com/gorilla/websocket"
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
	ServerHost *string
}

func NewArbitrader(ex Exchange, ma MarketAnalyzer, mainAsset models.Asset, serverHost *string) *Arbitrader {
	return &Arbitrader{
		Exchange:       ex,
		MarketAnalyzer: ma,
		MainAsset:      mainAsset,
		Status:         TradeWaiting,
		StatusLock:     new(sync.Mutex),
		Cache:          NewDepthCache(),
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

	var dch chan *models.Depth
	if arbit.ServerHost == nil || *arbit.ServerHost == "" {
		dch = arbit.Exchange.GetDepthOnUpdate()
	} else {
		dch = depthChannel(arbit.ServerHost)
	}

	for {
		depth := <-dch
		arbit.Cache.Set(depth)
		go arbit.Analyze(arbit.Cache.GetRelevantDepthes(depth))
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
