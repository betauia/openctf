package sessions

import (
	"strings"
	"testing"
	"time"
	"uuid"
)

func TestToken(t *testing.T) {
	secret := []byte("secret")
	id := uuid.NewV7()
	now := time.Unix(1_800_000_000, 0)
	tok := NewToken(secret, id, now)

	if got, ok := VerifyToken(secret, tok, now.Add(time.Hour)); !ok || got != id {
		t.Fatalf("VerifyToken = %v, %v; want %v, true", got, ok, id)
	}
	if _, ok := VerifyToken(secret, tok, now.Add(MaxAge+time.Second)); ok {
		t.Error("expired token accepted")
	}
	if _, ok := VerifyToken([]byte("other"), tok, now); ok {
		t.Error("token accepted with wrong secret")
	}
	forged := NewToken([]byte("other"), id, now)
	if _, ok := VerifyToken(secret, forged, now); ok {
		t.Error("forged token accepted")
	}
	if strings.Contains(tok, "=") {
		t.Error("token should be unpadded")
	}
	for _, bad := range []string{"", "!!!", "YQ"} {
		if _, ok := VerifyToken(secret, bad, now); ok {
			t.Errorf("accepted malformed token %q", bad)
		}
	}
}
