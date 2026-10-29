// Package sessions issues and verifies the signed tokens stored in session
// cookies, and generates the random refresh tokens used to renew them.
package sessions

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
	"uuid"
)

// MaxAge is how long a session token stays valid after it is issued.
const MaxAge = 15 * time.Minute

// RefreshMaxAge is how long a refresh token stays valid after it is issued.
const RefreshMaxAge = 14 * 24 * time.Hour

// RefreshReuseWindow is how long a rotated refresh token can still be used, so
// concurrent refreshes from several tabs, or a lost response, aren't mistaken for theft.
const RefreshReuseWindow = 10 * time.Second

// NewToken returns base64url("<user id>:<unix time>:<hex HMAC-SHA256 of the first two parts>").
func NewToken(secret []byte, userID uuid.UUID, now time.Time) string {
	payload := userID.String() + ":" + strconv.FormatInt(now.Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + sign(secret, payload)))
}

// VerifyToken returns the user id in token if its signature is valid and it hasn't expired.
func VerifyToken(secret []byte, token string, now time.Time) (uuid.UUID, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return uuid.UUID{}, false
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 3 {
		return uuid.UUID{}, false
	}
	payload := parts[0] + ":" + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(sign(secret, payload))) {
		return uuid.UUID{}, false
	}
	issued, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Sub(time.Unix(issued, 0)) > MaxAge {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}

// NewRefreshToken returns a random token for the cookie and the hash to store in the database.
func NewRefreshToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken returns the value stored for token. SHA-256 is enough because the input is random.
func HashRefreshToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
