package message

import (
	"encoding/hex"
	"errors"

	"github.com/hectorchu/gonano/rpc"
)

// Balance encodes the wallet balance.
type Balance struct {
	Account string
	Balance *rpc.RawAmount
}

// PaymentRecord encodes a payment record.
type PaymentRecord struct {
	PaymentID string         `json:"payment_id"`
	ItemName  string         `json:"item_name"`
	Amount    *rpc.RawAmount `json:"amount"`
	Hash      rpc.BlockHash  `json:"block_hash"`
}

// PurchaseHistory encodes the purchase history.
type PurchaseHistory []*PaymentRecord

func messages() []interface{} {
	return []interface{}{
		new(Balance),
		new(PaymentRecord),
		new(PurchaseHistory),
	}
}

// Scan populates a payment record from the DB.
func (r *PaymentRecord) Scan(row interface{ Scan(...interface{}) error }) (err error) {
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
