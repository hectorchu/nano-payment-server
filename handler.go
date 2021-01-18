package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hectorchu/gonano/rpc"
)

func badRequest(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintln(w, err)
}

func serverError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintln(w, err)
}

func newPaymentHandler(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Account string
		Amount  *rpc.RawAmount
	}
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		badRequest(w, err)
		return
	}
	if v.Account == "" {
		badRequest(w, errors.New("Missing account"))
		return
	}
	if v.Amount == nil {
		badRequest(w, errors.New("Missing amount"))
		return
	}
	payment, err := newPaymentRequest(v.Account, &v.Amount.Int)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"id": payment.id}); err != nil {
		serverError(w, err)
		return
	}
}

func paymentHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := r.URL.Query()["id"]
	if !ok {
		badRequest(w, errors.New("Missing payment id"))
		return
	}
	payment, err := getPaymentRequest(id[0])
	if err == sql.ErrNoRows {
		badRequest(w, errors.New("Invalid payment id"))
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	var block rpc.Block
	if err = json.NewDecoder(r.Body).Decode(&block); err != nil {
		badRequest(w, err)
		return
	}
	hash, err := validateBlock(&block, payment.account, payment.amount.Raw)
	if err != nil {
		badRequest(w, err)
		return
	}
	if payment.hash != nil && !bytes.Equal(hash, payment.hash) {
		badRequest(w, errors.New("Block for this payment id has already been submitted"))
		return
	}
	if err = updatePaymentRequest(payment.id, hash); err != nil {
		serverError(w, err)
		return
	}
	if err = sendBlock(&block); err != nil {
		serverError(w, err)
		return
	}
	if *callbackURL != "" {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(map[string]string{
			"id":         payment.id,
			"block_hash": hash.String(),
		}); err != nil {
			serverError(w, err)
			return
		}
		resp, err := http.Post(*callbackURL, "application/json", &buf)
		if err != nil {
			serverError(w, err)
			return
		}
		resp.Body.Close()
	}
}
