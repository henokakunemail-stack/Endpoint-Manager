package integration

// E2E test for the admin console auth path: login -> JWT -> RBAC enforcement.
// Verifies that a viewer cannot perform admin-only actions such as creating
// enrollment tokens, and that token verification rejects tampered tokens.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"github.com/endpoint-mgmt/server/core/auth"
	"github.com/endpoint-mgmt/server/core/db"
	"github.com/endpoint-mgmt/server/core/rbac"
	devicemgmt "github.com/endpoint-mgmt/server/modules/device-management"
)

func newAuthEnv(t *testing.T) (*httptest.Server, *sqlx.DB, *auth.JWTService) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "auth-e2e.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	jwtSvc := auth.NewJWTService("e2e-secret-0123456789abcdef", time.Minute, time.Hour)
	repo := devicemgmt.NewRepository(d)
	deviceH := devicemgmt.NewHandler(repo, d, jwtSvc, 30*time.Minute)

	r := chi.NewRouter()
	loginH := auth.NewLoginHandler(d, jwtSvc)
	loginH.Register(r)
	deviceH.Register(r)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, d, jwtSvc
}

func TestE2ELoginAndRBAC(t *testing.T) {
	ts, d, _ := newAuthEnv(t)

	// Seed an admin and a viewer directly in the DB.
	adminID := seedUser(t, d, "adminalice", "pass-admin", rbac.RoleAdmin)
	viewerID := seedUser(t, d, "viewerbob", "pass-viewer", rbac.RoleViewer)
	t.Logf("seeded admin=%s viewer=%s", adminID, viewerID)

	// --- viewer logs in ---
	viewerTok := login(t, ts.URL, "viewerbob", "pass-viewer")
	if viewerTok == "" {
		t.Fatal("viewer login returned no token")
	}

	// --- admin logs in ---
	adminTok := login(t, ts.URL, "adminalice", "pass-admin")
	if adminTok == "" {
		t.Fatal("admin login returned no token")
	}

	// --- wrong password must be rejected ---
	if tok := login(t, ts.URL, "adminalice", "wrong"); tok != "" {
		t.Fatalf("login with wrong password must fail, got token %q", tok)
	}

	// --- viewer CANNOT create enrollment tokens (403) ---
	if code := tryCreateEnrollToken(t, ts.URL, viewerTok); code != http.StatusForbidden {
		t.Fatalf("viewer creating enroll token: expected 403, got %d", code)
	}

	// --- admin CAN create enrollment tokens (201) ---
	code, body := createEnrollToken(t, ts.URL, adminTok)
	if code != http.StatusCreated {
		t.Fatalf("admin creating enroll token: expected 201, got %d (body=%s)", code, body)
	}
	var resp struct {
		DeviceID        string `json:"device_id"`
		EnrollmentToken string `json:"enrollment_token"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode enroll token response: %v", err)
	}
	if resp.EnrollmentToken == "" {
		t.Fatal("enrollment token must not be empty")
	}
	t.Logf("admin created enroll token for device %s", resp.DeviceID)

	// --- a tampered token must be rejected ---
	parts := strings.SplitN(adminTok, ".", 3)
	if len(parts) != 3 {
		t.Fatalf("unexpected JWT shape: %q", adminTok)
	}
	parts[1] = strings.Repeat("A", len(parts[1]))
	tampered := strings.Join(parts, ".")
	if code := tryCreateEnrollToken(t, ts.URL, tampered); code != http.StatusUnauthorized {
		t.Fatalf("tampered token: expected 401, got %d", code)
	}

	// --- no token at all must be rejected ---
	if code := tryCreateEnrollToken(t, ts.URL, ""); code != http.StatusUnauthorized {
		t.Fatalf("missing token: expected 401, got %d", code)
	}

	// --- viewer CAN list devices (read allowed) ---
	if code := listDevices(t, ts.URL, viewerTok); code != http.StatusOK {
		t.Fatalf("viewer listing devices: expected 200, got %d", code)
	}
}

func seedUser(t *testing.T, d *sqlx.DB, username, password, role string) string {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	id := devicemgmt.NewID()
	now := time.Now().UTC()
	_, err = d.Exec(`INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, username, hash, role, now, now)
	if err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	return id
}

// login performs POST /api/auth/login and returns the access token ("").
func login(t *testing.T, baseURL, username, password string) string {
	t.Helper()
	body, err := postJSON(baseURL+"/api/auth/login", map[string]string{
		"username": username, "password": password,
	})
	if err != nil {
		// Expected for wrong credentials: report as failure only for success cases.
		t.Logf("login %s failed (may be expected): %v", username, err)
		return ""
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode login response for %s: %v (body=%s)", username, err, body)
	}
	return out.AccessToken
}

func authReq(method, url, token string, body any) (*http.Response, []byte) {
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return nil, nil
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp, buf.Bytes()
}

func tryCreateEnrollToken(t *testing.T, baseURL, token string) int {
	t.Helper()
	resp, _ := authReq(http.MethodPost, baseURL+"/api/devices/enroll-token", token,
		map[string]string{"hostname": "X", "os_name": "windows"})
	if resp == nil {
		t.Fatal("request failed entirely")
	}
	return resp.StatusCode
}

func createEnrollToken(t *testing.T, baseURL, token string) (int, []byte) {
	t.Helper()
	resp, body := authReq(http.MethodPost, baseURL+"/api/devices/enroll-token", token,
		map[string]string{"hostname": "PC-E2E", "os_name": "windows", "site": "pusat"})
	if resp == nil {
		t.Fatal("request failed entirely")
	}
	return resp.StatusCode, body
}

func listDevices(t *testing.T, baseURL, token string) int {
	t.Helper()
	resp, _ := authReq(http.MethodGet, baseURL+"/api/devices", token, nil)
	if resp == nil {
		t.Fatal("request failed entirely")
	}
	return resp.StatusCode
}
