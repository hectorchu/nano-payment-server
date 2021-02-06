package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	_ "github.com/mattn/go-sqlite3"
)

type paymentRecord struct {
	id      string
	account string
	amount  util.NanoAmount
	hash    rpc.BlockHash
}

func withDB(f func(*sql.Tx) error) (err error) {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		return
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return
	}
	if err = f(tx); err != nil {
		tx.Rollback()
		return
	}
	return tx.Commit()
}

func getConfig(key string) (value string, err error) {
	err = withDB(func(tx *sql.Tx) (err error) {
		value, err = getConfigWithTx(tx, key)
		return
	})
	return
}

func getConfigWithTx(tx *sql.Tx, key string) (value string, err error) {
	err = tx.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	return
}

func setConfig(key, value string) (err error) {
	return withDB(func(tx *sql.Tx) error {
		return setConfigWithTx(tx, key, value)
	})
}

func setConfigWithTx(tx *sql.Tx, key, value string) (err error) {
	if _, err = tx.Exec("CREATE TABLE IF NOT EXISTS config(key TEXT PRIMARY KEY, value TEXT)"); err != nil {
		return
	}
	_, err = tx.Exec("REPLACE INTO config VALUES(?,?)", key, value)
	return
}

func newPaymentRequest(account string, amount *big.Int) (payment *paymentRecord, err error) {
	id := make([]byte, 8)
	if _, err = rand.Read(id); err != nil {
		return
	}
	payment = &paymentRecord{
		id:      base64.RawURLEncoding.EncodeToString(id),
		account: account,
		amount:  util.NanoAmount{Raw: amount},
	}
	err = withDB(func(tx *sql.Tx) (err error) {
		if _, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS
			payments(id TEXT PRIMARY KEY, account TEXT, amount TEXT, block_hash TEXT)
		`); err != nil {
			return
		}
		_, err = tx.Exec("INSERT INTO payments VALUES(?,?,?,?)", payment.id, account, amount.String(), "")
		return
	})
	return
}

func getPaymentRequest(id string) (payment *paymentRecord, err error) {
	err = withDB(func(tx *sql.Tx) (err error) {
		payment = &paymentRecord{id: id}
		var amount, hash string
		if err = tx.QueryRow(`
			SELECT account, amount, block_hash FROM payments WHERE id = ?
		`, id).Scan(&payment.account, &amount, &hash); err != nil {
			return
		}
		var ok bool
		if payment.amount.Raw, ok = new(big.Int).SetString(amount, 10); !ok {
			return errors.New("Could not decode amount")
		}
		if hash != "" {
			if payment.hash, err = hex.DecodeString(hash); err != nil {
				return
			}
		}
		return
	})
	return
}

func updatePaymentRequest(id string, hash rpc.BlockHash) (err error) {
	return withDB(func(tx *sql.Tx) (err error) {
		_, err = tx.Exec("UPDATE payments SET block_hash = ? WHERE id = ?", hash.String(), id)
		return
	})
}
