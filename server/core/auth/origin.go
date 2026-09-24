package auth

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// OriginChecker decides whether a WebSocket upgrade may proceed. The
// configured implementation is built by NewOriginChecker, which folds in the
// operator's ALLOWED_ORIGIN_DOMAINS list.
type OriginChecker func(r *http.Request) bool

// NewOriginChecker builds a CSWSH origin validator from the configured domain
// list. An empty list means only loopback and same-host origins are accepted,
// which is the correct default: the server should never trust a domain that
// nobody explicitly configured.
func NewOriginChecker(allowedDomains []string) OriginChecker {
	// Normalise once, at startup, rather than on every handshake.
	exact := make(map[string]struct{}, len(allowedDomains))
	wildcards := make([]string, 0, len(allowedDomains))
	for _, d := range allowedDomains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if rest, ok := strings.CutPrefix(d, "*."); ok && rest != "" {
			wildcards = append(wildcards, rest)
			continue
		}
		exact[d] = struct{}{}
	}

	return func(r *http.Request) bool {
		return validateOrigin(r, exact, wildcards)
	}
}

// ValidateWebSocketOrigin verifies that incoming WebSocket upgrade requests
// originate from trusted origins to prevent Cross-Site WebSocket Hijacking
// (CSWSH). It only trusts loopback and same-host origins; deployments that serve
// a console from a different hostname must build their checker with
// NewOriginChecker so the extra domains are actually allowed.
func ValidateWebSocketOrigin(r *http.Request) bool {
	return validateOrigin(r, nil, nil)
}

func validateOrigin(r *http.Request, exact map[string]struct{}, wildcards []string) bool {
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
	originHost = strings.ToLower(originHost)

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

	// 3. Allow any operator-configured domain. Exact entries match the host
	// itself; a "*.example.com" entry matches a subdomain of example.com.
	// Matching is on a label boundary, so a configured "example.com" never
	// matches "example.com.attacker.example" or "notexample.com".
	if _, ok := exact[originHost]; ok {
		return true
	}
	for _, suffix := range wildcards {
		if strings.HasSuffix(originHost, "."+suffix) {
			return true
		}
	}

	log.Warn().Str("origin", origin).Str("host", r.Host).Msg("ws upgrade rejected: untrusted origin")
	return false
}
