// Package config loads settings from the environment.
package config

import (
	"errors"
	"os"
)

// Config is the server configuration.
type Config struct {
	DatabaseURL   string // postgres://user:pass@host:5432/db
	Addr          string
	SecretKey     string // signs session cookies
	AdminEmail    string // email for the admin user
	AdminPassword string // default password for the admin user
}

// Load reads the configuration from the environment, applying defaults for optional
// variables. It returns an error if DATABASE_URL or SECRET_KEY is unset.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		Addr:          env("ADDR", ":8000"),
		SecretKey:     os.Getenv("SECRET_KEY"),
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL must be set")
	}
	if cfg.SecretKey == "" {
		return cfg, errors.New("SECRET_KEY must be set")
	}
	if cfg.AdminEmail == "" {
		return cfg, errors.New("ADMIN_EMAIL must be set")
	}
	if cfg.AdminPassword == "" {
		return cfg, errors.New("ADMIN_PASSWORD must be set")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
