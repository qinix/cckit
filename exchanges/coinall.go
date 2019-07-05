package exchanges

import mm "github.com/qinix/cckit/market_monitor"

var Coinall = OkexImp{
	monitor:              mm.NewExchange(),
	instrumentidToMarket: make(map[string]okexMarket),
	baseURL:              "https://www.coinall.com",
}

func init() {
	Register("coinall", &Coinall)
}
