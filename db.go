package main

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/hectorchu/gonano/rpc"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/xid"
)

func withDB(cb func(*sql.DB) error) (err error) {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		return
	}
	defer db.Close()
	return cb(db)
}

func newPaymentRequest(account string, amount *big.Int) (id string, err error) {
	id = xid.New().String()
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
		`, id, account, amount.String(), ""); err != nil {
			tx.Rollback()
			return
		}
		return tx.Commit()
	})
	return
}

func getPaymentRequest(id string) (account string, amount *big.Int, hash rpc.BlockHash, err error) {
	err = withDB(func(db *sql.DB) (err error) {
		var amountStr, hashStr string
		if err = db.QueryRow(`
			SELECT account, amount, block_hash FROM payments WHERE id = ?
		`, id).Scan(&account, &amountStr, &hashStr); err != nil {
			return
		}
		var ok bool
		if amount, ok = new(big.Int).SetString(amountStr, 10); !ok {
			return errors.New("Could not decode amount")
		}
		if hashStr != "" {
			if hash, err = hex.DecodeString(hashStr); err != nil {
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
