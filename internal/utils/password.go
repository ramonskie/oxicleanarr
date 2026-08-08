package utils

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// bcryptPrefixes are the valid bcrypt hash prefixes
var bcryptPrefixes = []string{"$2a$", "$2b$", "$2y$"}

// GenerateRandomSecret returns a cryptographically random secret of n bytes
// encoded as hex (2n characters).
func GenerateRandomSecret(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// IsBcryptHash reports whether the stored value is a bcrypt hash
func IsBcryptHash(stored string) bool {
	for _, p := range bcryptPrefixes {
		if strings.HasPrefix(stored, p) {
			return true
		}
	}
	return false
}

// HashPassword hashes a plaintext password with bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a stored value.
// Supports both bcrypt hashes (new) and legacy plaintext (pre-migration configs).
// Returns true on match, false otherwise. Errors are treated as non-matches.
func CheckPassword(stored, plaintext string) bool {
	if stored == "" {
		return false
	}
	if IsBcryptHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plaintext)) == nil
	}
	// Legacy plaintext comparison (constant-time to avoid trivial timing attacks)
	return subtle.ConstantTimeCompare([]byte(stored), []byte(plaintext)) == 1
}
