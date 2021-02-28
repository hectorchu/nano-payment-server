package main

import (
	"errors"
	"sync"

	"github.com/hectorchu/gonano/websocket"
)

type wsMux struct {
	url string
	m   sync.Mutex
	c   *websocket.Client
	ch  map[string]chan<- interface{}
}

func newWSMux(url string) *wsMux {
	return &wsMux{
		url: url,
		ch:  make(map[string]chan<- interface{}),
	}
}

func (ws *wsMux) connect(account string) (msg <-chan interface{}, err error) {
	ws.m.Lock()
	defer ws.m.Unlock()
	if ws.c == nil {
		c := &websocket.Client{URL: ws.url}
		if err = c.Connect(); err != nil {
			return
		}
		ws.c = c
		go ws.loop()
	}
	ch := make(chan interface{}, 32)
	ws.ch[account] = ch
	return ch, nil
}

func (ws *wsMux) disconnect(account string) {
	ws.m.Lock()
	delete(ws.ch, account)
	ws.m.Unlock()
}

func (ws *wsMux) loop() {
	for {
		switch m := (<-ws.c.Messages).(type) {
		case *websocket.Confirmation:
			ws.m.Lock()
			if ch, ok := ws.ch[m.Block.Account]; ok {
				ws.send(ch, m)
			}
			if m.Block.Account != m.Block.LinkAsAccount {
				if ch, ok := ws.ch[m.Block.LinkAsAccount]; ok {
					ws.send(ch, m)
				}
			}
			ws.m.Unlock()
		case error:
			ws.m.Lock()
			for _, ch := range ws.ch {
				ws.send(ch, m)
			}
			ws.c.Close()
			ws.c = nil
			ws.m.Unlock()
			return
		}
	}
}

func (ws *wsMux) send(ch chan<- interface{}, m interface{}) {
	switch len(ch) {
	case cap(ch):
	case cap(ch) - 1:
		ch <- errors.New("channel buffer full")
	default:
		ch <- m
	}
}
