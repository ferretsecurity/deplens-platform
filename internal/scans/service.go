package scans

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type Repository interface {
	CreateScan(ctx context.Context, params store.UploadScanParams) (string, error)
}

type Service struct {
	Blob  blob.Store
	Store Repository
}

func (s Service) Upload(ctx context.Context, tenantID string, input UploadRequest) (string, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	artifactKey, artifactSHA, err := s.Blob.Put(ctx, payload)
	if err != nil {
		return "", err
	}

	summary := BuildSummary(input)
	scannedAt, err := time.Parse(time.RFC3339, input.Source.ScannedAt)
	if err != nil {
		return "", err
	}

	manifests := make([]store.UploadManifestParams, 0, len(input.Snapshot.Manifests))
	for manifestIdx, manifest := range input.Snapshot.Manifests {
		dependencies := make([]store.UploadDependencyParams, 0, len(manifest.Dependencies))
		for dependencyIdx, dependency := range manifest.Dependencies {
			dependencies = append(dependencies, store.UploadDependencyParams{
				Position:   dependencyIdx,
				Raw:        dependency.Raw,
				Name:       dependency.Name,
				Version:    dependency.Version,
				Constraint: dependency.Constraint,
				Section:    dependency.Section,
				Source:     dependency.Source,
				Extras:     dependency.Extras,
			})
		}
		manifests = append(manifests, store.UploadManifestParams{
			Position:        manifestIdx,
			Type:            manifest.Type,
			Path:            manifest.Path,
			HasDependencies: manifest.HasDependencies,
			Warnings:        manifest.Warnings,
			Dependencies:    dependencies,
		})
	}

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
}
