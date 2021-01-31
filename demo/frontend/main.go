package main

import (
	"context"
	"fmt"
	"syscall/js"
	"time"

	"github.com/hectorchu/gonano/rpc"
	"github.com/hectorchu/gonano/util"
	"github.com/hectorchu/nano-payment-server/demo/message"
	"github.com/hexops/vecty"
	"github.com/hexops/vecty/elem"
	"github.com/hexops/vecty/event"
	"github.com/hexops/vecty/prop"
	"nhooyr.io/websocket"
)

var (
	page     = newPageView()
	wsClient *message.Client
)

func main() {
	vecty.SetTitle("Nano Payment Server Demo")
	wsConnect()
	vecty.RenderBody(page)
}

func wsConnect() (err error) {
	host := js.Global().Get("location").Get("host").String()
	conn, _, err := websocket.Dial(context.Background(), "ws://"+host+"/ws", nil)
	if err != nil {
		return
	}
	wsClient = message.NewClient(message.NhooyrAdapter{conn}, message.Messages)
	go func() {
		for {
			v, err := wsClient.Read()
			if err != nil {
				conn.Close(websocket.StatusProtocolError, err.Error())
				for wsConnect() != nil {
					time.Sleep(5 * time.Second)
				}
				return
			}
			page.process(v)
		}
	}()
	return
}

type pageView struct {
	vecty.Core
	wallet  *walletView
	items   []*purchaseItem
	history *purchaseHistory
}

func newPageView() *pageView {
	nanoAmount := func(s string) (n util.NanoAmount) {
		n, _ = util.NanoAmountFromString(s)
		return
	}
	return &pageView{
		wallet: &walletView{},
		items: []*purchaseItem{
			{name: "Apple", price: nanoAmount("0.000001")},
			{name: "Banana", price: nanoAmount("0.000002")},
		},
		history: &purchaseHistory{},
	}
}

func (p *pageView) process(v interface{}) {
	p.wallet.process(v)
	for _, item := range p.items {
		item.process(v)
	}
	p.history.process(v)
}

func (p *pageView) Render() vecty.ComponentOrHTML {
	var items vecty.List
	for _, item := range p.items {
		items = append(items, item)
	}
	return elem.Body(
		p.wallet,
		elem.Div(vecty.Markup(vecty.Style("margin-top", "10px"))),
		items,
		elem.Div(vecty.Markup(vecty.Style("margin-top", "10px"))),
		p.history,
	)
}

type walletView struct {
	vecty.Core
	balance *message.Balance
}

func (w *walletView) process(v interface{}) {
	if v, ok := v.(*message.Balance); ok {
		w.balance = v
		vecty.Rerender(w)
	}
}

func (w *walletView) Render() vecty.ComponentOrHTML {
	if w.balance == nil {
		return vecty.Text("Wallet")
	}
	return vecty.Text(fmt.Sprintf(
		"Wallet %s = %s NANO",
		w.balance.Account,
		util.NanoAmount{Raw: &w.balance.Balance.Int},
	))
}

type purchaseItem struct {
	vecty.Core
	name       string
	price      util.NanoAmount
	paymentID  string
	paymentURL string
	hash       rpc.BlockHash
}

func (r *purchaseItem) onClick(event *vecty.Event) {
	wsClient.Write(&message.BuyRequest{
		Payment: &message.PaymentRecord{
			ItemName: r.name,
			Amount:   &rpc.RawAmount{*r.price.Raw},
		},
	})
}

func (r *purchaseItem) process(v interface{}) {
	switch v := v.(type) {
	case *message.BuyRequest:
		if v.Payment.ItemName == r.name {
			r.paymentID = v.Payment.PaymentID
			r.paymentURL = v.PaymentURL
			r.hash = nil
			vecty.Rerender(r)
		}
	case *message.PaymentRecord:
		if v.PaymentID == r.paymentID {
			r.hash = v.Hash
			vecty.Rerender(r)
		}
	}
	return
}

func (r *purchaseItem) Render() vecty.ComponentOrHTML {
	return elem.Div(
		vecty.Text(fmt.Sprintf("%s: %s NANO", r.name, r.price)),
		elem.Span(vecty.Markup(vecty.Style("margin-left", "10px"))),
		elem.Button(
			vecty.Markup(event.Click(r.onClick).PreventDefault()),
			vecty.Text("Buy"),
		),
		elem.Span(vecty.Markup(vecty.Style("margin-left", "10px"))),
		vecty.If(r.paymentURL != "" && r.hash == nil, elem.Anchor(
			vecty.Markup(prop.Href(r.paymentURL)),
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
	rows message.PurchaseHistory
}

func (h *purchaseHistory) process(v interface{}) {
	if v, ok := v.(*message.PurchaseHistory); ok {
		h.rows = *v
		vecty.Rerender(h)
	}
}

func (h *purchaseHistory) Render() vecty.ComponentOrHTML {
	var rows vecty.List
	for i := len(h.rows) - 1; i >= 0; i-- {
		rows = append(rows, &paymentRecord{Payment: h.rows[i]})
	}
	return elem.Table(
		vecty.Markup(vecty.Class("table")),
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

type paymentRecord struct {
	vecty.Core
	Payment *message.PaymentRecord `vecty:"prop"`
}

func (r *paymentRecord) Render() vecty.ComponentOrHTML {
	return elem.TableRow(
		elem.TableData(vecty.Text(r.Payment.PaymentID)),
		elem.TableData(vecty.Text(r.Payment.ItemName)),
		elem.TableData(vecty.Text(fmt.Sprintf("%s NANO", util.NanoAmount{Raw: &r.Payment.Amount.Int}))),
		elem.TableData(&blockHash{Hash: r.Payment.Hash}),
	)
}
