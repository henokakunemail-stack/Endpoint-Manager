package transport

import (
	"os"
	"testing"
	"time"
)

func TestGetTLSConfig_Defaults(t *testing.T) {
	os.Unsetenv("AGENT_INSECURE_SKIP_VERIFY")
	os.Unsetenv("AGENT_CA_FILE")

	cfg := GetTLSConfig()
	if cfg == nil {
		t.Fatal("expected non-nil tls.Config")
	}
	if cfg.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify=false, got true")
	}
}

func TestGetTLSConfig_InsecureSkipVerify(t *testing.T) {
	os.Setenv("AGENT_INSECURE_SKIP_VERIFY", "1")
	defer os.Unsetenv("AGENT_INSECURE_SKIP_VERIFY")

	cfg := GetTLSConfig()
	if !cfg.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify=true when AGENT_INSECURE_SKIP_VERIFY=1")
	}

	os.Setenv("AGENT_INSECURE_SKIP_VERIFY", "true")
	cfg2 := GetTLSConfig()
	if !cfg2.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify=true when AGENT_INSECURE_SKIP_VERIFY=true")
	}
}

func TestNewHTTPClient_Configuration(t *testing.T) {
	client := NewHTTPClient(10 * time.Second)
	if client == nil {
		t.Fatal("expected non-nil http.Client")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("expected Timeout=10s, got %v", client.Timeout)
	}
}

func TestNewWebSocketDialer_Configuration(t *testing.T) {
	dialer := NewWebSocketDialer(20 * time.Second)
	if dialer == nil {
		t.Fatal("expected non-nil websocket.Dialer")
	}
	if dialer.HandshakeTimeout != 20*time.Second {
		t.Errorf("expected HandshakeTimeout=20s, got %v", dialer.HandshakeTimeout)
	}
	if dialer.TLSClientConfig == nil {
		t.Errorf("expected non-nil TLSClientConfig on dialer")
	}
}
