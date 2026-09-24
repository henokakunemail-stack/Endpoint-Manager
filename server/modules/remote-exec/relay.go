package remoteexec

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// ActiveTerminalSession holds the active browser WebSocket for an operator.
type ActiveTerminalSession struct {
	SessionID string
	DeviceID  string
	UserID    string
	ws        *websocket.Conn
	writeMu   sync.Mutex
	closed    bool
}

// TerminalRelay manages live browser-to-agent terminal sessions.
// It implements transport.TerminalReceiver.
type TerminalRelay struct {
	mu       sync.RWMutex
	sessions map[string]*ActiveTerminalSession
}

func NewTerminalRelay() *TerminalRelay {
	return &TerminalRelay{
		sessions: make(map[string]*ActiveTerminalSession),
	}
}

// Register registers an active browser terminal session.
func (r *TerminalRelay) Register(sessionID, deviceID, userID string, ws *websocket.Conn) *ActiveTerminalSession {
	sess := &ActiveTerminalSession{
		SessionID: sessionID,
		DeviceID:  deviceID,
		UserID:    userID,
		ws:        ws,
	}
	r.mu.Lock()
	r.sessions[sessionID] = sess
	r.mu.Unlock()
	return sess
}

// Unregister removes a session.
func (r *TerminalRelay) Unregister(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sess, ok := r.sessions[sessionID]; ok {
		sess.closed = true
		delete(r.sessions, sessionID)
	}
}

// Get returns the active session if registered.
func (r *TerminalRelay) Get(sessionID string) *ActiveTerminalSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions[sessionID]
}

// WriteToBrowser sends a terminal message to the browser WebSocket.
func (s *ActiveTerminalSession) WriteToBrowser(msgType string, data string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.closed || s.ws == nil {
		return errors.New("terminal session closed")
	}
	payload := map[string]string{
		"type":       msgType,
		"session_id": s.SessionID,
		"data":       data,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_ = s.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return s.ws.WriteMessage(websocket.TextMessage, b)
}

// AcceptTerminalData forwards agent terminal stdout data to the operator's browser.
func (r *TerminalRelay) AcceptTerminalData(ctx context.Context, deviceID string, sessionID string, data string) error {
	sess := r.Get(sessionID)
	if sess == nil {
		log.Debug().Str("session_id", sessionID).Msg("terminal data for unknown or closed session")
		return nil
	}
	return sess.WriteToBrowser("term.data", data)
}

// AcceptTerminalClose handles closure initiated by the agent.
func (r *TerminalRelay) AcceptTerminalClose(ctx context.Context, deviceID string, sessionID string) error {
	sess := r.Get(sessionID)
	if sess == nil {
		return nil
	}
	_ = sess.WriteToBrowser("term.close", "Shell process terminated by remote host.")
	r.Unregister(sessionID)
	return nil
}
