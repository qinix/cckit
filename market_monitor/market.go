package market_monitor

import (
	"time"

	"github.com/shopspring/decimal"
)

type Market struct {
	BaseSymbol, QuoteSymbol string

	Depth    Depth
	Trades   TradesContainer
	Exchange *Exchange
}

func newMarket(base, quote string, tradesExpiration time.Duration, exchange *Exchange) *Market {
	depth := newDepth([]PriceLevel{})
	tc := newTradesContainer(tradesExpiration)
	return &Market{
		BaseSymbol:  base,
		QuoteSymbol: quote,
		Depth:       depth,
		Trades:      tc,
		Exchange:    exchange,
	}
}

func (m *Market) Price() decimal.Decimal {
	return m.Depth.MidPrice()
}

func (m *Market) MoneyFlow(from, to time.Time) (inflow, outflow decimal.Decimal) {
	trades := m.Trades.ListTrades(from, to)
	for _, trade := range trades {
		if !trade.IsBuyerMaker {
			inflow = inflow.Add(trade.Price.Mul(trade.Amount))
		} else {
			outflow = outflow.Add(trade.Price.Mul(trade.Amount))
		}
	}
	return
}

func (m *Market) NewTrade(t Trade) {
	if quotePrice := m.Exchange.PriceInUSD(m.QuoteSymbol); quotePrice != nil {
		t.turnoverInUSD = t.Price.Mul(t.Amount).Mul(*quotePrice)
	}
	m.Trades.newTrade(t)
}
