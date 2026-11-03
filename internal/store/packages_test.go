package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListPackagesSearchesCurrentUsageAcrossSources(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	require.NoError(t, Migrate(databaseURL))
	tenantID := mustCreateTenant(t, ctx, db)
	otherTenantID := mustCreateTenantWithSlug(t, ctx, db, "other-packages")
	writeStore := Store{DB: db}
	createScan := func(tenant, commit string, manifests []UploadManifestParams) string {
		t.Helper()
		id, err := writeStore.CreateScan(ctx, UploadScanParams{
			TenantID: tenant, RepositoryName: "CLI repo", URL: "https://example.com/repo.git", DefaultBranch: "main",
			ArtifactKey: commit, ArtifactSHA256: commit, SchemaVersion: "v1alpha1", CommitSHA: commit,
			ScannedAt: time.Now(), Manifests: manifests,
		})
		require.NoError(t, err)
		var repositoryID string
		require.NoError(t, db.QueryRow(ctx, "select repository_id from scans where id = $1", id).Scan(&repositoryID))
		return repositoryID
	}
	createScan(tenantID, "old", []UploadManifestParams{
		{Position: 0, Type: "npm", Path: "gone.json", Dependencies: []UploadDependencyParams{{Name: "gone", Version: "1"}}},
		{Position: 1, Type: "npm", Path: "package.json", Dependencies: []UploadDependencyParams{{Name: "superseded", Version: "1"}}},
	})
	repositoryID := createScan(tenantID, "current", []UploadManifestParams{
		{Position: 0, Type: "npm-package", Path: "package.json", Dependencies: []UploadDependencyParams{
			{Position: 0, Name: "react", Version: "19", Source: "registry"},
			{Position: 1, Name: "react", Version: "19", Raw: "same usage with another raw value"},
			{Position: 2, Name: "react", Version: "18"},
			{Position: 3, Name: "react", Constraint: "^19"},
			{Position: 4, Name: "literal%pkg", Version: "1"},
			{Position: 5, Name: "literal_pkg", Version: "1"},
			{Position: 6, Name: `literal\pkg`, Version: "1"},
			{Position: 7, Name: "other", Raw: "react", Version: "react"},
			{Position: 8, Raw: "react unnamed"},
			{Position: 9, Name: "@scope/widget", Version: "2"},
		}},
		{Position: 1, Type: "npm", Path: "packages/app/package.json", Dependencies: []UploadDependencyParams{{Name: "react", Version: "19"}}},
	})
	otherRepositoryID := createScan(otherTenantID, "other", []UploadManifestParams{{Type: "npm", Path: "package.json", Dependencies: []UploadDependencyParams{{Name: "secret-cli", Version: "1"}}}})

	var socketConnection, snykConnection, otherSocketConnection, otherSnykConnection string
	for _, connection := range []struct {
		table, tenant string
		id            *string
	}{
		{"socket_connections", tenantID, &socketConnection},
		{"snyk_connections", tenantID, &snykConnection},
		{"socket_connections", otherTenantID, &otherSocketConnection},
		{"snyk_connections", otherTenantID, &otherSnykConnection},
	} {
		require.NoError(t, db.QueryRow(ctx, "insert into "+connection.table+" (tenant_id, organization, token_cipher) values ($1, 'org', 'cipher') returning id", connection.tenant).Scan(connection.id))
	}
	insertSocket := func(connection, name, snapshot string, cliID *string, available bool) string {
		t.Helper()
		var id string
		err := db.QueryRow(ctx, `insert into socket_repositories (connection_id, remote_id, name, snapshot, cli_repository_id, available)
			values ($1, $2, $2, $3, $4, $5) returning id`, connection, name, snapshot, cliID, available).Scan(&id)
		require.NoError(t, err)
		return id
	}
	insertSnyk := func(connection, name, snapshot string, cliID, socketID *string, available bool) string {
		t.Helper()
		var id string
		err := db.QueryRow(ctx, `insert into snyk_repositories (connection_id, remote_id, name, snapshot, cli_repository_id, socket_repository_id, available)
			values ($1, $2, $2, $3, $4, $5, $6) returning id`, connection, name, snapshot, cliID, socketID, available).Scan(&id)
		require.NoError(t, err)
		return id
	}
	socketID := insertSocket(socketConnection, "Socket linked name", `{"artifacts":[
		{"name":"react","version":"19","type":"npm","manifestFiles":[{"file":"package.json"},{"file":"package.json"}]},
		{"name":"widget","namespace":"@scope","version":"2","type":"npm"},
		{"name":"socket-no-file","version":"1","manifestFiles":null},
		{"version":"1"}
	]}`, &repositoryID, true)
	socketOnlyID := insertSocket(socketConnection, "Socket only", `{"artifacts":[{"name":"provider-only","version":"1","manifestFiles":[]}]}`, nil, true)
	insertSocket(socketConnection, "Unavailable Socket", `{"artifacts":[{"name":"unavailable-socket"}]}`, nil, false)
	insertSocket(otherSocketConnection, "Other Socket", `{"artifacts":[{"name":"secret-socket"}]}`, nil, true)
	insertSocket(socketConnection, "Cross tenant Socket mapping", `{"artifacts":[{"name":"secret-socket-mapping"}]}`, &otherRepositoryID, true)
	insertSnyk(snykConnection, "Snyk linked name", `{"artifacts":[{"name":"react","version":"19","type":"npm","manifestFiles":[{"file":"package.json"},{"file":"package.json"}]}]}`, nil, &socketID, true)
	snykOnlyID := insertSnyk(snykConnection, "Snyk only", `{"artifacts":[{"name":"snyk-no-file","manifestFiles":[]}]}`, nil, nil, true)
	insertSnyk(snykConnection, "Snyk linked Socket only", `{"artifacts":[{"name":"provider-linked","version":"1"}]}`, nil, &socketOnlyID, true)
	insertSnyk(snykConnection, "Snyk direct CLI", `{"artifacts":[{"name":"direct-cli-link","version":"1"}]}`, &repositoryID, nil, true)
	insertSnyk(snykConnection, "Unavailable Snyk", `{"artifacts":[{"name":"unavailable-snyk"}]}`, nil, nil, false)
	insertSnyk(otherSnykConnection, "Other Snyk", `{"artifacts":[{"name":"secret-snyk"}]}`, nil, nil, true)
	insertSnyk(snykConnection, "Cross tenant Snyk mapping", `{"artifacts":[{"name":"secret-snyk-mapping"}]}`, &otherRepositoryID, nil, true)

	reads := ScanStore{DB: db}
	all, err := reads.ListPackages(ctx, PackageListFilter{TenantID: tenantID, PageSize: 100})
	require.NoError(t, err)
	require.Len(t, all.Items, 17)
	require.Equal(t, 17, all.Pagination.Total)
	for _, item := range all.Items {
		require.NotEmpty(t, item.Name)
		require.NotContains(t, item.Name, "secret")
		require.NotContains(t, item.Name, "unavailable")
		require.NotEqual(t, "gone", item.Name)
		require.NotEqual(t, "superseded", item.Name)
		if item.Source == "cli" {
			require.Empty(t, item.Ecosystem)
		}
	}

	page, err := reads.ListPackages(ctx, PackageListFilter{TenantID: tenantID, Query: " REACT ", PageSize: 100})
	require.NoError(t, err)
	path := "package.json"
	appPath := "packages/app/package.json"
	require.Equal(t, []PackageUsageItem{
		{Source: "cli", Name: "react", Constraint: "^19", RepositoryID: repositoryID, RepositoryName: "CLI repo", ManifestPath: &path},
		{Source: "cli", Name: "react", Version: "18", RepositoryID: repositoryID, RepositoryName: "CLI repo", ManifestPath: &path},
		{Source: "cli", Name: "react", Version: "19", RepositoryID: repositoryID, RepositoryName: "CLI repo", ManifestPath: &path},
		{Source: "cli", Name: "react", Version: "19", RepositoryID: repositoryID, RepositoryName: "CLI repo", ManifestPath: &appPath},
		{Source: "snyk", Name: "react", Version: "19", Ecosystem: "npm", RepositoryID: repositoryID, RepositoryName: "CLI repo", ManifestPath: &path},
		{Source: "socket", Name: "react", Version: "19", Ecosystem: "npm", RepositoryID: repositoryID, RepositoryName: "CLI repo", ManifestPath: &path},
	}, page.Items)
	require.Equal(t, "REACT", page.Filters.Query)

	for _, literal := range []string{"%", "_", `\`} {
		found, err := reads.ListPackages(ctx, PackageListFilter{TenantID: tenantID, Query: literal})
		require.NoError(t, err)
		require.Len(t, found.Items, 1)
		require.Contains(t, found.Items[0].Name, literal)
	}
	scoped, err := reads.ListPackages(ctx, PackageListFilter{TenantID: tenantID, Query: "@SCOPE/WID"})
	require.NoError(t, err)
	require.Len(t, scoped.Items, 2)
	for _, item := range scoped.Items {
		require.Equal(t, "@scope/widget", item.Name)
	}
	for _, usage := range []struct{ name, id, repository string }{
		{"provider-only", socketOnlyID, "Socket only"},
		{"provider-linked", socketOnlyID, "Socket only"},
		{"snyk-no-file", snykOnlyID, "Snyk only"},
		{"socket-no-file", repositoryID, "CLI repo"},
		{"direct-cli-link", repositoryID, "CLI repo"},
	} {
		found, err := reads.ListPackages(ctx, PackageListFilter{TenantID: tenantID, Query: usage.name})
		require.NoError(t, err)
		require.Len(t, found.Items, 1)
		require.Equal(t, usage.id, found.Items[0].RepositoryID)
		require.Equal(t, usage.repository, found.Items[0].RepositoryName)
		require.Nil(t, found.Items[0].ManifestPath)
	}

	var paginated []PackageUsageItem
	for n := 1; n <= 6; n++ {
		page, err := reads.ListPackages(ctx, PackageListFilter{TenantID: tenantID, Page: n, PageSize: 3})
		require.NoError(t, err)
		require.Equal(t, 17, page.Pagination.Total)
		require.Equal(t, 6, page.Pagination.TotalPages)
		require.Equal(t, n > 1, page.Pagination.HasPrevious)
		require.Equal(t, n < 6, page.Pagination.HasNext)
		paginated = append(paginated, page.Items...)
	}
	require.Equal(t, all.Items, paginated)
	for _, filter := range []PackageListFilter{
		{TenantID: tenantID, Page: 100, PageSize: 3},
		{TenantID: tenantID, Query: "no such package", Page: 1, PageSize: 3},
	} {
		page, err := reads.ListPackages(ctx, filter)
		require.NoError(t, err)
		require.NotNil(t, page.Items)
		require.Empty(t, page.Items)
		if filter.Query == "" {
			require.Equal(t, 17, page.Pagination.Total)
		} else {
			require.Zero(t, page.Pagination.Total)
			require.Zero(t, page.Pagination.TotalPages)
		}
	}
}
