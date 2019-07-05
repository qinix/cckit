package market_monitor

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

const TradesExpiration time.Duration = 24 * time.Hour

type Exchange struct {
	markets map[string]map[string]*Market
	rwMutex sync.RWMutex
}

func NewExchange() *Exchange {
	return &Exchange{
		markets: make(map[string]map[string]*Market),
	}
}

func (e *Exchange) NewMarket(base, quote string) *Market {
	e.rwMutex.Lock()
	defer e.rwMutex.Unlock()

	market := newMarket(base, quote, TradesExpiration, e)

	basedMarkets, found := e.markets[base]
	if !found {
		basedMarkets = make(map[string]*Market)
		e.markets[base] = basedMarkets
	}
	basedMarkets[quote] = market
	return market
}

func (e *Exchange) Market(base, quote string) *Market {
	e.rwMutex.RLock()
	defer e.rwMutex.RUnlock()

	basedMarkets, found := e.markets[base]
	if !found {
		return nil
	}

	return basedMarkets[quote]
}

func (e *Exchange) Markets() (ms []*Market) {
	for _, basedMarkets := range e.markets {
		for _, market := range basedMarkets {
			ms = append(ms, market)
		}
	}
	return
}

func (e *Exchange) DepthValue(base, quote string, rng float64) (amount decimal.Decimal) {
	priceInUSD := e.PriceInUSD(base)
	if priceInUSD == nil {
		return decimal.Zero
	}

	return e.Market(base, quote).Depth.TotalAmount(rng).Mul(*priceInUSD)
}

func (e *Exchange) PriceInUSD(symbol string) (price *decimal.Decimal) {
	if symbol == "USD" || symbol == "USDT" {
		one := decimal.New(1, 0)
		return &one
	}

	market := e.Market(symbol, "USD")
	if market == nil {
		market = e.Market(symbol, "USDT")
	}
	if market != nil {
		p := market.Price()
		price = &p
		return
	}

	for _, interSymbol := range []string{"BTC", "ETH", "EOS"} {
		price = e.indirectPrice(symbol, interSymbol)
		if price != nil {
			return
		}
	}
	return
}

func (e *Exchange) indirectPrice(symbol, interSymbol string) *decimal.Decimal {
	if symbol == interSymbol {
		return nil
	}

	market := e.Market(symbol, interSymbol)
	if market != nil {
		interPrice := e.PriceInUSD(interSymbol)
		if interPrice != nil {
			p := market.Price().Mul(*interPrice)
			return &p
		}
	}

	return nil
}
