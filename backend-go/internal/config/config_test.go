package config

import "testing"

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for _, k := range []string{"DATABASE_URL", "SECRET_KEY", "INSTANCE_PROVIDER", "HOST_DOMAIN", "UPLOAD_DIR", "ADDR"} {
		t.Setenv(k, vars[k])
	}
}

func TestLoadDefaults(t *testing.T) {
	setEnv(t, map[string]string{"DATABASE_URL": "postgres://db", "SECRET_KEY": "s"})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		DatabaseURL:      "postgres://db",
		SecretKey:        "s",
		InstanceProvider: "docker",
		HostDomain:       "localhost",
		UploadDir:        "/app/uploads",
		Addr:             ":8000",
	}
	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	setEnv(t, map[string]string{
		"DATABASE_URL":      "postgres://db",
		"SECRET_KEY":        "s",
		"INSTANCE_PROVIDER": "kubernetes",
		"HOST_DOMAIN":       "ctf.example",
		"UPLOAD_DIR":        "/tmp/up",
		"ADDR":              ":9000",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InstanceProvider != "kubernetes" || cfg.HostDomain != "ctf.example" || cfg.UploadDir != "/tmp/up" || cfg.Addr != ":9000" {
		t.Errorf("Load() = %+v", cfg)
	}
}

func TestLoadRequired(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"no database url": {"SECRET_KEY": "s"},
		"no secret key":   {"DATABASE_URL": "postgres://db"},
	} {
		t.Run(name, func(t *testing.T) {
			setEnv(t, vars)
			if _, err := Load(); err == nil {
				t.Error("Load() succeeded")
			}
		})
	}
}
