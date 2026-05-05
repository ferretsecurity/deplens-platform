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

	return s.Store.CreateScan(ctx, store.UploadScanParams{
		TenantID:            tenantID,
		ProjectSlug:         input.Project.Slug,
		ProjectName:         input.Project.Name,
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
	})
}
