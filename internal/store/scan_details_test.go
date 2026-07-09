package store

import (
	"context"
	"database/sql"
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

func TestCreateScanRemovesPreviousScanManifestForSameManifestPath(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	require.NoError(t, Migrate(databaseURL))

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)
	hasDependencies := true

	_, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:            tenantID,
		RepositoryName:      "Repo",
		URL:                 "https://example.com/repo.git",
		DefaultBranch:       "main",
		ArtifactKey:         "artifact-1",
		ArtifactSHA256:      "sha-1",
		SchemaVersion:       "v1alpha1",
		RootPath:            ".",
		CommitSHA:           "commit-1",
		SourceRef:           "refs/heads/main",
		ScannedAt:           time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
		ManifestCount:       1,
		WithDependencies:    1,
		WithoutDependencies: 0,
		UnknownDependencies: 0,
		DependencyCount:     1,
		Labels:              map[string]string{},
		Manifests: []UploadManifestParams{
			{
				Position:        0,
				Type:            "npm-package-lock",
				Path:            "package-lock.json",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "lodash@4.17.21", Name: "lodash", Version: "4.17.21"},
				},
			},
		},
	})
	require.NoError(t, err)

	secondScanID, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:            tenantID,
		RepositoryName:      "Repo",
		URL:                 "https://example.com/repo.git",
		DefaultBranch:       "main",
		ArtifactKey:         "artifact-2",
		ArtifactSHA256:      "sha-2",
		SchemaVersion:       "v1alpha1",
		RootPath:            ".",
		CommitSHA:           "commit-2",
		SourceRef:           "refs/heads/main",
		ScannedAt:           time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC),
		ManifestCount:       1,
		WithDependencies:    1,
		WithoutDependencies: 0,
		UnknownDependencies: 0,
		DependencyCount:     1,
		Labels:              map[string]string{},
		Manifests: []UploadManifestParams{
			{
				Position:        0,
				Type:            "npm-package-lock",
				Path:            "package-lock.json",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "react@19.1.0", Name: "react", Version: "19.1.0"},
				},
			},
		},
	})
	require.NoError(t, err)

	var scanManifestCount int
	err = db.QueryRow(ctx, `
		select count(*)
		from scan_manifests sm
		join manifests m on m.id = sm.manifest_id
		where m.repository_id = (
			select repository_id
			from scans
			where id = $1
		)
		  and m.path = 'package-lock.json'
	`, secondScanID).Scan(&scanManifestCount)
	require.NoError(t, err)
	require.Equal(t, 1, scanManifestCount)

	var rawDependencies []string
	err = db.QueryRow(ctx, `
		select coalesce(array_agg(d.raw order by d.position), '{}'::text[])
		from manifest_dependencies d
		join scan_manifests sm on sm.id = d.scan_manifest_id
		join manifests m on m.id = sm.manifest_id
		where m.repository_id = (
			select repository_id
			from scans
			where id = $1
		)
		  and m.path = 'package-lock.json'
	`, secondScanID).Scan(&rawDependencies)
	require.NoError(t, err)
	require.Equal(t, []string{"react@19.1.0"}, rawDependencies)
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
		select path, first_seen_at, last_seen_at, disappeared_at
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
		Disappeared sql.NullTime
	}

	got := make([]manifestRow, 0, 2)
	for rows.Next() {
		var row manifestRow
		if err := rows.Scan(&row.Path, &row.FirstSeenAt, &row.LastSeenAt, &row.Disappeared); err != nil {
			t.Fatalf("scan manifest row error = %v", err)
		}
		row.FirstSeenAt = row.FirstSeenAt.UTC()
		row.LastSeenAt = row.LastSeenAt.UTC()
		if row.Disappeared.Valid {
			row.Disappeared.Time = row.Disappeared.Time.UTC()
		}
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
			Disappeared: sql.NullTime{Time: secondScanTime, Valid: true},
		},
		{
			Path:        "package.json",
			FirstSeenAt: firstScanTime,
			LastSeenAt:  secondScanTime,
		},
	}, got)

	var packageScanManifestCount int
	var packageManifestUsesLatestScan bool
	if err := db.QueryRow(ctx, `
		select count(*), bool_or(sm.scan_id = $2)
		from scan_manifests sm
		join manifests m on m.id = sm.manifest_id
		where m.repository_id = (
			select repository_id
			from scans
			where id = $1
		)
		  and m.path = 'package.json'
	`, firstID, secondID).Scan(&packageScanManifestCount, &packageManifestUsesLatestScan); err != nil {
		t.Fatalf("query package scan manifest error = %v", err)
	}
	require.Equal(t, 1, packageScanManifestCount)
	require.True(t, packageManifestUsesLatestScan)
}

