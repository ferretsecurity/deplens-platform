package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	DB *pgxpool.Pool
}

func (s Store) FindToken(ctx context.Context, tokenHash string) (string, []string, error) {
	var record TokenRecord
	err := s.DB.QueryRow(ctx, `
		select tenant_id, scopes
		from api_tokens
		where token_hash = $1
	`, tokenHash).Scan(&record.TenantID, &record.Scopes)
	if err != nil {
		return "", nil, err
	}
	return record.TenantID, record.Scopes, nil
}

func (s Store) FindLocalIdentityByEmail(ctx context.Context, email string) (string, string, error) {
	var userID string
	var passwordHash string
	err := s.DB.QueryRow(ctx, `
		select u.id, ai.password_hash
		from auth_identities ai
		join auth_providers ap on ap.id = ai.provider_id
		join users u on u.id = ai.user_id
		where ap.slug = 'local' and ai.email = $1
		limit 1
	`, email).Scan(&userID, &passwordHash)
	if err != nil {
		return "", "", err
	}
	return userID, passwordHash, nil
}

func (s Store) ListMemberships(ctx context.Context, userID string) ([]MembershipRecord, error) {
	rows, err := s.DB.Query(ctx, `
		select tm.tenant_id, t.slug, tm.role
		from tenant_memberships tm
		join tenants t on t.id = tm.tenant_id
		where tm.user_id = $1
		order by t.slug asc
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]MembershipRecord, 0)
	for rows.Next() {
		var item MembershipRecord
		if err := rows.Scan(&item.TenantID, &item.TenantSlug, &item.Role); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Store) CreateScan(ctx context.Context, params UploadScanParams) (string, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var projectID string
	err = tx.QueryRow(ctx, `
		insert into projects (tenant_id, slug, name)
		values ($1, $2, $3)
		on conflict (tenant_id, slug) do update set name = excluded.name
		returning id
	`, params.TenantID, params.ProjectSlug, params.ProjectName).Scan(&projectID)
	if err != nil {
		return "", err
	}

	var repositoryID string
	err = tx.QueryRow(ctx, `
		insert into repositories (tenant_id, project_id, slug, name, url, default_branch)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (tenant_id, project_id, slug)
		do update set name = excluded.name, url = excluded.url, default_branch = excluded.default_branch
		returning id
	`, params.TenantID, projectID, params.RepositorySlug, params.RepositoryName, params.URL, params.DefaultBranch).Scan(&repositoryID)
	if err != nil {
		return "", err
	}

	labelsJSON, err := json.Marshal(params.Labels)
	if err != nil {
		return "", err
	}

	var scanID string
	err = tx.QueryRow(ctx, `
		insert into scans (
			tenant_id, project_id, repository_id, artifact_key, artifact_sha256, schema_version,
			root_path, commit_sha, source_ref, scanned_at, manifest_count,
			manifests_with_dependencies_count, manifests_without_dependencies_count,
			manifests_unknown_count, dependency_count, labels, annotation
		)
		values (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16::jsonb, $17
		)
		returning id
	`, params.TenantID, projectID, repositoryID, params.ArtifactKey, params.ArtifactSHA256, params.SchemaVersion, params.RootPath, params.CommitSHA, params.SourceRef, params.ScannedAt, params.ManifestCount, params.WithDependencies, params.WithoutDependencies, params.UnknownDependencies, params.DependencyCount, string(labelsJSON), params.Annotation).Scan(&scanID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return scanID, nil
}
