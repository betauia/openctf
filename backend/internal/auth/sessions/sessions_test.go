package sessions

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"
)

var (
	secret = []byte("test-secret")
	issued = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
)

// rewrite decodes token, passes its parts to edit, and re-encodes them without re-signing.
func rewrite(t *testing.T, token string, edit func(parts []string)) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	parts := strings.Split(string(raw), ":")
	edit(parts)
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, ":")))
}

// signed builds a token for arbitrary parts with a valid signature, to reach the checks after it.
func signed(secret []byte, user, family, unix string) string {
	payload := user + ":" + family + ":" + unix
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + sign(secret, payload)))
}

func TestVerifyTokenRoundTrip(t *testing.T) {
	user, family := uuid.NewV7(), uuid.NewV7()

	gotUser, gotFamily, ok := VerifyToken(secret, NewToken(secret, user, family, issued), issued)

	if !ok {
		t.Fatal("fresh token was rejected")
	}
	if gotUser != user || gotFamily != family {
		t.Errorf("got user %v family %v, want user %v family %v", gotUser, gotFamily, user, family)
	}
}

func TestVerifyTokenExpiry(t *testing.T) {
	token := NewToken(secret, uuid.NewV7(), uuid.NewV7(), issued)
	tests := []struct {
		name string
		age  time.Duration
		want bool
	}{
		{"just issued", 0, true},
		{"just inside MaxAge", MaxAge - time.Second, true},
		{"exactly MaxAge", MaxAge, true},
		{"just past MaxAge", MaxAge + time.Second, false},
		{"long expired", 24 * time.Hour, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := VerifyToken(secret, token, issued.Add(tc.age)); ok != tc.want {
				t.Errorf("ok = %v, want %v", ok, tc.want)
			}
		})
	}
}

// VerifyToken only checks the token isn't too old. This pins that a token dated in the future
// is accepted, so changing it is a deliberate decision.
func TestVerifyTokenAcceptsFutureIssueTime(t *testing.T) {
	token := NewToken(secret, uuid.NewV7(), uuid.NewV7(), issued.Add(time.Hour))

	if _, _, ok := VerifyToken(secret, token, issued); !ok {
		t.Error("token issued in the future was rejected")
	}
}

func TestVerifyTokenRejectsTampering(t *testing.T) {
	user, family := uuid.NewV7(), uuid.NewV7()
	token := NewToken(secret, user, family, issued)
	other := uuid.NewV7().String()

	tests := []struct {
		name  string
		token string
	}{
		{"different user", rewrite(t, token, func(p []string) { p[0] = other })},
		{"different family", rewrite(t, token, func(p []string) { p[1] = other })},
		{"later timestamp", rewrite(t, token, func(p []string) { p[2] = strconv.FormatInt(issued.Add(time.Hour).Unix(), 10) })},
		{"altered signature", rewrite(t, token, func(p []string) { p[3] = strings.Repeat("0", len(p[3])) })},
		{"empty signature", rewrite(t, token, func(p []string) { p[3] = "" })},
		{"signed with another secret", NewToken([]byte("other-secret"), user, family, issued)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := VerifyToken(secret, tc.token, issued); ok {
				t.Error("tampered token was accepted")
			}
		})
	}
}

func TestVerifyTokenRejectsMalformedInput(t *testing.T) {
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	id := uuid.NewV7().String()
	unix := strconv.FormatInt(issued.Unix(), 10)

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"not base64", "!!!not base64!!!"},
		{"no colons", b64("nocolons")},
		{"three parts", b64("a:b:c")},
		{"five parts", b64("a:b:c:d:e")},
		// These carry a valid signature, so they get past the HMAC check and test the parsing after it.
		{"non-numeric timestamp", signed(secret, id, id, "yesterday")},
		{"user is not a uuid", signed(secret, "not-a-uuid", id, unix)},
		{"family is not a uuid", signed(secret, id, "not-a-uuid", unix)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := VerifyToken(secret, tc.token, issued); ok {
				t.Error("malformed token was accepted")
			}
		})
	}
}

func TestVerifyDeviceTokenRoundTrip(t *testing.T) {
	user := uuid.NewV7()

	got, ok := VerifyDeviceToken(secret, NewDeviceToken(secret, user))

	if !ok || got != user {
		t.Errorf("got %v, %v; want %v, true", got, ok, user)
	}
}

func TestVerifyDeviceTokenRejectsTampering(t *testing.T) {
	user := uuid.NewV7()
	token := NewDeviceToken(secret, user)
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

	tests := []struct {
		name  string
		token string
	}{
		{"different user", rewrite(t, token, func(p []string) { p[0] = uuid.NewV7().String() })},
		{"altered signature", rewrite(t, token, func(p []string) { p[1] = strings.Repeat("0", len(p[1])) })},
		{"signed with another secret", NewDeviceToken([]byte("other-secret"), user)},
		{"empty", ""},
		{"not base64", "!!!not base64!!!"},
		{"no colon", b64("nocolon")},
		{"user is not a uuid", b64("not-a-uuid:" + sign(secret, "device:not-a-uuid"))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := VerifyDeviceToken(secret, tc.token); ok {
				t.Error("bad device token was accepted")
			}
		})
	}
}

// Both tokens are signed with the same secret, so the "device:" prefix in the signed payload is
// what stops one being passed off as the other.
func TestSessionAndDeviceTokensAreNotInterchangeable(t *testing.T) {
	user, family := uuid.NewV7(), uuid.NewV7()

	if _, ok := VerifyDeviceToken(secret, NewToken(secret, user, family, issued)); ok {
		t.Error("session token was accepted as a device token")
	}
	if _, _, ok := VerifyToken(secret, NewDeviceToken(secret, user), issued); ok {
		t.Error("device token was accepted as a session token")
	}
}

func TestNewRefreshToken(t *testing.T) {
	token, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token is not base64url: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("token carries %d random bytes, want 32", len(raw))
	}
	if !bytes.Equal(hash, HashRefreshToken(token)) {
		t.Error("returned hash does not match HashRefreshToken(token)")
	}
	if len(hash) != 32 {
		t.Errorf("hash is %d bytes, want 32 (SHA-256)", len(hash))
	}
}

func TestNewRefreshTokenIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		token, _, err := NewRefreshToken()
		if err != nil {
			t.Fatal(err)
		}
		if seen[token] {
			t.Fatalf("token %q was generated twice", token)
		}
		seen[token] = true
	}
}

func TestHashRefreshToken(t *testing.T) {
	if !bytes.Equal(HashRefreshToken("same"), HashRefreshToken("same")) {
		t.Error("hash is not deterministic")
	}
	if bytes.Equal(HashRefreshToken("one"), HashRefreshToken("two")) {
		t.Error("different tokens hash to the same value")
	}
	if bytes.Contains(HashRefreshToken("secret-token"), []byte("secret-token")) {
		t.Error("hash contains the token")
	}
}
