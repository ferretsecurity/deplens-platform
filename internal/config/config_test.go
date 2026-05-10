package config

import "testing"

func TestLoadUsesDefaultHTTPAddressTenantModeAndFrontendFlags(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/deplens?sslmode=disable")
	t.Setenv("BLOB_BACKEND", "filesystem")
	t.Setenv("BLOB_FILESYSTEM_ROOT", "./var/blobs")
	t.Setenv("BOOTSTRAP_OWNER_EMAIL", "admin@example.com")
	t.Setenv("BOOTSTRAP_OWNER_PASSWORD", "change-me-now")
	t.Setenv("SESSION_COOKIE_SECRET", "replace-me")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("HTTPAddress = %q, want %q", cfg.HTTPAddress, ":8080")
	}
	if cfg.Mode != "self-hosted" {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, "self-hosted")
	}
	if cfg.AppBaseURL != "http://localhost:8080" {
		t.Fatalf("AppBaseURL = %q, want %q", cfg.AppBaseURL, "http://localhost:8080")
	}
	if cfg.MigrateOnStart != true {
		t.Fatalf("MigrateOnStart = %v, want true", cfg.MigrateOnStart)
	}
}

func TestLoadHonorsCookieAndProxyFlags(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/deplens?sslmode=disable")
	t.Setenv("BLOB_BACKEND", "filesystem")
	t.Setenv("BLOB_FILESYSTEM_ROOT", "./var/blobs")
	t.Setenv("BOOTSTRAP_OWNER_EMAIL", "admin@example.com")
	t.Setenv("BOOTSTRAP_OWNER_PASSWORD", "change-me-now")
	t.Setenv("SESSION_COOKIE_SECRET", "replace-me")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("TRUST_PROXY", "true")
	t.Setenv("MIGRATE_ON_START", "false")
	t.Setenv("APP_BASE_URL", "https://deplens.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !cfg.SessionCookieSecure {
		t.Fatal("SessionCookieSecure = false, want true")
	}
	if !cfg.TrustProxy {
		t.Fatal("TrustProxy = false, want true")
	}
	if cfg.MigrateOnStart {
		t.Fatal("MigrateOnStart = true, want false")
	}
	if cfg.AppBaseURL != "https://deplens.example.com" {
		t.Fatalf("AppBaseURL = %q, want %q", cfg.AppBaseURL, "https://deplens.example.com")
	}
}

func TestLoadFailsWithoutDatabaseURL(t *testing.T) {
	t.Setenv("BLOB_BACKEND", "filesystem")
	t.Setenv("BLOB_FILESYSTEM_ROOT", "./var/blobs")
	t.Setenv("BOOTSTRAP_OWNER_EMAIL", "admin@example.com")
	t.Setenv("BOOTSTRAP_OWNER_PASSWORD", "change-me-now")
	t.Setenv("SESSION_COOKIE_SECRET", "replace-me")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
