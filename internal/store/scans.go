package store

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const DefaultRepositoryPageSize = 25

type ScanListItem struct {
	ID              string            `json:"id"`
	RepositoryID    string            `json:"repository_id"`
	CommitSHA       string            `json:"commit_sha"`
	ScannedAt       time.Time         `json:"scanned_at"`
	ManifestCount   int               `json:"manifest_count"`
	DependencyCount int               `json:"dependency_count"`
	Labels          map[string]string `json:"labels"`
	Annotation      string            `json:"annotation"`
}

type RepositoryListItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}

type RepositoryListFilter struct {
	TenantID string
	Query    string
	Page     int
	PageSize int
}

type RepositoryListPage struct {
	Items      []RepositoryListItem `json:"items"`
	Pagination PaginationMetadata   `json:"pagination"`
	Filters    RepositoryFilters    `json:"filters"`
}

type PaginationMetadata struct {
	Page        int  `json:"page"`
	PageSize    int  `json:"page_size"`
	Total       int  `json:"total"`
	TotalPages  int  `json:"total_pages"`
	HasPrevious bool `json:"has_previous"`
	HasNext     bool `json:"has_next"`
}

type RepositoryFilters struct {
	Query string `json:"q"`
}

type RepositoryManifestItem struct {
	ID            string            `json:"id"`
	Path          string            `json:"path"`
	FirstSeenAt   time.Time         `json:"first_seen_at"`
	LastSeenAt    time.Time         `json:"last_seen_at"`
	DisappearedAt *time.Time        `json:"disappeared_at"`
	IsActive      bool              `json:"is_active"`
	Labels        map[string]string `json:"labels"`
}

type ScanFilter struct {
	TenantID     string
	RepositoryID string
	From         time.Time
	To           time.Time
}

type ScanStore struct {
	DB *pgxpool.Pool
}

func (s ScanStore) ListRepositories(ctx context.Context, filter RepositoryListFilter) (RepositoryListPage, error) {
	filter = normalizeRepositoryListFilter(filter)
	offset := (filter.Page - 1) * filter.PageSize
	pattern := "%" + escapeLikePattern(filter.Query) + "%"

	rows, err := s.DB.Query(ctx, `
		with filtered as (
			select r.id, r.name, r.url, r.default_branch
			from repositories r
			where r.tenant_id = $1
			  and (
				$2 = ''
				or r.name ilike $3 escape '\'
				or r.url ilike $3 escape '\'
			  )
		),
		counted as (
			select count(*) as total
			from filtered
		)
		select f.id, f.name, f.url, f.default_branch, c.total
		from filtered f
		cross join counted c
		order by lower(f.name) asc, f.id asc
		limit $4 offset $5
	`, filter.TenantID, filter.Query, pattern, filter.PageSize, offset)
	if err != nil {
		return RepositoryListPage{}, err
	}
	defer rows.Close()

	items := make([]RepositoryListItem, 0)
	total := 0
	for rows.Next() {
		var item RepositoryListItem
		if err := rows.Scan(&item.ID, &item.Name, &item.URL, &item.DefaultBranch, &total); err != nil {
			return RepositoryListPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return RepositoryListPage{}, err
	}
	if len(items) == 0 {
		if err := s.DB.QueryRow(ctx, `
			select count(*)
			from repositories r
			where r.tenant_id = $1
			  and (
				$2 = ''
				or r.name ilike $3 escape '\'
				or r.url ilike $3 escape '\'
			  )
		`, filter.TenantID, filter.Query, pattern).Scan(&total); err != nil {
			return RepositoryListPage{}, err
		}
	}

	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(filter.PageSize)))
	}

	return RepositoryListPage{
		Items: items,
		Pagination: PaginationMetadata{
			Page:        filter.Page,
			PageSize:    filter.PageSize,
			Total:       total,
			TotalPages:  totalPages,
			HasPrevious: filter.Page > 1,
			HasNext:     totalPages > filter.Page,
		},
		Filters: RepositoryFilters{
			Query: filter.Query,
		},
	}, nil
}

func normalizeRepositoryListFilter(filter RepositoryListFilter) RepositoryListFilter {
	filter.Query = strings.TrimSpace(filter.Query)
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = DefaultRepositoryPageSize
	}
	return filter
}

