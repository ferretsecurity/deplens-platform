# Repository Manifest Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the project-scoped v1 schema with a repository-only model and add durable manifest lifecycle records that are updated during immutable scan ingestion.

**Architecture:** The schema becomes repository-centric: `projects` are removed, `repositories` become the top-level scan target, `manifests` become durable lifecycle rows keyed by `(repository_id, path)`, and `scan_manifests` become immutable observations that point to `manifests`. The upload path continues to normalize scanner payloads on write, and the read path joins durable manifest state only where needed for `path`.

**Tech Stack:** Go, PostgreSQL, pgx/pgxpool, goose migrations, net/http, JSON/OpenAPI

---

## File Structure

**Database**

- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/db/migrations/001_initial.sql`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/db/migrations/002_scan_detail_tables.sql`
- Create: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/db/migrations/003_repository_manifest_lifecycle.sql` only if the implementation chooses a forward migration instead of rewriting the dev-only baseline migrations

**Upload and store path**

- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/scans/payload.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/scans/service.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/models.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/uploads.go`

**Read path**

- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/scans.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/query_handlers.go`

**Tests**

- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/upload_handler_test.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/query_handlers_test.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/scans/service_test.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/scan_details_test.go`
- Create or modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/uploads_test.go` if no upload-focused store test exists yet
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/api/spec_test.go`

**Docs**

- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/api/openapi.yaml`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/README.md`

### Task 1: Redesign the Schema

**Files:**
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/db/migrations/001_initial.sql`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/db/migrations/002_scan_detail_tables.sql`
- Test: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/scan_details_test.go`

- [ ] **Step 1: Write the failing store test for durable manifests**

Add a test that uploads two scans for the same repository, with one manifest path present in both scans and one path removed in the second scan.

```go
func TestCreateScanTracksManifestLifecycle(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	firstID, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
		RepositorySlug: "repo",
		RepositoryName: "Repo",
		URL:            "https://example.com/repo.git",
		DefaultBranch:  "main",
		ArtifactKey:    "artifact-1",
		ArtifactSHA256: "sha-1",
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      "commit-1",
		SourceRef:      "refs/heads/main",
		ScannedAt:      time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
		Manifests: []UploadManifestParams{
			{Position: 0, Type: "js", Path: "package.json"},
			{Position: 1, Type: "rust", Path: "Cargo.lock"},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, firstID)

	secondID, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
		RepositorySlug: "repo",
		RepositoryName: "Repo",
		URL:            "https://example.com/repo.git",
		DefaultBranch:  "main",
		ArtifactKey:    "artifact-2",
		ArtifactSHA256: "sha-2",
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      "commit-2",
		SourceRef:      "refs/heads/main",
		ScannedAt:      time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC),
		Manifests: []UploadManifestParams{
			{Position: 0, Type: "js", Path: "package.json"},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, secondID)

	rows, err := store.DB.Query(ctx, `
		select path, first_seen_at, last_seen_at, is_active
		from manifests
		order by path asc
	`)
	require.NoError(t, err)
	defer rows.Close()
}
```

- [ ] **Step 2: Run the focused store test to verify it fails**

Run: `go test ./internal/store -run TestCreateScanTracksManifestLifecycle -count=1`

Expected: FAIL because the `manifests` table does not exist and `CreateScan` still expects project fields.

- [ ] **Step 3: Rewrite the schema to match the new model**

Apply these structural changes:

```sql
drop table if exists scans;
drop table if exists repositories;
drop table if exists projects;

create table if not exists repositories (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    slug text not null,
    name text not null,
    url text not null,
    default_branch text not null,
    created_at timestamptz not null default now(),
    unique (tenant_id, slug)
);

create table if not exists scans (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    repository_id uuid not null references repositories(id) on delete cascade,
    artifact_key text not null unique,
    artifact_sha256 text not null,
    schema_version text not null,
    root_path text not null,
    commit_sha text not null,
    source_ref text not null,
    scanned_at timestamptz not null,
    manifest_count integer not null,
    manifests_with_dependencies_count integer not null,
    manifests_without_dependencies_count integer not null,
    manifests_unknown_count integer not null,
    dependency_count integer not null,
    labels jsonb not null default '{}'::jsonb,
    annotation text not null default '',
    created_at timestamptz not null default now()
);

create index if not exists scans_tenant_repository_scanned_at_idx
    on scans (tenant_id, repository_id, scanned_at desc);
