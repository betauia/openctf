package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/betauia/openctf/backend/internal/testutil/testdb"
)

// TestMain starts one Postgres container for the whole package (see testdb) and fails the run
// if it can't, so the tests are never skipped.
func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

const adminEmail = "admin@example.com"

// configure sets a complete, valid configuration that stops at the listen step: ADDR has no port,
// so run gets as far as starting the server and then returns the listen error.
func configure(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", testdb.Pool.Config().ConnString())
	t.Setenv("SECRET_KEY", "test-secret-key")
	t.Setenv("ADMIN_EMAIL", adminEmail)
	t.Setenv("ADMIN_PASSWORD", "admin-password")
	t.Setenv("POLICY_FILE", "../../internal/policies/default.json")
	t.Setenv("FILES_DIR", t.TempDir())
	t.Setenv("ADDR", "no-port-here")
}

func TestRunFailsWhenTheConfigurationIsMissing(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "SECRET_KEY", "ADMIN_EMAIL", "ADMIN_PASSWORD"} {
		t.Run(key, func(t *testing.T) {
			configure(t)
			t.Setenv(key, "")

			err := run()

			if err == nil || !strings.Contains(err.Error(), key) {
				t.Errorf("run() = %v, want an error naming %s", err, key)
			}
		})
	}
}

func TestRunFailsWhenTheDatabaseCannotBeReached(t *testing.T) {
	configure(t)
	t.Setenv("DATABASE_URL", "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=2")

	if err := run(); err == nil {
		t.Error("run() succeeded without a database")
	}
}

func TestRunFailsOnAMissingPolicyFile(t *testing.T) {
	configure(t)
	missing := filepath.Join(t.TempDir(), "nope.json")
	t.Setenv("POLICY_FILE", missing)

	err := run()

	if err == nil || !strings.Contains(err.Error(), "load policies from "+missing) {
		t.Errorf("run() = %v, want an error naming the policy file", err)
	}
}

func TestRunFailsOnAnInvalidPolicyFile(t *testing.T) {
	tests := map[string]string{
		"not JSON":           `this is not json`,
		"an unknown action":  `[{"id":"p","statements":[{"sid":"s","effect":"allow","subjects":["*"],"actions":["team:jion"],"resources":["/x"]}]}]`,
		"a duplicate policy": `[{"id":"p","statements":[]},{"id":"p","statements":[]}]`,
		"a misspelled field": `[{"id":"p","statements":[{"sid":"s","efect":"allow","subjects":["*"],"actions":["team:join"],"resources":["/x"]}]}]`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			configure(t)
			path := filepath.Join(t.TempDir(), "policies.json")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("POLICY_FILE", path)

			err := run()

			if err == nil || !strings.Contains(err.Error(), "load policies from "+path) {
				t.Errorf("run() = %v, want a policy loading error naming the file", err)
			}
		})
	}
}

// With everything valid, run migrates, creates the admin, loads the policies and starts the
// server. The address has no port, so listening fails at once instead of blocking.
func TestRunStartsEverythingUpToListening(t *testing.T) {
	configure(t)

	err := run()

	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("run() = %v, want the listen error", err)
	}
	if !strings.Contains(err.Error(), "no-port-here") {
		t.Errorf("run() = %v, want an error about the bad address", err)
	}
	var role, email string
	row := testdb.Pool.QueryRow(t.Context(), `SELECT role, email FROM users WHERE username = 'admin'`)
	if err := row.Scan(&role, &email); err != nil {
		t.Fatalf("the default admin was not created: %v", err)
	}
	if role != "admin" || email != adminEmail {
		t.Errorf("admin = role %q email %q, want an admin with %s", role, email, adminEmail)
	}
}
