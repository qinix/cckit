package exchanges

import (
	mm "github.com/qinix/cckit/market_monitor"
)

var CBX = PxnImp{
	monitor:        mm.NewExchange(),
	domain:         "www.cbx.one",
	symbolToMarket: make(map[string]pxnMarket),
}

func init() {
	Register("cbx", &CBX)
}