```

And in the scan detail migration:

```sql
create table if not exists manifests (
    id uuid primary key default gen_random_uuid(),
    repository_id uuid not null references repositories(id) on delete cascade,
    path text not null,
    first_seen_at timestamptz not null,
    last_seen_at timestamptz not null,
    is_active boolean not null,
    labels jsonb not null default '{}'::jsonb,
    unique (repository_id, path)
);

create index if not exists manifests_repository_active_path_idx
    on manifests (repository_id, is_active, path);

create table if not exists scan_manifests (
    id uuid primary key default gen_random_uuid(),
    scan_id uuid not null references scans(id) on delete cascade,
    manifest_id uuid not null references manifests(id) on delete cascade,
    position integer not null,
    type text not null,
    has_dependencies boolean null,
    warnings jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    unique (scan_id, position),
    unique (scan_id, manifest_id)
);

create table if not exists manifest_dependencies (
    id uuid primary key default gen_random_uuid(),
    scan_manifest_id uuid not null references scan_manifests(id) on delete cascade,
    position integer not null,
    raw text not null,
    name text not null default '',
    version text not null default '',
    "constraint" text not null default '',
    section text not null default '',
    source text not null default '',
    extras jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    unique (scan_manifest_id, position)
);
```

- [ ] **Step 4: Run the focused store test again**

Run: `go test ./internal/store -run TestCreateScanTracksManifestLifecycle -count=1`

Expected: FAIL with compile or query errors in store code, not migration errors.

- [ ] **Step 5: Commit the schema rewrite**

```bash
git add db/migrations/001_initial.sql db/migrations/002_scan_detail_tables.sql internal/store/scan_details_test.go
git commit -m "refactor: redesign scan schema around repositories and manifests"
```

### Task 2: Remove Project Fields from Payload and Upload Params

**Files:**
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/scans/payload.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/scans/service.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/models.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/upload_handler.go`
- Test: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/upload_handler_test.go`
- Test: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/scans/service_test.go`

- [ ] **Step 1: Write the failing HTTP test for repository-only raw uploads**

Add a test that omits `X-Deplens-Project-Slug` and `X-Deplens-Project-Name` but still expects a decoded upload request.

```go
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
	require.NoError(t, err)
	require.Equal(t, "repo", input.Repository.Slug)
}
```

- [ ] **Step 2: Run the focused upload and service tests**

Run: `go test ./internal/http ./internal/scans -run 'TestDecodeUploadRequestAcceptsRawRepositoryOnlyHeaders|TestServiceUpload' -count=1`

Expected: FAIL because the payload and service still require `Project`.

- [ ] **Step 3: Remove project fields from request and store parameter types**

Update the types to remove the project layer.

```go
type UploadRequest struct {
	SchemaVersion string            `json:"schema_version"`
	Repository    RepositoryInput   `json:"repository"`
	Source        SourceInput       `json:"source"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotation    string            `json:"annotation,omitempty"`
	Snapshot      SnapshotInput     `json:"snapshot"`
}

