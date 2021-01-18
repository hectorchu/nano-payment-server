package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	"github.com/hectorchu/gonano/wallet"
)

func main() {
	seed := make([]byte, 32)
	rand.Read(seed)
	w, _ := wallet.NewWallet(seed)
	w.RPC.URL = "http://[::1]:7076"
	a, _ := w.NewAccount(nil)
	http.HandleFunc("/account", func(w http.ResponseWriter, r *http.Request) {
		balance, pending, _ := a.Balance()
		json.NewEncoder(w).Encode(map[string]string{
			"account": a.Address(),
			"balance": util.NanoAmount{Raw: balance.Add(balance, pending)}.String(),
		})
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
	})
	http.HandleFunc("/payment", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		io.Copy(&buf, r.Body)
		r.Body.Close()
		resp, _ := http.Post("http://[::1]:8090"+r.RequestURI, "application/json", &buf)
		if resp.StatusCode != http.StatusOK {
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
	})
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID string `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&v)
		r.Body.Close()
		payment, _ := getPaymentRequest(v.ID)
		json.NewEncoder(w).Encode(map[string]string{
			"block_hash": payment.hash.String(),
		})
	})
	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.ListenAndServe(":8080", nil)
}
