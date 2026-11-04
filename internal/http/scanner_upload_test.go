package httpapi

import (
	"context"
	"encoding/json"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
	"github.com/stretchr/testify/require"
)

const nativeReport = `{"schema_version":1,"root":"/repo","sources":[{"detector":"npm-package","path":"package.json","form":"manifest","roles":["dependency-declaration"],"analysis":{"presence":"present","extraction":"partial"},"dependencies":[{"raw":"react@^19","name":"react","package_type":"npm","version_constraint":"^19","source_group":"dependencies","origin_kind":"registry","scope":"runtime","attributes":{"custom":"value"},"maven_exclusions":[{"group_id":"a","artifact_id":"b"}]}],"diagnostics":[{"severity":"warning","code":"partial","message":"Partial extraction"}]},{"detector":"other","path":"other.lock","analysis":{"presence":"unknown","extraction":"unsupported"}}],"check_runs":[{"check_id":"test","status":"skipped"}],"findings":[{"check_id":"test","summary":"Finding","evidence":{"custom":"kept"}}],"generation":{"format":"maven-pom","paths":[],"outcomes":[]},"future_field":{"kept":true}}`

func scannerUploadRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("X-Deplens-Repository-Name", "Repo")
	req.Header.Set("X-Deplens-Repository-URL", "https://example.com/repo")
	req.Header.Set("X-Deplens-Default-Branch", "main")
	req.Header.Set("X-Deplens-Commit-SHA", "abc123")
	req.Header.Set("X-Deplens-Ref", "refs/heads/main")
	req.Header.Set("X-Deplens-Scanned-At", "2026-11-01T12:00:00Z")
	return req
}

func TestNativeScannerUploadPreservesReportAndProjectsInventory(t *testing.T) {
	service := &fakeUploadService{allowedToken: "bootstrap-token"}
	rr := httptest.NewRecorder()
	NewRouter(service).ServeHTTP(rr, scannerUploadRequest(nativeReport))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.JSONEq(t, nativeReport, string(service.lastInput.Report))
	manifests := service.lastInput.Snapshot.Manifests
	require.Len(t, manifests, 2)
	require.Equal(t, "npm-package", manifests[0].Type)
	require.Equal(t, "package.json", manifests[0].Path)
	require.NotNil(t, manifests[0].HasDependencies)
	require.True(t, *manifests[0].HasDependencies)
	require.Nil(t, manifests[1].HasDependencies)
	require.Equal(t, "warning [partial]: Partial extraction", manifests[0].Warnings[0])
	dependency := manifests[0].Dependencies[0]
	require.Equal(t, "npm", dependency.PackageType)
	require.Equal(t, "^19", dependency.Constraint)
	require.Equal(t, "dependencies", dependency.Section)
	require.Equal(t, "registry", dependency.Source)
	require.Equal(t, "value", dependency.Extras["custom"])
	summary := scans.BuildSummary(service.lastInput)
	require.Equal(t, 2, summary.ManifestCount)
	require.Equal(t, 1, summary.DependencyCount)
	require.Equal(t, 1, summary.ManifestsUnknownCount)
}

func TestScannerUploadRejectsUnsupportedAndMalformedReports(t *testing.T) {
	for _, body := range []string{
		strings.Replace(nativeReport, `"schema_version":1`, `"schema_version":2`, 1),
		strings.Replace(nativeReport, `"schema_version":1`, `"schema_version":"v2"`, 1),
		strings.Replace(nativeReport, `"presence":"present"`, `"presence":"maybe"`, 1),
		`{"schema_version":1,"root":"/repo","sources":[]}`,
		`{"schema_version":null,"root":"/repo","sources":[],"check_runs":[],"findings":[]}`,
		`{"root":"/repo","sources":[]}`,
		nativeReport + `{}`,
	} {
		t.Run(body[:min(50, len(body))], func(t *testing.T) {
			service := &fakeUploadService{allowedToken: "bootstrap-token"}
			rr := httptest.NewRecorder()
			NewRouter(service).ServeHTTP(rr, scannerUploadRequest(body))
			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			require.Empty(t, service.lastInput.Repository.Name)
		})
	}
}

func TestScannerUploadAcceptsNullCollections(t *testing.T) {
	input, err := decodeUploadRequest(scannerUploadRequest(`{"schema_version":1,"root":"/repo","sources":null,"check_runs":null,"findings":null}`))
	require.NoError(t, err)
	require.NotNil(t, input.Snapshot.Manifests)
	require.Empty(t, input.Snapshot.Manifests)
	require.True(t, json.Valid(input.Report))
}

type reportReader struct {
	payload []byte
	key     string
}

