package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the server runtime configuration. Loaded once at startup.
type Config struct {
	HTTPAddr      string        // HTTP listen address, e.g. ":8443"
	DBPath        string        // SQLite database file path
	JWTSecret     string        // secret used to sign JWTs
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// TLS: optional cert/key paths. When both are set, the server uses
	// ListenAndServeTLS; otherwise it falls back to plain HTTP and logs a
	// warning. In production, either set these or terminate TLS at the reverse
	// proxy (nginx/Caddy).
	TLSCertFile string
	TLSKeyFile  string

	// Enrollment token TTL and agent offline threshold.
	EnrollmentTTL     time.Duration
	AgentOfflineAfter time.Duration // a device is offline if last_seen older than this

	LogLevel string // zerolog level: debug|info|warn|error
}

// Load reads configuration from environment variables with sane defaults.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          getEnv("HTTP_ADDR", ":8443"),
		DBPath:            getEnv("DB_PATH", "data/endpoint-mgmt.db"),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		AccessTokenTTL:    getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:   getDuration("REFRESH_TOKEN_TTL", 24*7*time.Hour),
		TLSCertFile:       getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:        getEnv("TLS_KEY_FILE", ""),
		EnrollmentTTL:     getDuration("ENROLLMENT_TTL", 30*time.Minute),
		AgentOfflineAfter: getDuration("AGENT_OFFLINE_AFTER", 90*time.Second),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
	}
	if cfg.JWTSecret == "" {
		return cfg, fmt.Errorf("JWT_SECRET must be set (generate one, e.g. 32+ random bytes)")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}
