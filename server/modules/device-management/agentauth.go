package devicemanagement

import (
	"context"
	"encoding/json"
	"net/http"
)

// SecretLookup is the minimal repository surface AuthenticateAgent needs.
type SecretLookup interface {
	FindBySecretHash(ctx context.Context, secretHash string) (Device, error)
}

// AuthenticateAgent verifies the X-Device-Id / X-Device-Secret pair presented by
// an agent on an agent-facing HTTP endpoint.
//
// Those endpoints are authenticated by the per-device secret rather than a user
// JWT, because agents have no interactive user. Every such handler must call
// this before trusting the request body: without it any client that can reach
// the port can report progress for arbitrary tasks, mark a rollout as
// successful, or download packages.
//
// It returns the authenticated device ID. On failure it has already written the
// HTTP error response and the caller must return immediately.
func AuthenticateAgent(w http.ResponseWriter, r *http.Request, devices SecretLookup) (string, bool) {
	deviceID := r.Header.Get("X-Device-Id")
	secret := r.Header.Get("X-Device-Secret")
	if deviceID == "" || secret == "" {
		writeAgentAuthError(w, http.StatusUnauthorized, "missing device credentials")
		return "", false
	}

	dev, err := devices.FindBySecretHash(r.Context(), HashToken(secret))
	if err != nil || dev.ID != deviceID {
		// A single message for both cases: distinguishing "no such device" from
		// "no such secret" would let a caller probe which device IDs exist, and
		// a valid secret paired with the wrong device ID is still a rejection.
		writeAgentAuthError(w, http.StatusUnauthorized, "invalid device credentials")
		return "", false
	}
	return dev.ID, true
}

func writeAgentAuthError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
