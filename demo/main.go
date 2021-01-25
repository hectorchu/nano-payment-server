package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/wallet"
	nanows "github.com/hectorchu/gonano/websocket"
	"github.com/hectorchu/nano-payment-server/demo/message"
	"github.com/lpar/gzipped/v2"
)

func main() {
	seed := make([]byte, 32)
	rand.Read(seed)
	w, _ := wallet.NewWallet(seed)
	w.RPC.URL = "http://[::1]:7076"
	a, _ := w.NewAccount(nil)
	ws := nanows.Client{URL: "ws://[::1]:7078"}
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
			c        = message.NewClient(conn)
			key      = group.Add(c)
		)
		sendBalance()
		sendHistory()
		for {
			select {
			case <-c.In:
			case err := <-c.Err:
				c.Err <- err
				group.Remove(key)
				conn.Close()
				return
			}
		}
	})
	http.HandleFunc("/buy", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Name   string
			Amount *rpc.RawAmount
		}
		json.NewDecoder(r.Body).Decode(&v)
		r.Body.Close()

		var buf bytes.Buffer
		json.NewEncoder(&buf).Encode(map[string]string{
			"account": a.Address(),
			"amount":  v.Amount.String(),
		})
		resp, _ := http.Post("http://[::1]:8090/new_payment", "application/json", &buf)
		var v2 struct {
			ID string `json:"id"`
		}
		json.NewDecoder(resp.Body).Decode(&v2)
		resp.Body.Close()
		newPaymentRequest(v2.ID, v.Name, &v.Amount.Int)
		json.NewEncoder(w).Encode(map[string]string{
			"payment_id":  v2.ID,
			"payment_url": "/payment?id=" + v2.ID,
		})
		sendHistory()
	})
	http.HandleFunc("/payment", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		io.Copy(&buf, r.Body)
		r.Body.Close()
		resp, _ := http.Post("http://[::1]:8090"+r.RequestURI, "application/json", &buf)
		if resp.StatusCode != http.StatusOK {
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			resp.Body.Close()
			return
		}
		var v struct {
			ID   string        `json:"id"`
			Hash rpc.BlockHash `json:"block_hash"`
		}
		json.NewDecoder(resp.Body).Decode(&v)
		resp.Body.Close()
		updatePaymentRequest(v.ID, v.Hash)
		sendPaymentRecord(v.ID)
		sendHistory()
	})
	http.Handle("/", withIndexHTML(gzipped.FileServer(gzipped.Dir("./public"))))
	http.ListenAndServe(":8080", nil)
}

func withIndexHTML(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || len(r.URL.Path) == 0 {
			newpath := path.Join(r.URL.Path, "index.html")
			r.URL.Path = newpath
		}
		h.ServeHTTP(w, r)
	})
}
