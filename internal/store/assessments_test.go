package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAssessPackagesPersistsEvidenceAndIsolatesTenants(t *testing.T) {
	ctx := context.Background()
	db, url := openTestDatabase(t)
	require.NoError(t, Migrate(url))
	tenant := mustCreateTenant(t, ctx, db)
	other := mustCreateTenantWithSlug(t, ctx, db, "other-assessments")
	manifest := func() UploadManifestParams {
		return UploadManifestParams{Path: "package.json", Type: "js", Form: "manifest", Extraction: "complete", Lockfiles: []string{"package-lock.json", "yarn.lock"}, Dependencies: []UploadDependencyParams{{Name: "widget", PackageType: "npm", Constraint: "^2.3.0"}}}
	}
	lock := func(file, version string) UploadManifestParams {
		return UploadManifestParams{Path: file, Type: "lock", Form: "lockfile", Extraction: "complete", Dependencies: []UploadDependencyParams{{Name: "widget", PackageType: "npm", Version: version}}}
	}
	upload := func(tenant, repo, commit string, sources ...UploadManifestParams) {
		t.Helper()
		for i := range sources {
			sources[i].Position = i
		}
		_, err := (Store{DB: db}).CreateScan(ctx, UploadScanParams{TenantID: tenant, RepositoryName: repo, URL: "https://example.com/" + repo, ArtifactKey: commit, ArtifactSHA256: commit, SchemaVersion: "v1alpha1", ScannedAt: time.Now(), Manifests: sources})
		require.NoError(t, err)
	}
	upload(tenant, "competing", "a", manifest(), lock("package-lock.json", "2.3.5"), lock("yarn.lock", "2.3.3"))
	upload(other, "secret", "secret", manifest(), lock("package-lock.json", "2.3.3"))
	read := ScanStore{DB: db}
	filter := PackageListFilter{TenantID: tenant, Query: "widget", Ecosystem: "npm", AffectedRange: ">0.0.0 <=2.3.4", Page: 1, PageSize: 1}
	result, err := read.ListPackages(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, result.Pagination.Total)
	require.Equal(t, "issue", result.Items[0].Assessment.Status)
	require.Equal(t, "yarn.lock", *result.Items[0].ManifestPath)
	require.Equal(t, "competing", result.Items[0].RepositoryName)
	m := manifest()
	m.Lockfiles = []string{"package-lock.json"}
	upload(tenant, "competing", "b", m, lock("package-lock.json", "2.3.5"))
	result, err = read.ListPackages(ctx, filter)
	require.NoError(t, err)
	require.Zero(t, result.Pagination.Total)
	padded := manifest()
	padded.Dependencies[0].Name = " widget "
	upload(tenant, "unlocked", "c", padded)
	result, err = read.ListPackages(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, result.Pagination.Total)
	require.Equal(t, "warning", result.Items[0].Assessment.Status)
	filter.Query = "widg"
	result, err = read.ListPackages(ctx, filter)
	require.NoError(t, err)
	require.Empty(t, result.Items)
	var connection string
	require.NoError(t, db.QueryRow(ctx, "insert into socket_connections (tenant_id, organization, token_cipher) values ($1, 'assessment-org', 'cipher') returning id", tenant).Scan(&connection))
	_, err = db.Exec(ctx, `insert into socket_repositories (connection_id, remote_id, name, snapshot, available)
 values ($1, 'provider', 'Provider', '{"artifacts":[{"name":"widget","type":"npm","version":"2.3.2"},{"name":"widget-extra","type":"npm","version":"2.3.2"},{"name":"widget","type":"pypi","version":"2.3.2"}]}', true)`, connection)
	require.NoError(t, err)
	filter.Query = "widget"
	filter.PageSize = 100
	result, err = read.ListPackages(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 2, result.Pagination.Total)
	var providerCount int
	for _, item := range result.Items {
		if item.Source == "socket" {
			providerCount++
			require.Equal(t, "issue", item.Assessment.Status)
			require.Equal(t, "2.3.2", item.Version)
		}
	}
	require.Equal(t, 1, providerCount)
	filter.TenantID = other
	result, err = read.ListPackages(ctx, filter)
	require.NoError(t, err)
	for _, item := range result.Items {
		require.Equal(t, "cli", item.Source)
		require.Equal(t, "secret", item.RepositoryName)
	}
}
