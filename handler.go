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
	id, err := newPaymentRequest(v.Account, &v.Amount.Int)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
}

func paymentHandler(w http.ResponseWriter, r *http.Request) {
	var (
		params = r.URL.Query()
		block  rpc.Block
	)
	id, ok := params["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Missing payment id")
		return
	}
	account, amount, hash, err := getPaymentRequest(id[0])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	if err = json.NewDecoder(r.Body).Decode(&block); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, err)
		return
	}
	hash2, err := validateBlock(&block, account, amount)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, err)
		return
	}
	if hash != nil && !bytes.Equal(hash, hash2) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Block for this payment id has already been submitted")
		return
	}
	if err = updatePaymentRequest(id[0], hash2); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
	if err = sendBlock(&block); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
}
