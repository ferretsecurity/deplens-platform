package app

import (
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/config"
)

func TestNewInitializesBlobStoreForFilesystemBackend(t *testing.T) {
	cfg := config.Config{
		Mode:                   "self-hosted",
		DatabaseURL:            "postgres://postgres:postgres@localhost:5432/deplens?sslmode=disable",
		BlobBackend:            "filesystem",
		BlobFilesystemRoot:     "./var/blobs",
		BootstrapOwnerEmail:    "admin@example.com",
		BootstrapOwnerPassword: "change-me-now",
		BootstrapAPIToken:      "bootstrap-token",
		SessionCookieSecret:    "replace-me",
	}

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer application.DB.Close()

	if application.BlobStore == nil {
		t.Fatal("BlobStore = nil, want initialized store")
	}
}
