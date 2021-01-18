package main

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/hectorchu/gonano/rpc"
	_ "github.com/mattn/go-sqlite3"
)

type paymentRecord struct {
	PaymentID string         `json:"payment_id"`
	ItemName  string         `json:"item_name"`
	Amount    *rpc.RawAmount `json:"amount"`
	Hash      rpc.BlockHash  `json:"block_hash"`
}

func (r *paymentRecord) Scan(row interface{ Scan(...interface{}) error }) (err error) {
	var amount, hash string
	if err = row.Scan(&r.PaymentID, &r.ItemName, &amount, &hash); err != nil {
		return
	}
	var ok bool
	r.Amount = new(rpc.RawAmount)
	if _, ok = r.Amount.SetString(amount, 10); !ok {
		return errors.New("Could not decode amount")
	}
	if hash != "" {
		r.Hash, err = hex.DecodeString(hash)
	}
	return
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
		PaymentID: paymentID,
		ItemName:  itemName,
		Amount:    &rpc.RawAmount{Int: *amount},
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
		payment = new(paymentRecord)
		return payment.Scan(db.QueryRow("SELECT * FROM payments WHERE payment_id = ?", paymentID))
	})
	return
}

func getPaymentRequests() (payments []*paymentRecord, err error) {
	err = withDB(func(db *sql.DB) (err error) {
		rows, err := db.Query("SELECT * FROM payments")
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			payment := new(paymentRecord)
			if err = payment.Scan(rows); err != nil {
				return
			}
			payments = append(payments, payment)
		}
		return rows.Err()
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
