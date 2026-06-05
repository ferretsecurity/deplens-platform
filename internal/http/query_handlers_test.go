package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/stretchr/testify/require"
)

func TestListScansSupportsRepositoryIDAndTimeFilters(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans?repository_id=11111111-1111-1111-1111-111111111111&from=2026-05-01T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestListScansRejectsInvalidRepositoryIDBeforeStore(t *testing.T) {
	store := &fakeQueryStore{}
	handler := newTestQueryRouterWithStore(t, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans?repository_id=repo-123&from=2026-05-01T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Zero(t, store.listScansCalls)
}

func TestGetScanReturnsRepositoryID(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-123", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotContains(t, rr.Body.String(), "project_slug")
	require.NotContains(t, rr.Body.String(), "repository_slug")
	require.Contains(t, rr.Body.String(), "repository_id")
}

func TestListRepositoriesReturnsRepositoryID(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/repositories", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotContains(t, rr.Body.String(), "project_slug")
	require.NotContains(t, rr.Body.String(), `"slug"`)
	require.Contains(t, rr.Body.String(), `"id":"repo-123"`)
}

func TestListRepositoriesAcceptsSessionAuth(t *testing.T) {
	sessions := auth.NewSessionManager(auth.SessionConfig{})
	handler := sessions.LoadAndSave(newTestQueryRouterWithSessions(t, sessions))

	req := requestWithQuerySession(t, sessions, http.MethodGet, "/api/v1/repositories")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"id":"repo-123"`)
}

func TestListRepositoryManifestsReturnsItems(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/repositories/11111111-1111-1111-1111-111111111111/manifests", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"path":"package-lock.json"`)
	require.Contains(t, rr.Body.String(), `"is_active":true`)
	require.Contains(t, rr.Body.String(), `"disappeared_at":null`)
}

func TestListRepositoryManifestsRejectsInvalidRepositoryIDBeforeStore(t *testing.T) {
	store := &fakeQueryStore{}
	handler := newTestQueryRouterWithStore(t, store)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/repositories/repo-123/manifests", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Zero(t, store.listRepositoryManifestsCalls)
}

func TestListRepositoryManifestsAcceptsSessionAuth(t *testing.T) {
	sessions := auth.NewSessionManager(auth.SessionConfig{})
	handler := sessions.LoadAndSave(newTestQueryRouterWithSessions(t, sessions))

	req := requestWithQuerySession(t, sessions, http.MethodGet, "/api/v1/repositories/11111111-1111-1111-1111-111111111111/manifests")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"path":"package-lock.json"`)
}

func TestListDependenciesReturnsItems(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dependencies", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"name":"react"`)
	require.Contains(t, rr.Body.String(), `"version":"19.1.0"`)
	require.Contains(t, rr.Body.String(), `"repository_count":2`)
	require.Contains(t, rr.Body.String(), `"manifest_file_count":3`)
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
	return newTestQueryRouterWithStore(t, &fakeQueryStore{})
}

func newTestQueryRouterWithStore(t *testing.T, reads QueryStore) http.Handler {
	t.Helper()
	service := NewProductionQueryService(fakeTokenLookup{}, reads, nil)
	return NewQueryRouter(service)
}

func newTestQueryRouterWithSessions(t *testing.T, sessions *scs.SessionManager) http.Handler {
	t.Helper()
	service := NewProductionQueryService(fakeTokenLookup{}, &fakeQueryStore{}, sessions)
	return NewQueryRouter(service)
}

type fakeTokenLookup struct{}

func (fakeTokenLookup) FindToken(_ context.Context, tokenHash string) (string, []string, error) {
	if tokenHash != auth.HashToken("bootstrap-token") {
		return "", nil, errUnauthorized
	}
	return "tenant-1", []string{"scan:read", "scan:metadata:write"}, nil
}

type fakeQueryStore struct {
	listScansCalls               int
	listRepositoryManifestsCalls int
}

