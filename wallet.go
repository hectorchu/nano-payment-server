package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"sync"
	"time"

	"github.com/hectorchu/gonano/wallet"
)

// Wallet is a simple wallet.
type Wallet struct {
	seed []byte
	m    sync.Mutex
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
	w.m.Lock()
	defer w.m.Unlock()
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

func getFreeWalletIndex(id string, min uint32) (index uint32, err error) {
	now := time.Now().Unix()
	err = withDB(func(tx *sql.Tx) (err error) {
		if tx.QueryRow(`SELECT rowid FROM wallet WHERE id = "" AND rowid > ? LIMIT 1`, min).Scan(&index) == nil {
			_, err = tx.Exec("UPDATE wallet SET id = ?, time = ? WHERE rowid = ?", id, now, index)
			return
		}
		if _, err = tx.Exec("CREATE TABLE IF NOT EXISTS wallet(id TEXT, time INTEGER)"); err != nil {
			return
		}
		result, err := tx.Exec("INSERT INTO wallet VALUES(?,?)", id, now)
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

func getWalletIndexesOlderThan(t time.Time) (ids []string, err error) {
	err = withDB(func(tx *sql.Tx) (err error) {
		rows, err := tx.Query("SELECT id FROM wallet WHERE time < ?", t.Unix())
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			id := ""
			if err = rows.Scan(&id); err != nil {
				return
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return
}

func freeWalletIndex(id string) (err error) {
	return withDB(func(tx *sql.Tx) (err error) {
		_, err = tx.Exec(`UPDATE wallet SET id = "", time = NULL WHERE id = ?`, id)
		return
	})
}
