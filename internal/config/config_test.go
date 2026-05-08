package config

import "testing"

func TestLoadUsesDefaultHTTPAddressAndTenantMode(t *testing.T) {
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
