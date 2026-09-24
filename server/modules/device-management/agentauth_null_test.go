package devicemanagement

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mustTime is a fixed timestamp so the seeded rows are byte-for-byte stable
// across runs.
func mustTime(t *testing.T) time.Time {
	t.Helper()
	return time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
}

// Regression test for a failure that only appears once a device has enrolled
// but not yet reported a version.
//
// os_version, agent_version and site are all nullable in the schema, yet the
// Device struct originally declared them as plain string. The SQLite driver
// refuses to scan a NULL into a non-pointer string, so FindBySecretHash —
// the exact query every authenticated agent request depends on — returned an
// error and the device was rejected with 401 on every command, forever. The
// device could never fix it either, because it could not report a version
// without first authenticating.
func TestAuthenticateAgent_AcceptsDeviceWithUnsetNullableColumns(t *testing.T) {
	d := newTestDB(t)
	repo := NewRepository(d)
	ctx := context.Background()

	const secret = "secret-for-a-freshly-enrolled-device"

	// os_version, agent_version and site are all left NULL: exactly the state
	// of a device created by the enrollment token flow.
	if _, err := d.ExecContext(ctx, `
		INSERT INTO devices (id, hostname, os_name, status, enrolled_at, device_secret_hash, created_at, updated_at)
		VALUES ('dev-fresh', 'PC-FRESH', ?, ?, ?, ?, ?, ?)`,
		OSWindows, StatusOffline, mustTime(t), HashToken(secret), mustTime(t), mustTime(t),
	); err != nil {
		t.Fatalf("seed device: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agent/x", nil)
	req.Header.Set("X-Device-Id", "dev-fresh")
	req.Header.Set("X-Device-Secret", secret)
	rec := httptest.NewRecorder()

	deviceID, ok := AuthenticateAgent(rec, req, repo)
	if !ok {
		t.Fatalf("device with NULL os_version must still authenticate; got %d %s",
			rec.Code, rec.Body.String())
	}
	if deviceID != "dev-fresh" {
		t.Fatalf("expected device id dev-fresh, got %q", deviceID)
	}
}

// The device must also be rejected when the secret is wrong, even in this
// partially-populated state — the NULL columns must not change the auth path.
func TestAuthenticateAgent_RejectsWrongSecretWithUnsetNullableColumns(t *testing.T) {
	d := newTestDB(t)
	repo := NewRepository(d)
	ctx := context.Background()

	if _, err := d.ExecContext(ctx, `
		INSERT INTO devices (id, hostname, os_name, status, enrolled_at, device_secret_hash, created_at, updated_at)
		VALUES ('dev-fresh', 'PC-FRESH', ?, ?, ?, ?, ?, ?)`,
		OSWindows, StatusOffline, mustTime(t), HashToken("the-real-secret"), mustTime(t), mustTime(t),
	); err != nil {
		t.Fatalf("seed device: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agent/x", nil)
	req.Header.Set("X-Device-Id", "dev-fresh")
	req.Header.Set("X-Device-Secret", "a-guess")
	rec := httptest.NewRecorder()

	if _, ok := AuthenticateAgent(rec, req, repo); ok {
		t.Fatal("wrong secret must be rejected")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// The accessors must report the empty string rather than panicking when a
// caller reaches for a value the device has not supplied.
func TestDeviceAccessorsFlattenNullToEmptyString(t *testing.T) {
	var d Device
	if got := d.OSVersionString(); got != "" {
		t.Errorf("OSVersionString() = %q, want empty", got)
	}
	if got := d.AgentVersionString(); got != "" {
		t.Errorf("AgentVersionString() = %q, want empty", got)
	}
	if got := d.SiteValue(); got != "" {
		t.Errorf("SiteValue() = %q, want empty", got)
	}

	v := "11.0"
	d = Device{OSVersion: &v, AgentVersion: &v, Site: &v}
	if d.OSVersionString() != v || d.AgentVersionString() != v || d.SiteValue() != v {
		t.Error("accessors must pass through a present value unchanged")
	}
}
