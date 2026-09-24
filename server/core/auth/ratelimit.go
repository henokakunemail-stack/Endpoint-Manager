package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ipRecord struct {
	count       int
	firstSeen   time.Time
	lockedUntil time.Time
}

// IPRateLimiter provides sliding-window brute force protection on sensitive endpoints like login.
type IPRateLimiter struct {
	maxAttempts int
	window      time.Duration
	lockout     time.Duration
	records     map[string]*ipRecord
	mu          sync.Mutex
	stop        chan struct{}
}

// NewIPRateLimiter creates a new rate limiter with background cleanup.
func NewIPRateLimiter(maxAttempts int, window, lockout time.Duration) *IPRateLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if window <= 0 {
		window = 1 * time.Minute
	}
	if lockout <= 0 {
		lockout = 5 * time.Minute
	}
	limiter := &IPRateLimiter{
		maxAttempts: maxAttempts,
		window:      window,
		lockout:     lockout,
		records:     make(map[string]*ipRecord),
		stop:        make(chan struct{}),
	}
	go limiter.cleanupLoop()
	return limiter
}

// IsAllowed checks whether the IP is allowed to attempt authentication.
// Returns false and remaining lockout duration if locked.
func (l *IPRateLimiter) IsAllowed(ip string) (bool, time.Duration) {
	if l == nil || ip == "" {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	rec, exists := l.records[ip]
	if !exists {
		return true, 0
	}

	now := time.Now().UTC()
	if !rec.lockedUntil.IsZero() {
		if now.Before(rec.lockedUntil) {
			return false, rec.lockedUntil.Sub(now)
		}
		// Lockout expired, reset record
		delete(l.records, ip)
		return true, 0
	}

	return true, 0
}

// RecordFailure records a failed authentication attempt.
func (l *IPRateLimiter) RecordFailure(ip string) {
	if l == nil || ip == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	rec, exists := l.records[ip]
	if !exists {
		l.records[ip] = &ipRecord{
			count:     1,
			firstSeen: now,
		}
		return
	}

	// If window expired without locking, restart window
	if now.Sub(rec.firstSeen) > l.window {
		rec.count = 1
		rec.firstSeen = now
		rec.lockedUntil = time.Time{}
		return
	}

	rec.count++
	if rec.count >= l.maxAttempts {
		rec.lockedUntil = now.Add(l.lockout)
	}
}

// RecordSuccess clears any failure records for the IP upon successful login.
func (l *IPRateLimiter) RecordSuccess(ip string) {
	if l == nil || ip == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.records, ip)
}

// Close stops the background cleaner goroutine.
func (l *IPRateLimiter) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	select {
	case <-l.stop:
		l.mu.Unlock()
		return
	default:
		close(l.stop)
	}
	l.mu.Unlock()
}

func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stop:
			return
		}
	}
}

func (l *IPRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	for ip, rec := range l.records {
		if !rec.lockedUntil.IsZero() {
			if now.After(rec.lockedUntil) {
				delete(l.records, ip)
			}
		} else if now.Sub(rec.firstSeen) > l.window*2 {
			delete(l.records, ip)
		}
	}
}

// ClientIP extracts the real remote IP address taking reverse proxies into account.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
