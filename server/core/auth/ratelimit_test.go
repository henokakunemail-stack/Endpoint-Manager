package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIPRateLimiter_LockoutAndRecovery(t *testing.T) {
	// 3 max attempts within 200ms, lockout for 300ms
	limiter := NewIPRateLimiter(3, 200*time.Millisecond, 300*time.Millisecond)
	defer limiter.Close()

	ip := "192.168.1.100"

	// Initially allowed
	if allowed, _ := limiter.IsAllowed(ip); !allowed {
		t.Fatal("expected IP to be initially allowed")
	}

	// Record 2 failures -> should still be allowed
	limiter.RecordFailure(ip)
	limiter.RecordFailure(ip)
	if allowed, _ := limiter.IsAllowed(ip); !allowed {
		t.Fatal("expected IP to still be allowed after 2 failures")
	}

	// 3rd failure -> locks out
	limiter.RecordFailure(ip)
	allowed, remaining := limiter.IsAllowed(ip)
	if allowed {
		t.Fatal("expected IP to be locked out after 3 failures")
	}
	if remaining <= 0 {
		t.Fatalf("expected positive remaining lockout time, got %v", remaining)
	}

	// Another IP is not locked out
	if allowed, _ := limiter.IsAllowed("192.168.1.101"); !allowed {
		t.Fatal("unrelated IP should not be locked out")
	}

	// Wait for lockout to expire
	time.Sleep(350 * time.Millisecond)
	if allowed, _ := limiter.IsAllowed(ip); !allowed {
		t.Fatal("expected IP to be allowed after lockout expired")
	}
}

func TestIPRateLimiter_SuccessResets(t *testing.T) {
	limiter := NewIPRateLimiter(3, 1*time.Minute, 1*time.Minute)
	defer limiter.Close()

	ip := "10.0.0.50"
	limiter.RecordFailure(ip)
	limiter.RecordFailure(ip)

	// Record success
	limiter.RecordSuccess(ip)

	// Should allow 2 more failures without locking out
	limiter.RecordFailure(ip)
	limiter.RecordFailure(ip)
	if allowed, _ := limiter.IsAllowed(ip); !allowed {
		t.Fatal("expected IP to be allowed after reset")
	}
}

func TestValidateWebSocketOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		host    string
		allowed bool
	}{
		{
			name:    "empty origin (native agent / CLI)",
			origin:  "",
			host:    "endpoint.esta.co.id",
			allowed: true,
		},
		{
			name:    "localhost origin",
			origin:  "http://localhost:8443",
			host:    "localhost:8443",
			allowed: true,
		},
		{
			name:    "loopback 127.0.0.1 origin",
			origin:  "http://127.0.0.1:5173",
			host:    "127.0.0.1:8443",
			allowed: true,
		},
		{
			name:    "wildcard domain esta.co.id sub-domain",
			origin:  "https://endpoint.esta.co.id",
			host:    "endpoint.esta.co.id:443",
			allowed: true,
		},
		{
			name:    "wildcard domain console.esta.co.id",
			origin:  "https://console.esta.co.id",
			host:    "endpoint.esta.co.id",
			allowed: true,
		},
		{
			name:    "same host origin",
			origin:  "https://corp-mgmt.internal:8443",
			host:    "corp-mgmt.internal:8443",
			allowed: true,
		},
		{
			name:    "malicious external origin",
			origin:  "https://evil-attacker.com",
			host:    "endpoint.esta.co.id",
			allowed: false,
		},
		{
			name:    "subdomain spoofing attempt",
			origin:  "https://esta.co.id.attacker.com",
			host:    "endpoint.esta.co.id",
			allowed: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			got := ValidateWebSocketOrigin(req)
			if got != tc.allowed {
				t.Errorf("ValidateWebSocketOrigin(%q, host=%q) = %v, want %v", tc.origin, tc.host, got, tc.allowed)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	if ip := ClientIP(req); ip != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %q", ip)
	}

	// With X-Forwarded-For
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
	if ip := ClientIP(req); ip != "203.0.113.195" {
		t.Fatalf("expected 203.0.113.195, got %q", ip)
	}

	// With X-Real-IP
	req.Header.Del("X-Forwarded-For")
	req.Header.Set("X-Real-IP", "198.51.100.42")
	if ip := ClientIP(req); ip != "198.51.100.42" {
		t.Fatalf("expected 198.51.100.42, got %q", ip)
	}
}
