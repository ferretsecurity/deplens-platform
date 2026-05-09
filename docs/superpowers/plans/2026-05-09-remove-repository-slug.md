# Remove Repository Slug Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove repository slug from the API, ingest flow, store layer, and schema so repositories are identified by `id` in APIs and matched by `name` during ingest.

**Architecture:** Cut over directly to a single repository identity model. Upload decoding and service/store inputs stop carrying slug, repository upserts move to `(tenant_id, name)`, and scan/read APIs expose `repository_id` instead of `repository_slug`.

**Tech Stack:** Go, `net/http`, pgx, PostgreSQL, goose migrations, testify

---

### Task 1: Remove Repository Slug From Upload Payloads

**Files:**
- Modify: `internal/scans/payload.go`
- Modify: `internal/scans/service.go`
- Modify: `internal/scans/service_test.go`
- Modify: `internal/http/upload_handler.go`
- Modify: `internal/http/upload_handler_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestServiceUploadConvertsManifestsAndDependencies(t *testing.T) {
	ctx := context.Background()
	hasDependencies := true
	input := UploadRequest{
		SchemaVersion: "v1alpha1",
		Repository:    RepositoryInput{Name: "Repo", URL: "https://example.com/repo.git", DefaultBranch: "main"},
		Source:        SourceInput{CommitSHA: "abc123", Ref: "refs/heads/main", ScannedAt: "2026-05-06T08:00:00Z"},
		Snapshot: SnapshotInput{
			Root: ".",
			Manifests: []ManifestInput{{
				Type:            "npm-package-lock",
				Path:            "package-lock.json",
				HasDependencies: &hasDependencies,
			}},
		},
	}

	repo := &captureRepository{}
	svc := Service{Blob: noopBlobStore{}, Store: repo}

	_, err := svc.Upload(ctx, "tenant-123", input)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if repo.params.RepositoryName != "Repo" {
		t.Fatalf("repository name = %q, want Repo", repo.params.RepositoryName)
	}
}

func TestDecodeUploadRequestAcceptsRawRepositoryOnlyHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", strings.NewReader(`{"root":".","manifests":[]}`))
	req.Header.Set("Content-Type", "application/json")
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
	if input.Repository.Name != "Repo" {
		t.Fatalf("repository name = %q, want Repo", input.Repository.Name)
	}
}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run: `go test ./internal/scans ./internal/http -run 'TestServiceUploadConvertsManifestsAndDependencies|TestDecodeUploadRequestAcceptsRawRepositoryOnlyHeaders'`

Expected: FAIL because the code and tests still reference `RepositoryInput.Slug`, `RepositorySlug`, and `X-Deplens-Repository-Slug`.

- [ ] **Step 3: Write the minimal implementation**

```go
type RepositoryInput struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}
```

```go
return s.Store.CreateScan(ctx, store.UploadScanParams{
	TenantID:       tenantID,
	RepositoryName: input.Repository.Name,
	URL:            input.Repository.URL,
	DefaultBranch:  input.Repository.DefaultBranch,
	ArtifactKey:    artifactKey,
	ArtifactSHA256: artifactSHA,
	SchemaVersion:  input.SchemaVersion,
	RootPath:       input.Snapshot.Root,
	CommitSHA:      input.Source.CommitSHA,
	SourceRef:      input.Source.Ref,
	ScannedAt:      scannedAt,
	ManifestCount:  summary.ManifestCount,
	Manifests:      manifests,
})
```

```go
type rawUploadMetadata struct {
	repositoryName string
	repositoryURL  string
	defaultBranch  string
	commitSHA      string
	ref            string
	scannedAt      string
}

