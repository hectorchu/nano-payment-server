package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

var (
	port        = flag.Int("p", 7080, "Listen port")
	dbPath      = flag.String("db", "./data.db", "Path to DB")
	rpcURL      = flag.String("rpc", "http://[::1]:7076", "RPC URL")
	powURL      = flag.String("pow", "", "RPC Proof-of-Work URL")
	wsURL       = flag.String("ws", "ws://[::1]:7078", "WebSocket URL")
	callbackURL = flag.String("cb", "", "Callback URL when payment is fulfilled")
)

func main() {
	flag.Parse()
	if err := initDB(); err != nil {
		log.Fatal(err)
	}
	w, err := loadWallet()
	if err != nil {
		log.Fatal(err)
	}
	go scavenger(w)
	ws := newWSMux(*wsURL)
	http.HandleFunc("/payment/new", newPaymentHandler(w))
	http.HandleFunc("/payment/wait", waitPaymentHandler(w, ws))
	http.HandleFunc("/payment/cancel", cancelPaymentHandler(w))
	http.HandleFunc("/payment/pay", handoffPaymentHandler)
	http.HandleFunc("/payment/status", statusPaymentHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
