package message

import (
	"context"
	"io"

	gorilla "github.com/gorilla/websocket"
	nhooyr "nhooyr.io/websocket"
)

// GorillaAdapter is an adapter for gorilla websockets.
type GorillaAdapter struct{ Conn *gorilla.Conn }

// Reader returns a reader.
func (a GorillaAdapter) Reader() (r io.Reader, err error) {
	_, r, err = a.Conn.NextReader()
	return
}

// Writer returns a writer.
func (a GorillaAdapter) Writer() (w io.WriteCloser, err error) {
	return a.Conn.NextWriter(gorilla.BinaryMessage)
}

// NhooyrAdapter is an adapter for nhooyr websockets.
type NhooyrAdapter struct{ Conn *nhooyr.Conn }

// Reader returns a reader.
func (a NhooyrAdapter) Reader() (r io.Reader, err error) {
	_, r, err = a.Conn.Reader(context.Background())
	return
}

// Writer returns a writer.
func (a NhooyrAdapter) Writer() (w io.WriteCloser, err error) {
	return a.Conn.Writer(context.Background(), nhooyr.MessageBinary)
}