type UploadScanParams struct {
	TenantID            string
	RepositorySlug      string
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

And update raw upload header parsing:

```go
type rawUploadMetadata struct {
	repositorySlug string
	repositoryName string
	repositoryURL  string
	defaultBranch  string
	commitSHA      string
	ref            string
	scannedAt      string
}
```

- [ ] **Step 4: Update service mapping to emit repository-only uploads**

In `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/scans/service.go`, remove the `ProjectSlug` and `ProjectName` assignments and build `UploadScanParams` without project fields.

```go
return s.Store.CreateScan(ctx, store.UploadScanParams{
	TenantID:            tenantID,
	RepositorySlug:      input.Repository.Slug,
	RepositoryName:      input.Repository.Name,
	URL:                 input.Repository.URL,
	DefaultBranch:       input.Repository.DefaultBranch,
	ArtifactKey:         artifactKey,
	ArtifactSHA256:      artifactSHA,
	SchemaVersion:       input.SchemaVersion,
	RootPath:            input.Snapshot.Root,
	CommitSHA:           input.Source.CommitSHA,
	SourceRef:           input.Source.Ref,
	ScannedAt:           scannedAt,
	ManifestCount:       summary.ManifestCount,
	WithDependencies:    summary.ManifestsWithDependenciesCount,
	WithoutDependencies: summary.ManifestsWithoutDependencies,
	UnknownDependencies: summary.ManifestsUnknownCount,
	DependencyCount:     summary.DependencyCount,
	Labels:              input.Labels,
	Annotation:          input.Annotation,
	Manifests:           manifests,
})
```

- [ ] **Step 5: Run the focused tests to verify they pass**

Run: `go test ./internal/http ./internal/scans -run 'TestDecodeUploadRequestAcceptsRawRepositoryOnlyHeaders|TestServiceUpload' -count=1`

Expected: PASS

- [ ] **Step 6: Commit the payload and upload contract change**

```bash
git add internal/scans/payload.go internal/scans/service.go internal/store/models.go internal/http/upload_handler.go internal/http/upload_handler_test.go internal/scans/service_test.go
git commit -m "refactor: remove project metadata from scan uploads"
```

### Task 3: Rework Scan Persistence Around Durable Manifests

**Files:**
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/uploads.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/models.go`
- Test: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/scan_details_test.go`
- Test: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/uploads_test.go`

- [ ] **Step 1: Add failing assertions for manifest reuse and inactive marking**

Extend the store test from Task 1 so it checks:

```go
require.Equal(t, []manifestRow{
	{
		Path:        "Cargo.lock",
		FirstSeenAt: time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
		LastSeenAt:  time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
		IsActive:    false,
	},
	{
		Path:        "package.json",
		FirstSeenAt: time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
		LastSeenAt:  time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC),
		IsActive:    true,
	},
}, got)
```

Also assert that the two `scan_manifests` rows for `package.json` point at the same `manifest_id`.

- [ ] **Step 2: Run the focused store tests**

Run: `go test ./internal/store -run 'TestCreateScanTracksManifestLifecycle|TestListScanManifests' -count=1`

Expected: FAIL because `CreateScan` still inserts projects, stores path on `scan_manifests`, and inserts dependency rows with `manifest_id`.

- [ ] **Step 3: Rewrite `CreateScan` to upsert repositories and manifests**

Replace the current `projects` upsert and direct `scan_manifests(path)` insert with repository and manifest lifecycle logic.

```go
var repositoryID string
err = tx.QueryRow(ctx, `
	insert into repositories (tenant_id, slug, name, url, default_branch)
	values ($1, $2, $3, $4, $5)
	on conflict (tenant_id, slug)
	do update set name = excluded.name, url = excluded.url, default_branch = excluded.default_branch
	returning id
`, params.TenantID, params.RepositorySlug, params.RepositoryName, params.URL, params.DefaultBranch).Scan(&repositoryID)
```

Insert `scans` without `project_id`, then upsert manifests:

```go
var manifestID string
err = tx.QueryRow(ctx, `
	insert into manifests (repository_id, path, first_seen_at, last_seen_at, is_active)
	values ($1, $2, $3, $3, true)
	on conflict (repository_id, path)
	do update set last_seen_at = excluded.last_seen_at, is_active = true
	returning id
`, repositoryID, manifest.Path, params.ScannedAt).Scan(&manifestID)
```

Insert `scan_manifests` with `manifest_id` instead of `path`:

```go
err = tx.QueryRow(ctx, `
	insert into scan_manifests (scan_id, manifest_id, position, type, has_dependencies, warnings)
	values ($1, $2, $3, $4, $5, $6::jsonb)
	returning id
`, scanID, manifestID, manifest.Position, manifest.Type, manifest.HasDependencies, string(warningsJSON)).Scan(&scanManifestID)
```

Insert dependencies against `scan_manifest_id`:

```go
_, err = tx.Exec(ctx, `
	insert into manifest_dependencies (
		scan_manifest_id, position, raw, name, version, "constraint", section, source, extras
	)
	values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
`, scanManifestID, dependency.Position, dependency.Raw, dependency.Name, dependency.Version, dependency.Constraint, dependency.Section, dependency.Source, string(extrasJSON))
```

- [ ] **Step 4: Mark absent manifests inactive in the same transaction**

After all manifest paths have been processed, deactivate rows missing from the current scan.

```go
_, err = tx.Exec(ctx, `
	update manifests
	set is_active = false
	where repository_id = $1
	  and path <> all($2::text[])
`, repositoryID, presentPaths)
if err != nil {
	return "", err
}
```

Build `presentPaths` from the current upload:

```go
presentPaths := make([]string, 0, len(params.Manifests))
for _, manifest := range params.Manifests {
	presentPaths = append(presentPaths, manifest.Path)
}
```

- [ ] **Step 5: Run the focused store tests to verify they pass**

Run: `go test ./internal/store -run 'TestCreateScanTracksManifestLifecycle|TestListScanManifests' -count=1`

Expected: PASS

- [ ] **Step 6: Commit the persistence rewrite**

```bash
git add internal/store/uploads.go internal/store/models.go internal/store/scan_details_test.go internal/store/uploads_test.go
git commit -m "feat: track manifest lifecycle during scan ingestion"
```

### Task 4: Rewrite Query Models and Handlers for Repository-Only Reads

**Files:**
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/scans.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/query_handlers.go`
- Test: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/query_handlers_test.go`

- [ ] **Step 1: Write failing query tests for repository-only responses**

Add assertions that scan and repository responses no longer contain `project_slug`, and that `GET /api/v1/scans/{scan_id}/manifests` still returns `path`.

```go
func TestGetScanOmitsProjectSlug(t *testing.T) {
	// existing token setup
	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-1", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	rr := httptest.NewRecorder()
	NewQueryRouter(service).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.NotContains(t, rr.Body.String(), "project_slug")
	require.Contains(t, rr.Body.String(), "repository_slug")
}
```

- [ ] **Step 2: Run the focused query tests**

Run: `go test ./internal/http -run 'TestGetScan|TestListRepositories|TestListScanManifests' -count=1`

Expected: FAIL because the read models and queries still join `projects`.

- [ ] **Step 3: Remove project read models and rewrite queries**

Update `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/store/scans.go`:

```go
type ScanListItem struct {
	ID              string            `json:"id"`
	RepositorySlug  string            `json:"repository_slug"`
	CommitSHA       string            `json:"commit_sha"`
	ScannedAt       time.Time         `json:"scanned_at"`
	ManifestCount   int               `json:"manifest_count"`
	DependencyCount int               `json:"dependency_count"`
	Labels          map[string]string `json:"labels"`
	Annotation      string            `json:"annotation"`
}

