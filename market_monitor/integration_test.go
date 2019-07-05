package market_monitor

// func TestPriceInUSD(t *testing.T) {
// 	e := createExchange()

// 	assert.True(t, e.PriceInUSD("BTC").Equal(decimal.RequireFromString("8000")))
// 	assert.True(t, e.PriceInUSD("BNB").Equal(decimal.RequireFromString("16000")))
// }

// func TestDepthValue(t *testing.T) {
// 	e := createExchange()

// 	assert.True(t, e.DepthValue("BNB", "BTC", 0.01).Equal(decimal.RequireFromString("4800")))
// }

// func createExchange() *Exchange {
// 	e := NewExchange()
// 	e.NewMarket("BTC", "USDT", decimal.RequireFromString("8000"))
// 	e.NewMarket("BNB", "BTC", decimal.RequireFromString("2"))

// 	e.Market("BNB", "BTC").Depth.Merge([]PriceLevel{
// 		PriceLevel{Price: decimal.RequireFromString("1.99"), Amount: decimal.RequireFromString("0.1"), IsBid: true},
// 		PriceLevel{Price: decimal.RequireFromString("1"), Amount: decimal.RequireFromString("10"), IsBid: true},
// 		PriceLevel{Price: decimal.RequireFromString("2.01"), Amount: decimal.RequireFromString("0.2"), IsBid: false},
// 		PriceLevel{Price: decimal.RequireFromString("10"), Amount: decimal.RequireFromString("10"), IsBid: false},
// 	})
// 	return e
// }
