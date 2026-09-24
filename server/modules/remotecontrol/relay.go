package remotecontrol

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type activeRelay struct {
	sessionID   string
	mode        string
	operatorWS  *websocket.Conn
	agentWS     *websocket.Conn
	mu          sync.Mutex
	framesCount int
	bytesCount  int64
	inputsCount int
	closed      bool
	closeChan   chan struct{}
}

type RelayManager struct {
	repo     *Repository
	relays   map[string]*activeRelay
	relaysMu sync.RWMutex
}

func NewRelayManager(repo *Repository) *RelayManager {
	return &RelayManager{
		repo:   repo,
		relays: make(map[string]*activeRelay),
	}
}

func (rm *RelayManager) RegisterSession(sessionID, mode string) *activeRelay {
	rm.relaysMu.Lock()
	defer rm.relaysMu.Unlock()

	r := &activeRelay{
		sessionID: sessionID,
		mode:      mode,
		closeChan: make(chan struct{}),
	}
	rm.relays[sessionID] = r
	return r
}

func (rm *RelayManager) GetRelay(sessionID string) *activeRelay {
	rm.relaysMu.RLock()
	defer rm.relaysMu.RUnlock()
	return rm.relays[sessionID]
}

func (rm *RelayManager) AttachOperator(sessionID string, ws *websocket.Conn) (*activeRelay, bool) {
	rm.relaysMu.Lock()
	r, exists := rm.relays[sessionID]
	if !exists {
		r = &activeRelay{
			sessionID: sessionID,
			mode:      "full_control",
			closeChan: make(chan struct{}),
		}
		rm.relays[sessionID] = r
	}
	rm.relaysMu.Unlock()

	r.mu.Lock()
	r.operatorWS = ws
	r.mu.Unlock()

	return r, true
}

func (rm *RelayManager) AttachAgent(sessionID string, ws *websocket.Conn) (*activeRelay, bool) {
	rm.relaysMu.Lock()
	r, exists := rm.relays[sessionID]
	if !exists {
		r = &activeRelay{
			sessionID: sessionID,
			mode:      "full_control",
			closeChan: make(chan struct{}),
		}
		rm.relays[sessionID] = r
	}
	rm.relaysMu.Unlock()

	r.mu.Lock()
	r.agentWS = ws
	r.mu.Unlock()

	return r, true
}

func (rm *RelayManager) CloseRelay(sessionID string) {
	rm.relaysMu.Lock()
	r, exists := rm.relays[sessionID]
	if exists {
		delete(rm.relays, sessionID)
	}
	rm.relaysMu.Unlock()

	if !exists || r == nil {
		return
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	close(r.closeChan)

	if r.operatorWS != nil {
		_ = r.operatorWS.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session ended"))
		_ = r.operatorWS.Close()
	}
	if r.agentWS != nil {
		_ = r.agentWS.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session ended"))
		_ = r.agentWS.Close()
	}
	frames := r.framesCount
	totalBytes := r.bytesCount
	inputs := r.inputsCount
	r.mu.Unlock()

	// Update DB stats
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = rm.repo.EndSession(ctx, sessionID, frames, totalBytes, inputs)
	log.Info().Str("session", sessionID).Int("frames", frames).Int("inputs", inputs).Msg("remote control relay closed")
}

// ForwardAgentFrame forwards screen frame from agent to operator
func (r *activeRelay) ForwardAgentFrame(msgType int, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.framesCount++
	r.bytesCount += int64(len(data))

	if r.operatorWS == nil {
		return nil
	}
	return r.operatorWS.WriteMessage(msgType, data)
}

// ForwardOperatorInput forwards mouse/keyboard input from operator to agent
func (r *activeRelay) ForwardOperatorInput(msgType int, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	// If view_only mode, drop input events
	if r.mode == "view_only" {
		return nil
	}

	r.inputsCount++

	if r.agentWS == nil {
		return nil
	}
	return r.agentWS.WriteMessage(msgType, data)
}
