package main

import (
	"database/sql"
	"log"
	"time"
)

func scavenger(wallet *Wallet) {
	for range time.Tick(time.Minute) {
		ids, err := getWalletIndexesOlderThan(time.Now().Add(-time.Hour))
		if err != nil {
			log.Print(err)
			continue
		}
		for _, id := range ids {
			if err = scavenge(wallet, id); err != nil {
				log.Print(err)
			}
		}
	}
}

func scavenge(wallet *Wallet, id string) (err error) {
	if !paymentMutex.tryLock(id) {
		return
	}
	defer paymentMutex.unlock(id)
	payment, err := getPaymentRequest(id)
	if err == sql.ErrNoRows {
		return nil
	} else if err != nil || payment.hash != nil {
		return
	}
	return cancel(wallet, id)
}

func cancel(wallet *Wallet, id string) (err error) {
	index, err := getWalletIndex(id)
	if err != nil {
		return
	}
	a, err := wallet.getAccount(index)
	if err != nil {
		return
	}
	if err = refund(a); err != nil {
		return
	}
	if err = deletePaymentRequest(id); err != nil {
		return
	}
	return freeWalletIndex(id)
}
