package util

import (
	"path"
	"runtime"
	"strconv"
	"time"

	"github.com/OopsMouse/arbitgo/models"
	"github.com/jpillora/backoff"
	log "github.com/sirupsen/logrus"
)

func Index(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

func Include(vs []string, t string) bool {
	return Index(vs, t) >= 0
}

func Any(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if f(v) {
			return true
		}
	}
	return false
}

func All(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if !f(v) {
			return false
		}
	}
	return true
}

func Filter(vs []string, f func(string) bool) []string {
	vsf := make([]string, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func Map(vs []string, f func(string) string) []string {
	vsm := make([]string, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

type Set struct {
	buff map[string]struct{}
}

func NewSet() *Set {
	return &Set{buff: map[string]struct{}{}}
}

func (s *Set) Append(i string) {
	s.buff[i] = struct{}{}
}

func (s *Set) Include(i string) bool {
	return Include(s.ToSlice(), i)
}

func (s *Set) ToSlice() []string {
	keys := make([]string, 0, len(s.buff))
	for k := range s.buff {
		keys = append(keys, k)
	}
	return keys
}

func GetCurrentFile() string {
	_, filename, _, _ := runtime.Caller(1)
	return filename
}

func GetCurrentDir() string {
	_, filename, _, _ := runtime.Caller(1)
	return path.Dir(filename)
}

type Operation func() error

func BackoffRetry(retry int, op Operation) error {
	b := &backoff.Backoff{
		Max: 5 * time.Minute,
	}
	var err error
	for i := 0; i < retry; i++ {
		err = op()
		if err == nil {
			return nil
		}
		d := b.Duration()
		time.Sleep(d)
	}
	return err
}

func Floor(a float64, b float64) float64 {
	return float64(int(a/b)) * b
}

func LogOrder(order models.Order) {
	log.Info("----------------- orders #" + strconv.Itoa(order.Step) + " -------------------")
	log.Info(" OrderID  : ", order.ClientOrderID)
	log.Info(" Symbol   : ", order.Symbol.String())
	log.Info(" Side     : ", order.Side)
	log.Info(" Type     : ", order.OrderType)
	log.Info(" Price    : ", order.Price)
	log.Info(" Quantity : ", order.Qty)

	if order.SourceDepth != nil {
		log.Info(" Time     : ", time.Now().Sub(order.SourceDepth.Time), " Ago")
	}

	log.Info("-----------------------------------------------")
}

func LogOrders(orders []models.Order) {
	for _, order := range orders {
		LogOrder(order)
	}
}
