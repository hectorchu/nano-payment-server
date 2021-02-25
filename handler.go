package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
)

var paymentMutex = newMutexMap()

func badRequest(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintln(w, err)
}

func serverError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintln(w, err)
}

func newPaymentHandler(wallet *Wallet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var v struct{ Account, Amount string }
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			badRequest(w, err)
			return
		}
		if v.Account == "" {
			badRequest(w, errors.New("Missing account"))
			return
		}
		if _, err := util.AddressToPubkey(v.Account); err != nil {
			badRequest(w, err)
			return
		}
		if v.Amount == "" {
			badRequest(w, errors.New("Missing amount"))
			return
		}
		amount, err := util.NanoAmountFromString(v.Amount)
		if err != nil {
			badRequest(w, err)
			return
		}
		if amount.Raw.Sign() <= 0 {
			badRequest(w, errors.New("Amount must be positive"))
			return
		}
		payment, err := newPaymentRequest(v.Account, amount.Raw)
		if err != nil {
			serverError(w, err)
			return
		}
		index, err := getFreeWalletIndex(payment.id)
		if err != nil {
			serverError(w, err)
			return
		}
		a, err := wallet.getAccount(index)
		if err != nil {
			serverError(w, err)
			return
		}
		if err = json.NewEncoder(w).Encode(map[string]string{
			"id":      payment.id,
			"account": a.Address(),
		}); err != nil {
			serverError(w, err)
			return
		}
	}
}

func waitPaymentHandler(wallet *Wallet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID      string
			Timeout time.Duration
		}
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			badRequest(w, err)
			return
		}
		if v.ID == "" {
			badRequest(w, errors.New("Missing payment id"))
			return
		}
		paymentMutex.lock(v.ID)
		defer paymentMutex.unlock(v.ID)
		if r.Context().Err() != nil {
			return
		}
		payment, err := getPaymentRequest(v.ID)
		if err == sql.ErrNoRows {
			badRequest(w, errors.New("Invalid payment id"))
			return
		} else if err != nil {
			serverError(w, err)
			return
		}
		if payment.hash != nil {
			if err = json.NewEncoder(w).Encode(map[string]string{
				"id":         payment.id,
				"block_hash": payment.hash.String(),
			}); err != nil {
				serverError(w, err)
			}
			return
		}
		index, err := getWalletIndex(payment.id)
		if err != nil {
			serverError(w, err)
			return
		}
		a, err := wallet.getAccount(index)
		if err != nil {
			serverError(w, err)
			return
		}
		if v.Timeout == 0 {
			v.Timeout = 1800
		}
		hash, err := waitReceive(r.Context(), a, payment.account, payment.amount.Raw, v.Timeout*time.Second)
		if err != nil {
			serverError(w, err)
			return
		}
		if err = updatePaymentRequest(payment.id, hash); err != nil {
			serverError(w, err)
			return
		}
		if err = freeWalletIndex(payment.id); err != nil {
			serverError(w, err)
			return
		}
		if err = json.NewEncoder(w).Encode(map[string]string{
			"id":         payment.id,
			"block_hash": hash.String(),
		}); err != nil {
			serverError(w, err)
			return
		}
	}
}

func cancelPaymentHandler(wallet *Wallet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var v struct{ ID string }
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			badRequest(w, err)
			return
		}
		if v.ID == "" {
			badRequest(w, errors.New("Missing payment id"))
			return
		}
		paymentMutex.lock(v.ID)
		defer paymentMutex.unlock(v.ID)
		if r.Context().Err() != nil {
			return
		}
		payment, err := getPaymentRequest(v.ID)
		if err == sql.ErrNoRows {
			badRequest(w, errors.New("Invalid payment id"))
			return
		} else if err != nil {
			serverError(w, err)
			return
		}
		if payment.hash != nil {
			badRequest(w, errors.New("Payment already fulfilled"))
			return
		}
		if err = cancel(wallet, payment.id); err != nil {
			serverError(w, err)
			return
		}
		if err = json.NewEncoder(w).Encode(map[string]string{}); err != nil {
			serverError(w, err)
			return
		}
	}
}

func handoffPaymentHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := r.URL.Query()["id"]
	if !ok {
		badRequest(w, errors.New("Missing payment id"))
		return
	}
	paymentMutex.lock(id[0])
	defer paymentMutex.unlock(id[0])
	if r.Context().Err() != nil {
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
		if err == io.EOF {
			err = errors.New("Please paste this URL into a wallet which supports payment URLs")
		}
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
	if err = freeWalletIndex(payment.id); err != nil {
		serverError(w, err)
		return
	}
	if err = sendBlock(&block); err != nil {
		serverError(w, err)
		return
	}
	var buf bytes.Buffer
	if err = json.NewEncoder(&buf).Encode(map[string]string{
		"id":         payment.id,
		"block_hash": hash.String(),
	}); err != nil {
		serverError(w, err)
		return
	}
	if _, err = io.Copy(w, &buf); err != nil {
		serverError(w, err)
		return
	}
	if *callbackURL != "" {
		resp, err := http.Post(*callbackURL, "application/json", &buf)
		if err != nil {
			serverError(w, err)
			return
		}
		resp.Body.Close()
	}
}

func statusPaymentHandler(w http.ResponseWriter, r *http.Request) {
	var v struct{ ID string }
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		badRequest(w, err)
		return
	}
	if v.ID == "" {
		badRequest(w, errors.New("Missing payment id"))
		return
	}
	payment, err := getPaymentRequest(v.ID)
	if err == sql.ErrNoRows {
		badRequest(w, errors.New("Invalid payment id"))
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	if err = json.NewEncoder(w).Encode(map[string]string{
		"id":         payment.id,
		"block_hash": payment.hash.String(),
	}); err != nil {
		serverError(w, err)
		return
	}
}
