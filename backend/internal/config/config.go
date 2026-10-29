package config

import (
	"encoding/base64"
	"fmt"
	"os"
)

type InstanceProvider string

const (
	ProviderDocker InstanceProvider = "docker"
	ProviderKubernetes InstanceProvider = "kubernetes"
)

type Config struct {
	Env		  string
	HttpAddr  string
	SecretKey []byte
	DatabaseURL string
	InstanceProvider InstanceProvider
}

type Discord struct {
	ClientID	 string
	ClientSecret string
	RedirectURL	 string
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func LoadConfig() (*Config, error) {
	secretKeyBase64 := getEnv("SECRET_KEY", "")
	if secretKeyBase64 == "" {
		return nil, fmt.Errorf("SECRET_KEY environment variable is not set")
	}

	secretKey, err := base64.StdEncoding.DecodeString(secretKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SECRET_KEY: %v", err)
	}

	instanceProvider := InstanceProvider(getEnv("INSTANCE_PROVIDER", "docker"))
	if instanceProvider != ProviderDocker && instanceProvider != ProviderKubernetes {
		return nil, fmt.Errorf("invalid INSTANCE_PROVIDER: %s", instanceProvider)
	}

	cfg := &Config{
		Env:              getEnv("ENV", "development"),
		SecretKey:        secretKey,
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		InstanceProvider: instanceProvider,
	}

	return cfg, nil
}