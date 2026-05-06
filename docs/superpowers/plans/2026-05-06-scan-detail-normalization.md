# Scan Detail Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist uploaded Deplens manifests and dependencies in Postgres and expose them through a dedicated per-scan manifest detail API.

**Architecture:** Keep the current upload contract and blob artifact storage intact, but extend the synchronous upload transaction to insert normalized `scan_manifests` and `manifest_dependencies` rows after creating the parent `scans` row. Keep `GET /api/v1/scans/{scan_id}` as a summary endpoint and add `GET /api/v1/scans/{scan_id}/manifests` for nested manifest and dependency detail ordered by input position.

**Tech Stack:** Go, `net/http`, PostgreSQL, pgx, goose migrations, existing store/http layers, `go test`

---

## File Map

- Create: `db/migrations/002_scan_detail_tables.sql`
- Modify: `internal/store/models.go`
- Modify: `internal/store/uploads.go`
- Modify: `internal/store/scans.go`
- Modify: `internal/http/query_handlers.go`
- Modify: `internal/http/query_handlers_test.go`
- Modify: `internal/http/upload_handler_test.go`
- Modify: `internal/api/spec_test.go`
- Modify: `api/openapi.yaml`
- Modify: `README.md`
- Test: `internal/store/scan_details_test.go`

### Task 1: Add relational schema for scan details

**Files:**
- Create: `db/migrations/002_scan_detail_tables.sql`
- Test: `internal/store/scan_details_test.go`

- [ ] **Step 1: Write the failing integration test for normalized detail persistence**

Create `internal/store/scan_details_test.go` with a test that migrates a fresh Postgres container, inserts a scan through `Store.CreateScan`, and then asserts one manifest row and one dependency row exist:

```go
package store

import (
	"context"
	"testing"
	"time"
)

func TestCreateScanPersistsManifestsAndDependencies(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)

	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	store := Store{DB: db}
	hasDependencies := true
	scanID, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:            mustCreateTenant(t, ctx, db),
		ProjectSlug:         "core",
		ProjectName:         "Core",
		RepositorySlug:      "repo",
		RepositoryName:      "Repo",
		URL:                 "https://example.com/repo.git",
		DefaultBranch:       "main",
		ArtifactKey:         "artifacts/test.json",
		ArtifactSHA256:      "sha256",
		SchemaVersion:       "v1alpha1",
		RootPath:            ".",
		CommitSHA:           "abc123",
		SourceRef:           "refs/heads/main",
		ScannedAt:           time.Date(2026, 5, 6, 8, 0, 0, 0, time.UTC),
		ManifestCount:       1,
		WithDependencies:    1,
		WithoutDependencies: 0,
		UnknownDependencies: 0,
		DependencyCount:     1,
		Labels:              map[string]string{},
		Annotation:          "",
		Manifests: []UploadManifestParams{
			{
				Position:        0,
				Type:            "npm-package-lock",
				Path:            "package-lock.json",
				HasDependencies: &hasDependencies,
				Warnings:        []string{"warning"},
				Dependencies: []UploadDependencyParams{
					{
						Position: 0,
						Raw:      "react@19.1.0",
						Name:     "react",
						Version:  "19.1.0",
						Section:  "dependencies",
						Source:   "npm",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}

	var manifestCount int
	if err := db.QueryRow(ctx, `select count(*) from scan_manifests where scan_id = $1`, scanID).Scan(&manifestCount); err != nil {
		t.Fatalf("manifest count query error = %v", err)
	}
	if manifestCount != 1 {
		t.Fatalf("manifest count = %d, want 1", manifestCount)
	}

	var dependencyCount int
	if err := db.QueryRow(ctx, `
		select count(*)
		from manifest_dependencies d
		join scan_manifests m on m.id = d.manifest_id
		where m.scan_id = $1
	`, scanID).Scan(&dependencyCount); err != nil {
		t.Fatalf("dependency count query error = %v", err)
	}
	if dependencyCount != 1 {
		t.Fatalf("dependency count = %d, want 1", dependencyCount)
	}
}
```

