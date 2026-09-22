package devicemanagement

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/endpoint-mgmt/server/core/audit"
	"github.com/endpoint-mgmt/server/core/auth"
	"github.com/endpoint-mgmt/server/core/rbac"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Handler exposes the device-management HTTP API.
type Handler struct {
	repo   *Repository
	db     *sqlx.DB
	jwt    *auth.JWTService
	ttl    time.Duration // enrollment token TTL
}

func NewHandler(repo *Repository, db *sqlx.DB, jwt *auth.JWTService, enrollmentTTL time.Duration) *Handler {
	return &Handler{repo: repo, db: db, jwt: jwt, ttl: enrollmentTTL}
}

// Register mounts routes on the given router. Auth/RBAC middleware must be applied
// by the caller for protected routes (see cmd/server/main.go).
func (h *Handler) Register(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.With(h.jwt.RequireAuth, rbac.RequireRole(rbac.RoleViewer)).Get("/devices", h.listDevices)
		r.With(h.jwt.RequireAuth, rbac.RequireRole(rbac.RoleViewer)).Get("/devices/{id}", h.getDevice)
		// Creating enrollment tokens is admin-only.
		r.With(h.jwt.RequireAuth, rbac.RequireRole(rbac.RoleAdmin)).Post("/devices/enroll-token", h.createEnrollToken)
		r.With(h.jwt.RequireAuth, rbac.RequireRole(rbac.RoleTechnician)).Get("/audit-logs", h.listAudit)
	})
}

// deviceDTO is the API representation of a device. Never leaks secret hashes.
type deviceDTO struct {
	ID           string     `json:"id"`
	Hostname     string     `json:"hostname"`
	OSName       string     `json:"os_name"`
	OSVersion    string     `json:"os_version"`
	AgentVersion string     `json:"agent_version"`
	Status       string     `json:"status"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	Site         string     `json:"site"`
	EnrolledAt   time.Time  `json:"enrolled_at"`
}

func toDTO(d Device) deviceDTO {
	return deviceDTO{
		ID: d.ID, Hostname: d.Hostname, OSName: d.OSName, OSVersion: d.OSVersion,
		AgentVersion: d.AgentVersion, Status: d.Status, LastSeenAt: d.LastSeenAt,
		Site: d.Site, EnrolledAt: d.EnrolledAt,
	}
}

func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	site := r.URL.Query().Get("site")
	devices, err := h.repo.List(r.Context(), status, site)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]deviceDTO, 0, len(devices))
	for _, d := range devices {
		dtos = append(dtos, toDTO(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": dtos, "count": len(dtos)})
}

func (h *Handler) getDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := h.repo.GetByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDTO(d))
}

type createEnrollTokenReq struct {
	Hostname string `json:"hostname"`
	OSName   string `json:"os_name"`
	Site     string `json:"site"`
}

type createEnrollTokenResp struct {
	DeviceID        string    `json:"device_id"`
	EnrollmentToken string    `json:"enrollment_token"`
	ExpiresAt       time.Time `json:"expires_at"`
}

// createEnrollToken pre-registers a device and returns a one-time enrollment token
// the agent exchanges for a persistent secret. Only the token hash is stored.
func (h *Handler) createEnrollToken(w http.ResponseWriter, r *http.Request) {	var req createEnrollTokenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Hostname == "" || req.OSName == "" {
		writeErr(w, http.StatusBadRequest, "hostname and os_name are required")
		return
	}
	switch req.OSName {
	case OSWindows, OSLinux, OSMacOS:
	default:
		writeErr(w, http.StatusBadRequest, "os_name must be windows, linux or macos")
		return
	}

	plain := GenerateToken()
	now := time.Now().UTC()
	tokenHash := HashToken(plain) // consumed to NULL once the agent enrolls
	dev := Device{
		ID:                  NewID(),
		Hostname:            req.Hostname,
		OSName:              req.OSName,
		Status:              StatusOffline,
		EnrolledAt:          now,
		EnrollmentTokenHash: &tokenHash,
		Site:                req.Site,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := h.repo.Create(r.Context(), dev); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	actor := auth.UserIDFromContext(r.Context())
	_ = audit.Log(r.Context(), h.db, "user", actor, "device.enroll_token_created", dev.ID, map[string]string{
		"hostname": req.Hostname, "os_name": req.OSName, "site": req.Site,
	})

	writeJSON(w, http.StatusCreated, createEnrollTokenResp{
		DeviceID:        dev.ID,
		EnrollmentToken: plain,
		ExpiresAt:       now.Add(h.ttl),
	})
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	entries, err := audit.List(r.Context(), h.db, 100)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": entries, "count": len(entries)})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
