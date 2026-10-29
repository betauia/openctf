// Package sessions issues and verifies the signed tokens stored in session cookies.
package sessions

import (
	"crypto/hmac"
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

func sign(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
