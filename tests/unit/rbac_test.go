package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/endpoint-mgmt/server/core/rbac"
)

func runWithRole(role string, minRole string, t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	mw := rbac.RequireRole(minRole)
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	ctx := context.Background()
	if role != "" {
		ctx = rbac.WithRole(ctx, role)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	t.Logf("role=%q min=%q -> called=%v code=%d", role, minRole, called, rec.Code)
	return rec
}

func TestRBACHierarchy(t *testing.T) {
	if code := runWithRole(rbac.RoleAdmin, rbac.RoleAdmin, t).Code; code != http.StatusOK {
		t.Fatalf("admin vs admin: expected 200, got %d", code)
	}
	if code := runWithRole(rbac.RoleAdmin, rbac.RoleTechnician, t).Code; code != http.StatusOK {
		t.Fatalf("admin vs technician: expected 200, got %d", code)
	}
	if code := runWithRole(rbac.RoleViewer, rbac.RoleAdmin, t).Code; code != http.StatusForbidden {
		t.Fatalf("viewer vs admin: expected 403, got %d", code)
	}
	if code := runWithRole(rbac.RoleTechnician, rbac.RoleViewer, t).Code; code != http.StatusOK {
		t.Fatalf("technician vs viewer: expected 200, got %d", code)
	}
	if code := runWithRole("", rbac.RoleViewer, t).Code; code != http.StatusForbidden {
		t.Fatalf("no role vs viewer: expected 403, got %d", code)
	}
}
