package market_monitor

import (
	"sort"
)

func (e *Exchange) MarketsSortedByTurnover() []*Market {
	markets := e.Markets()
	sort.Slice(markets, func(i, j int) bool {
		return markets[i].Trades.TurnoverInUSD().GreaterThan(markets[j].Trades.TurnoverInUSD())
	})
	return markets
}
