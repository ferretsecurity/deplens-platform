package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/scans"
)

func TestUploadScanReturnsCreatedForValidBearerToken(t *testing.T) {
	handler := NewRouter(fakeUploadService{
		allowedToken: "bootstrap-token",
	})

	body := []byte(`{
	  "schema_version": "v1alpha1",
	  "project": {"slug":"core","name":"Core"},
	  "repository": {"slug":"repo","name":"Repo","url":"https://example.com/repo.git","default_branch":"main"},
	  "source": {"commit_sha":"abc123","ref":"refs/heads/main","scanned_at":"2026-05-04T10:00:00Z"},
	  "snapshot": {"root":".","manifests":[]}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

type fakeUploadService struct {
	allowedToken string
}

func (f fakeUploadService) Upload(_ *http.Request, token string, _ scans.UploadRequest) (string, error) {
	if token != f.allowedToken {
		return "", errUnauthorized
	}
	return "scan-123", nil
}
