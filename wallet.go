package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"

	"github.com/hectorchu/gonano/wallet"
)

// Wallet is a simple wallet.
type Wallet struct {
	seed []byte
	w    *wallet.Wallet
}

func newWallet(seed []byte) (w *Wallet, err error) {
	w = &Wallet{seed: seed}
	if w.w, err = wallet.NewWallet(seed); err != nil {
		return
	}
	w.w.RPC.URL = *rpcURL
	w.w.RPCWork.URL = *powURL
	return
}

func (w *Wallet) getAccount(index uint32) (a *wallet.Account, err error) {
	return w.w.NewAccount(&index)
}

func loadWallet() (w *Wallet, err error) {
	if err = withDB(func(db *sql.DB) (err error) {
		var seedStr string
		if err = db.QueryRow("SELECT seed FROM wallet").Scan(&seedStr); err != nil {
			return
		}
		seed, err := hex.DecodeString(seedStr)
		if err != nil {
			return
		}
		w, err = newWallet(seed)
		return
	}); err == nil {
		return
	}
	seed := make([]byte, 32)
	if _, err = rand.Read(seed); err != nil {
		return
	}
	if w, err = newWallet(seed); err != nil {
		return
	}
	err = w.save()
	return
}

func (w *Wallet) save() (err error) {
	return withDB(func(db *sql.DB) (err error) {
		tx, err := db.Begin()
		if err != nil {
			return
		}
		if _, err = tx.Exec("CREATE TABLE IF NOT EXISTS wallet (seed TEXT)"); err != nil {
			tx.Rollback()
			return
		}
		if _, err = tx.Exec("INSERT INTO wallet VALUES (?)", hex.EncodeToString(w.seed)); err != nil {
			tx.Rollback()
			return
		}
		return tx.Commit()
	})
}