func rawUploadHeaders(header http.Header) (rawUploadMetadata, error) {
	metadata := rawUploadMetadata{
		repositoryName: strings.TrimSpace(header.Get("X-Deplens-Repository-Name")),
		repositoryURL:  strings.TrimSpace(header.Get("X-Deplens-Repository-URL")),
		defaultBranch:  strings.TrimSpace(header.Get("X-Deplens-Default-Branch")),
		commitSHA:      strings.TrimSpace(header.Get("X-Deplens-Commit-SHA")),
		ref:            strings.TrimSpace(header.Get("X-Deplens-Ref")),
		scannedAt:      strings.TrimSpace(header.Get("X-Deplens-Scanned-At")),
	}

	for _, required := range []struct {
		name  string
		value string
	}{
		{"X-Deplens-Repository-Name", metadata.repositoryName},
		{"X-Deplens-Repository-URL", metadata.repositoryURL},
		{"X-Deplens-Default-Branch", metadata.defaultBranch},
		{"X-Deplens-Commit-SHA", metadata.commitSHA},
		{"X-Deplens-Ref", metadata.ref},
		{"X-Deplens-Scanned-At", metadata.scannedAt},
	} {
		if required.value == "" {
			return rawUploadMetadata{}, errors.New(required.name + " is required for raw deplens uploads")
		}
	}

	return metadata, nil
}
```

- [ ] **Step 4: Run the targeted tests to verify they pass**

Run: `go test ./internal/scans ./internal/http -run 'TestServiceUploadConvertsManifestsAndDependencies|TestDecodeUploadRequestAcceptsRawRepositoryOnlyHeaders|TestUploadRawDeplensJSONReturnsCreatedForValidBearerToken'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scans/payload.go internal/scans/service.go internal/scans/service_test.go internal/http/upload_handler.go internal/http/upload_handler_test.go
git commit -m "refactor: remove repository slug from uploads"
```

### Task 2: Move Store Writes And Schema To Repository Name Matching

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/uploads.go`
- Modify: `internal/store/scan_details_test.go`
- Modify: `db/migrations/001_initial.sql`

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateScanUpsertsRepositoryByNameAndOverwritesMetadata(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	require.NoError(t, Migrate(databaseURL))

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)

	_, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
		RepositoryName: "Repo",
		URL:            "https://example.com/one.git",
		DefaultBranch:  "main",
		ArtifactKey:    "artifact-1",
		ArtifactSHA256: "sha-1",
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      "commit-1",
		SourceRef:      "refs/heads/main",
		ScannedAt:      time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	_, err = store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
		RepositoryName: "Repo",
		URL:            "https://example.com/two.git",
		DefaultBranch:  "stable",
		ArtifactKey:    "artifact-2",
		ArtifactSHA256: "sha-2",
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      "commit-2",
		SourceRef:      "refs/heads/stable",
		ScannedAt:      time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	var url, branch string
	err = db.QueryRow(ctx, `select url, default_branch from repositories where tenant_id = $1 and name = $2`, tenantID, "Repo").Scan(&url, &branch)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/two.git", url)
	require.Equal(t, "stable", branch)
}
```

- [ ] **Step 2: Run the targeted test to verify it fails**

Run: `go test ./internal/store -run TestCreateScanUpsertsRepositoryByNameAndOverwritesMetadata`

Expected: FAIL because `UploadScanParams` and the repository insert/upsert still depend on `RepositorySlug` and the schema still requires `slug`.

- [ ] **Step 3: Write the minimal implementation**

```go
type UploadScanParams struct {
	TenantID            string
	RepositoryName      string
	URL                 string
	DefaultBranch       string
	ArtifactKey         string
	ArtifactSHA256      string
	SchemaVersion       string
	RootPath            string
	CommitSHA           string
	SourceRef           string
	ScannedAt           time.Time
	ManifestCount       int
	WithDependencies    int
	WithoutDependencies int
	UnknownDependencies int
	DependencyCount     int
	Labels              map[string]string
	Annotation          string
	Manifests           []UploadManifestParams
}
```

```sql
create table if not exists repositories (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    name text not null,
    url text not null,
    default_branch text not null,
    created_at timestamptz not null default now(),
    unique (tenant_id, name)
);
```

```go
err = tx.QueryRow(ctx, `
	insert into repositories (tenant_id, name, url, default_branch)
	values ($1, $2, $3, $4)
	on conflict (tenant_id, name)
	do update set name = excluded.name, url = excluded.url, default_branch = excluded.default_branch
	returning id
`, params.TenantID, params.RepositoryName, params.URL, params.DefaultBranch).Scan(&repositoryID)
```

- [ ] **Step 4: Run the targeted store tests to verify they pass**

Run: `go test ./internal/store -run 'TestCreateScanUpsertsRepositoryByNameAndOverwritesMetadata|TestCreateScanTracksManifestLifecycle|TestCreateScanPersistsManifestsAndDependencies'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/models.go internal/store/uploads.go internal/store/scan_details_test.go db/migrations/001_initial.sql
git commit -m "refactor: match repositories by name on ingest"
```

### Task 3: Replace Repository Slug With Repository ID In Read APIs

**Files:**
- Modify: `internal/store/scans.go`
- Modify: `internal/http/query_handlers.go`
- Modify: `internal/http/query_handlers_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestListScansSupportsRepositoryIDAndTimeFilters(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans?repository_id=repo-123&from=2026-05-01T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestGetScanReturnsRepositoryID(t *testing.T) {
	handler := newTestQueryRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-123", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotContains(t, rr.Body.String(), "repository_slug")
	require.Contains(t, rr.Body.String(), "repository_id")
}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run: `go test ./internal/http -run 'TestListScansSupportsRepositoryIDAndTimeFilters|TestGetScanReturnsRepositoryID'`

Expected: FAIL because the query layer still reads `repository_slug` and the response model still serializes `repository_slug`.

- [ ] **Step 3: Write the minimal implementation**

```go
type ScanListItem struct {
	ID              string            `json:"id"`
	RepositoryID    string            `json:"repository_id"`
	CommitSHA       string            `json:"commit_sha"`
	ScannedAt       time.Time         `json:"scanned_at"`
	ManifestCount   int               `json:"manifest_count"`
	DependencyCount int               `json:"dependency_count"`
	Labels          map[string]string `json:"labels"`
	Annotation      string            `json:"annotation"`
}

type RepositoryListItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}

type ScanFilter struct {
	TenantID     string
	RepositoryID string
	From         time.Time
	To           time.Time
}
```

```go
return s.Reads.ListScans(r.Context(), store.ScanFilter{
	TenantID:     tenantID,
	RepositoryID: r.URL.Query().Get("repository_id"),
	From:         from,
	To:           to,
})
```

```go
rows, err := s.DB.Query(ctx, `
	select s.id, s.repository_id, s.commit_sha, s.scanned_at, s.manifest_count, s.dependency_count, s.labels, s.annotation
	from scans s
	where s.tenant_id = $1 and s.repository_id = $2 and s.scanned_at between $3 and $4
	order by s.scanned_at desc
`, filter.TenantID, filter.RepositoryID, filter.From, filter.To)
```

- [ ] **Step 4: Run the targeted tests to verify they pass**

Run: `go test ./internal/http ./internal/store -run 'TestListScansSupportsRepositoryIDAndTimeFilters|TestGetScanReturnsRepositoryID|TestListRepositoriesOmitsProjectSlug'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/scans.go internal/http/query_handlers.go internal/http/query_handlers_test.go
git commit -m "refactor: switch scan APIs to repository ids"
```

### Task 4: Remove Repository Slug From Public Contract And Examples

**Files:**
- Modify: `api/openapi.yaml`
- Modify: `internal/api/spec_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing tests**

```go
func TestOpenAPISpecDoesNotExposeRepositorySlug(t *testing.T) {
	data, err := os.ReadFile("../../api/openapi.yaml")
	require.NoError(t, err)
	require.NotContains(t, string(data), "repository_slug")
	require.NotContains(t, string(data), "X-Deplens-Repository-Slug")
	require.NotContains(t, string(data), "required: [slug, name, url, default_branch]")
}
```

- [ ] **Step 2: Run the targeted test to verify it fails**

Run: `go test ./internal/api -run TestOpenAPISpecDoesNotExposeRepositorySlug`

Expected: FAIL because the OpenAPI schema and examples still expose slug-based upload and query fields.

- [ ] **Step 3: Write the minimal implementation**

```yaml
RepositoryInput:
  type: object
  required: [name, url, default_branch]
  properties:
    name:
      type: string
    url:
      type: string
    default_branch:
      type: string
```

```yaml
/api/v1/scans:
  get:
    parameters:
      - in: query
        name: repository_id
        required: true
        schema:
          type: string
```

```bash
curl -X POST http://localhost:8080/api/v1/scans \
  -H 'Authorization: Bearer <token-from-/api/v1/tokens>' \
  -H 'Content-Type: application/json' \
  -d '{
    "schema_version":"v1alpha1",
    "repository":{"name":"Repo","url":"https://example.com/repo.git","default_branch":"main"},
    "source":{"commit_sha":"abc123","ref":"refs/heads/main","scanned_at":"2026-05-04T10:00:00Z"},
    "snapshot":{"root":".","manifests":[]}
  }'
