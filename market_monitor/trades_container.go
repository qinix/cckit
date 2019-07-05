package market_monitor

import (
	"sync"
	"time"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
	"github.com/shopspring/decimal"
)

type Trade struct {
	Time          time.Time
	Price, Amount decimal.Decimal
	IsBuyerMaker  bool

	turnoverInUSD decimal.Decimal
}

type TradesContainer struct {
	trades        *rbt.Tree // time.Time => Trade
	rwMutex       sync.RWMutex
	expiration    time.Duration
	cleanedAt     int64
	turnoverInUSD decimal.Decimal
}

func newTradesContainer(expiration time.Duration) TradesContainer {
	timeComparator := func(a, b interface{}) int {
		t1 := a.(time.Time)
		t2 := b.(time.Time)

		if t1.After(t2) {
			return 1
		} else if t1.Before(t2) {
			return -1
		} else {
			return 0
		}
	}

	c := TradesContainer{
		trades:     rbt.NewWith(timeComparator),
		expiration: expiration,
		cleanedAt:  time.Now().Unix(),
	}
	return c
}

func (tc *TradesContainer) ListTrades(from, to time.Time) (trades []Trade) {
	tc.tryClean()

	tc.rwMutex.RLock()
	defer tc.rwMutex.RUnlock()

	node, found := tc.trades.Ceiling(from)
	if found {
		iterator := tc.trades.IteratorAt(node)
		for {
			trade := iterator.Value().(Trade)
			if trade.Time.After(to) {
				break
			}
			trades = append(trades, iterator.Value().(Trade))
			if !iterator.Next() {
				break
			}
		}
	}

	return
}

func (tc *TradesContainer) TurnoverInUSD() decimal.Decimal {
	tc.rwMutex.RLock()
	defer tc.rwMutex.RUnlock()

	return tc.turnoverInUSD
}

func (tc *TradesContainer) tryClean() {
	tc.rwMutex.RLock()
	cleanedAt := tc.cleanedAt
	tc.rwMutex.RUnlock()
	if cleanedAt == time.Now().Unix() {
		return
	}

	tc.clean()
}

func (tc *TradesContainer) clean() {
	tc.rwMutex.Lock()
	defer tc.rwMutex.Unlock()

	if tc.cleanedAt == time.Now().Unix() {
		return
	}

	tc.cleanedAt = time.Now().Unix()
	deadline := time.Now().Add(-tc.expiration)

	for {
		oldestTrade := tc.trades.Left()
		if oldestTrade == nil || oldestTrade.Value.(Trade).Time.After(deadline) {
			break
		}

		tc.turnoverInUSD = tc.turnoverInUSD.Sub(oldestTrade.Value.(Trade).turnoverInUSD)
		tc.trades.Remove(oldestTrade.Key)
	}
}

func (tc *TradesContainer) newTrade(t Trade) {
	tc.tryClean()

	tc.rwMutex.Lock()
	defer tc.rwMutex.Unlock()

	tc.trades.Put(t.Time, t)
	tc.turnoverInUSD = tc.turnoverInUSD.Add(t.turnoverInUSD)
}
