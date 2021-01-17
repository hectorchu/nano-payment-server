package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

var (
	port        = flag.Int("p", 8080, "Listen port")
	rpcURL      = flag.String("rpc", "http://[::1]:7076", "RPC URL")
	powURL      = flag.String("pow", "", "RPC Proof-of-Work URL")
	wsURL       = flag.String("ws", "ws://[::1]:7078", "WebSocket URL")
	callbackURL = flag.String("cb", "", "Callback URL when payment is fulfilled")
)

func main() {
	flag.Parse()
	http.HandleFunc("/new_payment", newPaymentHandler)
	http.HandleFunc("/payment", paymentHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