```

- [ ] **Step 4: Run the targeted tests to verify they pass**

Run: `go test ./internal/api -run 'TestOpenAPIDefinesScanUploadAndHistoryPaths|TestOpenAPISpecDoesNotExposeProjects|TestOpenAPISpecDoesNotExposeRepositorySlug'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/openapi.yaml internal/api/spec_test.go README.md
git commit -m "docs: remove repository slug from public API contract"
```

### Task 5: Full Verification

**Files:**
- Modify: none
- Test: `internal/http/upload_handler_test.go`
- Test: `internal/http/query_handlers_test.go`
- Test: `internal/scans/service_test.go`
- Test: `internal/store/scan_details_test.go`
- Test: `internal/api/spec_test.go`

- [ ] **Step 1: Run the full targeted package suite**

Run: `go test ./internal/scans ./internal/http ./internal/store ./internal/api`

Expected: PASS

- [ ] **Step 2: Run a repository slug regression search**

Run: `rg -n 'RepositorySlug|repository_slug|X-Deplens-Repository-Slug|"slug"' internal api README.md db/migrations/001_initial.sql`

Expected: no matches for repository slug concepts in repository upload/query/schema paths; remaining `slug` matches should only be unrelated tenant/auth-provider code.

- [ ] **Step 3: Commit verification-only changes if needed**

```bash
git status --short
```

Expected: no unexpected unstaged files. If verification required minor test/doc cleanups, stage them and commit with a focused message before handing off.
