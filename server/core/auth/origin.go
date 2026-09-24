package auth

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// ValidateWebSocketOrigin verifies that incoming WebSocket upgrade requests
// originate from trusted origins to prevent Cross-Site WebSocket Hijacking (CSWSH).
func ValidateWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser clients (native Go agent, CLI, curl) do not send an Origin header.
		return true
	}

	u, err := url.Parse(origin)
	if err != nil {
		log.Warn().Str("origin", origin).Msg("ws upgrade rejected: invalid origin URL")
		return false
	}

	originHost := u.Hostname()
	if originHost == "" {
		log.Warn().Str("origin", origin).Msg("ws upgrade rejected: empty origin host")
		return false
	}

	// 1. Allow localhost and loopback interfaces for local development and testing
	if originHost == "localhost" || originHost == "127.0.0.1" || originHost == "::1" {
		return true
	}

	// 2. Allow same-host origin matching current HTTP Host header
	reqHost, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		reqHost = r.Host
	}
	if strings.EqualFold(originHost, reqHost) {
		return true
	}

	// 3. Allow enterprise wildcard domain *.esta.co.id and esta.co.id
	if strings.EqualFold(originHost, "esta.co.id") || strings.HasSuffix(strings.ToLower(originHost), ".esta.co.id") {
		return true
	}

	log.Warn().Str("origin", origin).Str("host", r.Host).Msg("ws upgrade rejected: untrusted origin")
	return false
}
