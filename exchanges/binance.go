package exchanges

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	mm "github.com/qinix/cckit/market_monitor"

	"golang.org/x/time/rate"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

const maxUrlLength = 2048

type BinanceImp struct {
	monitor                 *mm.Exchange
	conns                   []*websocket.Conn
	marketToConn            map[market]*websocket.Conn
	symbolToMarket          map[string]market
	marketReloadTicker      *time.Ticker
	rwMutex                 sync.RWMutex
	marketDepthInitialized  map[market]bool // nil => snapshot loader not started; false => snapshot loader started; true => snapshot loaded
	marketDepthUpdateBuffer map[market][]mm.PriceLevel
	binanceApiRateLimiter   *rate.Limiter
}

var Binance = BinanceImp{
	monitor:                 mm.NewExchange(),
	marketToConn:            make(map[market]*websocket.Conn),
	symbolToMarket:          make(map[string]market),
	marketDepthInitialized:  make(map[market]bool),
	marketDepthUpdateBuffer: make(map[market][]mm.PriceLevel),
	binanceApiRateLimiter:   rate.NewLimiter(19, 20), // Binance allows 1200 request weights per minute, which equals to 20 request weights per second
}

func init() {
	Register("binance", &Binance)
}

type market struct {
	Symbol string
	Status string
	Base   string `json:"baseAsset"`
	Quote  string `json:"quoteAsset"`
}

type streamType string

const (
	streamTypeAggTrade    streamType = "aggTrade"
	streamTypeDepthUpdate streamType = "depthUpdate"
)

type streamAggTradeEvent struct {
	StreamType streamType `json:"e"`
	Time       int64      `json:"E"`
	Symbol     string     `json:"s"`

	Price        string      `json:"p"`
	Quantity     string      `json:"q"`
	IsBuyerMaker bool        `json:"m"`
	Ignore       interface{} `json:"M"`
}

type streamDepthUpdateEvent struct {
	StreamType streamType `json:"e"`
	Time       int64      `json:"E"`
	Symbol     string     `json:"s"`

	FirstUpdateID uint64      `json:"U"`
	FinalUpdateID uint64      `json:"u"`
	Bids          [][2]string `json:"b"`
	Asks          [][2]string `json:"a"`
}

type streamEvent struct {
	Stream string
	Data   json.RawMessage
}

func (b *BinanceImp) Monitor() *mm.Exchange {
	return b.monitor
}

func (b *BinanceImp) Start() {
	b.marketReloadTicker = time.NewTicker(10 * time.Minute)

	go func() {
		b.loadNewMarkets()
		for {
			<-b.marketReloadTicker.C
			b.loadNewMarkets()
		}
	}()
}

func (b *BinanceImp) loadNewMarkets() {
	var pendingMarkets []market
	for _, market := range b.markets() {
		if _, found := b.marketToConn[market]; !found {
			pendingMarkets = append(pendingMarkets, market)
		}
	}

	b.startMarkets(pendingMarkets)
}

func (b *BinanceImp) startMarkets(markets []market) {
	b.rwMutex.Lock()
	defer b.rwMutex.Unlock()

	const streamBase string = "wss://stream.binance.com:9443/stream?streams="
	streams := ""
	chunkedMarkets := []market{}
	for _, m := range markets {
		b.monitor.NewMarket(m.Base, m.Quote)
		b.symbolToMarket[m.Symbol] = m
		newStream := strings.ToLower(m.Symbol) + "@aggTrade/" + strings.ToLower(m.Symbol) + "@depth"

		if len(streamBase+streams+"/"+newStream) > maxUrlLength {
			b.dialWS(streamBase+streams, chunkedMarkets)
			streams = ""
			chunkedMarkets = []market{}
		}

		if streams != "" {
			streams += "/" + newStream
		} else {
			streams = newStream
		}
		chunkedMarkets = append(chunkedMarkets, m)
	}

	if streams != "" {
		b.dialWS(streamBase+streams, chunkedMarkets)
	}
}

