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
	wallet        = &walletView{}
	purchaseItems = []*purchaseItem{
		{name: "Apple", price: nanoAmount("0.000001")},
		{name: "Banana", price: nanoAmount("0.000002")},
	}
	history  = &purchaseHistory{}
	wsClient *message.Client
)

func main() {
	vecty.SetTitle("Nano Payment Server Demo")
	wsConnect()
	vecty.RenderBody(&pageView{})
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
			switch m := v.(type) {
			case *message.Balance:
				wallet.balance = m
				vecty.Rerender(wallet)
			case *message.BuyRequest:
				for _, item := range purchaseItems {
					if item.name == m.Payment.ItemName {
						item.paymentID = m.Payment.PaymentID
						item.paymentURL = m.PaymentURL
						item.hash = nil
						vecty.Rerender(item)
					}
				}
			case *message.PaymentRecord:
				for _, item := range purchaseItems {
					if item.paymentID == m.PaymentID {
						item.hash = m.Hash
						vecty.Rerender(item)
					}
				}
			case *message.PurchaseHistory:
				history.rows = *m
				vecty.Rerender(history)
			}
		}
	}()
	return
}

func nanoAmount(s string) (n util.NanoAmount) {
	n, _ = util.NanoAmountFromString(s)
	return
}

type pageView struct {
	vecty.Core
}

func (p *pageView) Render() vecty.ComponentOrHTML {
	var items vecty.List
	for _, item := range purchaseItems {
		items = append(items, item)
	}
	return elem.Body(
		wallet,
		elem.Div(vecty.Markup(vecty.Style("margin-top", "10px"))),
		items,
		elem.Div(vecty.Markup(vecty.Style("margin-top", "10px"))),
		history,
	)
}

type walletView struct {
	vecty.Core
	balance *message.Balance
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

func (h *purchaseHistory) Render() vecty.ComponentOrHTML {
	var rows vecty.List
	for i := len(h.rows) - 1; i >= 0; i-- {
		rows = append(rows, &paymentRecord{Payment: h.rows[i]})
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
