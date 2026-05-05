package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ScanListItem struct {
	ID              string            `json:"id"`
	ProjectSlug     string            `json:"project_slug"`
	RepositorySlug  string            `json:"repository_slug"`
	CommitSHA       string            `json:"commit_sha"`
	ScannedAt       time.Time         `json:"scanned_at"`
	ManifestCount   int               `json:"manifest_count"`
	DependencyCount int               `json:"dependency_count"`
	Labels          map[string]string `json:"labels"`
	Annotation      string            `json:"annotation"`
}

type ProjectListItem struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type RepositoryListItem struct {
	ProjectSlug   string `json:"project_slug"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}

type ScanFilter struct {
	TenantID       string
	RepositorySlug string
	From           time.Time
	To             time.Time
}

type ScanStore struct {
	DB *pgxpool.Pool
}

func (s ScanStore) ListProjects(ctx context.Context, tenantID string) ([]ProjectListItem, error) {
	rows, err := s.DB.Query(ctx, `
		select slug, name
		from projects
		where tenant_id = $1
		order by slug asc
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ProjectListItem, 0)
	for rows.Next() {
		var item ProjectListItem
		if err := rows.Scan(&item.Slug, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s ScanStore) ListRepositories(ctx context.Context, tenantID string) ([]RepositoryListItem, error) {
	rows, err := s.DB.Query(ctx, `
		select p.slug, r.slug, r.name, r.url, r.default_branch
		from repositories r
		join projects p on p.id = r.project_id
		where r.tenant_id = $1
		order by p.slug asc, r.slug asc
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RepositoryListItem, 0)
	for rows.Next() {
		var item RepositoryListItem
		if err := rows.Scan(&item.ProjectSlug, &item.Slug, &item.Name, &item.URL, &item.DefaultBranch); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s ScanStore) ListScans(ctx context.Context, filter ScanFilter) ([]ScanListItem, error) {
	rows, err := s.DB.Query(ctx, `
		select s.id, p.slug, r.slug, s.commit_sha, s.scanned_at, s.manifest_count, s.dependency_count, s.labels, s.annotation
		from scans s
		join projects p on p.id = s.project_id
		join repositories r on r.id = s.repository_id
		where s.tenant_id = $1 and r.slug = $2 and s.scanned_at between $3 and $4
		order by s.scanned_at desc
	`, filter.TenantID, filter.RepositorySlug, filter.From, filter.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ScanListItem, 0)
	for rows.Next() {
		item, err := scanListItemFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s ScanStore) GetScan(ctx context.Context, tenantID string, scanID string) (ScanListItem, error) {
	var item ScanListItem
	var labelsJSON []byte
	err := s.DB.QueryRow(ctx, `
		select s.id, p.slug, r.slug, s.commit_sha, s.scanned_at, s.manifest_count, s.dependency_count, s.labels, s.annotation
		from scans s
		join projects p on p.id = s.project_id
		join repositories r on r.id = s.repository_id
		where s.tenant_id = $1 and s.id = $2
	`, tenantID, scanID).Scan(&item.ID, &item.ProjectSlug, &item.RepositorySlug, &item.CommitSHA, &item.ScannedAt, &item.ManifestCount, &item.DependencyCount, &labelsJSON, &item.Annotation)
	if err != nil {
		return ScanListItem{}, err
	}
	if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
		return ScanListItem{}, err
	}
	return item, nil
}

func (s ScanStore) UpdateScanMetadata(ctx context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error {
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(ctx, `
		update scans
		set labels = $3::jsonb, annotation = $4
		where tenant_id = $1 and id = $2
	`, tenantID, scanID, string(labelsJSON), annotation)
	return err
}

type scanRows interface {
	Scan(dest ...any) error
}

func scanListItemFromRows(row scanRows) (ScanListItem, error) {
	var item ScanListItem
	var labelsJSON []byte
	if err := row.Scan(&item.ID, &item.ProjectSlug, &item.RepositorySlug, &item.CommitSHA, &item.ScannedAt, &item.ManifestCount, &item.DependencyCount, &labelsJSON, &item.Annotation); err != nil {
		return ScanListItem{}, err
	}
	if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
		return ScanListItem{}, err
	}
	return item, nil
}
