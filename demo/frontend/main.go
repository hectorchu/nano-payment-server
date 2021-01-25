package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	"nhooyr.io/websocket/wsjson"
)

var (
	wallet        = &walletView{}
	purchaseItems = []*purchaseItem{
		{name: "Apple", price: nanoAmount("0.000001")},
		{name: "Banana", price: nanoAmount("0.000002")},
	}
	history  = &purchaseHistory{}
	rerender = make(chan vecty.Component)
)

func main() {
	vecty.SetTitle("Nano Payment Server Demo")
	go func() {
		for {
			vecty.Rerender(<-rerender)
		}
	}()
	wsConnect()
	vecty.RenderBody(&pageView{})
}

type wsAdapter struct{ c *websocket.Conn }

func (w wsAdapter) ReadJSON(v interface{}) error {
	return wsjson.Read(context.Background(), w.c, v)
}

func (w wsAdapter) WriteJSON(v interface{}) error {
	return wsjson.Write(context.Background(), w.c, v)
}

func wsConnect() (err error) {
	host := js.Global().Get("location").Get("host").String()
	conn, _, err := websocket.Dial(context.Background(), "ws://"+host+"/ws", nil)
	if err != nil {
		return
	}
	c := message.NewClient(wsAdapter{conn})
	go func() {
		for {
			select {
			case m := <-c.In:
				switch m := m.(type) {
				case *message.Balance:
					wallet.balance = m
					rerender <- wallet
				case *message.PaymentRecord:
					for _, item := range purchaseItems {
						if item.PaymentID == m.PaymentID {
							item.hash = m.Hash
							rerender <- item
						}
					}
				case *message.PurchaseHistory:
					history.rows = *m
					rerender <- history
				}
			case err = <-c.Err:
				c.Err <- err
				conn.Close(websocket.StatusProtocolError, err.Error())
				for wsConnect() != nil {
					time.Sleep(5 * time.Second)
				}
				return
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
	PaymentID  string `json:"payment_id"`
	PaymentURL string `json:"payment_url"`
	hash       rpc.BlockHash
}

func (r *purchaseItem) onClick(event *vecty.Event) {
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
		rerender <- r
	}()
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
		vecty.If(r.PaymentURL != "" && r.hash == nil, elem.Anchor(
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
