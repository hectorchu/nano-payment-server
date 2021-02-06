package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
)

var walletMutex sync.Mutex

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
		var v struct {
			Account, Amount string
			Handoff         bool `json:",string"`
		}
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
		payment, err := newPaymentRequest(v.Account, amount.Raw)
		if err != nil {
			serverError(w, err)
			return
		}
		result := map[string]string{"id": payment.id}
		if v.Handoff {
			result["url"] = "/payment/pay?id=" + payment.id
		} else {
			walletMutex.Lock()
			index, err := getNextAvailableWallet()
			if err == nil {
				err = updatePaymentRequest(payment.id, index, nil)
			}
			walletMutex.Unlock()
			if err != nil {
				serverError(w, err)
				return
			}
			a, err := wallet.getAccount(index)
			if err != nil {
				serverError(w, err)
				return
			}
			result["account"] = a.Address()
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			serverError(w, err)
			return
		}
	}
}

func waitPaymentHandler(wallet *Wallet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			ID      string
			Timeout time.Duration `json:",string"`
		}
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
		if payment.hash != nil {
			badRequest(w, errors.New("Payment already received"))
			return
		}
		a, err := wallet.getAccount(payment.wallet)
		if err != nil {
			serverError(w, err)
			return
		}
		hash, err := waitReceive(r.Context(), a, payment.account, payment.amount.Raw, v.Timeout*time.Second)
		if err != nil {
			serverError(w, err)
			return
		}
		if err = updatePaymentRequest(payment.id, 0, hash); err != nil {
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

func handoffPaymentHandler(w http.ResponseWriter, r *http.Request) {
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
	if err = updatePaymentRequest(payment.id, 0, hash); err != nil {
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