func (*fakeQueryStore) ListRepositories(_ context.Context, tenantID string) ([]store.RepositoryListItem, error) {
	if tenantID != "tenant-1" {
		return nil, errUnauthorized
	}
	return []store.RepositoryListItem{{
		ID:            "repo-123",
		Name:          "Repo",
		URL:           "https://example.invalid/repo.git",
		DefaultBranch: "main",
	}}, nil
}

func (f *fakeQueryStore) ListScans(_ context.Context, filter store.ScanFilter) ([]store.ScanListItem, error) {
	f.listScansCalls++
	if filter.TenantID != "tenant-1" || filter.RepositoryID != "11111111-1111-1111-1111-111111111111" {
		return nil, errUnauthorized
	}
	return []store.ScanListItem{{
		ID:              "scan-123",
		RepositoryID:    "repo-123",
		CommitSHA:       "abc123",
		ScannedAt:       time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC),
		ManifestCount:   1,
		DependencyCount: 2,
		Labels:          map[string]string{"env": "prod"},
		Annotation:      "blessed build",
	}}, nil
}

func (f *fakeQueryStore) ListRepositoryManifests(_ context.Context, tenantID string, repositoryID string) ([]store.RepositoryManifestItem, error) {
	f.listRepositoryManifestsCalls++
	if tenantID != "tenant-1" || repositoryID != "11111111-1111-1111-1111-111111111111" {
		return nil, errUnauthorized
	}
	return []store.RepositoryManifestItem{{
		ID:            "manifest-123",
		Path:          "package-lock.json",
		FirstSeenAt:   time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC),
		LastSeenAt:    time.Date(2026, time.May, 5, 12, 0, 0, 0, time.UTC),
		DisappearedAt: nil,
		IsActive:      true,
		Labels:        map[string]string{"owner": "ui"},
	}}, nil
}

func (*fakeQueryStore) ListDependencies(_ context.Context, tenantID string) ([]store.DependencyListItem, error) {
	if tenantID != "tenant-1" {
		return nil, errUnauthorized
	}
	return []store.DependencyListItem{{
		Raw:               "react@19.1.0",
		Name:              "react",
		Version:           "19.1.0",
		RepositoryCount:   2,
		ManifestFileCount: 3,
	}}, nil
}

func (*fakeQueryStore) GetScan(_ context.Context, tenantID string, scanID string) (store.ScanListItem, error) {
	if tenantID != "tenant-1" || scanID != "scan-123" {
		return store.ScanListItem{}, errUnauthorized
	}
	return store.ScanListItem{
		ID:              "scan-123",
		RepositoryID:    "repo-123",
		CommitSHA:       "abc123",
		ScannedAt:       time.Date(2026, time.May, 4, 12, 0, 0, 0, time.UTC),
		ManifestCount:   1,
		DependencyCount: 2,
		Labels:          map[string]string{"env": "prod"},
		Annotation:      "blessed build",
	}, nil
}

func (*fakeQueryStore) ListScanManifests(_ context.Context, tenantID string, scanID string) ([]store.ScanManifestItem, error) {
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

func (*fakeQueryStore) UpdateScanMetadata(_ context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error {
	if tenantID != "tenant-1" || scanID != "scan-123" || labels["env"] != "prod" || annotation != "blessed build" {
		return errUnauthorized
	}
	return nil
}

func requestWithQuerySession(t *testing.T, sessions *scs.SessionManager, method, target string) *http.Request {
	t.Helper()

	ctx, err := sessions.Load(context.Background(), "")
	require.NoError(t, err)

	sessions.Put(ctx, "user_id", "user-123")
	sessions.Put(ctx, "memberships", []store.MembershipRecord{{
		TenantID:   "tenant-1",
		TenantSlug: "default",
		Role:       "owner",
	}})
	sessions.Put(ctx, "active_tenant_id", "tenant-1")
	sessions.Put(ctx, "role", "owner")

	token, expiry, err := sessions.Commit(ctx)
	require.NoError(t, err)

	req := httptest.NewRequest(method, target, nil)
	req.AddCookie(&http.Cookie{
		Name:    "deplens_session",
		Value:   token,
		Expires: expiry,
		Path:    "/",
	})
	return req
}
