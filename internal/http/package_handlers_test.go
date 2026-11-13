package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/stretchr/testify/require"
)

func TestListPackagesParsesSearchAndPagination(t *testing.T) {
	reads := &fakeQueryStore{}
	handler := newTestQueryRouterWithStore(t, reads)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages?q=%20REAct%20&version=%201.103.1%20&page=2&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, store.PackageListFilter{TenantID: "tenant-1", Query: "REAct", Version: "1.103.1", Page: 2, PageSize: 10}, reads.listPackagesFilter)
	require.JSONEq(t, `{"items":[{"source":"cli","origin_kind":"","destination":"","name":"react","version":"19.1.0","constraint":"","ecosystem":"","repository_id":"11111111-1111-1111-1111-111111111111","repository_name":"Repo","manifest_path":null}],"pagination":{"page":2,"page_size":10,"total":1,"total_pages":1,"has_previous":false,"has_next":false},"filters":{"q":"REAct","version":"1.103.1"}}`, rr.Body.String())
}

func TestListPackagesRejectsInvalidFiltersBeforeStore(t *testing.T) {
	for _, query := range []string{"version=latest", "version=%3E%3D1", "version=1.*", "version=" + strings.Repeat("1", 201), "page=0", "page=-1", "page=no", "page=1.5", "page_size=0", "page_size=-1", "page_size=no", "page_size=101", "page=9223372036854775807&page_size=100", "page=99999999999999999999999", "q=" + strings.Repeat("a", 201)} {
		t.Run(query, func(t *testing.T) {
			reads := &fakeQueryStore{}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/packages?"+query, nil)
			req.Header.Set("Authorization", "Bearer bootstrap-token")
			rr := httptest.NewRecorder()
			newTestQueryRouterWithStore(t, reads).ServeHTTP(rr, req)
			require.Equal(t, http.StatusBadRequest, rr.Code)
			require.Zero(t, reads.listPackagesCalls)
		})
	}
}

func TestListPackagesRequiresReadAuthorization(t *testing.T) {
	for _, token := range []string{"", "invalid", "write-only"} {
		t.Run(token, func(t *testing.T) {
			reads := &fakeQueryStore{}
			service := NewProductionQueryService(packageTokenLookup{}, reads, nil, nil)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil)
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rr := httptest.NewRecorder()
			NewQueryRouter(service).ServeHTTP(rr, req)
			require.Equal(t, http.StatusUnauthorized, rr.Code)
			require.Zero(t, reads.listPackagesCalls)
		})
	}
}

func TestListPackagesAcceptsSessionAuthAndDefaultPaging(t *testing.T) {
	sessions := auth.NewSessionManager(auth.SessionConfig{})
	reads := &fakeQueryStore{}
	service := NewProductionQueryService(fakeTokenLookup{}, reads, sessions, nil)
	handler := sessions.LoadAndSave(NewQueryRouter(service))
	req := requestWithQuerySession(t, sessions, http.MethodGet, "/api/v1/packages")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, store.PackageListFilter{TenantID: "tenant-1", Page: 1, PageSize: 25}, reads.listPackagesFilter)
}

type packageTokenLookup struct{ fakeTokenLookup }

func (packageTokenLookup) FindToken(_ context.Context, hash string) (string, []string, error) {
	if hash == auth.HashToken("write-only") {
		return "tenant-1", []string{"scan:write"}, nil
	}
	return "", nil, errUnauthorized
}

func TestAssessmentFilters(t *testing.T) {
	reads := &fakeQueryStore{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages?q=widget&ecosystem=npm&affected_range=%3E0.0.0%20%3C%3D2.3.4", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()
	newTestQueryRouterWithStore(t, reads).ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "npm", reads.listPackagesFilter.Ecosystem)
	require.Equal(t, ">0.0.0 <=2.3.4", reads.listPackagesFilter.AffectedRange)
	for _, query := range []string{"q=widget&ecosystem=npm", "q=widget&affected_range=%3C3", "q=widget&ecosystem=unknown&affected_range=%3C3", "ecosystem=npm&affected_range=%3C3", "q=widget&ecosystem=npm&affected_range=latest", "q=widget&ecosystem=npm&affected_range=%3C3&version=2.3.4"} {
		reads := &fakeQueryStore{}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/packages?"+query, nil)
		req.Header.Set("Authorization", "Bearer bootstrap-token")
		rr := httptest.NewRecorder()
		newTestQueryRouterWithStore(t, reads).ServeHTTP(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code, query)
		require.Zero(t, reads.listPackagesCalls)
	}
}
