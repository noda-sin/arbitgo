package usecase

import (
	"encoding/json"
	"net/url"

	"github.com/OopsMouse/arbitgo/models"
	"github.com/OopsMouse/arbitgo/util"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) depthSubscriber() {
	var depthChan chan *models.Depth
	if trader.serverHost == nil || *trader.serverHost == "" {
		depthChan = trader.Exchange.GetDepthOnUpdate()
	} else {
		depthChan = depthServerChannel(trader.serverHost)
	}

	for {
		depth := <-depthChan
		trader.cache.Set(depth)
	}
}

func (trader *Trader) getDepthes(asset string) []*models.Depth {
	quotes := trader.Exchange.GetQuotes()
	all := trader.cache.GetAll()
	ret := []*models.Depth{}
	if util.Include(quotes, asset) {
		return all
	}
	for _, i := range all {
		if util.Include(quotes, i.BaseAsset) || asset == i.BaseAsset {
			ret = append(ret, i)
		}
	}
	return ret
}

func depthServerChannel(host *string) chan *models.Depth {
	dch := make(chan *models.Depth)
	u := url.URL{Scheme: "ws", Host: *host, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	go func() {
		defer close(dch)
		for {
			c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				continue
			}
			func() {
				defer c.Close()
				for {
					_, bytes, err := c.ReadMessage()
					if err != nil {
						log.Error(err)
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
