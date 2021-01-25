package main

import (
	"database/sql"
	"math/big"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/nano-payment-server/demo/message"
	_ "github.com/mattn/go-sqlite3"
)

func withDB(cb func(*sql.DB) error) (err error) {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		return
	}
	defer db.Close()
	return cb(db)
}

func newPaymentRequest(paymentID, itemName string, amount *big.Int) (payment *message.PaymentRecord, err error) {
	payment = &message.PaymentRecord{
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

func getPaymentRequest(paymentID string) (payment *message.PaymentRecord, err error) {
	err = withDB(func(db *sql.DB) (err error) {
		payment = new(message.PaymentRecord)
		return payment.Scan(db.QueryRow("SELECT * FROM payments WHERE payment_id = ?", paymentID))
	})
	return
}

func getPaymentRequests() (history message.PurchaseHistory, err error) {
	err = withDB(func(db *sql.DB) (err error) {
		rows, err := db.Query("SELECT * FROM payments")
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			payment := new(message.PaymentRecord)
			if err = payment.Scan(rows); err != nil {
				return
			}
			history = append(history, payment)
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
