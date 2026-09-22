package transport

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Conn is one live agent connection.
type Conn struct {
	DeviceID string
	ws       *websocket.Conn
	// send is the outbound queue. Writing to a full channel blocks the caller,
	// so callers should use Send() with a short timeout in hot paths.
	send chan []byte

	closeOnce sync.Once
	// writeMu guards ws writes: gorilla does not allow concurrent writers, and
	// pingLoop and writePump both write to this socket.
	writeMu sync.Mutex
}

func newConn(deviceID string, ws *websocket.Conn) *Conn {
	return &Conn{DeviceID: deviceID, ws: ws, send: make(chan []byte, 64)}
}

// closeSend signals writePump to stop. Safe to call any number of times and
// from any goroutine: the read loop, the hub, and the disconnect cleanup all
// may race to tear the connection down.
func (c *Conn) closeSend() {
	c.closeOnce.Do(func() { close(c.send) })
}

// writeText serialises a text-frame write to the peer.
func (c *Conn) writeText(b []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return c.ws.WriteMessage(websocket.TextMessage, b)
}

// writeControl serialises a control-frame write (ping/pong/close) to the peer.
func (c *Conn) writeControl(msgType int) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.ws.WriteMessage(msgType, nil)
}

// Send queues a message for delivery. Returns false if the queue is full
// (agent is slow/stuck) — the caller should treat that as an unhealthy connection.
func (c *Conn) Send(b []byte) bool {
	select {
	case c.send <- b:
		return true
	default:
		return false
	}
}

// writePump forwards queued messages to the peer. One per connection.
// It returns when closeSend() is called or a write fails; either way the
// connection is finished and ServeHTTP's deferred cleanup may proceed.
func (c *Conn) writePump(done chan struct{}) {
	defer close(done)
	for msg := range c.send {
		if err := c.writeText(msg); err != nil {
			return
		}
	}
}

// Hub is the in-memory registry of live agent connections, keyed by device ID.
// It is the server-side counterpart of the agent transport.
type Hub struct {
	mu    sync.RWMutex
	conns map[string]*Conn
}

func NewHub() *Hub {
	return &Hub{conns: make(map[string]*Conn)}
}

// Register attaches a connection, replacing any stale one for the same device.
func (h *Hub) Register(deviceID string, ws *websocket.Conn) *Conn {
	c := newConn(deviceID, ws)
	h.mu.Lock()
	if old, ok := h.conns[deviceID]; ok {
		// A previous session is still registered. Abandon it: its read loop will
		// hit a write error or a closed socket and run its own cleanup. Closing
		// old.send from here would race writePump on the same socket.
		delete(h.conns, deviceID)
		go old.closeAndLog(deviceID)
	}
	h.conns[deviceID] = c
	h.mu.Unlock()
	return c
}

// closeAndLog closes a superseded connection out-of-band and logs its fate.
// It only touches the socket of the supplied Conn.
func (c *Conn) closeAndLog(deviceID string) {
	c.closeSend()
	_ = c.ws.Close()
	log.Warn().Str("device", deviceID).Msg("closed stale connection for device")
}

// Unregister removes a connection if it is still the registered one.
func (h *Hub) Unregister(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.conns[c.DeviceID]; ok && cur == c {
		delete(h.conns, c.DeviceID)
	}
}

// Get returns the live connection for a device, or nil.
func (h *Hub) Get(deviceID string) *Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conns[deviceID]
}

// Online returns true if the device currently has a live connection.
func (h *Hub) Online(deviceID string) bool {
	return h.Get(deviceID) != nil
}

// SendTo delivers a message to a device's live connection.
func (h *Hub) SendTo(deviceID string, b []byte) bool {
	c := h.Get(deviceID)
	if c == nil {
		return false
	}
	return c.Send(b)
}

// Count returns the number of live connections.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
