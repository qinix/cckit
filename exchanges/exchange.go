package exchanges

import (
	mm "github.com/qinix/cckit/market_monitor"
)

type Exchange interface {
	Start()
	Monitor() *mm.Exchange
}

var exchanges = make(map[string]Exchange)

func Register(id string, ex Exchange) {
	exchanges[id] = ex
}

func Find(id string) (Exchange, bool) {
	ex, found := exchanges[id]
	return ex, found
}