type RepositoryListItem struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}
```

Remove `ListProjects`, and rewrite repository and scan queries:

```go
select r.slug, r.name, r.url, r.default_branch
from repositories r
where r.tenant_id = $1
order by r.slug asc
```

```go
select s.id, r.slug, s.commit_sha, s.scanned_at, s.manifest_count, s.dependency_count, s.labels, s.annotation
from scans s
join repositories r on r.id = s.repository_id
where s.tenant_id = $1 and r.slug = $2 and s.scanned_at between $3 and $4
order by s.scanned_at desc
```

And join `manifests` when listing scan manifests:

```go
select sm.id, sm.type, m.path, sm.has_dependencies, sm.warnings
from scan_manifests sm
join scans s on s.id = sm.scan_id
join manifests m on m.id = sm.manifest_id
where s.tenant_id = $1 and s.id = $2
order by sm.position asc
```

- [ ] **Step 4: Remove the projects endpoint from the query handler**

Update `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/query_handlers.go`:

```go
type QueryService interface {
	ListRepositories(r *http.Request, token string) (any, error)
	ListScans(r *http.Request, token string) (any, error)
	GetScan(r *http.Request, token string) (any, error)
	ListScanManifests(r *http.Request, token string) (any, error)
	UpdateScanMetadata(r *http.Request, token string) error
}
```

Remove:

```go
mux.HandleFunc("GET /api/v1/projects", ...)
```

- [ ] **Step 5: Run the focused query tests to verify they pass**

Run: `go test ./internal/http -run 'TestGetScan|TestListRepositories|TestListScanManifests' -count=1`

Expected: PASS

- [ ] **Step 6: Commit the read-path rewrite**

```bash
git add internal/store/scans.go internal/http/query_handlers.go internal/http/query_handlers_test.go
git commit -m "refactor: remove project reads and join scan manifests to durable manifests"
```

### Task 5: Update API Contract, Docs, and Spec Tests

**Files:**
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/api/openapi.yaml`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/api/spec_test.go`
- Modify: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/README.md`
- Test: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/http/upload_handler_test.go`

- [ ] **Step 1: Write the failing API spec expectation**

Add or update a spec test to assert the OpenAPI document no longer mentions the project upload shape or `/api/v1/projects`.

```go
func TestOpenAPISpecDoesNotExposeProjects(t *testing.T) {
	data, err := os.ReadFile("api/openapi.yaml")
	require.NoError(t, err)
	require.NotContains(t, string(data), "/api/v1/projects")
	require.NotContains(t, string(data), "X-Deplens-Project-Slug")
	require.NotContains(t, string(data), "\"project\"")
}
```

- [ ] **Step 2: Run the docs/spec tests**

Run: `go test ./internal/api ./internal/http -run 'TestOpenAPISpecDoesNotExposeProjects|TestDecodeUploadRequestAcceptsRawRepositoryOnlyHeaders' -count=1`

Expected: FAIL because docs and spec still include project metadata.

- [ ] **Step 3: Rewrite the OpenAPI contract and README examples**

Update wrapped upload examples from:

```json
{
  "schema_version": "v1alpha1",
  "project": {"slug": "core", "name": "Core"},
  "repository": {"slug": "repo", "name": "Repo", "url": "https://example.com/repo.git", "default_branch": "main"},
  "source": {"commit_sha": "abc123", "ref": "refs/heads/main", "scanned_at": "2026-05-04T10:00:00Z"},
  "snapshot": {"root": ".", "manifests": []}
}
```

To:

```json
{
  "schema_version": "v1alpha1",
  "repository": {"slug": "repo", "name": "Repo", "url": "https://example.com/repo.git", "default_branch": "main"},
  "source": {"commit_sha": "abc123", "ref": "refs/heads/main", "scanned_at": "2026-05-04T10:00:00Z"},
  "snapshot": {"root": ".", "manifests": []}
}
```

And remove:

- `/api/v1/projects`
- `project_slug` response fields
- `X-Deplens-Project-Slug`
- `X-Deplens-Project-Name`

- [ ] **Step 4: Run the docs/spec tests to verify they pass**

Run: `go test ./internal/api ./internal/http -run 'TestOpenAPISpecDoesNotExposeProjects|TestDecodeUploadRequestAcceptsRawRepositoryOnlyHeaders' -count=1`

Expected: PASS

- [ ] **Step 5: Commit the contract and docs update**

```bash
git add api/openapi.yaml internal/api/spec_test.go README.md internal/http/upload_handler_test.go
git commit -m "docs: document repository-only upload and read contracts"
```

### Task 6: Full Verification and Cleanup

**Files:**
- Modify as needed: any touched file from earlier tasks
- Test: `/home/jekos/ghq/github.com/ferretsecurity/deplens-platform/internal/...`

- [ ] **Step 1: Run the full test suite**

Run: `go test ./... -count=1`

Expected: PASS

- [ ] **Step 2: Smoke-test the end-to-end API flow**

Run the local validation sequence:

```bash
docker compose up -d postgres
set -a && source .env.example && set +a
go run ./cmd/deplens-platform
```

In another shell:

```bash
curl -i -c /tmp/deplens.cookies \
  -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"change-me-now"}'
