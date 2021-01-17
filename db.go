package main

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/xid"
)

type paymentRecord struct {
	id      string
	account string
	amount  util.NanoAmount
	hash    rpc.BlockHash
}

func withDB(cb func(*sql.DB) error) (err error) {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		return
	}
	defer db.Close()
	return cb(db)
}

func newPaymentRequest(account string, amount *big.Int) (payment *paymentRecord, err error) {
	payment = &paymentRecord{
		id:      xid.New().String(),
		account: account,
		amount:  util.NanoAmount{Raw: amount},
	}
	err = withDB(func(db *sql.DB) (err error) {
		tx, err := db.Begin()
		if err != nil {
			return
		}
		if _, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS payments
			(id TEXT PRIMARY KEY, account TEXT, amount TEXT, block_hash TEXT)
		`); err != nil {
			tx.Rollback()
			return
		}
		if _, err = tx.Exec(`
			INSERT INTO payments (id, account, amount, block_hash) VALUES (?,?,?,?)
		`, payment.id, account, amount.String(), ""); err != nil {
			tx.Rollback()
			return
		}
		return tx.Commit()
	})
	return
}

func getPaymentRequest(id string) (payment *paymentRecord, err error) {
	err = withDB(func(db *sql.DB) (err error) {
		var account, amount, hash string
		if err = db.QueryRow(`
			SELECT account, amount, block_hash FROM payments WHERE id = ?
		`, id).Scan(&account, &amount, &hash); err != nil {
			return
		}
		payment = &paymentRecord{id: id, account: account}
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
	return withDB(func(db *sql.DB) (err error) {
		tx, err := db.Begin()
		if err != nil {
			return
		}
		if _, err = tx.Exec(`
			UPDATE payments SET block_hash = ? WHERE id = ?
		`, hash.String(), id); err != nil {
			tx.Rollback()
			return
		}
		return tx.Commit()
	})
}
