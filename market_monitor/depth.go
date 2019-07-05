package market_monitor

import (
	"sync"

	rbt "github.com/emirpasic/gods/trees/redblacktree"
	"github.com/shopspring/decimal"
)

type PriceLevel struct {
	Price, Amount decimal.Decimal
	IsBid         bool
}

type Depth struct {
	bids *rbt.Tree
	asks *rbt.Tree

	rwMutex sync.RWMutex
}

func newDepth(pls []PriceLevel) Depth {
	bidComparator := func(a, b interface{}) int {
		d1 := a.(decimal.Decimal)
		d2 := b.(decimal.Decimal)

		if d1.Equal(d2) {
			return 0
		} else if d1.GreaterThan(d2) {
			return -1
		} else {
			return 1
		}
	}

	askComparator := func(a, b interface{}) int {
		d1 := a.(decimal.Decimal)
		d2 := b.(decimal.Decimal)
		if d1.Equal(d2) {
			return 0
		} else if d1.GreaterThan(d2) {
			return 1
		} else {
			return -1
		}
	}

	depth := Depth{
		bids: rbt.NewWith(bidComparator),
		asks: rbt.NewWith(askComparator),
	}

	depth.Merge(pls)

	return depth
}

func (d *Depth) TotalAmount(rng float64) (amount decimal.Decimal) {
	d.rwMutex.RLock()
	defer d.rwMutex.RUnlock()

	mid := d.MidPrice()

	lower := mid.Mul(decimal.NewFromFloat(1 - rng))
	upper := mid.Mul(decimal.NewFromFloat(1 + rng))

	for _, value := range d.bids.Values() {
		if value.(PriceLevel).Price.LessThan(lower) {
			break
		}

		amount = amount.Add(value.(PriceLevel).Amount)
	}

	for _, value := range d.asks.Values() {
		if value.(PriceLevel).Price.GreaterThan(upper) {
			break
		}

		amount = amount.Add(value.(PriceLevel).Amount)
	}
	return
}

func (d *Depth) Snapshot() (bids []PriceLevel, asks []PriceLevel) {
	d.rwMutex.RLock()
	defer d.rwMutex.RUnlock()

	for _, value := range d.bids.Values() {
		bid := value.(PriceLevel)
		bids = append(bids, bid)
	}

	for _, value := range d.asks.Values() {
		ask := value.(PriceLevel)
		asks = append(asks, ask)
	}
	return
}

func (d *Depth) set(pl PriceLevel) {
	d.rwMutex.Lock()
	defer d.rwMutex.Unlock()

	var tree *rbt.Tree

	if pl.IsBid {
		tree = d.bids
	} else {
		tree = d.asks
	}

	if pl.Amount.Equal(decimal.Zero) {
		tree.Remove(pl.Price)
	} else {
		tree.Put(pl.Price, pl)
	}
}

func (d *Depth) Merge(pls []PriceLevel) {
	for _, pl := range pls {
		d.set(pl)
	}
}

func (d *Depth) MidPrice() decimal.Decimal {
	d.rwMutex.RLock()
	defer d.rwMutex.RUnlock()

	var bid, ask *decimal.Decimal
	var mid decimal.Decimal
	bidNode := d.bids.Left()
	if bidNode != nil {
		tmpBid := bidNode.Key.(decimal.Decimal)
		bid = &tmpBid
	}
	askNode := d.asks.Left()
	if askNode != nil {
		tmpAsk := askNode.Key.(decimal.Decimal)
		ask = &tmpAsk
	}
	if bid == nil && ask == nil {
		return decimal.Zero
	} else if bid == nil {
		mid = *ask
	} else if ask == nil {
		mid = *bid
	} else {
		mid = decimal.Avg(*bid, *ask)
	}

	return mid
}

func (d *Depth) Spread() float64 {
	d.rwMutex.RLock()
	defer d.rwMutex.RUnlock()

	bidNode, askNode := d.bids.Left(), d.asks.Left()
	if bidNode == nil || askNode == nil {
		return 0.999
	}

	bid, _ := bidNode.Key.(decimal.Decimal).Float64()
	ask, _ := askNode.Key.(decimal.Decimal).Float64()

	return (ask - bid) / bid
}
