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
	wallet  uint32
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
	id := make([]byte, 8)
	if _, err = rand.Read(id); err != nil {
		return
	}
	payment = &paymentRecord{
		id:      base64.RawURLEncoding.EncodeToString(id),
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
			(id TEXT PRIMARY KEY, account TEXT, amount TEXT, wallet INTEGER, block_hash TEXT)
		`); err != nil {
			tx.Rollback()
			return
		}
		if _, err = tx.Exec(`
			INSERT INTO payments VALUES (?,?,?,?,?)
		`, payment.id, account, amount.String(), 0, ""); err != nil {
			tx.Rollback()
			return
		}
		return tx.Commit()
	})
	return
}

func getPaymentRequest(id string) (payment *paymentRecord, err error) {
	err = withDB(func(db *sql.DB) (err error) {
		payment = &paymentRecord{id: id}
		var amount, hash string
		if err = db.QueryRow(`
			SELECT account, amount, wallet, block_hash FROM payments WHERE id = ?
		`, id).Scan(&payment.account, &amount, &payment.wallet, &hash); err != nil {
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

func updatePaymentRequest(id string, wallet uint32, hash rpc.BlockHash) (err error) {
	return withDB(func(db *sql.DB) (err error) {
		tx, err := db.Begin()
		if err != nil {
			return
		}
		if _, err = tx.Exec(`
			UPDATE payments SET wallet = ?, block_hash = ? WHERE id = ?
		`, wallet, hash.String(), id); err != nil {
			tx.Rollback()
			return
		}
		return tx.Commit()
	})
}

func getNextAvailableWallet() (wallet uint32, err error) {
	wallets := make(map[uint32]bool)
	if err = withDB(func(db *sql.DB) (err error) {
		rows, err := db.Query("SELECT wallet FROM payments WHERE wallet > 0")
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			if err = rows.Scan(&wallet); err != nil {
				return
			}
			wallets[wallet] = true
		}
		return rows.Err()
	}); err != nil {
		return
	}
	for wallet = 1; wallets[wallet]; wallet++ {
	}
	return
}