- [ ] **Step 2: Run the new store test and verify it fails because the tables/types do not exist**

Run: `go test ./internal/store -run TestCreateScanPersistsManifestsAndDependencies -count=1`

Expected: FAIL with compile errors for missing `UploadManifestParams` / `UploadDependencyParams` or SQL errors for missing `scan_manifests` and `manifest_dependencies`.

- [ ] **Step 3: Add the migration for normalized detail tables**

Create `db/migrations/002_scan_detail_tables.sql`:

```sql
-- +goose Up
create table if not exists scan_manifests (
    id uuid primary key default gen_random_uuid(),
    scan_id uuid not null references scans(id) on delete cascade,
    position integer not null,
    type text not null,
    path text not null,
    has_dependencies boolean null,
    warnings jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    unique (scan_id, position)
);

create index if not exists scan_manifests_scan_position_idx
    on scan_manifests (scan_id, position);

create table if not exists manifest_dependencies (
    id uuid primary key default gen_random_uuid(),
    manifest_id uuid not null references scan_manifests(id) on delete cascade,
    position integer not null,
    raw text not null,
    name text not null default '',
    version text not null default '',
    constraint text not null default '',
    section text not null default '',
    source text not null default '',
    extras jsonb not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    unique (manifest_id, position)
);

create index if not exists manifest_dependencies_manifest_position_idx
    on manifest_dependencies (manifest_id, position);

-- +goose Down
drop table if exists manifest_dependencies;
drop table if exists scan_manifests;
```

- [ ] **Step 4: Add upload parameter types required by the test**

Modify `internal/store/models.go` to add:

```go
type UploadManifestParams struct {
	Position        int
	Type            string
	Path            string
	HasDependencies *bool
	Warnings        []string
	Dependencies    []UploadDependencyParams
}

type UploadDependencyParams struct {
	Position   int
	Raw        string
	Name       string
	Version    string
	Constraint string
	Section    string
	Source     string
	Extras     []string
}
```

and extend `UploadScanParams`:

```go
	Manifests []UploadManifestParams
```

- [ ] **Step 5: Run the store test again**

Run: `go test ./internal/store -run TestCreateScanPersistsManifestsAndDependencies -count=1`

Expected: FAIL because `CreateScan` still does not insert manifest/dependency rows.

- [ ] **Step 6: Commit the migration/types checkpoint**

```bash
git add db/migrations/002_scan_detail_tables.sql internal/store/models.go internal/store/scan_details_test.go
git commit -m "feat: add schema for scan detail tables"
```

### Task 2: Persist manifests and dependencies during upload

**Files:**
- Modify: `internal/store/uploads.go`
- Modify: `internal/store/models.go`
- Test: `internal/store/scan_details_test.go`

- [ ] **Step 1: Extend the integration test to assert stored field values and ordering**

Add assertions in `internal/store/scan_details_test.go` for:

```go
var (
	position int
	manifestType string
	path string
	hasDependencies *bool
	warningsJSON []byte
)
err = db.QueryRow(ctx, `
	select position, type, path, has_dependencies, warnings
	from scan_manifests
	where scan_id = $1
`, scanID).Scan(&position, &manifestType, &path, &hasDependencies, &warningsJSON)
```

and one dependency row:

```go
var (
	depPosition int
	raw string
	name string
	version string
	section string
	source string
	extrasJSON []byte
)
err = db.QueryRow(ctx, `
	select d.position, d.raw, d.name, d.version, d.section, d.source, d.extras
	from manifest_dependencies d
	join scan_manifests m on m.id = d.manifest_id
	where m.scan_id = $1
`, scanID).Scan(&depPosition, &raw, &name, &version, &section, &source, &extrasJSON)
```

- [ ] **Step 2: Run the test and verify red**

Run: `go test ./internal/store -run TestCreateScanPersistsManifestsAndDependencies -count=1`

Expected: FAIL because counts or row lookups still come back empty.

- [ ] **Step 3: Implement manifest/dependency inserts inside `CreateScan`**

