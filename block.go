package main

import (
	"bytes"
	"errors"
	"math/big"
	"time"

	"github.com/hectorchu/gonano/pow"
	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	"github.com/hectorchu/gonano/wallet/ed25519"
	"github.com/hectorchu/gonano/websocket"
)

func validateBlock(block *rpc.Block, account string, amount *big.Int) (hash rpc.BlockHash, err error) {
	if block.Type != "state" {
		return nil, errors.New("Invalid block type")
	}
	destAccount, err := util.PubkeyToAddress(block.Link)
	if err != nil {
		return
	}
	if destAccount != account {
		return nil, errors.New("Incorrect destination account")
	}
	client := rpc.Client{URL: *rpcURL}
	ai, err := client.AccountInfo(block.Account)
	if err != nil {
		return
	}
	if !bytes.Equal(block.Previous, ai.Frontier) {
		return nil, errors.New("Previous block is not frontier")
	}
	if block.Balance.Cmp(&ai.Balance.Int) >= 0 {
		return nil, errors.New("Invalid block balance for send")
	}
	sendAmount := new(big.Int).Sub(&ai.Balance.Int, &block.Balance.Int)
	if sendAmount.Cmp(amount) != 0 {
		return nil, errors.New("Incorrect payment amount")
	}
	pubkey, err := util.AddressToPubkey(block.Account)
	if err != nil {
		return
	}
	if hash, err = block.Hash(); err != nil {
		return
	}
	if !ed25519.Verify(pubkey, hash, block.Signature) {
		return nil, errors.New("Invalid signature")
	}
	return
}

func sendBlock(block *rpc.Block) (err error) {
	var (
		rpcClient = rpc.Client{URL: *rpcURL}
		wsClient  = websocket.Client{URL: *wsURL}
	)
	hash, err := block.Hash()
	if err != nil {
		return
	}
	if err = generatePoW(block); err != nil {
		return
	}
	if err = wsClient.Connect(); err != nil {
		return
	}
	defer wsClient.Close()
	if _, err = rpcClient.Process(block, "send"); err != nil {
		return
	}
	for timer := time.After(time.Minute); ; {
		select {
		case m := <-wsClient.Messages:
			switch m := m.(type) {
			case *websocket.Confirmation:
				if bytes.Equal(m.Hash, hash) {
					return
				}
			case error:
				return m
			}
		case <-timer:
			return errors.New("Timed out")
		}
	}
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
