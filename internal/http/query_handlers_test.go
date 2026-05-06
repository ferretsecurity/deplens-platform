package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListScansSupportsRepositoryAndTimeFilters(t *testing.T) {
	handler := NewQueryRouter(fakeQueryService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans?repository_slug=repo&from=2026-05-01T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestGetScanReturnsOK(t *testing.T) {
	handler := NewQueryRouter(fakeQueryService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-123", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestListScanManifestsReturnsOK(t *testing.T) {
	handler := NewQueryRouter(fakeQueryService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-123/manifests", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestPatchScanMetadataReturnsOK(t *testing.T) {
	handler := NewQueryRouter(fakeQueryService{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/scans/scan-123/metadata", strings.NewReader(`{"labels":{"env":"prod"},"annotation":"blessed build"}`))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

type fakeQueryService struct{}

func (fakeQueryService) ListProjects(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return []map[string]string{{"slug": "core", "name": "Core"}}, nil
}

func (fakeQueryService) ListRepositories(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return []map[string]string{{"project_slug": "core", "slug": "repo", "name": "Repo"}}, nil
}

func (fakeQueryService) ListScans(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return []map[string]string{{"id": "scan-123"}}, nil
}

func (fakeQueryService) GetScan(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return map[string]string{"id": "scan-123"}, nil
}

func (fakeQueryService) ListScanManifests(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return map[string]any{"items": []map[string]any{{"id": "manifest-123"}}}, nil
}

func (fakeQueryService) UpdateScanMetadata(_ *http.Request, token string) error {
	if token != "bootstrap-token" {
		return errUnauthorized
	}
	return nil
}