In `internal/store/uploads.go`, after inserting the `scans` row and before `tx.Commit`, add helpers like:

```go
func insertManifests(ctx context.Context, tx pgx.Tx, scanID string, manifests []UploadManifestParams) error {
	for _, manifest := range manifests {
		warningsJSON, err := json.Marshal(manifest.Warnings)
		if err != nil {
			return err
		}

		var manifestID string
		err = tx.QueryRow(ctx, `
			insert into scan_manifests (scan_id, position, type, path, has_dependencies, warnings)
			values ($1, $2, $3, $4, $5, $6::jsonb)
			returning id
		`, scanID, manifest.Position, manifest.Type, manifest.Path, manifest.HasDependencies, string(warningsJSON)).Scan(&manifestID)
		if err != nil {
			return err
		}

		if err := insertDependencies(ctx, tx, manifestID, manifest.Dependencies); err != nil {
			return err
		}
	}
	return nil
}
```

and:

```go
func insertDependencies(ctx context.Context, tx pgx.Tx, manifestID string, dependencies []UploadDependencyParams) error {
	for _, dependency := range dependencies {
		extrasJSON, err := json.Marshal(dependency.Extras)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			insert into manifest_dependencies (
				manifest_id, position, raw, name, version, constraint, section, source, extras
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		`, manifestID, dependency.Position, dependency.Raw, dependency.Name, dependency.Version, dependency.Constraint, dependency.Section, dependency.Source, string(extrasJSON))
		if err != nil {
			return err
		}
	}
	return nil
}
```

Then call:

```go
	if err := insertManifests(ctx, tx, scanID, params.Manifests); err != nil {
		return "", err
	}
