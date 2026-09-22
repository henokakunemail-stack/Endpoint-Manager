package audit

import (
	"crypto/rand"
	"encoding/hex"
)

// readRand fills b with cryptographically secure random bytes.
func readRand(b []byte) (int, error) { return rand.Read(b) }

// hexEncode returns b as a lowercase hex string.
func hexEncode(b []byte) string { return hex.EncodeToString(b) }
