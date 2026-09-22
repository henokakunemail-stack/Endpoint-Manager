package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/endpoint-mgmt/server/core/audit"
)

// chiRouter is the subset of *chi.Mux used by Register. Declaring it as an
// interface lets Register accept both chi and net/http muxes without forcing a
// hard dependency on a single router type at this layer.
type chiRouter interface {
	Post(pattern string, handlerFn http.HandlerFunc)
	Get(pattern string, handlerFn http.HandlerFunc)
}

// LoginHandler issues JWTs for admin console users.
type LoginHandler struct {
	db  *sqlx.DB
	jwt *JWTService
}

func NewLoginHandler(db *sqlx.DB, jwt *JWTService) *LoginHandler {
	return &LoginHandler{db: db, jwt: jwt}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register mounts the auth endpoints. Accepts any standard mux that supports
// the Go 1.22+ "METHOD /path" pattern (http.ServeMux) or chi routes.
func (h *LoginHandler) Register(mux any) {
	switch m := mux.(type) {
	case *http.ServeMux:
		m.HandleFunc("POST /api/auth/login", h.login)
		m.HandleFunc("POST /api/auth/refresh", h.refresh)
	case chiRouter:
		m.Post("/api/auth/login", h.login)
		m.Post("/api/auth/refresh", h.refresh)
	default:
		panic("auth.LoginHandler.Register: unsupported mux type")
	}
}

func (h *LoginHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "username and password required")
		return
	}

	var u struct {
		ID           string `db:"id"`
		Username     string `db:"username"`
		PasswordHash string `db:"password_hash"`
		Role         string `db:"role"`
		Status       string `db:"status"`
	}
	err := h.db.GetContext(r.Context(), &u, `
		SELECT u.id, u.username, u.password_hash, u.role,
		       CASE WHEN u.password_hash = '' OR u.password_hash IS NULL THEN 'disabled'
		            ELSE 'active' END AS status
		FROM users u WHERE u.username = ?`, req.Username)
	if err != nil {
		// Do not leak whether the username exists.
		log.Debug().Err(err).Str("username", req.Username).Msg("login unknown user")
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if u.Status != "active" {
		writeErr(w, http.StatusUnauthorized, "account is disabled")
		return
	}
	if err := ComparePassword(u.PasswordHash, req.Password); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	pair, err := h.jwt.Issue(u.ID, u.Username, u.Role)
	if err != nil {
		log.Error().Err(err).Msg("issue jwt")
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	_ = audit.Log(r.Context(), h.db, "user", u.ID, "auth.login", u.ID, nil)
	writeJSON(w, http.StatusOK, pair)
}

func (h *LoginHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeErr(w, http.StatusBadRequest, "refresh_token required")
		return
	}
	claims, err := h.jwt.Parse(req.RefreshToken)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	pair, err := h.jwt.Issue(claims.UserID, claims.Username, claims.Role)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
	_ = time.Now
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