func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func (s ScanStore) ListRepositoryManifests(ctx context.Context, tenantID string, repositoryID string) ([]RepositoryManifestItem, error) {
	rows, err := s.DB.Query(ctx, `
		select m.id, m.path, m.first_seen_at, m.last_seen_at, m.disappeared_at, m.disappeared_at is null, m.labels
		from manifests m
		join repositories r on r.id = m.repository_id
		where r.tenant_id = $1 and m.repository_id = $2
		order by (m.disappeared_at is null) desc, m.path asc
	`, tenantID, repositoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RepositoryManifestItem, 0)
	for rows.Next() {
		var item RepositoryManifestItem
		var labelsJSON []byte
		if err := rows.Scan(&item.ID, &item.Path, &item.FirstSeenAt, &item.LastSeenAt, &item.DisappearedAt, &item.IsActive, &labelsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s ScanStore) ListDependencies(ctx context.Context, tenantID string) ([]DependencyListItem, error) {
	rows, err := s.DB.Query(ctx, `
		with active_dependencies as (
			select
				d.id,
				d.raw,
				d.name,
				d.version,
				d."constraint",
				m.id as manifest_id,
				r.id as repository_id,
				(d.name <> '' and d.version <> '') as is_grouped
			from manifest_dependencies d
			join scan_manifests sm on sm.id = d.scan_manifest_id
			join manifests m on m.id = sm.manifest_id
			join repositories r on r.id = m.repository_id
			where r.tenant_id = $1
			  and m.disappeared_at is null
		)
		select
			min(raw) as raw,
			case when is_grouped then max(name) else min(name) end as name,
			case when is_grouped then max(version) else min(version) end as version,
			min("constraint") as "constraint",
			count(distinct repository_id) as repository_count,
			count(distinct manifest_id) as manifest_file_count
		from active_dependencies
		group by
			is_grouped,
			case when is_grouped then name else id::text end,
			case when is_grouped then version else id::text end
		order by
			is_grouped desc,
			count(distinct repository_id) desc,
			count(distinct manifest_id) desc,
			case
				when is_grouped then max(name) || '@' || max(version)
				when min(name) <> '' and min("constraint") <> '' then min(name) || ' ' || min("constraint")
				when min(raw) <> '' then min(raw)
				else 'Unknown dependency'
			end asc
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DependencyListItem, 0)
	for rows.Next() {
		var item DependencyListItem
		if err := rows.Scan(&item.Raw, &item.Name, &item.Version, &item.Constraint, &item.RepositoryCount, &item.ManifestFileCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s ScanStore) ListScans(ctx context.Context, filter ScanFilter) ([]ScanListItem, error) {
	rows, err := s.DB.Query(ctx, `
		select s.id, s.repository_id, s.commit_sha, s.scanned_at, s.manifest_count, s.dependency_count, s.labels, s.annotation
		from scans s
		where s.tenant_id = $1 and s.repository_id = $2 and s.scanned_at between $3 and $4
		order by s.scanned_at desc
	`, filter.TenantID, filter.RepositoryID, filter.From, filter.To)
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
		select s.id, s.repository_id, s.commit_sha, s.scanned_at, s.manifest_count, s.dependency_count, s.labels, s.annotation
		from scans s
		where s.tenant_id = $1 and s.id = $2
	`, tenantID, scanID).Scan(&item.ID, &item.RepositoryID, &item.CommitSHA, &item.ScannedAt, &item.ManifestCount, &item.DependencyCount, &labelsJSON, &item.Annotation)
	if err != nil {
		return ScanListItem{}, err
	}
	if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
		return ScanListItem{}, err
	}
	return item, nil
}

func (s ScanStore) ListScanManifests(ctx context.Context, tenantID string, scanID string) ([]ScanManifestItem, error) {
	rows, err := s.DB.Query(ctx, `
		select sm.id, sm.type, m.path, sm.has_dependencies, sm.warnings
		from scan_manifests sm
		join scans s on s.id = sm.scan_id
		join manifests m on m.id = sm.manifest_id
		where s.tenant_id = $1 and s.id = $2
		order by sm.position asc
	`, tenantID, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ScanManifestItem, 0)
	manifestIDs := make([]string, 0)
	manifestIndexByID := make(map[string]int)
	for rows.Next() {
		var item ScanManifestItem
		var warningsJSON []byte
		if err := rows.Scan(&item.ID, &item.Type, &item.Path, &item.HasDependencies, &warningsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(warningsJSON, &item.Warnings); err != nil {
			return nil, err
		}

		manifestIndexByID[item.ID] = len(items)
		manifestIDs = append(manifestIDs, item.ID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}

	depRows, err := s.DB.Query(ctx, `
		select d.id, d.scan_manifest_id, d.raw, d.name, d.version, d."constraint", d.section, d.source, d.extras
		from manifest_dependencies d
		where d.scan_manifest_id = any($1::uuid[])
		order by d.scan_manifest_id asc, d.position asc
	`, manifestIDs)
	if err != nil {
		return nil, err
	}
	defer depRows.Close()

	for depRows.Next() {
		var manifestID string
		var dependency ManifestDependencyItem
		var extrasJSON []byte
		if err := depRows.Scan(&dependency.ID, &manifestID, &dependency.Raw, &dependency.Name, &dependency.Version, &dependency.Constraint, &dependency.Section, &dependency.Source, &extrasJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(extrasJSON, &dependency.Extras); err != nil {
			return nil, err
		}

		itemIndex, ok := manifestIndexByID[manifestID]
		if !ok {
			continue
		}
		items[itemIndex].Dependencies = append(items[itemIndex].Dependencies, dependency)
	}
	return items, depRows.Err()
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
	if err := row.Scan(&item.ID, &item.RepositoryID, &item.CommitSHA, &item.ScannedAt, &item.ManifestCount, &item.DependencyCount, &labelsJSON, &item.Annotation); err != nil {
		return ScanListItem{}, err
	}
	if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
		return ScanListItem{}, err
	}
	return item, nil
}
