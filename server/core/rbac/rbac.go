package rbac

import (
	"context"
	"net/http"
)

// Role levels. Higher = more privileged.
const (
	RoleViewer    = "viewer"
	RoleTechnician = "technician"
	RoleAdmin     = "admin"
)

var rank = map[string]int{
	RoleViewer:    1,
	RoleTechnician: 2,
	RoleAdmin:     3,
}

// RequireRole returns middleware allowing roles with rank >= minRole.
// It must be chained after auth.RequireAuth so the role is in the context.
func RequireRole(minRole string) func(http.Handler) http.Handler {
	minRank, ok := rank[minRole]
	if !ok {
		panic("rbac: unknown role " + minRole)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(roleKey{}).(string)
			if role == "" {
				http.Error(w, "unauthorized: no role in context", http.StatusForbidden)
				return
			}
			got, ok := rank[role]
			if !ok || got < minRank {
				http.Error(w, "forbidden: role '"+role+"' insufficient", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type roleKey struct{}

// WithRole stores the role in context. Used by auth middleware adapters.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey{}, role)
}
