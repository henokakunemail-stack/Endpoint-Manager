package remotecontrol

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type SessionConfig struct {
	SessionID string `json:"session_id"`
	Mode      string `json:"mode"`
	RelayURL  string `json:"relay_url"`
}

type ScreenCapturer interface {
	CaptureScreen() ([]byte, int, int, error)
	InjectMouseEvent(action string, x, y int, button string, delta int) error
	InjectKeyboardEvent(action string, key string, code int) error
}

type Session struct {
	config    SessionConfig
	serverURL string
	capturer  ScreenCapturer
	conn      *websocket.Conn
	mu        sync.Mutex
	stopChan  chan struct{}
	stopped   bool
}

func NewSession(cfg SessionConfig, serverBaseURL string, capturer ScreenCapturer) *Session {
	return &Session{
		config:    cfg,
		serverURL: serverBaseURL,
		capturer:  capturer,
		stopChan:  make(chan struct{}),
	}
}

func (s *Session) Start(ctx context.Context) error {
	u, err := url.Parse(s.serverURL)
	if err != nil {
		return fmt.Errorf("parse server url: %w", err)
	}

	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s%s", wsScheme, u.Host, s.config.RelayURL)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, http.Header{
		"User-Agent": []string{"EndpointMgmt-Agent-RC/1.0"},
	})
	if err != nil {
		return fmt.Errorf("dial rc relay ws: %w", err)
	}
	s.conn = conn

	log.Info().Str("session", s.config.SessionID).Str("mode", s.config.Mode).Msg("remote control agent session connected")

	// Goroutine 1: Read input events from operator
	go s.readInputLoop()

	// Goroutine 2: Capture screen frames and stream to operator
	go s.streamFramesLoop()

	return nil
}

func (s *Session) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}
	s.stopped = true
	close(s.stopChan)
	if s.conn != nil {
		_ = s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session closed by agent"))
		_ = s.conn.Close()
	}
}

type inputMessage struct {
	Type   string `json:"type"` // "mouse" or "keyboard"
	Action string `json:"action"`
	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
	Button string `json:"button,omitempty"`
	Delta  int    `json:"delta,omitempty"`
	Key    string `json:"key,omitempty"`
	Code   int    `json:"code,omitempty"`
}

func (s *Session) readInputLoop() {
	defer s.Stop()

	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			break
		}

		var input inputMessage
		if err := json.Unmarshal(msg, &input); err != nil {
			continue
		}

		if s.config.Mode == "view_only" {
			continue // Drop input in view_only mode
		}

		switch input.Type {
		case "mouse":
			_ = s.capturer.InjectMouseEvent(input.Action, input.X, input.Y, input.Button, input.Delta)
		case "keyboard":
			_ = s.capturer.InjectKeyboardEvent(input.Action, input.Key, input.Code)
		}
	}
}

func (s *Session) streamFramesLoop() {
	defer s.Stop()

	ticker := time.NewTicker(100 * time.Millisecond) // ~10 FPS
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			frameData, width, height, err := s.capturer.CaptureScreen()
			if err != nil {
				log.Debug().Err(err).Msg("capture screen error")
				continue
			}
			if len(frameData) == 0 {
				continue
			}

			// Prefix frame header: width (2 bytes) + height (2 bytes) + JPEG bytes
			header := []byte{
				byte(width >> 8), byte(width & 0xFF),
				byte(height >> 8), byte(height & 0xFF),
			}
			payload := append(header, frameData...)

			s.mu.Lock()
			err = s.conn.WriteMessage(websocket.BinaryMessage, payload)
			s.mu.Unlock()

			if err != nil {
				return
			}
		}
	}
}
