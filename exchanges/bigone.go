package exchanges

import (
	mm "github.com/qinix/cckit/market_monitor"
)

var BigONE = PxnImp{
	monitor:        mm.NewExchange(),
	domain:         "big.one",
	symbolToMarket: make(map[string]pxnMarket),
}

func init() {
	Register("bigone", &BigONE)
}
