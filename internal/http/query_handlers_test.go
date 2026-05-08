package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/stretchr/testify/require"
)

func TestListScansSupportsRepositoryAndTimeFilters(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans?repository_slug=repo&from=2026-05-01T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestGetScanOmitsProjectSlug(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-123", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotContains(t, rr.Body.String(), "project_slug")
	require.Contains(t, rr.Body.String(), "repository_slug")
}

func TestListRepositoriesOmitsProjectSlug(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/repositories", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotContains(t, rr.Body.String(), "project_slug")
}

func TestListScanManifestsReturnsPath(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-123/manifests", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"path":"manifest.yaml"`)
}

func TestPatchScanMetadataReturnsOK(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/scans/scan-123/metadata", strings.NewReader(`{"labels":{"env":"prod"},"annotation":"blessed build"}`))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func newTestQueryRouter(t *testing.T) http.Handler {
	t.Helper()
	service := NewProductionQueryService(fakeTokenLookup{}, fakeQueryStore{})
	return NewQueryRouter(service)
}

type fakeTokenLookup struct{}

func (fakeTokenLookup) FindToken(_ context.Context, tokenHash string) (string, []string, error) {
	if tokenHash != auth.HashToken("bootstrap-token") {
		return "", nil, errUnauthorized
	}
	return "tenant-1", []string{"scan:read", "scan:metadata:write"}, nil
}

type fakeQueryStore struct{}

func (fakeQueryStore) ListRepositories(_ context.Context, tenantID string) ([]store.RepositoryListItem, error) {
	if tenantID != "tenant-1" {
		return nil, errUnauthorized
	}
	return []store.RepositoryListItem{{
		Slug:          "repo",
		Name:          "Repo",
		URL:           "https://example.invalid/repo.git",
		DefaultBranch: "main",
	}}, nil
}

func (fakeQueryStore) ListScans(_ context.Context, filter store.ScanFilter) ([]store.ScanListItem, error) {
	if filter.TenantID != "tenant-1" || filter.RepositorySlug != "repo" {
		return nil, errUnauthorized
	}
	return []store.ScanListItem{{
		ID:              "scan-123",
		RepositorySlug:  "repo",
		CommitSHA:       "abc123",
		ScannedAt:       time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC),
		ManifestCount:   1,
		DependencyCount: 2,
		Labels:          map[string]string{"env": "prod"},
		Annotation:      "blessed build",
	}}, nil
}

func (fakeQueryStore) GetScan(_ context.Context, tenantID string, scanID string) (store.ScanListItem, error) {
	if tenantID != "tenant-1" || scanID != "scan-123" {
		return store.ScanListItem{}, errUnauthorized
	}
	return store.ScanListItem{
		ID:              "scan-123",
		RepositorySlug:  "repo",
		CommitSHA:       "abc123",
		ScannedAt:       time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC),
		ManifestCount:   1,
		DependencyCount: 2,
		Labels:          map[string]string{"env": "prod"},
		Annotation:      "blessed build",
	}, nil
}

func (fakeQueryStore) ListScanManifests(_ context.Context, tenantID string, scanID string) ([]store.ScanManifestItem, error) {
	if tenantID != "tenant-1" || scanID != "scan-123" {
		return nil, errUnauthorized
	}
	return []store.ScanManifestItem{{
		ID:              "manifest-123",
		Type:            "lockfile",
		Path:            "manifest.yaml",
		HasDependencies: nil,
		Warnings:        []string{"notice"},
	}}, nil
}

func (fakeQueryStore) UpdateScanMetadata(_ context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error {
	if tenantID != "tenant-1" || scanID != "scan-123" || labels["env"] != "prod" || annotation != "blessed build" {
		return errUnauthorized
	}
	return nil
}
