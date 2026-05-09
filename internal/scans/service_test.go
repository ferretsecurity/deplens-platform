package scans

import (
	"context"
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

func TestServiceUploadConvertsManifestsAndDependencies(t *testing.T) {
	ctx := context.Background()
	hasDependencies := true
	input := UploadRequest{
		SchemaVersion: "v1alpha1",
		Repository:    RepositoryInput{Name: "Repo", URL: "https://example.com/repo.git", DefaultBranch: "main"},
		Source:        SourceInput{CommitSHA: "abc123", Ref: "refs/heads/main", ScannedAt: "2026-05-06T08:00:00Z"},
		Snapshot: SnapshotInput{
			Root: ".",
			Manifests: []ManifestInput{
				{
					Type:            "npm-package-lock",
					Path:            "package-lock.json",
					HasDependencies: &hasDependencies,
					Warnings:        []string{"manifest warning"},
					Dependencies: []DependencyInput{
						{
							Raw:    "react@19.1.0",
							Name:   "react",
							Source: "npm",
							Extras: map[string]string{"scope": "ui"},
						},
					},
				},
			},
		},
	}

	repo := &captureRepository{}
	svc := Service{
		Blob:  noopBlobStore{},
		Store: repo,
	}

	_, err := svc.Upload(ctx, "tenant-123", input)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if len(repo.params.Manifests) != 1 {
		t.Fatalf("manifest count = %d, want 1", len(repo.params.Manifests))
	}
	manifest := repo.params.Manifests[0]
	if manifest.Position != 0 {
		t.Fatalf("manifest position = %d, want 0", manifest.Position)
	}
	if manifest.Type != "npm-package-lock" || manifest.Path != "package-lock.json" {
		t.Fatalf("manifest = %+v, want type/path preserved", manifest)
	}
	if manifest.HasDependencies == nil || !*manifest.HasDependencies {
		t.Fatalf("manifest HasDependencies = %v, want true", manifest.HasDependencies)
	}
	if len(manifest.Warnings) != 1 || manifest.Warnings[0] != "manifest warning" {
		t.Fatalf("manifest warnings = %#v, want preserved warnings", manifest.Warnings)
	}
	if len(manifest.Dependencies) != 1 {
		t.Fatalf("dependency count = %d, want 1", len(manifest.Dependencies))
	}

	dependency := manifest.Dependencies[0]
	if dependency.Position != 0 {
		t.Fatalf("dependency position = %d, want 0", dependency.Position)
	}
	if dependency.Raw != "react@19.1.0" || dependency.Name != "react" || dependency.Source != "npm" {
		t.Fatalf("dependency = %+v, want fields preserved", dependency)
	}
	if dependency.Extras["scope"] != "ui" {
		t.Fatalf("dependency extras = %#v, want scope preserved", dependency.Extras)
	}
	if repo.params.RepositoryName != "Repo" {
		t.Fatalf("repository name = %q, want Repo", repo.params.RepositoryName)
	}
}

type captureRepository struct {
	params store.UploadScanParams
}

func (r *captureRepository) CreateScan(_ context.Context, params store.UploadScanParams) (string, error) {
	r.params = params
	return "scan-123", nil
}

type noopBlobStore struct{}

func (noopBlobStore) Put(_ context.Context, payload []byte) (string, string, error) {
	return "artifact-key", "artifact-sha", nil
}

var _ blob.Store = noopBlobStore{}
var _ Repository = (*captureRepository)(nil)
