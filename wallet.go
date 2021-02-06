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
	if seed, err := getConfig("wallet_seed"); err == nil {
		seed, err := hex.DecodeString(seed)
		if err != nil {
			return nil, err
		}
		return newWallet(seed)
	}
	seed := make([]byte, 32)
	if _, err = rand.Read(seed); err != nil {
		return
	}
	if w, err = newWallet(seed); err != nil {
		return
	}
	err = setConfig("wallet_seed", hex.EncodeToString(seed))
	return
}

func getFreeWalletIndex(id string) (index uint32, err error) {
	err = withDB(func(tx *sql.Tx) (err error) {
		if tx.QueryRow(`SELECT rowid FROM wallet WHERE id = "" LIMIT 1`).Scan(&index) == nil {
			_, err = tx.Exec("UPDATE wallet SET id = ? WHERE rowid = ?", id, index)
			return
		}
		if _, err = tx.Exec("CREATE TABLE IF NOT EXISTS wallet(id TEXT)"); err != nil {
			return
		}
		result, err := tx.Exec("INSERT INTO wallet VALUES(?)", id)
		if err != nil {
			return
		}
		rowid, err := result.LastInsertId()
		if err != nil {
			return
		}
		index = uint32(rowid)
		return
	})
	return
}

func getWalletIndex(id string) (index uint32, err error) {
	err = withDB(func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT rowid FROM wallet WHERE id = ?", id).Scan(&index)
	})
	return
}

func freeWalletIndex(index uint32) (err error) {
	return withDB(func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(`UPDATE wallet SET id = "" WHERE rowid = ?`, index)
		return
	})
}
