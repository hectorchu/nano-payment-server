package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hectorchu/gonano/rpc"
)

func newPaymentHandler(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Account string
		Amount  *rpc.RawAmount
	}
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, err)
		return
	}
	if v.Account == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Missing account")
		return
	}
	if v.Amount == nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Missing amount")
		return
	}
	payment, err := newPaymentRequest(v.Account, &v.Amount.Int)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"id": payment.id}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
}

func paymentHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := r.URL.Query()["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Missing payment id")
		return
	}
	payment, err := getPaymentRequest(id[0])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	var block rpc.Block
	if err = json.NewDecoder(r.Body).Decode(&block); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, err)
		return
	}
	hash, err := validateBlock(&block, payment.account, payment.amount.Raw)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, err)
		return
	}
	if payment.hash != nil && !bytes.Equal(hash, payment.hash) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Block for this payment id has already been submitted")
		return
	}
	if err = updatePaymentRequest(payment.id, hash); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	if err = sendBlock(&block); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	if *callbackURL != "" {
		resp, err := http.Get(*callbackURL + "?id=" + payment.id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}
		resp.Body.Close()
	}
}
