package exchanges

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	mm "github.com/qinix/cckit/market_monitor"

	pxnpb "github.com/qinix/cckit/exchanges/pxn/pusherpb"

	"github.com/golang/protobuf/ptypes"

	"github.com/golang/protobuf/proto"
	"github.com/shopspring/decimal"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type BigoneImp struct {
	monitor            *mm.Exchange
	conn               *websocket.Conn
	marketReloadTicker *time.Ticker
	rwMutex            sync.RWMutex
	symbolToMarket     map[string]bigoneMarket
}

var Bigone = BigoneImp{
	monitor:        mm.NewExchange(),
	symbolToMarket: make(map[string]bigoneMarket),
}

type bigoneMarket struct {
	ID    string
	Base  string
	Quote string
}

func init() {
	Register("bigone", &Bigone)
}

func (b *BigoneImp) Monitor() *mm.Exchange {
	return b.monitor
}

func (b *BigoneImp) Start() {
	b.marketReloadTicker = time.NewTicker(10 * time.Minute)
	var err error
	b.conn, _, err = websocket.DefaultDialer.Dial("wss://big.one/ws/v2", http.Header{"Sec-WebSocket-Protocol": []string{"protobuf"}})
	if err != nil {
		log.Fatal("dial: ", err)
	}

	go func() {
		for {
			_, message, err := b.conn.ReadMessage()
			var response pxnpb.Response
			err = proto.Unmarshal(message, &response)
			if err != nil {
				log.Fatal("unmarshaling error: ", err)
			}

			b.handleMessage(response)
		}
	}()

	go func() {
		b.loadNewMarkets()
		for {
			<-b.marketReloadTicker.C
			b.loadNewMarkets()
		}
	}()
}

func (b *BigoneImp) loadNewMarkets() {
	for _, market := range b.getMarkets() {
		if b.monitor.Market(market.Base, market.Quote) == nil {
			b.symbolToMarket[market.ID] = market
			b.subscribeMarket(market)
			b.monitor.NewMarket(market.Base, market.Quote)
		}
	}
}

func (b *BigoneImp) subscribeMarket(m bigoneMarket) {
	request1 := &pxnpb.Request{
		RequestId: string(time.Now().UnixNano()),
		Payload: &pxnpb.Request_SubscribeMarketDepthRequest{
			SubscribeMarketDepthRequest: &pxnpb.SubscribeMarketDepthRequest{
				Market: m.ID,
			},
		},
	}
	data1, err := proto.Marshal(request1)
	if err != nil {
		log.Fatal("marshaling error: ", err)
	}
	err = b.conn.WriteMessage(websocket.BinaryMessage, data1)
	if err != nil {
		log.Fatal("writeMessage: ", err)
	}

	request2 := &pxnpb.Request{
		RequestId: string(time.Now().UnixNano()),
		Payload: &pxnpb.Request_SubscribeMarketTradesRequest{
			SubscribeMarketTradesRequest: &pxnpb.SubscribeMarketTradesRequest{
				Market: m.ID,
			},
		},
	}
	data2, err := proto.Marshal(request2)
	if err != nil {
		log.Fatal("marshaling error: ", err)
	}
	err = b.conn.WriteMessage(websocket.BinaryMessage, data2)
	if err != nil {
		log.Fatal("writeMessage: ", err)
	}

}

func (b *BigoneImp) getMarkets() (markets []bigoneMarket) {
	res, err := http.Get("https://big.one/api/v3/asset_pairs")
	if err != nil {
		log.Error(err)
		return
	}

	var response struct {
		Data []struct {
			Name       string
			QuoteAsset struct {
				Symbol string
			}
			BaseAsset struct {
				Symbol string
			}
		}
	}

	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		log.Error(err)
		return
	}

	for _, rawMarket := range response.Data {
		markets = append(markets, bigoneMarket{
			ID:    rawMarket.Name,
			Base:  rawMarket.BaseAsset.Symbol,
			Quote: rawMarket.QuoteAsset.Symbol,
		})
	}
	return
}

func (b *BigoneImp) handleMessage(resp pxnpb.Response) {
	switch resp.GetPayload().(type) {
	case *pxnpb.Response_DepthSnapshot:
		b.mergeDepth(resp.GetDepthSnapshot().Depth)
	case *pxnpb.Response_DepthUpdate:
		b.mergeDepth(resp.GetDepthUpdate().Depth)
	case *pxnpb.Response_TradeUpdate:
		trade := resp.GetTradeUpdate().Trade
		market := b.symbolToMarket[trade.Market]
		time, err := ptypes.Timestamp(trade.CreatedAt)
		if err != nil {
			log.Fatal(err)
		}
		b.monitor.Market(market.Base, market.Quote).NewTrade(mm.Trade{Time: time, Price: decimal.RequireFromString(trade.Price), Amount: decimal.RequireFromString(trade.Amount), IsBuyerMaker: (trade.MakerOrder.Side == pxnpb.Order_BID)})
	default:
	}
}

func (b *BigoneImp) mergeDepth(depth *pxnpb.Depth) {
	market := b.symbolToMarket[depth.Market]
	var pls []mm.PriceLevel

	for _, bid := range depth.Bids {
		pls = append(pls, mm.PriceLevel{Price: decimal.RequireFromString(bid.Price), Amount: decimal.RequireFromString(bid.Amount), IsBid: true})
	}
	for _, ask := range depth.Asks {
		pls = append(pls, mm.PriceLevel{Price: decimal.RequireFromString(ask.Price), Amount: decimal.RequireFromString(ask.Amount), IsBid: false})
	}
	b.monitor.Market(market.Base, market.Quote).Depth.Merge(pls)
}