func (r *reportReader) Get(_ context.Context, key string) ([]byte, error) {
	r.key = key
	return r.payload, nil
}

func TestScanReportReturnsPreservedReportWithTenantAuthorization(t *testing.T) {
	for _, report := range []string{nativeReport, "null"} {
		t.Run(report[:min(20, len(report))], func(t *testing.T) {
			reader := &reportReader{payload: []byte(`{"report":` + report + `}`)}
			service := NewProductionQueryService(fakeTokenLookup{}, &fakeQueryStore{}, nil, reader)
			handler := NewQueryRouter(service)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/11111111-1111-1111-1111-111111111111/report", nil)
			req.Header.Set("Authorization", "Bearer bootstrap-token")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			require.Equal(t, http.StatusOK, rr.Code)
			require.JSONEq(t, `{"report":`+report+`}`, rr.Body.String())
			require.Equal(t, "artifact-key", reader.key)
		})
	}
}

func TestScanReportRejectsUnauthenticatedAccessBeforeArtifactRead(t *testing.T) {
	reader := &reportReader{}
	service := NewProductionQueryService(fakeTokenLookup{}, &fakeQueryStore{}, nil, reader)
	rr := httptest.NewRecorder()
	NewQueryRouter(service).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/scans/11111111-1111-1111-1111-111111111111/report", nil))
	require.Equal(t, http.StatusUnauthorized, rr.Code)
	require.Empty(t, reader.key)
}

type writeOnlyTokenLookup struct{}

func (writeOnlyTokenLookup) ListMemberships(context.Context, string) ([]store.MembershipRecord, error) {
	return []store.MembershipRecord{{TenantID: "tenant-1", Role: "owner"}}, nil
}

func (writeOnlyTokenLookup) FindToken(_ context.Context, _ string) (string, []string, error) {
	return "tenant-1", []string{"scan:write"}, nil
}

func TestReadEndpointsRejectWriteOnlyBearerToken(t *testing.T) {
	for _, path := range []string{
		"/api/v1/scans/11111111-1111-1111-1111-111111111111/report",
		"/api/v1/scans/11111111-1111-1111-1111-111111111111",
		"/api/v1/scans/11111111-1111-1111-1111-111111111111/manifests",
		"/api/v1/repositories",
		"/api/v1/repositories/11111111-1111-1111-1111-111111111111/manifests",
		"/api/v1/dependencies",
		"/api/v1/scans?repository_id=11111111-1111-1111-1111-111111111111&from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z",
	} {
		t.Run(path, func(t *testing.T) {
			reader := &reportReader{payload: []byte(`{"report":null}`)}
			service := NewProductionQueryService(writeOnlyTokenLookup{}, &fakeQueryStore{}, nil, reader)
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer write-only-token")
			rr := httptest.NewRecorder()
			NewQueryRouter(service).ServeHTTP(rr, req)
			require.Equal(t, http.StatusUnauthorized, rr.Code)
			require.Empty(t, reader.key)
		})
	}
}

func TestScanReportAcceptsSessionWithoutBearerScopes(t *testing.T) {
	sessions := auth.NewSessionManager(auth.SessionConfig{})
	reader := &reportReader{payload: []byte(`{"report":null}`)}
	service := NewProductionQueryService(writeOnlyTokenLookup{}, &fakeQueryStore{}, sessions, reader)
	handler := sessions.LoadAndSave(NewQueryRouter(service))
	req := requestWithQuerySession(t, sessions, http.MethodGet, "/api/v1/scans/11111111-1111-1111-1111-111111111111/report")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"report":null}`, rr.Body.String())
}

func TestNativeScannerUploadKeepsMissingAndFuturePackageTypes(t *testing.T) {
	for _, packageType := range []string{"", "unknown", "future-type"} {
		t.Run(packageType, func(t *testing.T) {
			body := strings.Replace(nativeReport, `"package_type":"npm",`, ``, 1)
			if packageType != "" {
				body = strings.Replace(nativeReport, `"package_type":"npm"`, `"package_type":"`+packageType+`"`, 1)
			}
			body = strings.Replace(body, `"attributes":{"custom":"value"}`, `"attributes":{"package_type":"attribute"}`, 1)
			service := &fakeUploadService{allowedToken: "bootstrap-token"}
			rr := httptest.NewRecorder()
			NewRouter(service).ServeHTTP(rr, scannerUploadRequest(body))
			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			dependency := service.lastInput.Snapshot.Manifests[0].Dependencies[0]
			require.Equal(t, packageType, dependency.PackageType)
			require.Equal(t, "attribute", dependency.Extras["package_type"])
		})
	}
}
