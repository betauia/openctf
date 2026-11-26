package config

import (
	"os"
	"testing"
)

// setRequired sets the variables Load insists on, so a test can look at the optional ones.
func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/openctf")
	t.Setenv("SECRET_KEY", "signing-key")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "admin-password")
}

// unset removes a variable for the test; t.Setenv puts the original value back afterwards.
func unset(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadReadsTheRequiredVariables(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DatabaseURL != "postgres://user:pass@db:5432/openctf" || cfg.SecretKey != "signing-key" ||
		cfg.AdminEmail != "admin@example.com" || cfg.AdminPassword != "admin-password" {
		t.Errorf("cfg = %+v, want the four required variables passed through", cfg)
	}
}

func TestLoadDefaultsForOptionalVariables(t *testing.T) {
	setRequired(t)
	unset(t, "ADDR", "POLICY_FILE", "FILES_DIR")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != ":8000" {
		t.Errorf("Addr = %q, want :8000", cfg.Addr)
	}
	if cfg.PolicyFile != "internal/policies/default.json" {
		t.Errorf("PolicyFile = %q, want internal/policies/default.json", cfg.PolicyFile)
	}
	if cfg.FilesDir != "./data/files" {
		t.Errorf("FilesDir = %q, want ./data/files", cfg.FilesDir)
	}
}

func TestLoadOptionalVariablesOverrideTheDefaults(t *testing.T) {
	setRequired(t)
	t.Setenv("ADDR", "127.0.0.1:9090")
	t.Setenv("POLICY_FILE", "/etc/openctf/policies.json")
	t.Setenv("FILES_DIR", "/var/lib/openctf/files")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != "127.0.0.1:9090" || cfg.PolicyFile != "/etc/openctf/policies.json" || cfg.FilesDir != "/var/lib/openctf/files" {
		t.Errorf("cfg = %+v, want the three overrides", cfg)
	}
}

func TestLoadTreatsAnEmptyOptionalVariableAsUnset(t *testing.T) {
	setRequired(t)
	t.Setenv("ADDR", "")
	t.Setenv("POLICY_FILE", "")
	t.Setenv("FILES_DIR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != ":8000" || cfg.PolicyFile != "internal/policies/default.json" || cfg.FilesDir != "./data/files" {
		t.Errorf("cfg = %+v, want the defaults", cfg)
	}
}
