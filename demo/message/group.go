package message

import "sync"

// ClientGroup is a group of Clients.
type ClientGroup struct {
	m   sync.Mutex
	key int
	c   []groupClient
}

type groupClient struct {
	key int
	c   *Client
}

// Add adds a client to the group.
func (g *ClientGroup) Add(c *Client) (key int) {
	g.m.Lock()
	key = g.key
	g.key++
	g.c = append(g.c, groupClient{key: key, c: c})
	g.m.Unlock()
	return
}

// Remove removes a client from the group.
func (g *ClientGroup) Remove(key int) {
	g.m.Lock()
	for i, c := range g.c {
		if c.key == key {
			g.c = append(g.c[:i], g.c[i+1:]...)
			break
		}
	}
	g.m.Unlock()
}

// Broadcast sends a message to every Client.
func (g *ClientGroup) Broadcast(msg interface{}) {
	g.m.Lock()
	for _, c := range g.c {
		select {
		case c.c.Out <- msg:
		case err := <-c.c.Err:
			c.c.Err <- err
		}
	}
	g.m.Unlock()
}
