package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/scans"
)

func TestUploadScanReturnsCreatedForValidBearerToken(t *testing.T) {
	service := &fakeUploadService{
		allowedToken: "bootstrap-token",
	}
	handler := NewRouter(service)

	body := []byte(`{
	  "schema_version": "v1alpha1",
	  "repository": {"slug":"repo","name":"Repo","url":"https://example.com/repo.git","default_branch":"main"},
	  "source": {"commit_sha":"abc123","ref":"refs/heads/main","scanned_at":"2026-05-04T10:00:00Z"},
	  "snapshot": {
	    "root": ".",
	    "manifests": [
	      {
	        "type": "npm-package-lock",
	        "path": "package-lock.json",
	        "has_dependencies": true,
	        "warnings": ["manifest warning"],
	        "dependencies": [
	          {
	            "raw": "react@19.1.0",
	            "name": "react",
	            "version": "19.1.0",
	            "section": "dependencies",
	            "source": "npm",
	            "extras": {"scope":"ui"}
	          }
	        ]
	      }
	    ]
	  }
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	if len(service.lastInput.Snapshot.Manifests) != 1 {
		t.Fatalf("manifest count = %d, want 1", len(service.lastInput.Snapshot.Manifests))
	}

	manifest := service.lastInput.Snapshot.Manifests[0]
	if manifest.Type != "npm-package-lock" || manifest.Path != "package-lock.json" {
		t.Fatalf("manifest = %+v, want type/path preserved", manifest)
	}
	if manifest.HasDependencies == nil || !*manifest.HasDependencies {
		t.Fatalf("manifest HasDependencies = %v, want true", manifest.HasDependencies)
	}
	if len(manifest.Warnings) != 1 || manifest.Warnings[0] != "manifest warning" {
		t.Fatalf("manifest warnings = %#v, want preserved warnings", manifest.Warnings)
	}
	if len(manifest.Dependencies) != 1 {
		t.Fatalf("dependency count = %d, want 1", len(manifest.Dependencies))
	}

	dependency := manifest.Dependencies[0]
	if dependency.Raw != "react@19.1.0" || dependency.Name != "react" || dependency.Version != "19.1.0" || dependency.Section != "dependencies" || dependency.Source != "npm" {
		t.Fatalf("dependency = %+v, want fields preserved", dependency)
	}
	if len(dependency.Extras) != 1 || dependency.Extras["scope"] != "ui" {
		t.Fatalf("dependency extras = %#v, want map with scope=ui", dependency.Extras)
	}
}

func TestUploadRawDeplensJSONReturnsCreatedForValidBearerToken(t *testing.T) {
	service := &fakeUploadService{
		allowedToken: "bootstrap-token",
	}
	handler := NewRouter(service)

	body := []byte(`{
	  "root": ".",
	  "manifests": [
	    {
	      "type": "rust-cargo-lock",
	      "path": "Cargo.lock",
	      "has_dependencies": true,
	      "dependencies": [
	        {
	          "raw": "actix-codec@0.5.2",
	          "name": "actix-codec",
	          "version": "0.5.2",
	          "source": "registry",
	          "extras": {
	            "checksum": "abc",
	            "source_url": "https://example.com/index"
	          }
	        }
	      ]
	    }
	  ]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Deplens-Repository-Slug", "juice-shop")
	req.Header.Set("X-Deplens-Repository-Name", "juice-shop")
	req.Header.Set("X-Deplens-Repository-URL", "https://github.com/juice-shop/juice-shop")
	req.Header.Set("X-Deplens-Default-Branch", "master")
	req.Header.Set("X-Deplens-Commit-SHA", "abc123")
	req.Header.Set("X-Deplens-Ref", "refs/heads/master")
	req.Header.Set("X-Deplens-Scanned-At", "2026-05-05T18:00:00Z")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if len(service.lastInput.Snapshot.Manifests) != 1 {
		t.Fatalf("manifest count = %d, want 1", len(service.lastInput.Snapshot.Manifests))
	}
	dependency := service.lastInput.Snapshot.Manifests[0].Dependencies[0]
	if dependency.Extras["checksum"] != "abc" {
		t.Fatalf("dependency extras = %#v, want checksum preserved", dependency.Extras)
	}
}

func TestUploadRawDeplensJSONFailsWithoutRequiredHeaders(t *testing.T) {
	handler := NewRouter(&fakeUploadService{
		allowedToken: "bootstrap-token",
	})

	body := []byte(`{
	  "root": ".",
	  "manifests": []
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestDecodeUploadRequestAcceptsRawRepositoryOnlyHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", strings.NewReader(`{"root":".","manifests":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Deplens-Repository-Slug", "repo")
	req.Header.Set("X-Deplens-Repository-Name", "Repo")
	req.Header.Set("X-Deplens-Repository-URL", "https://example.com/repo.git")
	req.Header.Set("X-Deplens-Default-Branch", "main")
	req.Header.Set("X-Deplens-Commit-SHA", "abc123")
	req.Header.Set("X-Deplens-Ref", "refs/heads/main")
	req.Header.Set("X-Deplens-Scanned-At", "2026-05-08T10:00:00Z")

	input, err := decodeUploadRequest(req)
	if err != nil {
		t.Fatalf("decodeUploadRequest() error = %v", err)
	}
	if input.Repository.Slug != "repo" {
		t.Fatalf("repository slug = %q, want repo", input.Repository.Slug)
	}
}

type fakeUploadService struct {
	allowedToken string
	lastInput    scans.UploadRequest
}

func (f *fakeUploadService) Upload(_ *http.Request, token string, input scans.UploadRequest) (string, error) {
	if token != f.allowedToken {
		return "", errUnauthorized
	}
	f.lastInput = input
	return "scan-123", nil
}
