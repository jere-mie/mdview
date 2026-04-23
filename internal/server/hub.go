package server

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type hub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}
}

func newHub() *hub {
	return &hub{
		conns: make(map[*websocket.Conn]struct{}),
	}
}

func (h *hub) Add(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[conn] = struct{}{}
}

func (h *hub) Remove(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, conn)
}

func (h *hub) BroadcastReload() {
	h.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for conn := range h.conns {
		conns = append(conns, conn)
	}
	h.mu.Unlock()

	for _, conn := range conns {
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, []byte("reload")); err != nil {
			_ = conn.Close()
			h.Remove(conn)
		}
	}
}

func (h *hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.conns {
		_ = conn.Close()
		delete(h.conns, conn)
	}
}