func (b *BinanceImp) dialWS(url string, markets []market) *websocket.Conn {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("dial: ", err)
	}

	b.conns = append(b.conns, conn)
	for _, market := range markets {
		b.marketToConn[market] = conn
	}

	go func() {
		for {
			var event streamEvent
			err := conn.ReadJSON(&event)
			if err != nil {
				log.Error(err)
			}

			if strings.HasSuffix(event.Stream, "aggTrade") {
				var aggTradeEvent streamAggTradeEvent
				err = json.Unmarshal(event.Data, &aggTradeEvent)
				if err != nil {
					log.Error(err)
				}
				b.onEvent(aggTradeEvent)
			} else if strings.HasSuffix(event.Stream, "depth") {
				var depthUpdateEvent streamDepthUpdateEvent
				err = json.Unmarshal(event.Data, &depthUpdateEvent)
				if err != nil {
					log.Error(err)
				}
				b.onEvent(depthUpdateEvent)
			}
		}
	}()

	return conn
}

func (b *BinanceImp) markets() (markets []market) {
	err := b.binanceApiRateLimiter.WaitN(context.Background(), 1)
	if err != nil {
		log.Error(err)
	}

	res, err := http.Get("https://api.binance.com/api/v1/exchangeInfo")
	if err != nil {
		log.Error(err)
		return
	}

	var exchangeInfo struct {
		Symbols []market
	}

	err = json.NewDecoder(res.Body).Decode(&exchangeInfo)
	if err != nil {
		log.Error(err)
		return
	}

	for _, symbol := range exchangeInfo.Symbols {
		if symbol.Status == "TRADING" {
			markets = append(markets, symbol)
		}
	}

	return
}

func (b *BinanceImp) loadMarketDepthSnapshot(m market) {
	err := b.binanceApiRateLimiter.WaitN(context.Background(), 10)
	if err != nil {
		log.Error(err)
	}

	res, err := http.Get("https://api.binance.com/api/v1/depth?symbol=" + m.Symbol + "&limit=1000")
	if err != nil {
		log.Error(res)
		log.Fatal(err)
		return
	}

	var depthResponse struct {
		Bids [][2]string
		Asks [][2]string
	}

	err = json.NewDecoder(res.Body).Decode(&depthResponse)
	if err != nil {
		log.Fatal(err)
		return
	}

	var pls []mm.PriceLevel
	for _, bid := range depthResponse.Bids {
		pls = append(pls, parseBinancePriceLevel(bid, true))
	}
	for _, ask := range depthResponse.Asks {
		pls = append(pls, parseBinancePriceLevel(ask, false))
	}

	b.rwMutex.Lock()
	defer b.rwMutex.Unlock()

	b.monitor.Market(m.Base, m.Quote).Depth.Merge(pls)
	if buffer, found := b.marketDepthUpdateBuffer[m]; found {
		b.monitor.Market(m.Base, m.Quote).Depth.Merge(buffer)
		delete(b.marketDepthUpdateBuffer, m)
		b.marketDepthInitialized[m] = true
	}
}

func (b *BinanceImp) onEvent(event interface{}) {
	switch e := event.(type) {
	case streamAggTradeEvent:
		b.rwMutex.RLock()
		m := b.symbolToMarket[e.Symbol]
		b.rwMutex.RUnlock()

		b.monitor.Market(m.Base, m.Quote).NewTrade(mm.Trade{Time: time.Unix(e.Time/1000, e.Time%1000*1000), Price: decimal.RequireFromString(e.Price), Amount: decimal.RequireFromString(e.Quantity), IsBuyerMaker: e.IsBuyerMaker})
	case streamDepthUpdateEvent:
		var pls []mm.PriceLevel

		for _, bid := range e.Bids {
			pls = append(pls, parseBinancePriceLevel(bid, true))
		}

		for _, ask := range e.Asks {
			pls = append(pls, parseBinancePriceLevel(ask, false))
		}

		b.rwMutex.Lock()
		defer b.rwMutex.Unlock()

		m := b.symbolToMarket[e.Symbol]

		if b.marketDepthInitialized[m] {
			b.monitor.Market(m.Base, m.Quote).Depth.Merge(pls)
		} else {
			if _, found := b.marketDepthInitialized[m]; !found {
				go b.loadMarketDepthSnapshot(m)
				b.marketDepthInitialized[m] = false
			}

			currentBuffer, found := b.marketDepthUpdateBuffer[m]
			if !found {
				currentBuffer = make([]mm.PriceLevel, 0)
			}
			b.marketDepthUpdateBuffer[m] = append(currentBuffer, pls...)
		}

	}
}

func parseBinancePriceLevel(bpl [2]string, isBid bool) mm.PriceLevel {
	price := decimal.RequireFromString(bpl[0])
	amount := decimal.RequireFromString(bpl[1])
	return mm.PriceLevel{
		Price:  price,
		Amount: amount,
		IsBid:  isBid,
	}
}
