package scans

import (
	"context"
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPackageTypesServiceStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:17", postgres.WithDatabase("deplens"), postgres.WithUsername("postgres"), postgres.WithPassword("postgres"), postgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, store.Migrate(url))
	db, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	defer db.Close()
	var tenant string
	require.NoError(t, db.QueryRow(ctx, `insert into tenants (slug,name) values ('types','Types') returning id`).Scan(&tenant))
	types := []string{"", "future-ecosystem", "npm", "pip", "pypi", "unknown"}
	var dependencies []DependencyInput
	for _, value := range types {
		dependencies = append(dependencies, DependencyInput{Name: "same", Version: "1", PackageType: value, Extras: map[string]string{"package_type": "independent"}})
	}
	svc := Service{Blob: blob.NewFileSystemStore(t.TempDir()), Store: store.Store{DB: db}}
	id, err := svc.Upload(ctx, tenant, UploadRequest{SchemaVersion: "v1alpha1", Repository: RepositoryInput{Name: "Repo"}, Source: SourceInput{ScannedAt: "2026-11-01T12:00:00Z"}, Snapshot: SnapshotInput{Root: ".", Manifests: []ManifestInput{{Path: "mixed", Type: "not-an-ecosystem", Dependencies: dependencies}}}})
	require.NoError(t, err)
	reader := store.ScanStore{DB: db}
	manifests, err := reader.ListScanManifests(ctx, tenant, id)
	require.NoError(t, err)
	require.Len(t, manifests, 1)
	for i, dep := range manifests[0].Dependencies {
		require.Equal(t, types[i], dep.PackageType)
		require.Equal(t, "independent", dep.Extras["package_type"])
	}
	groups, err := reader.ListDependencies(ctx, tenant)
	require.NoError(t, err)
	require.Len(t, groups, len(types))
	got := []string{}
	for _, group := range groups {
		got = append(got, group.Ecosystem)
	}
	require.ElementsMatch(t, types, got)
	got = nil
	for page := 1; page <= 3; page++ {
		result, err := reader.ListPackages(ctx, store.PackageListFilter{TenantID: tenant, Page: page, PageSize: 2})
		require.NoError(t, err)
		require.Len(t, result.Items, 2)
		for _, item := range result.Items {
			got = append(got, item.Ecosystem)
		}
	}
	require.Equal(t, types, got)
	for _, provider := range []struct{ name, packageType string }{{"socket", "pypi"}, {"snyk", "pip"}} {
		var connection string
		require.NoError(t, db.QueryRow(ctx, "insert into "+provider.name+"_connections (tenant_id, organization, token_cipher) values ($1, 'org', 'cipher') returning id", tenant).Scan(&connection))
		_, err := db.Exec(ctx, "insert into "+provider.name+"_repositories (connection_id,remote_id,name,snapshot,available) values ($1,'repo','Provider',$2,true)", connection, `{"artifacts":[{"id":"pkg","name":"same","version":"1","type":"`+provider.packageType+`","manifestFiles":[{"file":"manifest"}]}]}`)
		require.NoError(t, err)
	}
	result, err := reader.ListPackages(ctx, store.PackageListFilter{TenantID: tenant, Page: 1, PageSize: 25})
	require.NoError(t, err)
	require.Len(t, result.Items, 8)
	for _, item := range result.Items {
		if item.Source == "socket" {
			require.Equal(t, "pypi", item.Ecosystem)
		}
		if item.Source == "snyk" {
			require.Equal(t, "pip", item.Ecosystem)
		}
	}
	groups, err = reader.ListDependencies(ctx, tenant)
	require.NoError(t, err)
	require.Len(t, groups, 6)
	for _, group := range groups {
		if group.Ecosystem == "pypi" || group.Ecosystem == "pip" {
			require.EqualValues(t, 2, group.RepositoryCount)
		}
	}
}
