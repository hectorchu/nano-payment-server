package message

import (
	"encoding/hex"
	"errors"
	"reflect"

	"github.com/hectorchu/gonano/rpc"
)

// Balance encodes the wallet balance.
type Balance struct {
	Account string
	Balance *rpc.RawAmount
}

// BuyRequest encodes a buy request.
type BuyRequest struct {
	Payment    *PaymentRecord
	PaymentURL string
}

// PaymentRecord encodes a payment record.
type PaymentRecord struct {
	PaymentID string
	ItemName  string
	Amount    *rpc.RawAmount
	Hash      rpc.BlockHash
}

// PurchaseHistory encodes the purchase history.
type PurchaseHistory []*PaymentRecord

// Messages is the list of possible message types.
var Messages = []reflect.Type{
	reflect.TypeOf(Balance{}),
	reflect.TypeOf(BuyRequest{}),
	reflect.TypeOf(PaymentRecord{}),
	reflect.TypeOf(PurchaseHistory{}),
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
