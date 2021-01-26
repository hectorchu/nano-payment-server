package message

import "reflect"

// Client is a websocket client.
type Client struct {
	conn     jsonReadWriter
	messages []reflect.Type
	in, out  chan interface{}
	err      chan error
}

type jsonReadWriter interface {
	ReadJSON(interface{}) error
	WriteJSON(interface{}) error
}

// NewClient creates a Client.
func NewClient(conn jsonReadWriter, messages []reflect.Type) (c *Client) {
	c = &Client{
		conn:     conn,
		messages: messages,
		in:       make(chan interface{}),
		out:      make(chan interface{}),
		err:      make(chan error, 1),
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
		for _, t := range c.messages {
			if reflect.PtrTo(t).String() == Type {
				v = reflect.New(t).Interface()
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
