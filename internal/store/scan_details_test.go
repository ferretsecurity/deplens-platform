package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestCreateScanPersistsManifestsAndDependencies(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)

	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	store := Store{DB: db}
	scanID := mustCreateScanWithDetails(t, ctx, store, mustCreateTenant(t, ctx, db))

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
		join scan_manifests m on m.id = d.scan_manifest_id
		where m.scan_id = $1
	`, scanID).Scan(&dependencyCount); err != nil {
		t.Fatalf("dependency count query error = %v", err)
	}
	if dependencyCount != 1 {
		t.Fatalf("dependency count = %d, want 1", dependencyCount)
	}
}

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
	if items[0].Type != "npm-package-lock" {
		t.Fatalf("manifest type = %q, want npm-package-lock", items[0].Type)
	}
	if items[0].Path != "package-lock.json" {
		t.Fatalf("manifest path = %q, want package-lock.json", items[0].Path)
	}
	if items[0].HasDependencies == nil || !*items[0].HasDependencies {
		t.Fatalf("manifest has_dependencies = %v, want true", items[0].HasDependencies)
	}
	if !reflect.DeepEqual(items[0].Warnings, []string{"warning"}) {
		t.Fatalf("manifest warnings = %#v, want [warning]", items[0].Warnings)
	}
	if got := len(items[0].Dependencies); got != 1 {
		t.Fatalf("dependency count = %d, want 1", got)
	}
	if items[0].Dependencies[0].Raw != "react@19.1.0" {
		t.Fatalf("dependency raw = %q, want react@19.1.0", items[0].Dependencies[0].Raw)
	}
	if items[0].Dependencies[0].Name != "react" {
		t.Fatalf("dependency name = %q, want react", items[0].Dependencies[0].Name)
	}
	if items[0].Dependencies[0].Version != "19.1.0" {
		t.Fatalf("dependency version = %q, want 19.1.0", items[0].Dependencies[0].Version)
	}
	if items[0].Dependencies[0].Section != "dependencies" {
		t.Fatalf("dependency section = %q, want dependencies", items[0].Dependencies[0].Section)
	}
	if items[0].Dependencies[0].Source != "npm" {
		t.Fatalf("dependency source = %q, want npm", items[0].Dependencies[0].Source)
	}
	if !reflect.DeepEqual(items[0].Dependencies[0].Extras, map[string]string{"scope": "ui"}) {
		t.Fatalf("dependency extras = %#v, want map with scope=ui", items[0].Dependencies[0].Extras)
	}
}

func TestCreateScanTracksManifestLifecycle(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)

	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)

	firstID, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
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
	if err != nil {
		t.Fatalf("CreateScan() first error = %v", err)
	}
	if firstID == "" {
		t.Fatal("CreateScan() first ID is empty")
	}

	secondID, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
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
	if err != nil {
		t.Fatalf("CreateScan() second error = %v", err)
	}
	if secondID == "" {
		t.Fatal("CreateScan() second ID is empty")
	}

	rows, err := store.DB.Query(ctx, `
		select path, first_seen_at, last_seen_at, is_active
		from manifests
		order by path asc
	`)
	if err != nil {
		t.Fatalf("query manifests error = %v", err)
	}
	defer rows.Close()

	type manifestRow struct {
		Path        string
		FirstSeenAt time.Time
		LastSeenAt  time.Time
		IsActive    bool
	}

	got := make([]manifestRow, 0, 2)
	for rows.Next() {
		var row manifestRow
		if err := rows.Scan(&row.Path, &row.FirstSeenAt, &row.LastSeenAt, &row.IsActive); err != nil {
			t.Fatalf("scan manifest row error = %v", err)
		}
		row.FirstSeenAt = row.FirstSeenAt.UTC()
		row.LastSeenAt = row.LastSeenAt.UTC()
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate manifests error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("manifest row count = %d, want 2", len(got))
	}

	firstScanTime := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	secondScanTime := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)

	require.Equal(t, []manifestRow{
		{
			Path:        "Cargo.lock",
			FirstSeenAt: firstScanTime,
			LastSeenAt:  firstScanTime,
			IsActive:    false,
		},
		{
			Path:        "package.json",
			FirstSeenAt: firstScanTime,
			LastSeenAt:  secondScanTime,
			IsActive:    true,
		},
	}, got)

	var packageManifestIDs []string
	if err := db.QueryRow(ctx, `
		select array_agg(manifest_id order by scan_id asc)
		from scan_manifests
		where scan_id in ($1, $2)
		  and manifest_id = (
			  select id
			  from manifests
			  where repository_id = (
				  select repository_id
				  from scans
				  where id = $1
			  )
			  and path = 'package.json'
		  )
	`, firstID, secondID).Scan(&packageManifestIDs); err != nil {
		t.Fatalf("query package manifest ids error = %v", err)
	}
	require.Len(t, packageManifestIDs, 2)
	require.Equal(t, packageManifestIDs[0], packageManifestIDs[1])
}

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

	var url string
	var branch string
	err = db.QueryRow(ctx, `
		select url, default_branch
		from repositories
		where tenant_id = $1 and name = $2
	`, tenantID, "Repo").Scan(&url, &branch)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/two.git", url)
	require.Equal(t, "stable", branch)
}

func TestCreateScanWithoutSlugKeepsRepositoriesDistinctByName(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	require.NoError(t, Migrate(databaseURL))

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)

	_, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
		RepositoryName: "Repo One",
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
		RepositoryName: "Repo Two",
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

	var repositoryCount int
	err = db.QueryRow(ctx, `
		select count(*)
		from repositories
		where tenant_id = $1
	`, tenantID).Scan(&repositoryCount)
	require.NoError(t, err)
	require.Equal(t, 2, repositoryCount)
}

func mustCreateScanWithDetails(t *testing.T, ctx context.Context, store Store, tenantID string) string {
	t.Helper()

	hasDependencies := true
	scanID, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:            tenantID,
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
						Extras:   map[string]string{"scope": "ui"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateScan() error = %v", err)
	}

	return scanID
}

func mustCreateTenant(t *testing.T, ctx context.Context, db *pgxpool.Pool) string {
	t.Helper()

	var tenantID string
	if err := db.QueryRow(ctx, `
		insert into tenants (slug, name)
		values ('scan-detail-tests', 'Scan Detail Tests')
		returning id
	`).Scan(&tenantID); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	return tenantID
}
