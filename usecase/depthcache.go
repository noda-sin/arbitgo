package usecase

import (
	"sync"

	"github.com/OopsMouse/arbitgo/models"
)

type DepthCache struct {
	cache map[string]*models.Depth
	lock  *sync.Mutex
}

func NewDepthCache() *DepthCache {
	return &DepthCache{
		cache: map[string]*models.Depth{},
		lock:  new(sync.Mutex),
	}
}

func (c *DepthCache) Set(depth *models.Depth) {
	defer c.lock.Unlock()
	c.lock.Lock()
	c.cache[depth.Symbol.String()] = depth
}

func (c *DepthCache) Get(symbol models.Symbol) *models.Depth {
	defer c.lock.Unlock()
	c.lock.Lock()
	return c.cache[symbol.String()]
}

func (c *DepthCache) GetAll() []*models.Depth {
	defer c.lock.Unlock()
	c.lock.Lock()
	depthList := []*models.Depth{}
	for _, v := range c.cache {
		depthList = append(depthList, v)
	}
	return depthList
}
