package main

import "sync"

type mutexMap struct {
	m sync.Mutex
	c *sync.Cond
	k map[string]bool
}

func newMutexMap() (m *mutexMap) {
	m = &mutexMap{k: make(map[string]bool)}
	m.c = sync.NewCond(&m.m)
	return
}

func (m *mutexMap) lock(key string) {
	m.m.Lock()
	for m.k[key] {
		m.c.Wait()
	}
	m.k[key] = true
	m.m.Unlock()
}

func (m *mutexMap) tryLock(key string) bool {
	m.m.Lock()
	defer m.m.Unlock()
	if m.k[key] {
		return false
	}
	m.k[key] = true
	return true
}

func (m *mutexMap) unlock(key string) {
	m.m.Lock()
	delete(m.k, key)
	m.m.Unlock()
	m.c.Broadcast()
}
