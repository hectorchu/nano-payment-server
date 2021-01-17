package main

import (
	"flag"
	"log"
	"net/http"
)

var (
	rpcURL = flag.String("rpc", "http://[::1]:7076", "RPC URL")
	powURL = flag.String("pow", "", "RPC Proof-of-Work URL")
	wsURL  = flag.String("ws", "ws://[::1]:7078", "WebSocket URL")
)

func main() {
	flag.Parse()
	http.HandleFunc("/new_payment", newPaymentHandler)
	http.HandleFunc("/payment", paymentHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
