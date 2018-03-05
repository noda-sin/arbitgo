package util

import (
	"sync"
	"time"

	"github.com/OopsMouse/arbitgo/models"
)

type DepthCache struct {
	cache      map[string]*models.Depth
	lock       *sync.Mutex
	expireTime time.Duration
}

func NewDepthCache() *DepthCache {
	d := &DepthCache{
		cache:      map[string]*models.Depth{},
		lock:       new(sync.Mutex),
		expireTime: 1 * time.Minute,
	}
	return d
}

func (c *DepthCache) Set(depth *models.Depth) {
	defer c.lock.Unlock()
	c.lock.Lock()
	c.cache[depth.Symbol.String()] = depth
}

func (c *DepthCache) Get(symbol models.Symbol) *models.Depth {
	defer c.lock.Unlock()
	c.lock.Lock()
	depth := c.cache[symbol.String()]
	if time.Now().Sub(depth.Time) < c.expireTime {
		return depth
	}
	return nil
}

func (c *DepthCache) GetAll() []*models.Depth {
	defer c.lock.Unlock()
	c.lock.Lock()
	depthList := []*models.Depth{}
	for _, v := range c.cache {
		if time.Now().Sub(v.Time) < c.expireTime {
			depthList = append(depthList, v)
		}
	}
	return depthList
}