func TestListRepositoryManifestsReturnsLifecycleRowsForTenantRepository(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	require.NoError(t, Migrate(databaseURL))

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)
	otherTenantID := mustCreateTenantWithSlug(t, ctx, db, "other-tenant")

	firstScanTime := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	secondScanTime := time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC)

	_, err := store.CreateScan(ctx, UploadScanParams{
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
		ScannedAt:      firstScanTime,
		Manifests: []UploadManifestParams{
			{Position: 0, Type: "rust", Path: "Cargo.lock"},
			{Position: 1, Type: "npm", Path: "package-lock.json"},
		},
	})
	require.NoError(t, err)

	_, err = store.CreateScan(ctx, UploadScanParams{
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
		ScannedAt:      secondScanTime,
		Manifests: []UploadManifestParams{
			{Position: 0, Type: "npm", Path: "package-lock.json"},
		},
	})
	require.NoError(t, err)

	_, err = store.CreateScan(ctx, UploadScanParams{
		TenantID:       otherTenantID,
		RepositoryName: "Other Repo",
		URL:            "https://example.com/other.git",
		DefaultBranch:  "main",
		ArtifactKey:    "artifact-other",
		ArtifactSHA256: "sha-other",
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      "commit-other",
		SourceRef:      "refs/heads/main",
		ScannedAt:      firstScanTime,
		Manifests: []UploadManifestParams{
			{Position: 0, Type: "go", Path: "go.mod"},
		},
	})
	require.NoError(t, err)

	repositoryPage, err := ScanStore{DB: db}.ListRepositories(ctx, RepositoryListFilter{TenantID: tenantID})
	require.NoError(t, err)
	repositories := repositoryPage.Items
	require.Len(t, repositories, 1)

	items, err := ScanStore{DB: db}.ListRepositoryManifests(ctx, tenantID, repositories[0].ID)
	require.NoError(t, err)
	require.Len(t, items, 2)

	require.Equal(t, "package-lock.json", items[0].Path)
	require.True(t, items[0].IsActive)
	require.Nil(t, items[0].DisappearedAt)
	require.Equal(t, firstScanTime, items[0].FirstSeenAt.UTC())
	require.Equal(t, secondScanTime, items[0].LastSeenAt.UTC())

	require.Equal(t, "Cargo.lock", items[1].Path)
	require.False(t, items[1].IsActive)
	require.NotNil(t, items[1].DisappearedAt)
	require.Equal(t, secondScanTime, items[1].DisappearedAt.UTC())
}

