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

// RequireAuth validates the Authorization: Bearer <token> header and stores
// the claims in the request context.
func (s *JWTService) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}
		claims, err := s.Parse(parts[1])
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
