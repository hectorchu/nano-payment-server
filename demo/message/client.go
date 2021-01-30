package message

import (
	"encoding/gob"
	"io"
	"reflect"
)

// Client is a websocket client.
type Client struct {
	conn     wsConn
	messages []reflect.Type
	in, out  chan interface{}
	err      chan error
	enc      *gob.Encoder
	dec      *gob.Decoder
	r        struct{ io.Reader }
	w        struct{ io.WriteCloser }
}

type wsConn interface {
	Reader() (io.Reader, error)
	Writer() (io.WriteCloser, error)
}

// NewClient creates a Client.
func NewClient(conn wsConn, messages []reflect.Type) (c *Client) {
	c = &Client{
		conn:     conn,
		messages: messages,
		in:       make(chan interface{}),
		out:      make(chan interface{}),
		err:      make(chan error, 1),
	}
	c.enc = gob.NewEncoder(&c.w)
	c.dec = gob.NewDecoder(&c.r)
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
	var err error
	for err == nil {
		if c.r.Reader, err = c.conn.Reader(); err != nil {
			break
		}
		i := 0
		if err = c.dec.Decode(&i); err != nil {
			break
		}
		v := reflect.New(c.messages[i]).Interface()
		if err = c.dec.Decode(v); err != nil {
			break
		}
		select {
		case c.in <- v:
		case err = <-c.err:
		}
	}
	c.err <- err
}

func (c *Client) writeLoop() {
	var err error
	for err == nil {
		select {
		case v := <-c.out:
			if c.w.WriteCloser, err = c.conn.Writer(); err != nil {
				break
			}
			for i, t := range c.messages {
				if reflect.TypeOf(v) == reflect.PtrTo(t) {
					if err = c.enc.Encode(i); err != nil {
						break
					}
					if err = c.enc.Encode(v); err != nil {
						break
					}
					err = c.w.WriteCloser.Close()
				}
			}
		case err = <-c.err:
		}
	}
	c.err <- err
}
