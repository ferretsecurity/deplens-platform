package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
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

func (s Store) FindLocalIdentityByEmail(ctx context.Context, email string) (string, string, string, error) {
	var userID string
	var displayName string
	var passwordHash string
	err := s.DB.QueryRow(ctx, `
		select u.id, u.display_name, ai.password_hash
		from auth_identities ai
		join auth_providers ap on ap.id = ai.provider_id
		join users u on u.id = ai.user_id
		where ap.slug = 'local' and ai.email = $1
		limit 1
	`, email).Scan(&userID, &displayName, &passwordHash)
	if err != nil {
		return "", "", "", err
	}
	return userID, displayName, passwordHash, nil
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

	var repositoryID string
	err = tx.QueryRow(ctx, `
		insert into repositories (tenant_id, name, url, default_branch)
		values ($1, $2, $3, $4)
		on conflict (tenant_id, name)
		do update set url = excluded.url, default_branch = excluded.default_branch
		returning id
	`, params.TenantID, params.RepositoryName, params.URL, params.DefaultBranch).Scan(&repositoryID)
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
			tenant_id, repository_id, artifact_key, artifact_sha256, schema_version,
			root_path, commit_sha, source_ref, scanned_at, manifest_count,
			manifests_with_dependencies_count, manifests_without_dependencies_count,
			manifests_unknown_count, dependency_count, labels, annotation
		)
		values (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15::jsonb, $16
		)
		returning id
	`, params.TenantID, repositoryID, params.ArtifactKey, params.ArtifactSHA256, params.SchemaVersion, params.RootPath, params.CommitSHA, params.SourceRef, params.ScannedAt, params.ManifestCount, params.WithDependencies, params.WithoutDependencies, params.UnknownDependencies, params.DependencyCount, string(labelsJSON), params.Annotation).Scan(&scanID)
	if err != nil {
		return "", err
	}

	presentPaths := make([]string, 0, len(params.Manifests))
	for _, manifest := range params.Manifests {
		presentPaths = append(presentPaths, manifest.Path)
	}

	if err := insertScanManifests(ctx, tx, scanID, repositoryID, params.ScannedAt, params.Manifests); err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, `
		update manifests
		set is_active = false
		where repository_id = $1
		  and path <> all($2::text[])
	`, repositoryID, presentPaths)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return scanID, nil
}

func insertScanManifests(ctx context.Context, tx pgx.Tx, scanID string, repositoryID string, scannedAt time.Time, manifests []UploadManifestParams) error {
	for _, manifest := range manifests {
		warningsJSON, err := json.Marshal(manifest.Warnings)
		if err != nil {
			return err
		}

		var manifestID string
		err = tx.QueryRow(ctx, `
			insert into manifests (repository_id, path, first_seen_at, last_seen_at, is_active)
			values ($1, $2, $3, $3, true)
			on conflict (repository_id, path)
			do update set last_seen_at = excluded.last_seen_at, is_active = true
			returning id
		`, repositoryID, manifest.Path, scannedAt).Scan(&manifestID)
		if err != nil {
			return err
		}

		var scanManifestID string
		err = tx.QueryRow(ctx, `
			insert into scan_manifests (scan_id, manifest_id, position, type, has_dependencies, warnings)
			values ($1, $2, $3, $4, $5, $6::jsonb)
			returning id
		`, scanID, manifestID, manifest.Position, manifest.Type, manifest.HasDependencies, string(warningsJSON)).Scan(&scanManifestID)
		if err != nil {
			return err
		}

		if err := insertManifestDependencies(ctx, tx, scanManifestID, manifest.Dependencies); err != nil {
			return err
		}
	}

	return nil
}

func insertManifestDependencies(ctx context.Context, tx pgx.Tx, scanManifestID string, dependencies []UploadDependencyParams) error {
	for _, dependency := range dependencies {
		extrasJSON, err := json.Marshal(dependency.Extras)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			insert into manifest_dependencies (
				scan_manifest_id, position, raw, name, version, "constraint", section, source, extras
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		`, scanManifestID, dependency.Position, dependency.Raw, dependency.Name, dependency.Version, dependency.Constraint, dependency.Section, dependency.Source, string(extrasJSON))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s Store) CreateAPIToken(ctx context.Context, tenantID string, label string, scopes []string) (string, APITokenMetadata, error) {
	token, err := randomToken()
	if err != nil {
		return "", APITokenMetadata{}, err
	}

	var item APITokenMetadata
	err = s.DB.QueryRow(ctx, `
		insert into api_tokens (tenant_id, label, token_hash, scopes)
		values ($1, $2, $3, $4)
		returning id, label, scopes, created_at
	`, tenantID, label, hashToken(token), scopes).Scan(&item.ID, &item.Label, &item.Scopes, &item.CreatedAt)
	if err != nil {
		return "", APITokenMetadata{}, err
	}

	return token, item, nil
}

func (s Store) ListAPITokens(ctx context.Context, tenantID string) ([]APITokenMetadata, error) {
	rows, err := s.DB.Query(ctx, `
		select id, label, scopes, created_at
		from api_tokens
		where tenant_id = $1
		order by created_at desc, label asc
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]APITokenMetadata, 0)
	for rows.Next() {
		var item APITokenMetadata
		if err := rows.Scan(&item.ID, &item.Label, &item.Scopes, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Store) UpdateAPIToken(ctx context.Context, tenantID string, tokenID string, label string, scopes []string) (APITokenMetadata, error) {
	var item APITokenMetadata
	err := s.DB.QueryRow(ctx, `
		update api_tokens
		set label = $3, scopes = $4
		where tenant_id = $1 and id = $2
		returning id, label, scopes, created_at
	`, tenantID, tokenID, label, scopes).Scan(&item.ID, &item.Label, &item.Scopes, &item.CreatedAt)
	return item, err
}

func (s Store) DeleteAPIToken(ctx context.Context, tenantID string, tokenID string) error {
	tag, err := s.DB.Exec(ctx, `
		delete from api_tokens
		where tenant_id = $1 and id = $2
	`, tenantID, tokenID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "dpt_" + base64.RawURLEncoding.EncodeToString(buf), nil
}
