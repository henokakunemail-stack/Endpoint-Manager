// Package transport provides the agent-side connection to the central server.
// The agent always initiates the connection (outbound), satisfying the
// constraint that the server never reaches into branch networks.
package transport

import (
	"context"
	"encoding/json"
	"sync"
)

// CommandHandler runs one server-issued command and returns its result.
// Return nil to report failure.
type CommandHandler func(ctx context.Context, command, id string, payload json.RawMessage) any

// Dispatcher routes server commands to registered handlers by command type.
// Later modules (patch, software deployment, inventory) register their own
// command types without touching the transport client.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]CommandHandler
	fallback CommandHandler
}

// NewDispatcher creates a dispatcher with no registered handlers.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[string]CommandHandler)}
}

// Register maps a command type to its handler. Overwriting is allowed so tests
// and modules can swap implementations.
func (d *Dispatcher) Register(commandType string, h CommandHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[commandType] = h
}

// OnUnknown installs a fallback for unregistered command types.
func (d *Dispatcher) OnUnknown(h CommandHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fallback = h
}

// Handle looks up the handler for a command and runs it.
func (d *Dispatcher) Handle(ctx context.Context, command, id string, payload json.RawMessage) any {
	d.mu.RLock()
	h, ok := d.handlers[command]
	fallback := d.fallback
	d.mu.RUnlock()
	if !ok {
		if fallback != nil {
			return fallback(ctx, command, id, payload)
		}
		return map[string]string{"error": "unsupported command: " + command}
	}
	return h(ctx, command, id, payload)
}
