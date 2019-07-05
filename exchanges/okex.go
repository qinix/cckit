package exchanges

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/qinix/cckit/utils"

	mm "github.com/qinix/cckit/market_monitor"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"

	log "github.com/sirupsen/logrus"
)

const (
	okexMarketsPerConn = 200
	okexWSUrl          = "wss://real.okex.com:10442/ws/v3"
)

type OkexImp struct {
	monitor              *mm.Exchange
	wsPool               *utils.WebsocketPool
	instrumentidToMarket map[string]okexMarket
	marketReloadTicker   *time.Ticker
	rwMutex              sync.RWMutex
	baseURL              string
}

var OKEx = OkexImp{
	monitor:              mm.NewExchange(),
	instrumentidToMarket: make(map[string]okexMarket),
	baseURL:              "https://www.okex.com",
}

func init() {
	Register("okex", &OKEx)
}

type okexMarket struct {
	InstrumentID string `json:"instrument_id"`
	Base         string `json:"base_currency"`
	Quote        string `json:"quote_currency"`
}

func (o *OkexImp) Monitor() *mm.Exchange {
	return o.monitor
}

func (o *OkexImp) Start() {
	o.marketReloadTicker = time.NewTicker(10 * time.Minute)
	o.wsPool = utils.NewWebsocketPool(func(messageType int, message []byte, err error) {
		(*OkexImp).handleMessage(o, messageType, message, err)
	})

	go func() {
		o.loadNewMarkets()
		for {
			<-o.marketReloadTicker.C
			o.loadNewMarkets()
		}
	}()
}

func (o *OkexImp) handleMessage(messageType int, message []byte, err error) {
	if err != nil {
		log.Fatal("handleMessage: ", err)
	}

	var response struct {
		Event   string
		Channel string
		Table   string
		Action  string // only in depth
		Message string
		Data    []struct {
			InstrumentID string `json:"instrument_id"`
			Timestamp    string

			// trade fields
			TradeID string `json:"trade_id"`
			Price   string
			Side    string
			Size    string

			// depth fields
			Asks     [][3]interface{}
			Bids     [][3]interface{}
			Checksum int64
		}
	}

	switch messageType {
	case websocket.TextMessage:
		if err := json.Unmarshal(message, &response); err != nil {
			log.Error(err)
			return
		}
	case websocket.BinaryMessage:
		if err := json.NewDecoder(flate.NewReader(bytes.NewReader(message))).Decode(&response); err != nil {
			log.Error(err)
			return
		}
	}

	if response.Event == "error" {
		log.Error(response.Message)
		return
	}

	switch response.Table {
	case "spot/trade":
		for _, trade := range response.Data {
			o.rwMutex.RLock()
			m := o.instrumentidToMarket[trade.InstrumentID]
			o.rwMutex.RUnlock()

			t, err := time.Parse(time.RFC3339, trade.Timestamp)
			if err != nil {
				log.Fatal(err)
			}
			o.monitor.Market(m.Base, m.Quote).NewTrade(mm.Trade{Time: t, Price: decimal.RequireFromString(trade.Price), Amount: decimal.RequireFromString(trade.Size), IsBuyerMaker: (trade.Side == "sell")})
		}
	case "spot/depth":
		for _, depth := range response.Data {
			o.rwMutex.RLock()
			m := o.instrumentidToMarket[depth.InstrumentID]
			o.rwMutex.RUnlock()

			var pls []mm.PriceLevel
			for _, bid := range depth.Bids {
				pls = append(pls, mm.PriceLevel{
					Price:  decimal.RequireFromString(bid[0].(string)),
					Amount: decimal.RequireFromString(bid[1].(string)),
					IsBid:  true,
				})
			}
			for _, ask := range depth.Asks {
				pls = append(pls, mm.PriceLevel{
					Price:  decimal.RequireFromString(ask[0].(string)),
					Amount: decimal.RequireFromString(ask[1].(string)),
					IsBid:  false,
				})
			}

			o.monitor.Market(m.Base, m.Quote).Depth.Merge(pls)
		}
	}
}

func (o *OkexImp) loadNewMarkets() {
	o.rwMutex.Lock()
	defer o.rwMutex.Unlock()

	var pendingMarkets []okexMarket
	for _, market := range o.getMarkets() {
		if o.monitor.Market(market.Base, market.Quote) == nil {
			pendingMarkets = append(pendingMarkets, market)
			o.instrumentidToMarket[market.InstrumentID] = market
			o.monitor.NewMarket(market.Base, market.Quote)
		}
	}

	o.subscribeMarkets(pendingMarkets)
}

func (o *OkexImp) subscribeMarkets(ms []okexMarket) {
	var chunkSize = 0
	var chunk []okexMarket

	for _, m := range ms {
		chunk = append(chunk, m)
		chunkSize += 1

		if chunkSize >= okexMarketsPerConn {
			o.subscribeChunkedMarkets(chunk)

			chunk = []okexMarket{}
			chunkSize = 0
		}
	}

	if chunkSize > 0 {
		o.subscribeChunkedMarkets(chunk)
	}
}

func (o *OkexImp) subscribeChunkedMarkets(chunk []okexMarket) {
	// https://stackoverflow.com/questions/12753805/type-converting-slices-of-interfaces-in-go/12754757#12754757
	objs := make([]interface{}, len(chunk))
	for i := range chunk {
		objs[i] = chunk[i]
	}

	conn, err := o.wsPool.NewConn(okexWSUrl, objs)
	if err != nil {
		log.Fatal(err)
	}

	for _, subscribingMarket := range chunk {
		var request struct {
			Op   string   `json:"op"`
			Args []string `json:"args"`
		}
		request.Op = "subscribe"
		request.Args = []string{"spot/trade:" + subscribingMarket.InstrumentID, "spot/depth:" + subscribingMarket.InstrumentID}

		err := conn.WriteJSON(request)
		if err != nil {
			log.Error(err)
		}
	}
}

func (o *OkexImp) getMarkets() (markets []okexMarket) {
	res, err := http.Get(o.baseURL + "/api/spot/v3/instruments")
	if err != nil {
		log.Error(err)
		return
	}

	err = json.NewDecoder(res.Body).Decode(&markets)
	if err != nil {
		log.Error(err)
		return
	}

	return
}
