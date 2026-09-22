package devicemanagement

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Device is a registered endpoint. Mirrors the `devices` table.
// Pointer fields map to NULL-able columns.
type Device struct {
	ID                  string     `db:"id"`
	Hostname            string     `db:"hostname"`
	OSName              string     `db:"os_name"`
	OSVersion           string     `db:"os_version"`
	AgentVersion        string     `db:"agent_version"`
	Status              string     `db:"status"`
	LastSeenAt          *time.Time `db:"last_seen_at"`
	EnrolledAt          time.Time  `db:"enrolled_at"`
	EnrollmentTokenHash *string    `db:"enrollment_token_hash"` // NULL once consumed
	DeviceSecretHash    string     `db:"device_secret_hash"`
	Site                string     `db:"site"`
	CreatedAt           time.Time  `db:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"`
}

const (
	StatusOnline  = "online"
	StatusOffline = "offline"
)

const (
	OSWindows = "windows"
	OSLinux   = "linux"
	OSMacOS   = "macos"
)

// NewID returns a cryptographically random hex ID (no external dep needed).
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// HashToken hashes a plaintext token/secret with SHA-256. Only the hash is stored.
func HashToken(plain string) string {
	h := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(h[:])
}

// GenerateToken returns a random URL-safe token.
func GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
