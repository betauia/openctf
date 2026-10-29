// Package config loads settings from the environment.
package config

import (
	"errors"
	"os"
)

// Config is the server configuration.
type Config struct {
	DatabaseURL      string // postgres://user:pass@host:5432/db
	SecretKey        string
	InstanceProvider string // "docker" or "kubernetes"
	HostDomain       string
	UploadDir        string
	Addr             string
}

// Load reads the configuration from the environment, applying defaults for optional
// variables. It returns an error if DATABASE_URL or SECRET_KEY is unset.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		SecretKey:        os.Getenv("SECRET_KEY"),
		InstanceProvider: env("INSTANCE_PROVIDER", "docker"),
		HostDomain:       env("HOST_DOMAIN", "localhost"),
		UploadDir:        env("UPLOAD_DIR", "/app/uploads"),
		Addr:             env("ADDR", ":8000"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL must be set")
	}
	if cfg.SecretKey == "" {
		return cfg, errors.New("SECRET_KEY must be set")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
