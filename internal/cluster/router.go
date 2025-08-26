package cluster

import (
	"sync"

	"p2poker/internal/protocol"
)

// Router delivers NetMessages to the appropriate table goroutine by TableID.
type Router struct {
	mu      sync.RWMutex
	byTable map[protocol.TableID]chan<- protocol.NetMessage
}

func NewRouter() *Router {
	return &Router{byTable: make(map[protocol.TableID]chan<- protocol.NetMessage)}
}

func (r *Router) Register(id protocol.TableID, inbox chan<- protocol.NetMessage) {
	r.mu.Lock()
	r.byTable[id] = inbox
	r.mu.Unlock()
}

func (r *Router) Unregister(id protocol.TableID) {
	r.mu.Lock()
	delete(r.byTable, id)
	r.mu.Unlock()
}

func (r *Router) Route(msg protocol.NetMessage) bool {
	r.mu.RLock()
	ch, ok := r.byTable[msg.Table]
	r.mu.RUnlock()
	if ok {
		ch <- msg
	}
	return ok
}
