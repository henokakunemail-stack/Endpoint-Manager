package transport

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// GetTLSConfig returns a tls.Config honoring AGENT_INSECURE_SKIP_VERIFY
// and AGENT_CA_FILE environment variables for enterprise branch networks.
func GetTLSConfig() *tls.Config {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if val := os.Getenv("AGENT_INSECURE_SKIP_VERIFY"); val == "1" || strings.EqualFold(val, "true") {
		tlsCfg.InsecureSkipVerify = true
	}

	if caPath := os.Getenv("AGENT_CA_FILE"); caPath != "" {
		caData, err := os.ReadFile(caPath)
		if err == nil {
			pool, err := x509.SystemCertPool()
			if err != nil || pool == nil {
				pool = x509.NewCertPool()
			}
			pool.AppendCertsFromPEM(caData)
			tlsCfg.RootCAs = pool
		}
	}

	return tlsCfg
}

// NewHTTPClient returns an *http.Client configured with enterprise TLS and timeout.
func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSClientConfig:     GetTLSConfig(),
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// NewWebSocketDialer returns a *websocket.Dialer configured with enterprise TLS and timeout.
func NewWebSocketDialer(handshakeTimeout time.Duration) *websocket.Dialer {
	if handshakeTimeout <= 0 {
		handshakeTimeout = 15 * time.Second
	}
	return &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: handshakeTimeout,
		TLSClientConfig:  GetTLSConfig(),
	}
}
