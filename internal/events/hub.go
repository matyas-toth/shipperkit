// Package events provides bounded, nonblocking SSE invalidations.
package events

import (
	"sync"
)

type Event struct {
	Revision uint64 `json:"revision"`
}
type Hub struct {
	mu       sync.Mutex
	revision uint64
	clients  map[chan Event]struct{}
}

func New() *Hub { return &Hub{clients: map[chan Event]struct{}{}} }
func (h *Hub) Subscribe() (<-chan Event, func(), bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.clients) >= 64 {
		return nil, func() {}, false
	}
	ch := make(chan Event, 8)
	h.clients[ch] = struct{}{}
	ch <- Event{Revision: h.revision}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.clients[ch]; ok {
			delete(h.clients, ch)
			close(ch)
		}
	}, true
}
func (h *Hub) Publish() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.revision++
	for ch := range h.clients {
		select {
		case ch <- Event{Revision: h.revision}:
		default:
			delete(h.clients, ch)
			close(ch)
		}
	}
}
