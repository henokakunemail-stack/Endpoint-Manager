package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/endpoint-mgmt/server/core/rbac"
)

type contextKey string

const (
	// CtxUserID / CtxUsername are set on the request context by RequireAuth.
	CtxUserID   contextKey = "uid"
	CtxUsername contextKey = "usr"
)

// RequireAuth validates the Authorization: Bearer <token> header (or ?token= query parameter)
// and stores the claims in the request context.
func (s *JWTService) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken := ""
		h := r.Header.Get("Authorization")
		if h != "" {
			parts := strings.SplitN(h, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				rawToken = parts[1]
			}
		}
		if rawToken == "" {
			rawToken = r.URL.Query().Get("token")
		}
		if rawToken == "" {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		claims, err := s.Parse(rawToken)
		if err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxUserID, claims.UserID)
		ctx = context.WithValue(ctx, CtxUsername, claims.Username)
		// rbac.RequireRole reads the role from its own key.
		ctx = rbac.WithRole(ctx, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext returns the authenticated user's ID.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CtxUserID).(string); ok {
		return v
	}
	return ""
}