```

- [ ] **Step 4: Run the targeted store test and verify green**

Run: `go test ./internal/store -run TestCreateScanPersistsManifestsAndDependencies -count=1`

Expected: PASS

- [ ] **Step 5: Run the full store test package**

Run: `go test ./internal/store -count=1`

Expected: PASS or Docker-dependent tests skip cleanly if Docker is unavailable.

- [ ] **Step 6: Commit the upload persistence work**

```bash
git add internal/store/uploads.go internal/store/models.go internal/store/scan_details_test.go
git commit -m "feat: persist normalized scan details"
```

### Task 3: Pass manifest data from scan service into store writes

**Files:**
- Modify: `internal/scans/service.go`
- Test: `internal/http/upload_handler_test.go`

- [ ] **Step 1: Add a failing upload test that proves manifest data reaches the service boundary**

Extend `internal/http/upload_handler_test.go` with a wrapped upload body containing one manifest and one dependency, then assert the fake service captured it:

```go
func TestUploadWrappedPayloadPreservesManifestDetails(t *testing.T) {
	service := fakeUploadService{allowedToken: "bootstrap-token"}
	handler := NewRouter(service)

	body := []byte(`{
	  "schema_version": "v1alpha1",
	  "project": {"slug":"core","name":"Core"},
	  "repository": {"slug":"repo","name":"Repo","url":"https://example.com/repo.git","default_branch":"main"},
	  "source": {"commit_sha":"abc123","ref":"refs/heads/main","scanned_at":"2026-05-04T10:00:00Z"},
	  "snapshot": {
	    "root": ".",
	    "manifests": [{
	      "type": "npm-package-lock",
	      "path": "package-lock.json",
	      "has_dependencies": true,
	      "dependencies": [{"raw":"react@19.1.0","name":"react","version":"19.1.0"}]
	    }]
	  }
	}`)
```

Use a pointer-backed fake so the assertion sees captured state:

```go
service := &fakeUploadService{allowedToken: "bootstrap-token"}
handler := NewRouter(service)
```

Assert:

```go
if got := len(service.lastInput.Snapshot.Manifests); got != 1 {
	t.Fatalf("manifest count = %d, want 1", got)
}
if got := len(service.lastInput.Snapshot.Manifests[0].Dependencies); got != 1 {
	t.Fatalf("dependency count = %d, want 1", got)
}
```

- [ ] **Step 2: Run the upload handler tests**

Run: `go test ./internal/http -run 'TestUpload(WrappedPayloadPreservesManifestDetails|ScanReturnsCreatedForValidBearerToken|RawDeplensJSONReturnsCreatedForValidBearerToken|RawDeplensJSONFailsWithoutRequiredHeaders)$' -count=1`

Expected: PASS for the new test if decoding already preserves structure. If the fake cannot capture state due to value receiver semantics, FAIL for that reason.

- [ ] **Step 3: Fix the fake upload service to capture the last request reliably**

In `internal/http/upload_handler_test.go`, change:

```go
type fakeUploadService struct {
	allowedToken string
	lastInput    scans.UploadRequest
}

func (f fakeUploadService) Upload(_ *http.Request, token string, input scans.UploadRequest) (string, error) {
```

to:

```go
type fakeUploadService struct {
	allowedToken string
	lastInput    scans.UploadRequest
}

func (f *fakeUploadService) Upload(_ *http.Request, token string, input scans.UploadRequest) (string, error) {
```

and update existing tests to pass `&fakeUploadService{...}` to `NewRouter`.

- [ ] **Step 4: Extend `internal/scans/service.go` to pass manifests into `UploadScanParams`**

Add a helper:

```go
func toUploadManifestParams(input []ManifestInput) []store.UploadManifestParams {
	items := make([]store.UploadManifestParams, 0, len(input))
	for i, manifest := range input {
		dependencies := make([]store.UploadDependencyParams, 0, len(manifest.Dependencies))
		for j, dependency := range manifest.Dependencies {
			dependencies = append(dependencies, store.UploadDependencyParams{
				Position:   j,
				Raw:        dependency.Raw,
				Name:       dependency.Name,
				Version:    dependency.Version,
				Constraint: dependency.Constraint,
				Section:    dependency.Section,
				Source:     dependency.Source,
				Extras:     dependency.Extras,
			})
		}
		items = append(items, store.UploadManifestParams{
			Position:        i,
			Type:            manifest.Type,
			Path:            manifest.Path,
			HasDependencies: manifest.HasDependencies,
			Warnings:        manifest.Warnings,
			Dependencies:    dependencies,
		})
	}
	return items
}
```

and set:

```go
		Manifests:           toUploadManifestParams(input.Snapshot.Manifests),
```

inside the `store.UploadScanParams` literal.

- [ ] **Step 5: Run the focused HTTP tests and the scan package**

Run: `go test ./internal/http ./internal/scans -count=1`

Expected: PASS

- [ ] **Step 6: Commit the manifest mapping checkpoint**

```bash
git add internal/http/upload_handler_test.go internal/scans/service.go
git commit -m "feat: carry manifest details through upload service"
```

### Task 4: Add scan manifest read models and store query

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/scans.go`
- Test: `internal/store/scan_details_test.go`

- [ ] **Step 1: Write the failing store read test for manifest detail**

Add to `internal/store/scan_details_test.go`:

```go
func TestListScanManifestsReturnsNestedDependenciesInOrder(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)

	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)
	scanID := mustCreateScanWithDetails(t, ctx, store, tenantID)

	readStore := ScanStore{DB: db}
	items, err := readStore.ListScanManifests(ctx, tenantID, scanID)
	if err != nil {
		t.Fatalf("ListScanManifests() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("manifest count = %d, want 1", len(items))
	}
	if got := len(items[0].Dependencies); got != 1 {
		t.Fatalf("dependency count = %d, want 1", got)
	}
	if items[0].Dependencies[0].Name != "react" {
		t.Fatalf("dependency name = %q, want react", items[0].Dependencies[0].Name)
	}
}
```

- [ ] **Step 2: Run the new store read test**

Run: `go test ./internal/store -run TestListScanManifestsReturnsNestedDependenciesInOrder -count=1`

Expected: FAIL because `ListScanManifests` and its response types do not exist.

- [ ] **Step 3: Add response models for manifest detail**

In `internal/store/models.go`, add:

```go
type ScanManifestItem struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	Path            string                 `json:"path"`
	HasDependencies *bool                  `json:"has_dependencies"`
	Warnings        []string               `json:"warnings"`
	Dependencies    []ManifestDependencyItem `json:"dependencies"`
}

type ManifestDependencyItem struct {
	ID         string   `json:"id"`
	Raw        string   `json:"raw"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Constraint string   `json:"constraint"`
	Section    string   `json:"section"`
	Source     string   `json:"source"`
	Extras     []string `json:"extras"`
}
```

- [ ] **Step 4: Implement `ListScanManifests` in `internal/store/scans.go`**

Add:

```go
func (s ScanStore) ListScanManifests(ctx context.Context, tenantID string, scanID string) ([]ScanManifestItem, error) {
	rows, err := s.DB.Query(ctx, `
		select m.id, m.position, m.type, m.path, m.has_dependencies, m.warnings
		from scan_manifests m
		join scans s on s.id = m.scan_id
		where s.tenant_id = $1 and s.id = $2
		order by m.position asc
	`, tenantID, scanID)
```

Collect manifest IDs, unmarshal `warnings`, then fetch dependencies:

```go
	depRows, err := s.DB.Query(ctx, `
		select d.id, d.manifest_id, d.position, d.raw, d.name, d.version, d.constraint, d.section, d.source, d.extras
		from manifest_dependencies d
		where d.manifest_id = any($1)
		order by d.manifest_id asc, d.position asc
	`, manifestIDs)
```

Assemble into nested `[]ScanManifestItem`.

- [ ] **Step 5: Run the store tests**

Run: `go test ./internal/store -count=1`

Expected: PASS

- [ ] **Step 6: Commit the store read path**

```bash
git add internal/store/models.go internal/store/scans.go internal/store/scan_details_test.go
git commit -m "feat: add scan manifest detail queries"
```

### Task 5: Expose `GET /api/v1/scans/{scan_id}/manifests`

**Files:**
- Modify: `internal/http/query_handlers.go`
- Modify: `internal/http/query_handlers_test.go`

- [ ] **Step 1: Write the failing HTTP handler test**

Add to `internal/http/query_handlers_test.go`:

```go
func TestGetScanManifestsReturnsOK(t *testing.T) {
	handler := NewQueryRouter(fakeQueryService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-123/manifests", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}
```

and extend the fake service:

```go
func (fakeQueryService) ListScanManifests(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return map[string]any{
		"items": []map[string]any{
			{"id": "manifest-123", "type": "npm-package-lock", "dependencies": []map[string]string{{"id": "dep-123"}}},
		},
	}, nil
}
```

- [ ] **Step 2: Run the HTTP handler tests and verify red**

Run: `go test ./internal/http -run 'Test(GetScanManifestsReturnsOK|GetScanReturnsOK|ListScansSupportsRepositoryAndTimeFilters|PatchScanMetadataReturnsOK)$' -count=1`

Expected: FAIL because the interface and router do not yet include `ListScanManifests`.

- [ ] **Step 3: Add the query service/store methods and route**

In `internal/http/query_handlers.go`:

```go
type QueryService interface {
	ListProjects(r *http.Request, token string) (any, error)
	ListRepositories(r *http.Request, token string) (any, error)
	ListScans(r *http.Request, token string) (any, error)
	GetScan(r *http.Request, token string) (any, error)
	ListScanManifests(r *http.Request, token string) (any, error)
	UpdateScanMetadata(r *http.Request, token string) error
}
```

```go
type QueryStore interface {
	ListProjects(ctx context.Context, tenantID string) ([]store.ProjectListItem, error)
	ListRepositories(ctx context.Context, tenantID string) ([]store.RepositoryListItem, error)
	ListScans(ctx context.Context, filter store.ScanFilter) ([]store.ScanListItem, error)
	GetScan(ctx context.Context, tenantID string, scanID string) (store.ScanListItem, error)
	ListScanManifests(ctx context.Context, tenantID string, scanID string) ([]store.ScanManifestItem, error)
	UpdateScanMetadata(ctx context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error
}
```

Implement:

```go
func (s ProductionQueryService) ListScanManifests(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}
	items, err := s.Reads.ListScanManifests(r.Context(), tenantID, r.PathValue("scan_id"))
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items}, nil
}
```

Wire the route:

```go
	mux.HandleFunc("GET /api/v1/scans/{scan_id}/manifests", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResult(w, r, service.ListScanManifests)
	})
```

- [ ] **Step 4: Run the HTTP package tests**

Run: `go test ./internal/http -count=1`

Expected: PASS

- [ ] **Step 5: Commit the HTTP endpoint**

```bash
git add internal/http/query_handlers.go internal/http/query_handlers_test.go
git commit -m "feat: expose scan manifest detail endpoint"
```

### Task 6: Document the new API contract

**Files:**
- Modify: `api/openapi.yaml`
- Modify: `internal/api/spec_test.go`
- Modify: `README.md`

- [ ] **Step 1: Add a failing spec test expectation**

In `internal/api/spec_test.go`, extend the required path list with:

```go
"/api/v1/scans/{scan_id}/manifests:",
```

- [ ] **Step 2: Run the API spec test**

Run: `go test ./internal/api -count=1`

Expected: FAIL because the OpenAPI spec does not yet document the new path.

- [ ] **Step 3: Update `api/openapi.yaml`**

Add:

```yaml
  /api/v1/scans/{scan_id}/manifests:
    get:
      summary: List manifests for a scan
      parameters:
        - in: path
          name: scan_id
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Scan manifest detail
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ScanManifestListResponse'
```

Add component schemas:

```yaml
    ScanManifestListResponse:
      type: object
      required: [items]
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/ScanManifest'
```

```yaml
    ScanManifest:
      type: object
      required: [id, type, path, warnings, dependencies]
      properties:
        id:
          type: string
        type:
          type: string
        path:
          type: string
        has_dependencies:
          type: boolean
          nullable: true
        warnings:
          type: array
          items:
            type: string
        dependencies:
          type: array
          items:
            $ref: '#/components/schemas/ManifestDependency'
```

```yaml
    ManifestDependency:
      type: object
      required: [id, raw, name, version, constraint, section, source, extras]
      properties:
        id:
          type: string
        raw:
          type: string
        name:
          type: string
        version:
          type: string
        constraint:
          type: string
        section:
          type: string
        source:
          type: string
        extras:
          type: array
          items:
            type: string
```

- [ ] **Step 4: Update `README.md` with a curl example**

Add a section showing:

```bash
curl -s \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/scans/$SCAN_ID/manifests" | jq
```

and explain that `GET /api/v1/scans/{scan_id}` remains a summary endpoint.

- [ ] **Step 5: Run the API/documentation verification**

Run: `go test ./internal/api -count=1`

Expected: PASS

- [ ] **Step 6: Commit the contract/docs update**

```bash
git add api/openapi.yaml internal/api/spec_test.go README.md
git commit -m "docs: describe scan manifest detail API"
```

### Task 7: Final verification

**Files:**
- Verify all touched files above

- [ ] **Step 1: Run the full test suite**

Run: `go test ./... -count=1`

Expected: PASS

- [ ] **Step 2: Smoke test the endpoint manually**

Run the app and verify the detail endpoint returns data:

```bash
curl -s \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/scans/$SCAN_ID/manifests" | jq '.items[0].dependencies[0].name'
```

Expected output:

```text
"react"
```

- [ ] **Step 3: Review git diff for accidental scope creep**

Run: `git diff --stat origin/main...HEAD`

Expected: only migration, store, HTTP, spec, and README changes related to scan detail normalization.

- [ ] **Step 4: Create the final implementation commit if needed**

```bash
git add db/migrations/002_scan_detail_tables.sql internal/store/models.go internal/store/uploads.go internal/store/scans.go internal/store/scan_details_test.go internal/scans/service.go internal/http/query_handlers.go internal/http/query_handlers_test.go internal/http/upload_handler_test.go internal/api/spec_test.go api/openapi.yaml README.md
git commit -m "feat: persist and expose scan manifest details"
```
