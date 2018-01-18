package main

import (
	"fmt"
	"os"
	"strings"

	binance "github.com/binance-exchange/go-binance"

	"github.com/fatih/color"

	ws "github.com/gorilla/websocket"
	gdax "github.com/preichenberger/go-gdax"
)

const gdaxWsEndpoint = "wss://ws-feed.gdax.com"

type trade struct {
	Source string
	Side   string
	Price  float64
}

type gdaxFeed struct {
	dialer ws.Dialer
	conn   *ws.Conn
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("please enter a market")
		os.Exit(1)
	}

	var dialer ws.Dialer
	conn, _, err := dialer.Dial(gdaxWsEndpoint, nil)
	if err != nil {
		panic(err)
	}

	product := strings.ToUpper(strings.TrimSpace(os.Args[1]))
	fmt.Printf("Following market %s:\n", product)

	subscribe := gdax.Message{
		Type: "subscribe",
		Channels: []gdax.MessageChannel{
			gdax.MessageChannel{
				Name:       "ticker",
				ProductIds: []string{product},
			},
		},
	}

	if err := conn.WriteJSON(subscribe); err != nil {
		panic(err)
	}

	msgCh := make(chan trade, 32)

	go func() {
		var msg gdax.Message
		for {
			if err := conn.ReadJSON(&msg); err != nil {
				panic(err)
			}

			if msg.Type == "ticker" {
				msgCh <- trade{
					Price:  msg.Price,
					Side:   msg.Side,
					Source: "G",
				}
			}
		}
	}()

	binanceSym := strings.Replace(product, "-", "", -1)
	if strings.HasSuffix(binanceSym, "USD") {
		binanceSym += "T"
	}

	bs := binance.NewAPIService("", "", nil, nil, nil)
	kCh, done, err := bs.TradeWebsocket(binance.TradeWebsocketRequest{
		Symbol: binanceSym,
	})
	if err != nil {
		panic(err)
	}

	go func() {
		var lastPrice float64
		side := "buy"
		for {
			select {
			case k := <-kCh:
				diff := k.Price - lastPrice
				if diff < 0 && side != "sell" {
					side = "sell"
				} else if diff > 0 && side != "buy" {
					side = "buy"
				}
				lastPrice = k.Price

				msgCh <- trade{
					Source: "B",
					Price:  k.Price,
					Side:   side,
				}
			case <-done:
				return
			}
		}
	}()

	for msg := range msgCh {
		color.Set(color.FgRed)
		side := "⬇️"
		if msg.Side == "buy" {
			color.Set(color.FgGreen)
			side = "⬆️"
		}

		fmt.Printf("%s %9.2f (%s)\n", side, msg.Price, msg.Source)
	}
}
