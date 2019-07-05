package main

import (
	"time"

	"github.com/qinix/cckit/exchanges"

	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetReportCaller(true)

	// exchanges.OKEx.Start()
	// exchanges.Binance.Start()
	exchanges.BigONE.Start()

	go func() {
		for {
			m := exchanges.BigONE.Monitor().Market("BTC", "USDT")
			if m != nil {
				// log.Info(m.Depth.Snapshot())
				log.Info("midprice: ", m.Price())
				// log.Info(m.Trades)
				inflow, outflow := m.MoneyFlow(time.Now().Add(-5*time.Minute), time.Now())
				if inflow.IsPositive() || outflow.IsPositive() {
					ioRatio := inflow.Div(inflow.Add(outflow)).Mul(decimal.New(1, 2))
					log.Info(m.Price(), inflow, outflow, ioRatio)
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()
	select {}
}
