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
	apple  = &itemRow{name: "Apple", price: nanoAmount("0.000001")}
	banana = &itemRow{name: "Banana", price: nanoAmount("0.000002")}
)

func main() {
	vecty.SetTitle("Nano Payment Server Demo")
	p := &pageView{wv: new(walletView)}
	go p.wv.loop()
	go apple.loop()
	go banana.loop()
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
		elem.Div(apple),
		elem.Div(banana),
	)
}

type walletView struct {
	vecty.Core
	Account, Balance string
}

func (w *walletView) loop() {
	for tick := time.Tick(5 * time.Second); ; <-tick {
		resp, _ := http.Get("/account")
		json.NewDecoder(resp.Body).Decode(w)
		resp.Body.Close()
		vecty.Rerender(w)
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
			vecty.Rerender(r)
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
		vecty.Rerender(r)
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
		vecty.If(r.hash != nil, elem.Anchor(
			vecty.Markup(
				prop.Href("https://nanolooker.com/block/"+r.hash.String()),
				vecty.Property("target", "_blank"),
			),
			vecty.Text("Payment Received, Thank You!"),
		)),
	)
}
