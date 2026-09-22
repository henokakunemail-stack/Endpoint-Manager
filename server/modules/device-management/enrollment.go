package devicemanagement

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/endpoint-mgmt/server/core/audit"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// chiRouter is the subset of *chi.Mux used by Register (see auth.login.go).
type chiRouter interface {
	Post(pattern string, handlerFn http.HandlerFunc)
	Get(pattern string, handlerFn http.HandlerFunc)
}

// EnrollmentHandler handles POST /api/agent/enroll — the one-time token exchange.
// It lives in device-management because enrollment is how a device joins the fleet.
type EnrollmentHandler struct {
	repo *Repository
	db   *sqlx.DB
}

func NewEnrollmentHandler(repo *Repository, db *sqlx.DB) *EnrollmentHandler {
	return &EnrollmentHandler{repo: repo, db: db}
}

// Register mounts the enrollment endpoint. Accepts *http.ServeMux or chi mux.
func (h *EnrollmentHandler) Register(mux any) {
	switch m := mux.(type) {
	case *http.ServeMux:
		m.HandleFunc("POST /api/agent/enroll", h.enroll)
	case chiRouter:
		m.Post("/api/agent/enroll", h.enroll)
	default:
		panic("device-management.EnrollmentHandler.Register: unsupported mux type")
	}
}

type enrollRequest struct {
	EnrollmentToken string `json:"enrollment_token"`
	Hostname        string `json:"hostname"`
	OSName          string `json:"os_name"`
	OSVersion       string `json:"os_version"`
	AgentVersion    string `json:"agent_version"`
}

type enrollResponse struct {
	DeviceID     string `json:"device_id"`
	DeviceSecret string `json:"device_secret"`
}

// enroll exchanges a one-time enrollment token for a persistent device secret.
// The token is hashed on arrival; only the hash is stored, so the plaintext token
// and the generated secret are known only to the caller.
func (h *EnrollmentHandler) enroll(w http.ResponseWriter, r *http.Request) {
	var req enrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.EnrollmentToken == "" {
		writeErr(w, http.StatusBadRequest, "enrollment_token is required")
		return
	}

	tokenHash := HashToken(req.EnrollmentToken)
	// Confirm the token is registered before minting a secret.
	dev, err := h.repo.findByEnrollmentTokenHash(r.Context(), tokenHash)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusUnauthorized, "invalid or expired enrollment token")
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("lookup enrollment token")
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	plain := GenerateToken()
	if err := h.repo.ConsumeEnrollmentToken(r.Context(), tokenHash, HashToken(plain)); err != nil {
		log.Error().Err(err).Msg("consume enrollment token")
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	if req.OSVersion != "" || req.AgentVersion != "" {
		_ = h.repo.UpdateOSInfo(r.Context(), dev.ID, req.OSVersion, req.AgentVersion)
	}
	_ = audit.Log(r.Context(), h.db, "agent", dev.ID, "device.enroll", dev.ID, map[string]string{
		"hostname": req.Hostname, "os_name": req.OSName,
	})

	writeJSON(w, http.StatusOK, enrollResponse{DeviceID: dev.ID, DeviceSecret: plain})
}
