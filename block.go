package main

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/hectorchu/gonano/pow"
	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	"github.com/hectorchu/gonano/wallet"
	"github.com/hectorchu/gonano/wallet/ed25519"
	"github.com/hectorchu/gonano/websocket"
)

func validateBlock(block *rpc.Block, account string, amount *big.Int) (hash rpc.BlockHash, err error) {
	if block.Type != "state" {
		return nil, errors.New("invalid block type")
	}
	destAccount, err := util.PubkeyToAddress(block.Link)
	if err != nil {
		return
	}
	if destAccount != account {
		return nil, errors.New("incorrect destination account")
	}
	client := rpc.Client{URL: *rpcURL}
	ai, err := client.AccountInfo(block.Account)
	if err != nil {
		return
	}
	if !bytes.Equal(block.Previous, ai.Frontier) {
		return nil, errors.New("previous block is not frontier")
	}
	if block.Balance.Cmp(&ai.Balance.Int) >= 0 {
		return nil, errors.New("invalid block balance for send")
	}
	sendAmount := new(big.Int).Sub(&ai.Balance.Int, &block.Balance.Int)
	if sendAmount.Cmp(amount) != 0 {
		return nil, errors.New("incorrect payment amount")
	}
	pubkey, err := util.AddressToPubkey(block.Account)
	if err != nil {
		return
	}
	if hash, err = block.Hash(); err != nil {
		return
	}
	if !ed25519.Verify(pubkey, hash, block.Signature) {
		return nil, errors.New("invalid signature")
	}
	return
}

func sendBlock(block *rpc.Block) (err error) {
	if err = generatePoW(block); err != nil {
		return
	}
	client := rpc.Client{URL: *rpcURL}
	_, err = client.Process(block, "send")
	return
}

func waitReceive(
	ctx context.Context, ws *wsMux, a *wallet.Account,
	account string, amount *big.Int, timeout time.Duration,
) (hash rpc.BlockHash, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	msg, err := ws.connect(a.Address())
	if err != nil {
		return
	}
	defer ws.disconnect(a.Address())
	if err = a.ReceivePendings(); err != nil {
		return
	}
	client := rpc.Client{URL: *rpcURL, Ctx: ctx}
	if ai, err := client.AccountInfo(a.Address()); err == nil {
		if excess := new(big.Int).Sub(&ai.Balance.Int, amount); excess.Sign() >= 0 {
			if excess.Sign() > 0 {
				for hash := ai.Frontier; ; {
					bi, err := client.BlockInfo(hash)
					if err != nil {
						return nil, err
					}
					if bi.Subtype == "receive" {
						if bi, err = client.BlockInfo(bi.Contents.Link); err != nil {
							return nil, err
						}
						if _, err = a.Send(bi.BlockAccount, excess); err != nil {
							return nil, err
						}
						break
					}
					hash = bi.Contents.Previous
				}
			}
			return a.Send(account, amount)
		}
	} else if err.Error() != "Account not found" {
		return nil, err
	}
	for {
		select {
		case m := <-msg:
			switch m := m.(type) {
			case *websocket.Confirmation:
				switch a.Address() {
				case m.Block.LinkAsAccount:
					if _, err = a.ReceivePending(m.Hash); err != nil && err.Error() != "Unreceivable" {
						return
					}
				case m.Block.Account:
					if excess := new(big.Int).Sub(&m.Block.Balance.Int, amount); excess.Sign() >= 0 {
						if excess.Sign() > 0 {
							bi, err := client.BlockInfo(m.Block.Link)
							if err != nil {
								return nil, err
							}
							if _, err = a.Send(bi.BlockAccount, excess); err != nil {
								return nil, err
							}
						}
						return a.Send(account, amount)
					}
				}
			case error:
				return nil, m
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func refund(a *wallet.Account) (err error) {
	client := rpc.Client{URL: *rpcURL}
	if err = a.ReceivePendings(); err != nil {
		return
	}
	ai, err := client.AccountInfo(a.Address())
	if err != nil {
		if err.Error() == "Account not found" {
			err = nil
		}
		return
	}
	for hash, balance := ai.Frontier, &ai.Balance.Int; balance.Sign() > 0; {
		bi, err := client.BlockInfo(hash)
		if err != nil {
			return err
		}
		if bi.Subtype == "receive" {
			bi, err := client.BlockInfo(bi.Contents.Link)
			if err != nil {
				return err
			}
			amount := &bi.Amount.Int
			if amount.Cmp(balance) > 0 {
				amount = balance
			}
			if _, err = a.Send(bi.BlockAccount, amount); err != nil {
				return err
			}
			balance.Sub(balance, amount)
		}
		hash = bi.Contents.Previous
	}
	return
}

func generatePoW(block *rpc.Block) (err error) {
	client := rpc.Client{URL: *rpcURL}
	_, difficulty, _, _, _, _, err := client.ActiveDifficulty()
	if err != nil {
		return
	}
	if *powURL != "" {
		client.URL = *powURL
		block.Work, _, _, err = client.WorkGenerate(block.Previous, difficulty)
	} else {
		block.Work, err = pow.Generate(block.Previous, difficulty)
	}
	return
}
