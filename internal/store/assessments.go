package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/ferretsecurity/deplens-platform/internal/assessment"
)

func (s ScanStore) assessPackages(ctx context.Context, filter PackageListFilter) (PackageListPage, error) {
	if filter.Version != "" {
		return PackageListPage{}, fmt.Errorf("use either version or affected_range")
	}
	query, err := assessment.NewQuery(filter.Ecosystem, filter.Query, filter.AffectedRange)
	if err != nil {
		return PackageListPage{}, err
	}
	rows, err := s.DB.Query(ctx, `
 select r.id, r.name, sm.scan_id, m.path, sm.form, sm.extraction, sm.lockfiles,
 coalesce((select jsonb_agg(jsonb_build_object('name',d.name,'ecosystem',d.package_type,
 'version',d.version,'constraint',d."constraint",'origin',d.source))
 from manifest_dependencies d where d.scan_manifest_id=sm.id and
 (case when $2='pypi' then regexp_replace(lower(btrim(d.name)),'[-_.]+','-','g')
 when $2 in ('npm','nuget','composer') then lower(btrim(d.name)) else btrim(d.name) end)=$3), '[]'::jsonb)
 from scan_manifests sm join manifests m on m.id=sm.manifest_id
 join repositories r on r.id=m.repository_id
 where r.tenant_id=$1 and m.disappeared_at is null
 order by r.id, sm.scan_id, m.path`, filter.TenantID, query.Ecosystem, query.Name)
	if err != nil {
		return PackageListPage{}, err
	}
	type scan struct {
		id, name string
		sources  []assessment.Source
	}
	scans := map[string]*scan{}
	for rows.Next() {
		var repositoryID, repositoryName, scanID string
		var source assessment.Source
		var lockfiles, dependencies []byte
		if err := rows.Scan(&repositoryID, &repositoryName, &scanID, &source.Path, &source.Form, &source.Extraction, &lockfiles, &dependencies); err != nil {
			rows.Close()
			return PackageListPage{}, err
		}
		if err := json.Unmarshal(lockfiles, &source.Lockfiles); err != nil {
			rows.Close()
			return PackageListPage{}, err
		}
		if err := json.Unmarshal(dependencies, &source.Dependencies); err != nil {
			rows.Close()
			return PackageListPage{}, err
		}
		group := scans[scanID]
		if group == nil {
			group = &scan{id: repositoryID, name: repositoryName}
			scans[scanID] = group
		}
		group.sources = append(group.sources, source)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return PackageListPage{}, err
	}
	items := []PackageUsageItem{}
	for _, scan := range scans {
		for _, finding := range query.Assess(scan.sources) {
			file := finding.Path
			items = append(items, PackageUsageItem{Source: "cli", Name: query.Name, Ecosystem: query.Ecosystem,
				Version: finding.Version, Constraint: finding.Constraint, RepositoryID: scan.id, RepositoryName: scan.name,
				ManifestPath: &file, Assessment: &finding.Evidence})
		}
	}
	providerRows, err := s.DB.Query(ctx, packageCandidatesSQL+` select * from filtered where source <> 'cli' `+packageOrderSQL,
		filter.TenantID, "", "")
	if err != nil {
		return PackageListPage{}, err
	}
	defer providerRows.Close()
	for providerRows.Next() {
		var item PackageUsageItem
		if err := providerRows.Scan(&item.Source, &item.Name, &item.Version, &item.Constraint, &item.Ecosystem, &item.RepositoryID, &item.RepositoryName, &item.ManifestPath, &item.OriginKind, &item.Destination); err != nil {
			return PackageListPage{}, err
		}
		file := ""
		if item.ManifestPath != nil {
			file = *item.ManifestPath
		}
		finding := query.AssessObservation(assessment.Dependency{Name: item.Name, Ecosystem: item.Ecosystem, Version: item.Version}, file)
		if finding != nil {
			item.Assessment = &finding.Evidence
			items = append(items, item)
		}
	}
	if err := providerRows.Err(); err != nil {
		return PackageListPage{}, err
	}
	slices.SortFunc(items, func(a, b PackageUsageItem) int {
		aPath, bPath := "", ""
		if a.ManifestPath != nil {
			aPath = *a.ManifestPath
		}
		if b.ManifestPath != nil {
			bPath = *b.ManifestPath
		}
		return strings.Compare(strings.Join([]string{a.RepositoryName, a.RepositoryID, a.Source, aPath, a.Version, a.Constraint, a.Assessment.Status}, "\x00"), strings.Join([]string{b.RepositoryName, b.RepositoryID, b.Source, bPath, b.Version, b.Constraint, b.Assessment.Status}, "\x00"))
	})
	items = slices.CompactFunc(items, func(a, b PackageUsageItem) bool { return reflect.DeepEqual(a, b) })
	total := len(items)
	offset := int64(filter.Page-1) * int64(filter.PageSize)
	pageItems := []PackageUsageItem{}
	if offset < int64(total) {
		pageItems = items[int(offset):min(int(offset)+filter.PageSize, total)]
	}
	totalPages := (total + filter.PageSize - 1) / filter.PageSize
	return PackageListPage{Items: pageItems, Pagination: PaginationMetadata{Page: filter.Page, PageSize: filter.PageSize, Total: total, TotalPages: totalPages, HasPrevious: filter.Page > 1, HasNext: filter.Page < totalPages},
		Filters: PackageFilters{Query: query.Name, Ecosystem: query.Ecosystem, AffectedRange: query.Range}}, nil
}
