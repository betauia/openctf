package auth

import (
	"context"
	"testing"

	"github.com/betauia/openctf/backend/internal/testutil/testdb"
)

func TestDefaultAdminEmailIsStoredLowercase(t *testing.T) {
	// The server creates the user named admin once, so remove it before and after this test.
	remove := func() { testdb.Exec(t, `DELETE FROM users WHERE username = 'admin'`) }
	remove()
	t.Cleanup(remove)

	if err := EnsureDefaultAdmin(context.Background(), testdb.Pool, "a-password", "Admin@Example.COM"); err != nil {
		t.Fatalf("EnsureDefaultAdmin: %v", err)
	}
	if got := testdb.QueryRow[string](t, `SELECT email FROM users WHERE username = 'admin'`); got != "admin@example.com" {
		t.Errorf("stored email = %q, want %q", got, "admin@example.com")
	}
}
