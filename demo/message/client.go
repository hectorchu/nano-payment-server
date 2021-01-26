package message

import "reflect"

// Client is a websocket client.
type Client struct {
	conn    jsonReadWriter
	in, out chan interface{}
	err     chan error
}

type jsonReadWriter interface {
	ReadJSON(interface{}) error
	WriteJSON(interface{}) error
}

// NewClient creates a Client.
func NewClient(conn jsonReadWriter) (c *Client) {
	c = &Client{
		conn: conn,
		in:   make(chan interface{}),
		out:  make(chan interface{}),
		err:  make(chan error, 1),
	}
	go c.readLoop()
	go c.writeLoop()
	return
}

func (c *Client) Read() (v interface{}, err error) {
	select {
	case v = <-c.in:
	case err = <-c.err:
		c.err <- err
	}
	return
}

func (c *Client) Write(v interface{}) (err error) {
	select {
	case c.out <- v:
	case err = <-c.err:
		c.err <- err
	}
	return
}

func (c *Client) readLoop() {
	for {
		var (
			Type string
			v    interface{}
		)
		if err := c.conn.ReadJSON(&Type); err != nil {
			c.err <- err
			return
		}
		for _, v = range messages() {
			if reflect.TypeOf(v).String() == Type {
				break
			}
		}
		if err := c.conn.ReadJSON(v); err != nil {
			c.err <- err
			return
		}
		select {
		case c.in <- v:
		case err := <-c.err:
			c.err <- err
			return
		}
	}
}

func (c *Client) writeLoop() {
	for {
		select {
		case v := <-c.out:
			if err := c.conn.WriteJSON(reflect.TypeOf(v).String()); err != nil {
				c.err <- err
				return
			}
			if err := c.conn.WriteJSON(v); err != nil {
				c.err <- err
				return
			}
		case err := <-c.err:
			c.err <- err
			return
		}
	}
}