func TestListRepositoriesSearchesAndPaginatesTenantRepositories(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	require.NoError(t, Migrate(databaseURL))

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)
	otherTenantID := mustCreateTenantWithSlug(t, ctx, db, "repository-list-other-tenant")

	createRepositoryScan(t, ctx, store, tenantID, "Alpha API", "https://example.com/alpha-api.git", "commit-alpha")
	createRepositoryScan(t, ctx, store, tenantID, "Beta Worker", "https://git.example.com/workers/beta.git", "commit-beta")
	createRepositoryScan(t, ctx, store, tenantID, "Gamma UI", "https://example.com/frontend/gamma.git", "commit-gamma")
	createRepositoryScan(t, ctx, store, otherTenantID, "Alpha Other", "https://example.com/alpha-other.git", "commit-other")

	page, err := ScanStore{DB: db}.ListRepositories(ctx, RepositoryListFilter{
		TenantID: tenantID,
		Page:     1,
		PageSize: 2,
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.Equal(t, 3, page.Pagination.Total)
	require.Equal(t, 2, page.Pagination.TotalPages)
	require.False(t, page.Pagination.HasPrevious)
	require.True(t, page.Pagination.HasNext)
	require.Equal(t, "Alpha API", page.Items[0].Name)
	require.Equal(t, "Beta Worker", page.Items[1].Name)

	secondPage, err := ScanStore{DB: db}.ListRepositories(ctx, RepositoryListFilter{
		TenantID: tenantID,
		Page:     2,
		PageSize: 2,
	})
	require.NoError(t, err)
	require.Len(t, secondPage.Items, 1)
	require.Equal(t, "Gamma UI", secondPage.Items[0].Name)
	require.True(t, secondPage.Pagination.HasPrevious)
	require.False(t, secondPage.Pagination.HasNext)

	searchPage, err := ScanStore{DB: db}.ListRepositories(ctx, RepositoryListFilter{
		TenantID: tenantID,
		Query:    "WORKERS",
		Page:     1,
		PageSize: 25,
	})
	require.NoError(t, err)
	require.Len(t, searchPage.Items, 1)
	require.Equal(t, "Beta Worker", searchPage.Items[0].Name)
	require.Equal(t, "WORKERS", searchPage.Filters.Query)
}

func TestListRepositoriesEscapesSearchWildcards(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	require.NoError(t, Migrate(databaseURL))

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)

	createRepositoryScan(t, ctx, store, tenantID, "Regular Repo", "https://example.com/regular.git", "commit-regular")
	createRepositoryScan(t, ctx, store, tenantID, "Literal 100% Repo", "https://example.com/literal.git", "commit-literal")

	page, err := ScanStore{DB: db}.ListRepositories(ctx, RepositoryListFilter{
		TenantID: tenantID,
		Query:    "%",
		Page:     1,
		PageSize: 25,
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "Literal 100% Repo", page.Items[0].Name)
	require.Equal(t, 1, page.Pagination.Total)
}

func TestListDependenciesAggregatesActiveManifestDependencies(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	require.NoError(t, Migrate(databaseURL))

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)
	otherTenantID := mustCreateTenantWithSlug(t, ctx, db, "dependency-other-tenant")
	hasDependencies := true

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
		Manifests: []UploadManifestParams{
			{
				Position:        0,
				Type:            "npm",
				Path:            "package-lock.json",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "react@19.1.0", Name: "react", Version: "19.1.0"},
					{Position: 1, Raw: "internal-lib ^2", Name: "internal-lib", Constraint: "^2"},
				},
			},
			{
				Position:        1,
				Type:            "npm",
				Path:            "packages/app/package-lock.json",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "react@19.1.0", Name: "react", Version: "19.1.0"},
				},
			},
			{
				Position:        2,
				Type:            "npm",
				Path:            "old/package-lock.json",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "gone@1.0.0", Name: "gone", Version: "1.0.0"},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
		RepositoryName: "Repo One",
		URL:            "https://example.com/one.git",
		DefaultBranch:  "main",
		ArtifactKey:    "artifact-2",
		ArtifactSHA256: "sha-2",
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      "commit-2",
		SourceRef:      "refs/heads/main",
		ScannedAt:      time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC),
		Manifests: []UploadManifestParams{
			{
				Position:        0,
				Type:            "npm",
				Path:            "package-lock.json",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "react@19.1.0", Name: "react", Version: "19.1.0"},
				},
			},
			{
				Position:        1,
				Type:            "npm",
				Path:            "packages/app/package-lock.json",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "react@19.1.0", Name: "react", Version: "19.1.0"},
					{Position: 1, Raw: "internal-lib ^2", Name: "internal-lib", Constraint: "^2"},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
		RepositoryName: "Repo Two",
		URL:            "https://example.com/two.git",
		DefaultBranch:  "main",
		ArtifactKey:    "artifact-3",
		ArtifactSHA256: "sha-3",
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      "commit-3",
		SourceRef:      "refs/heads/main",
		ScannedAt:      time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC),
		Manifests: []UploadManifestParams{
			{
				Position:        0,
				Type:            "go",
				Path:            "go.mod",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "react@19.1.0", Name: "react", Version: "19.1.0"},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = store.CreateScan(ctx, UploadScanParams{
		TenantID:       otherTenantID,
		RepositoryName: "Other Repo",
		URL:            "https://example.com/other.git",
		DefaultBranch:  "main",
		ArtifactKey:    "artifact-other",
		ArtifactSHA256: "sha-other",
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      "commit-other",
		SourceRef:      "refs/heads/main",
		ScannedAt:      time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC),
		Manifests: []UploadManifestParams{
			{
				Position:        0,
				Type:            "npm",
				Path:            "package-lock.json",
				HasDependencies: &hasDependencies,
				Dependencies: []UploadDependencyParams{
					{Position: 0, Raw: "zod@3.24.4", Name: "zod", Version: "3.24.4"},
				},
			},
		},
	})
	require.NoError(t, err)

	items, err := ScanStore{DB: db}.ListDependencies(ctx, tenantID)
	require.NoError(t, err)
	require.Equal(t, []DependencyListItem{
		{
			Raw:               "react@19.1.0",
			Name:              "react",
			Version:           "19.1.0",
			RepositoryCount:   2,
			ManifestFileCount: 3,
		},
		{
			Raw:               "internal-lib ^2",
			Name:              "internal-lib",
			Constraint:        "^2",
			RepositoryCount:   1,
			ManifestFileCount: 1,
		},
	}, items)
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

func createRepositoryScan(t *testing.T, ctx context.Context, store Store, tenantID string, name string, url string, commitSHA string) {
	t.Helper()

	_, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:       tenantID,
		RepositoryName: name,
		URL:            url,
		DefaultBranch:  "main",
		ArtifactKey:    "artifacts/" + commitSHA + ".json",
		ArtifactSHA256: "sha-" + commitSHA,
		SchemaVersion:  "v1alpha1",
		RootPath:       ".",
		CommitSHA:      commitSHA,
		SourceRef:      "refs/heads/main",
		ScannedAt:      time.Date(2026, 5, 6, 8, 0, 0, 0, time.UTC),
		Labels:         map[string]string{},
		Annotation:     "",
		Manifests: []UploadManifestParams{
			{Position: 0, Type: "npm", Path: "package-lock.json"},
		},
	})
	require.NoError(t, err)
}

func mustCreateTenant(t *testing.T, ctx context.Context, db *pgxpool.Pool) string {
	t.Helper()

	return mustCreateTenantWithSlug(t, ctx, db, "scan-detail-tests")
}
