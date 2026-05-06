package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
		join scan_manifests m on m.id = d.manifest_id
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

func mustCreateScanWithDetails(t *testing.T, ctx context.Context, store Store, tenantID string) string {
	t.Helper()

	hasDependencies := true
	scanID, err := store.CreateScan(ctx, UploadScanParams{
		TenantID:            tenantID,
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
