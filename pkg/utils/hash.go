package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashString creates a SHA-256 hash of the input string
func HashString(input string) string {
	// Create a new SHA-256 hash
	h := sha256.New()
	h.Write([]byte(input))

	// Return the hex-encoded hash
	return hex.EncodeToString(h.Sum(nil))
}
