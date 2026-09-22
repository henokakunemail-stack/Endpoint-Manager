// Package transport is the agent-side connection to the central server.
// The agent always initiates the connection (outbound), satisfying the
// constraint that the server never reaches into branch networks.
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Client is the persistent agent->server connection with automatic reconnect.
type Client struct {
	serverURL   string // e.g. "ws://localhost:8443"
	deviceID    string
	deviceSecret string

	mu       sync.Mutex
	ws       *websocket.Conn
	closed   bool
	onCommand func(ctx context.Context, command, id string, payload json.RawMessage) any
}

func NewClient(serverURL, deviceID, deviceSecret string) *Client {
	return &Client{
		serverURL:   wsURL(serverURL),
		deviceID:    deviceID,
		deviceSecret: deviceSecret,
	}
}

// wsURL normalises an http(s):// server base URL to ws(s):// so callers can use
// the same "-server" value for both the REST enrollment call and the socket.
func wsURL(serverURL string) string {
	if s := strings.TrimPrefix(serverURL, "https://"); s != serverURL {
		return "wss://" + s
	}
	if s := strings.TrimPrefix(serverURL, "http://"); s != serverURL {
		return "ws://" + s
	}
	// Already ws://, wss://, or a bare host:port — use as-is.
	return strings.TrimRight(serverURL, "/")
}

// SetCommandHandler installs the callback invoked for each server command.
// The returned value is sent back as the command result.
func (c *Client) SetCommandHandler(h func(ctx context.Context, command, id string, payload json.RawMessage) any) {
	c.onCommand = h
}

// Close stops the client. Run blocks until ctx is cancelled or Close is called.
func (c *Client) Close() {
	c.mu.Lock()
	c.closed = true
	if c.ws != nil {
		_ = c.ws.Close()
	}
	c.mu.Unlock()
}

// Run connects and stays connected until ctx is done. Between failed attempts it
// backs off with jitter so 500 agents never retry in lockstep after an outage.
func (c *Client) Run(ctx context.Context, hello any) error {
	backoff := time.Second
	const maxBackoff = 60 * time.Second

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := c.connectAndServe(ctx, hello)
		if c.isClosed() {
			// Graceful shutdown requested via Close(): do not reconnect.
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Exponential backoff with full jitter.
		jitter := time.Duration(0)
		if backoff > time.Second {
			jitter = time.Duration(time.Now().UnixNano()%int64(backoff)) - backoff/2
		}
		sleep := backoff + jitter
		log.Warn().Err(err).Dur("retry_in", sleep).Msg("disconnected, retrying")
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return ctx.Err()
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (c *Client) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *Client) connectAndServe(ctx context.Context, hello any) error {
	header := http.Header{}
	header.Set("X-Device-Id", c.deviceID)
	header.Set("X-Device-Secret", c.deviceSecret)

	url := c.serverURL + "/api/agent/connect"
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, url, header)
	if err != nil {
		return fmt.Errorf("dial server: %w", err)
	}
	c.mu.Lock()
	c.ws = ws
	c.mu.Unlock()
	defer func() {
		_ = ws.Close()
		c.mu.Lock()
		c.ws = nil
		c.mu.Unlock()
	}()

	if err := c.send(Envelope{Type: TypeHello, Payload: hello}); err != nil {
		return err
	}
	log.Info().Str("server", c.serverURL).Msg("connected to server")

	// Answer server pings so a half-open connection is detected on both ends.
	// The standard library does not auto-reply pings while a read loop is
	// blocked elsewhere, so we handle the ping frame explicitly here.
	ws.SetPongHandler(func(string) error {
		_ = ws.SetReadDeadline(time.Now().Add(45 * time.Second))
		return nil
	})
	_ = ws.SetReadDeadline(time.Now().Add(45 * time.Second))

	// Heartbeat keeps the connection warm through NATs and updates last_seen.
	var heartbeatDone sync.WaitGroup
	heartbeatDone.Add(1)
	go func() {
		defer heartbeatDone.Done()
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := c.send(Envelope{Type: TypeHeartbeat}); err != nil {
				return
			}
			case <-ctx.Done():
				return
			}
		}
	}()
	defer heartbeatDone.Wait()

	for {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		// Any traffic proves the connection is alive; extend the deadline.
		_ = ws.SetReadDeadline(time.Now().Add(45 * time.Second))
		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			log.Warn().Bytes("msg", data).Msg("bad message from server")
			continue
		}
		if env.Type == TypeCommand && c.onCommand != nil {
			var raw json.RawMessage
			if env.Payload != nil {
				raw, _ = json.Marshal(env.Payload)
			}
			res := c.onCommand(ctx, env.Command, env.ID, raw)
			status := StatusDone
			if res == nil {
				status = StatusFailed
			}
			_ = c.send(Envelope{Type: TypeCommandResult, ID: env.ID, Status: status, Result: res})
		}
	}
}

// Send transmits an envelope to the server.
func (c *Client) Send(e Envelope) error { return c.send(e) }

func (c *Client) send(e Envelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ws == nil {
		return errors.New("not connected")
	}
	_ = c.ws.SetWriteDeadline(time.Now().Add(15 * time.Second))
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return c.ws.WriteMessage(websocket.TextMessage, b)
}
