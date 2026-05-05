package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddress            string
	Mode                   string
	DatabaseURL            string
	BlobBackend            string
	BlobFilesystemRoot     string
	BootstrapOwnerEmail    string
	BootstrapOwnerPassword string
	BootstrapAPIToken      string
	SessionCookieSecret    string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:            getEnv("HTTP_ADDRESS", ":8080"),
		Mode:                   getEnv("DEPLOYMENT_MODE", "self-hosted"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		BlobBackend:            os.Getenv("BLOB_BACKEND"),
		BlobFilesystemRoot:     os.Getenv("BLOB_FILESYSTEM_ROOT"),
		BootstrapOwnerEmail:    os.Getenv("BOOTSTRAP_OWNER_EMAIL"),
		BootstrapOwnerPassword: os.Getenv("BOOTSTRAP_OWNER_PASSWORD"),
		BootstrapAPIToken:      os.Getenv("BOOTSTRAP_API_TOKEN"),
		SessionCookieSecret:    os.Getenv("SESSION_COOKIE_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.BlobBackend == "" {
		return Config{}, errors.New("BLOB_BACKEND is required")
	}
	if cfg.BlobBackend == "filesystem" && cfg.BlobFilesystemRoot == "" {
		return Config{}, errors.New("BLOB_FILESYSTEM_ROOT is required when BLOB_BACKEND=filesystem")
	}
	if cfg.BootstrapOwnerEmail == "" {
		return Config{}, errors.New("BOOTSTRAP_OWNER_EMAIL is required")
	}
	if cfg.BootstrapOwnerPassword == "" {
		return Config{}, errors.New("BOOTSTRAP_OWNER_PASSWORD is required")
	}
	if cfg.BootstrapAPIToken == "" {
		return Config{}, errors.New("BOOTSTRAP_API_TOKEN is required")
	}
	if cfg.SessionCookieSecret == "" {
		return Config{}, errors.New("SESSION_COOKIE_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
