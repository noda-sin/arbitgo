package usecase

import (
	"encoding/json"
	"net/url"

	"github.com/OopsMouse/arbitgo/models"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func (trader *Trader) initDepthChan() {
	var depthChan chan *models.Depth
	if trader.serverHost == nil || *trader.serverHost == "" {
		depthChan = trader.Exchange.GetDepthOnUpdate()
	} else {
		depthChan = depthServerChannel(trader.serverHost)
	}
	trader.depthChan = &depthChan
}

func (trader *Trader) newDepth() *models.Depth {
	if trader.depthChan == nil {
		trader.initDepthChan()
	}
	depth := <-*trader.depthChan
	trader.cache.Set(depth)
	return depth
}

func (trade *Trader) relationalDepthes(symbol models.Symbol) []*models.Depth {
	base := symbol.BaseAsset
	quote := symbol.QuoteAsset
	all := trade.cache.GetAll()
	ret := []*models.Depth{}

	for _, i := range all { // BaseAssetが同じDepthのみ取得
		if base.Equal(i.BaseAsset) {
			ret = append(ret, i)
		}
	}
	for _, a := range ret {
		for _, b := range all {
			if (b.BaseAsset.Equal(quote) &&
				b.QuoteAsset.Equal(a.QuoteAsset)) ||
				(b.QuoteAsset.Equal(quote) &&
					b.BaseAsset.Equal(a.QuoteAsset)) {
				ret = append(ret, b)
			}
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
