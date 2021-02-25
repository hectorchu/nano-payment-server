package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	"github.com/hectorchu/gonano/wallet"
	nanows "github.com/hectorchu/gonano/websocket"
	"github.com/hectorchu/nano-payment-server/demo/message"
	"github.com/lpar/gzipped/v2"
)

func main() {
	var (
		rpcURL = flag.String("rpc", "https://gonano.dev/rpc", "RPC URL")
		wsURL  = flag.String("ws", "wss://gonano.dev/ws", "WebSocket URL")
	)
	flag.Parse()
	seed := make([]byte, 32)
	rand.Read(seed)
	w, _ := wallet.NewWallet(seed)
	w.RPC.URL = *rpcURL
	a, _ := w.NewAccount(nil)
	ws := nanows.Client{URL: *wsURL}
	ws.Connect()
	group := message.ClientGroup{}
	var (
		sendBalance = func() {
			if balance, pending, err := a.Balance(); err == nil {
				group.Broadcast(&message.Balance{
					Account: a.Address(),
					Balance: &rpc.RawAmount{Int: *balance.Add(balance, pending)},
				})
			}
		}
		sendPaymentRecord = func(id string) {
			payment, _ := getPaymentRequest(id)
			group.Broadcast(payment)
		}
		sendHistory = func() {
			history, _ := getPaymentRequests()
			if len(history) > 10 {
				history = history[len(history)-10:]
			}
			group.Broadcast(&history)
		}
	)
	go func() {
		for {
			switch m := (<-ws.Messages).(type) {
			case *nanows.Confirmation:
				if m.Block.LinkAsAccount == a.Address() {
					sendBalance()
				}
			case error:
				ws.Close()
				for ws.Connect() != nil {
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		var (
			upgrader = websocket.Upgrader{}
			conn, _  = upgrader.Upgrade(w, r, nil)
			c        = message.NewClient(message.GorillaAdapter{Conn: conn}, message.Messages)
			key      = group.Add(c)
		)
		defer conn.Close()
		defer group.Remove(key)
		sendBalance()
		sendHistory()
		for {
			v, err := c.Read()
			if err != nil {
				return
			}
			switch m := v.(type) {
			case *message.BuyRequest:
				var buf bytes.Buffer
				json.NewEncoder(&buf).Encode(map[string]string{
					"account": a.Address(),
					"amount":  util.NanoAmount{Raw: &m.Payment.Amount.Int}.String(),
				})
				resp, _ := http.Post("http://[::1]:7080/payment/new", "application/json", &buf)
				var v struct{ ID string }
				json.NewDecoder(resp.Body).Decode(&v)
				resp.Body.Close()
				payment, _ := newPaymentRequest(v.ID, m.Payment.ItemName, &m.Payment.Amount.Int)
				if err = c.Write(&message.BuyRequest{
					Payment:    payment,
					PaymentURL: "/payment?id=" + v.ID,
				}); err != nil {
					return
				}
				sendHistory()
			}
		}
	})
	http.HandleFunc("/payment", func(w http.ResponseWriter, r *http.Request) {
		id, _ := r.URL.Query()["id"]
		var buf bytes.Buffer
		io.Copy(&buf, r.Body)
		resp, err := http.Post("http://[::1]:7080/payment/pay?id="+id[0], "application/json", &buf)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}
		var v struct {
			ID   string
			Hash rpc.BlockHash `json:"block_hash"`
		}
		json.NewDecoder(resp.Body).Decode(&v)
		updatePaymentRequest(v.ID, v.Hash)
		sendPaymentRecord(v.ID)
		sendHistory()
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var (
			dir = gzipped.Dir("./public")
			fs  = gzipped.FileServer(dir)
		)
		if r.URL.Path == "/" {
			fs = http.FileServer(dir)
		}
		fs.ServeHTTP(w, r)
	})
	http.ListenAndServe(":8080", nil)
}
