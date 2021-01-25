package message

import "reflect"

// Client is a websocket client.
type Client struct {
	conn    jsonOps
	In, Out chan interface{}
	Err     chan error
}

type jsonOps interface {
	ReadJSON(interface{}) error
	WriteJSON(interface{}) error
}

// NewClient creates a Client.
func NewClient(conn jsonOps) (c *Client) {
	c = &Client{
		conn: conn,
		In:   make(chan interface{}),
		Out:  make(chan interface{}),
		Err:  make(chan error, 1),
	}
	go c.read()
	go c.write()
	return
}

func (c *Client) read() {
	for {
		var (
			msgType string
			msg     interface{}
		)
		if err := c.conn.ReadJSON(&msgType); err != nil {
			c.Err <- err
			return
		}
		for _, msg = range messages() {
			if reflect.TypeOf(msg).String() == msgType {
				break
			}
		}
		if err := c.conn.ReadJSON(msg); err != nil {
			c.Err <- err
			return
		}
		select {
		case c.In <- msg:
		case err := <-c.Err:
			c.Err <- err
			return
		}
	}
}

func (c *Client) write() {
	for {
		select {
		case msg := <-c.Out:
			if err := c.conn.WriteJSON(reflect.TypeOf(msg).String()); err != nil {
				c.Err <- err
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				c.Err <- err
				return
			}
		case err := <-c.Err:
			c.Err <- err
			return
		}
	}
}
