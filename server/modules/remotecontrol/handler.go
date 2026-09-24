package remotecontrol

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/endpoint-mgmt/server/core/auth"
	"github.com/endpoint-mgmt/server/core/rbac"
	"github.com/endpoint-mgmt/server/core/transport"
	devicemgmt "github.com/endpoint-mgmt/server/modules/device-management"
)

type Hub interface {
	Online(deviceID string) bool
	SendTo(deviceID string, msg []byte) bool
}

type AuditLogger interface {
	Log(ctx context.Context, actorType, actorID, action, targetID string, details map[string]string) error
}

type Handler struct {
	repo           *Repository
	relay          *RelayManager
	hub            Hub
	devices        *devicemgmt.Repository
	audit          AuditLogger
	jwtSvc         *auth.JWTService
	authMiddleware func(http.Handler) http.Handler
	upgrader       websocket.Upgrader
}

func NewHandler(
	repo *Repository,
	relay *RelayManager,
	hub Hub,
	devices *devicemgmt.Repository,
	audit AuditLogger,
	jwtSvc *auth.JWTService,
	authMiddleware func(http.Handler) http.Handler,
) *Handler {
	return &Handler{
		repo:           repo,
		relay:          relay,
		hub:            hub,
		devices:        devices,
		audit:          audit,
		jwtSvc:         jwtSvc,
		authMiddleware: authMiddleware,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Handler) Register(r chi.Router) {
	// 1. Session initialization and management (Technician minimum)
	r.With(h.authMiddleware, rbac.RequireRole(rbac.RoleTechnician)).
		Post("/api/devices/{id}/remotecontrol/session", h.startSession)

	r.With(h.authMiddleware, rbac.RequireRole(rbac.RoleTechnician)).
		Post("/api/devices/{id}/remotecontrol/sessions/{sessionId}/stop", h.stopSession)

	// 2. Session history (Viewer minimum)
	r.With(h.authMiddleware).
		Get("/api/devices/{id}/remotecontrol/sessions", h.listSessions)

	// 3. Technician Interactive WebSocket Relay (auth via query param token)
	r.Get("/api/devices/{id}/remotecontrol/ws", h.handleOperatorWS)

	// 4. Agent Streaming WebSocket Relay (auth via session ID check)
	r.Get("/api/agent/devices/{id}/remotecontrol/ws", h.handleAgentWS)
}

type startSessionReq struct {
	Mode string `json:"mode"` // 'full_control' or 'view_only'
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	if _, err := h.devices.GetByID(r.Context(), deviceID); err != nil {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}

	if h.hub == nil || !h.hub.Online(deviceID) {
		writeErr(w, http.StatusConflict, "device is offline")
		return
	}

	var req startSessionReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Mode == "" {
		req.Mode = "full_control"
	}
	if req.Mode != "full_control" && req.Mode != "view_only" {
		writeErr(w, http.StatusBadRequest, "invalid mode (must be full_control or view_only)")
		return
	}

	operatorID := auth.UserIDFromContext(r.Context())
	session := &RemoteControlSession{
		DeviceID:    deviceID,
		OperatorID:  operatorID,
		SessionMode: req.Mode,
	}

	if err := h.repo.CreateSession(r.Context(), session); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create session: "+err.Error())
		return
	}

	// Register relay session
	h.relay.RegisterSession(session.ID, session.SessionMode)

	// Notify agent via WebSocket command
	cmdPayload := map[string]any{
		"session_id": session.ID,
		"mode":       session.SessionMode,
		"relay_url":  "/api/agent/devices/" + deviceID + "/remotecontrol/ws?session=" + session.ID,
	}
	env := transport.Envelope{
		Type:    transport.TypeCommand,
		ID:      session.ID,
		Command: "rc.start",
		Payload: cmdPayload,
	}
	envBytes, _ := json.Marshal(env)
	h.hub.SendTo(deviceID, envBytes)

	_ = h.audit.Log(r.Context(), "user", operatorID, "remotecontrol.session_start", session.ID, map[string]string{
		"device_id": deviceID,
		"mode":      session.SessionMode,
	})

	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) stopSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	h.relay.CloseRelay(sessionID)

	operatorID := auth.UserIDFromContext(r.Context())
	_ = h.audit.Log(r.Context(), "user", operatorID, "remotecontrol.session_stop", sessionID, nil)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ended"})
}

func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	sessions, err := h.repo.ListSessionsByDevice(r.Context(), deviceID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sessions == nil {
		sessions = []RemoteControlSession{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *Handler) handleOperatorWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	sessionID := r.URL.Query().Get("session")

	if token == "" || sessionID == "" {
		http.Error(w, "missing token or session", http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtSvc.Parse(token)
	if err != nil || claims.Role == rbac.RoleViewer {
		http.Error(w, "unauthorized or insufficient privileges", http.StatusForbidden)
		return
	}

	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("upgrade operator remote control ws")
		return
	}
	defer ws.Close()

	relay, ok := h.relay.AttachOperator(sessionID, ws)
	if !ok {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "failed to attach to relay"))
		return
	}

	// Read operator input events and forward to agent
	for {
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if err := relay.ForwardOperatorInput(msgType, msg); err != nil {
			break
		}
	}

	h.relay.CloseRelay(sessionID)
}

func (h *Handler) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		http.Error(w, "missing session param", http.StatusBadRequest)
		return
	}

	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("upgrade agent remote control ws")
		return
	}
	defer ws.Close()

	relay, ok := h.relay.AttachAgent(sessionID, ws)
	if !ok {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "failed to attach to relay"))
		return
	}

	// Read screen frames from agent and forward to operator
	for {
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if err := relay.ForwardAgentFrame(msgType, msg); err != nil {
			break
		}
	}

	h.relay.CloseRelay(sessionID)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
