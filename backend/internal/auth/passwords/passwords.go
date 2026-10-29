// Package passwords hashes and verifies user passwords.
package passwords

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// Hashes are stored as base64(salt || PBKDF2-HMAC-SHA256(password, salt)).
const (
	pbkdf2Iterations = 260000
	saltLen          = 16
	keyLen           = sha256.Size
)

// Hash hashes password with a fresh random salt, in the form Verify accepts.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, keyLen)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(append(salt, key...)), nil
}

// Verify reports whether password matches a hash from Hash.
// Malformed hashes never match.
func Verify(password, stored string) bool {
	raw, err := base64.StdEncoding.DecodeString(stored)
	if err != nil || len(raw) <= saltLen {
		return false
	}
	salt, want := raw[:saltLen], raw[saltLen:]
	got, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, len(want))
	return err == nil && hmac.Equal(got, want)
}
