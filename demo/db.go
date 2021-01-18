package main

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	_ "github.com/mattn/go-sqlite3"
)

type paymentRecord struct {
	paymentID string
	itemName  string
	amount    util.NanoAmount
	hash      rpc.BlockHash
}

func withDB(cb func(*sql.DB) error) (err error) {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		return
	}
	defer db.Close()
	return cb(db)
}

func newPaymentRequest(paymentID, itemName string, amount *big.Int) (payment *paymentRecord, err error) {
	payment = &paymentRecord{
		paymentID: paymentID,
		itemName:  itemName,
		amount:    util.NanoAmount{Raw: amount},
	}
	err = withDB(func(db *sql.DB) (err error) {
		tx, err := db.Begin()
		if err != nil {
			return
		}
		if _, err = tx.Exec(`
			CREATE TABLE IF NOT EXISTS payments
			(payment_id TEXT PRIMARY KEY, item_name TEXT, amount TEXT, block_hash TEXT)
		`); err != nil {
			tx.Rollback()
			return
		}
		if _, err = tx.Exec(`
			INSERT INTO payments (payment_id, item_name, amount, block_hash) VALUES (?,?,?,?)
		`, paymentID, itemName, amount.String(), ""); err != nil {
			tx.Rollback()
			return
		}
		return tx.Commit()
	})
	return
}

func getPaymentRequest(paymentID string) (payment *paymentRecord, err error) {
	err = withDB(func(db *sql.DB) (err error) {
		var itemName, amount, hash string
		if err = db.QueryRow(`
			SELECT item_name, amount, block_hash FROM payments WHERE payment_id = ?
		`, paymentID).Scan(&itemName, &amount, &hash); err != nil {
			return
		}
		payment = &paymentRecord{
			paymentID: paymentID,
			itemName:  itemName,
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

func updatePaymentRequest(paymentID string, hash rpc.BlockHash) (err error) {
	return withDB(func(db *sql.DB) (err error) {
		tx, err := db.Begin()
		if err != nil {
			return
		}
		if _, err = tx.Exec(`
			UPDATE payments SET block_hash = ? WHERE payment_id = ?
		`, hash.String(), paymentID); err != nil {
			tx.Rollback()
			return
		}
		return tx.Commit()
	})
}
