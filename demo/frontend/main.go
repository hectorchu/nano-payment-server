package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	"github.com/hexops/vecty"
	"github.com/hexops/vecty/elem"
	"github.com/hexops/vecty/event"
	"github.com/hexops/vecty/prop"
)

var (
	apple    = &itemRow{name: "Apple", price: nanoAmount("0.000001")}
	banana   = &itemRow{name: "Banana", price: nanoAmount("0.000002")}
	history  = &purchaseHistory{}
	rerender = make(chan vecty.Component)
)

func main() {
	vecty.SetTitle("Nano Payment Server Demo")
	p := &pageView{wv: new(walletView)}
	p.wv.fetch()
	history.fetch()
	go p.wv.loop()
	go apple.loop()
	go banana.loop()
	go func() {
		for {
			vecty.Rerender(<-rerender)
		}
	}()
	vecty.RenderBody(p)
}

func nanoAmount(s string) (n util.NanoAmount) {
	n, _ = util.NanoAmountFromString(s)
	return
}

type pageView struct {
	vecty.Core
	wv *walletView
}

func (p *pageView) Render() vecty.ComponentOrHTML {
	return elem.Body(
		p.wv,
		elem.Div(vecty.Markup(vecty.Style("margin-top", "10px"))),
		apple,
		banana,
		elem.Div(vecty.Markup(vecty.Style("margin-top", "10px"))),
		history,
	)
}

type walletView struct {
	vecty.Core
	Account, Balance string
}

func (w *walletView) fetch() {
	resp, _ := http.Get("/account")
	json.NewDecoder(resp.Body).Decode(w)
	resp.Body.Close()
}

func (w *walletView) loop() {
	for range time.Tick(5 * time.Second) {
		w.fetch()
		rerender <- w
	}
}

func (w *walletView) Render() vecty.ComponentOrHTML {
	return vecty.Text(fmt.Sprintf("Wallet %s = %s", w.Account, w.Balance))
}

type itemRow struct {
	vecty.Core
	name       string
	price      util.NanoAmount
	PaymentID  string `json:"payment_id"`
	PaymentURL string `json:"payment_url"`
	hash       rpc.BlockHash
}

func (r *itemRow) loop() {
	for range time.Tick(5 * time.Second) {
		if r.PaymentID == "" {
			continue
		}
		var buf bytes.Buffer
		json.NewEncoder(&buf).Encode(map[string]string{"id": r.PaymentID})
		resp, _ := http.Post("/status", "application/json", &buf)
		var v struct {
			Hash rpc.BlockHash `json:"block_hash"`
		}
		json.NewDecoder(resp.Body).Decode(&v)
		resp.Body.Close()
		if len(v.Hash) > 0 {
			r.PaymentID = ""
			r.PaymentURL = ""
			r.hash = v.Hash
			history.fetch()
			rerender <- r
			rerender <- history
		}
	}
}

func (r *itemRow) onClick(event *vecty.Event) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]string{
		"name":   r.name,
		"amount": r.price.Raw.String(),
	})
	go func() {
		resp, _ := http.Post("/buy", "application/json", &buf)
		json.NewDecoder(resp.Body).Decode(r)
		resp.Body.Close()
		r.hash = nil
		history.fetch()
		rerender <- r
		rerender <- history
	}()
}

func (r *itemRow) Render() vecty.ComponentOrHTML {
	return elem.Div(
		vecty.Text(fmt.Sprintf("%s: %s NANO", r.name, r.price)),
		elem.Span(vecty.Markup(vecty.Style("margin-left", "10px"))),
		elem.Button(
			vecty.Markup(event.Click(r.onClick).PreventDefault()),
			vecty.Text("Buy"),
		),
		elem.Span(vecty.Markup(vecty.Style("margin-left", "10px"))),
		vecty.If(r.PaymentURL != "", elem.Anchor(
			vecty.Markup(prop.Href(r.PaymentURL)),
			vecty.Text("Payment Link"),
		)),
		vecty.If(r.hash != nil, &blockHash{Hash: r.hash, Text: "Payment Received, Thank You!"}),
	)
}

type blockHash struct {
	vecty.Core
	Hash rpc.BlockHash `vecty:"prop"`
	Text string        `vecty:"prop"`
}

func (h *blockHash) Render() vecty.ComponentOrHTML {
	text := h.Text
	if text == "" {
		text = h.Hash.String()
	}
	return elem.Anchor(
		vecty.Markup(
			prop.Href("https://nanolooker.com/block/"+h.Hash.String()),
			vecty.Property("target", "_blank"),
		),
		vecty.Text(text),
	)
}

type purchaseHistory struct {
	vecty.Core
	rows []*purchaseHistoryItem
}

func (h *purchaseHistory) fetch() {
	resp, _ := http.Get("/history")
	json.NewDecoder(resp.Body).Decode(&h.rows)
	resp.Body.Close()
}

func (h *purchaseHistory) Render() vecty.ComponentOrHTML {
	var rows vecty.List
	for i := len(h.rows) - 1; i >= 0; i-- {
		rows = append(rows, h.rows[i])
	}
	return elem.Table(
		elem.TableHead(
			vecty.Markup(vecty.Style("text-align", "left")),
			elem.TableRow(
				elem.TableHeader(vecty.Text("Payment ID")),
				elem.TableHeader(vecty.Text("Item name")),
				elem.TableHeader(vecty.Text("Amount")),
				elem.TableHeader(vecty.Text("Block hash")),
			),
		),
		elem.TableBody(rows),
	)
}

type purchaseHistoryItem struct {
	vecty.Core
	PaymentID string         `json:"payment_id"`
	ItemName  string         `json:"item_name"`
	Amount    *rpc.RawAmount `json:"amount"`
	Hash      rpc.BlockHash  `json:"block_hash"`
}

func (r *purchaseHistoryItem) Render() vecty.ComponentOrHTML {
	return elem.TableRow(
		elem.TableData(vecty.Text(r.PaymentID)),
		elem.TableData(vecty.Text(r.ItemName)),
		elem.TableData(vecty.Text(fmt.Sprintf("%s NANO", util.NanoAmount{Raw: &r.Amount.Int}))),
		elem.TableData(&blockHash{Hash: r.Hash}),
	)
}