```

Then:

```bash
curl -s -b /tmp/deplens.cookies \
  -X POST http://localhost:8080/api/v1/tokens \
  -H 'Content-Type: application/json' \
  -d '{"label":"scanner","scopes":["scan:write","scan:read","scan:metadata:write"]}'
```

And upload one raw repository-only scan:

```bash
curl -i \
  -X POST http://localhost:8080/api/v1/scans \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'X-Deplens-Repository-Slug: repo' \
  -H 'X-Deplens-Repository-Name: Repo' \
  -H 'X-Deplens-Repository-URL: https://example.com/repo.git' \
  -H 'X-Deplens-Default-Branch: main' \
  -H 'X-Deplens-Commit-SHA: abc123' \
  -H 'X-Deplens-Ref: refs/heads/main' \
  -H 'X-Deplens-Scanned-At: 2026-05-08T10:00:00Z' \
  --data-binary @/tmp/deplens-with-deps.json
```

Expected: `201 Created`, no project headers required, `GET /api/v1/scans/{scan_id}/manifests` returns paths through the joined `manifests` table.

- [ ] **Step 3: Review for dead code and remove project leftovers**

Search for stale references:

Run: `rg -n "project|ProjectSlug|ProjectName|/api/v1/projects|X-Deplens-Project" /home/jekos/ghq/github.com/ferretsecurity/deplens-platform`

Expected: only historical design docs and old plan/spec files should contain project references. No live code, tests, README, or OpenAPI should depend on them.

- [ ] **Step 4: Commit final cleanup**

```bash
git add -A
git commit -m "test: verify repository manifest lifecycle redesign"
```

## Self-Review

**Spec coverage:** Covered schema reshape, repository-only uploads, durable manifest lifecycle, immutable scan manifests and dependencies, read-path joins, inactive marking, docs, and verification.

**Placeholder scan:** No `TODO`, `TBD`, or deferred implementation language remains in executable tasks.

**Type consistency:** The plan consistently uses `repository_id + path` as manifest identity, `scan_manifest_id` as the dependency foreign key, and repository-only upload/read contracts.
