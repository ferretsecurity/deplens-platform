package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const DefaultPackagePageSize = 25

type PackageListFilter struct {
	TenantID string
	Query    string
	Version  string
	Page     int
	PageSize int
}

type PackageUsageItem struct {
	Source         string  `json:"source"`
	Name           string  `json:"name"`
	OriginKind     string  `json:"origin_kind"`
	Destination    string  `json:"destination"`
	Version        string  `json:"version"`
	Constraint     string  `json:"constraint"`
	Ecosystem      string  `json:"ecosystem"`
	RepositoryID   string  `json:"repository_id"`
	RepositoryName string  `json:"repository_name"`
	ManifestPath   *string `json:"manifest_path"`
}

type PackageListPage struct {
	Items      []PackageUsageItem `json:"items"`
	Pagination PaginationMetadata `json:"pagination"`
	Filters    PackageFilters     `json:"filters"`
}

type PackageFilters struct {
	Query   string `json:"q"`
	Version string `json:"version"`
}

func (s ScanStore) ListPackages(ctx context.Context, filter PackageListFilter) (PackageListPage, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Version = strings.TrimSpace(filter.Version)
	if err := ValidatePackageVersion(filter.Version); err != nil {
		return PackageListPage{}, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = DefaultPackagePageSize
	}
	if filter.PageSize > 100 || int64(filter.Page-1) > math.MaxInt64/int64(filter.PageSize) {
		return PackageListPage{}, fmt.Errorf("package pagination is out of range")
	}
	offset := int64(filter.Page-1) * int64(filter.PageSize)

	var itemsJSON []byte
	var total int
	items := make([]PackageUsageItem, 0)
	if filter.Version != "" {
		rows, err := s.DB.Query(ctx, packageCandidatesSQL+` select * from filtered `+packageOrderSQL,
			filter.TenantID, filter.Query, "%"+escapeLikePattern(filter.Query)+"%")
		if err != nil {
			return PackageListPage{}, err
		}
		defer rows.Close()
		for rows.Next() {
			var item PackageUsageItem
			if err := rows.Scan(&item.Source, &item.Name, &item.Version, &item.Constraint, &item.Ecosystem,
				&item.RepositoryID, &item.RepositoryName, &item.ManifestPath, &item.OriginKind, &item.Destination); err != nil {
				return PackageListPage{}, err
			}
			if !packageMatchesVersion(item, filter.Version) {
				continue
			}
			if int64(total) >= offset && len(items) < filter.PageSize {
				items = append(items, item)
			}
			total++
		}
		if err := rows.Err(); err != nil {
			return PackageListPage{}, err
		}
	} else {
		err := s.DB.QueryRow(ctx, packageCandidatesSQL+`, page as (
			select * from filtered `+packageOrderSQL+` limit $4 offset $5
		)
		select (select coalesce(jsonb_agg(p), '[]'::jsonb) from page p), (select count(*) from filtered)`,
			filter.TenantID, filter.Query, "%"+escapeLikePattern(filter.Query)+"%", filter.PageSize, offset).Scan(&itemsJSON, &total)
		if err != nil {
			return PackageListPage{}, err
		}
		if err := json.Unmarshal(itemsJSON, &items); err != nil {
			return PackageListPage{}, err
		}
	}

	totalPages := total / filter.PageSize
	if total%filter.PageSize != 0 {
		totalPages++
	}
	return PackageListPage{
		Items: items,
		Pagination: PaginationMetadata{
			Page: filter.Page, PageSize: filter.PageSize, Total: total, TotalPages: totalPages,
			HasPrevious: filter.Page > 1, HasNext: filter.Page < totalPages,
		},
		Filters: PackageFilters{Query: filter.Query, Version: filter.Version},
	}, nil
}

const packageCandidatesSQL = `
		with usages as (
			select 'cli'::text as source, d.name, d.version, d."constraint",
				d.package_type as ecosystem,
				r.id as repository_id, nullif(m.path, '') as manifest_path,
				d.source as origin_kind,
				case when d.source in ('url', 'git') then coalesce(btrim(d.extras->>'source_url'), '') else '' end as destination
			from manifest_dependencies d
			join scan_manifests sm on sm.id = d.scan_manifest_id
			join manifests m on m.id = sm.manifest_id
			join repositories r on r.id = m.repository_id
			where r.tenant_id = $1 and m.disappeared_at is null
				and d.source not in ('path', 'workspace')
				and (btrim(d.name) <> '' or (d.source in ('url', 'git') and btrim(coalesce(d.extras->>'source_url', '')) <> ''))
			union all
			select 'socket',
				case when coalesce(a->>'namespace', '') <> '' then (a->>'namespace') || '/' || (a->>'name') else a->>'name' end,
				coalesce(a->>'version', ''), '', coalesce(a->>'type', ''),
				coalesce(sr.cli_repository_id, sr.id), nullif(ref->>'file', ''), '' as origin_kind, '' as destination
			from socket_repositories sr join socket_connections c on c.id = sr.connection_id
			cross join lateral jsonb_array_elements(nullif(sr.snapshot->'artifacts', 'null'::jsonb)) a
			left join lateral jsonb_array_elements(nullif(a->'manifestFiles', 'null'::jsonb)) ref on true
			where c.tenant_id = $1 and sr.available and btrim(coalesce(a->>'name', '')) <> ''
			union all
			select 'snyk', a->>'name', coalesce(a->>'version', ''), '', coalesce(a->>'type', ''),
				coalesce(sr.cli_repository_id, linked.cli_repository_id, sr.socket_repository_id, sr.id), nullif(ref->>'file', ''), '' as origin_kind, '' as destination
			from snyk_repositories sr join snyk_connections c on c.id = sr.connection_id
			left join socket_repositories linked on linked.id = sr.socket_repository_id
				and exists (select 1 from socket_connections lc where lc.id = linked.connection_id and lc.tenant_id = $1)
			cross join lateral jsonb_array_elements(nullif(sr.snapshot->'artifacts', 'null'::jsonb)) a
			left join lateral jsonb_array_elements(nullif(a->'manifestFiles', 'null'::jsonb)) ref on true
			where c.tenant_id = $1 and sr.available and btrim(coalesce(a->>'name', '')) <> ''
		), filtered as (
			select distinct u.source, u.name, u.version, u."constraint", u.ecosystem,
				r.id as repository_id, r.name as repository_name, u.manifest_path, u.origin_kind, u.destination
			from usages u join inventory_repositories r on r.id = u.repository_id and r.tenant_id = $1
			where $2 = '' or u.name ilike $3 escape '\' or u.destination ilike $3 escape '\'
		)`

const packageOrderSQL = `order by lower(name), name, version, "constraint", ecosystem, source, repository_id, repository_name, manifest_path nulls first, origin_kind, destination`
