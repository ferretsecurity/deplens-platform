package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const repairReport = `{"schema_version":1,"root":"/repo","sources":[{"detector":"mixed","path":"first","analysis":{"presence":"present","extraction":"complete"},"dependencies":[{"raw":"a","name":"a","package_type":"npm"}]},{"detector":"mixed","path":"retained","analysis":{"presence":"present","extraction":"complete"},"dependencies":[{"raw":"x","name":"same","version":"1","package_type":"pypi","attributes":{"package_type":"independent"}},{"raw":"y","name":"same","version":"1","package_type":"future-type"},{"raw":"z","name":"missing"},{"raw":"u","name":"unknown","package_type":"unknown","attributes":{}}]}],"check_runs":[],"findings":[]}`

func TestRepairRetainedScans(t *testing.T) {
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
	require.NoError(t, db.QueryRow(ctx, `insert into tenants (slug,name) values ('repair','Repair') returning id`).Scan(&tenant))
	root := t.TempDir()
	artifacts := blob.NewFileSystemStore(root)
	service := scans.Service{Blob: artifacts, Store: store.Store{DB: db}}
	snapshot, err := scans.SnapshotFromReport(json.RawMessage(repairReport))
	require.NoError(t, err)
	input := scans.UploadRequest{Report: json.RawMessage(repairReport), SchemaVersion: "v1alpha1", Repository: scans.RepositoryInput{Name: "Repo"}, Source: scans.SourceInput{ScannedAt: "2026-11-01T12:00:00Z"}, Snapshot: snapshot}
	id, err := service.Upload(ctx, tenant, input)
	require.NoError(t, err)
	input.Report = nil
	input.Snapshot.Manifests = snapshot.Manifests[:1]
	input.Source.ScannedAt = "2026-11-02T12:00:00Z"
	_, err = service.Upload(ctx, tenant, input)
	require.NoError(t, err)
	var retained int
	require.NoError(t, db.QueryRow(ctx, `select count(*) from scan_manifests where scan_id=$1`, id).Scan(&retained))
	require.Equal(t, 1, retained)
	reset := func() {
		t.Helper()
		_, err := db.Exec(ctx, `update manifest_dependencies d set package_type='' from scan_manifests sm where sm.id=d.scan_manifest_id and sm.scan_id=$1`, id)
		require.NoError(t, err)
	}
	types := func() []string {
		t.Helper()
		rows, err := db.Query(ctx, `select d.package_type from manifest_dependencies d join scan_manifests sm on sm.id=d.scan_manifest_id where sm.scan_id=$1 order by d.position`, id)
		require.NoError(t, err)
		defer rows.Close()
		var result []string
		for rows.Next() {
			var value string
			require.NoError(t, rows.Scan(&value))
			result = append(result, value)
		}
		require.NoError(t, rows.Err())
		return result
	}
	state := func() string {
		t.Helper()
		var value string
		require.NoError(t, db.QueryRow(ctx, `select jsonb_build_object('scans',(select jsonb_agg(to_jsonb(s) order by s.id) from scans s),'manifests',(select jsonb_agg(to_jsonb(m) order by m.id) from manifests m),'observations',(select jsonb_agg(to_jsonb(sm) order by sm.id) from scan_manifests sm),'dependencies',(select jsonb_agg(to_jsonb(d)-'package_type' order by d.id) from manifest_dependencies d))::text`).Scan(&value))
		return value
	}
	var out bytes.Buffer
	reset()
	before := state()
	require.NoError(t, run(ctx, url, root, tenant, id, false, &out))
	require.Contains(t, out.String(), "apply=false package_types=3 retained_manifests=1")
	require.Equal(t, []string{"", "", "", ""}, types())
	require.NoError(t, run(ctx, url, root, tenant, id, true, &out))
	require.Equal(t, []string{"pypi", "future-type", "", "unknown"}, types())
	require.Equal(t, before, state())
	out.Reset()
	require.NoError(t, run(ctx, url, root, "", "", true, &out))
	require.Contains(t, out.String(), "package_types=0")
	require.Equal(t, before, state())

	t.Run("mismatch rolls back entire scan", func(t *testing.T) {
		reset()
		_, err := db.Exec(ctx, `update manifest_dependencies d set raw='changed' from scan_manifests sm where sm.id=d.scan_manifest_id and sm.scan_id=$1 and d.position=3`, id)
		require.NoError(t, err)
		err = run(ctx, url, root, tenant, id, true, &out)
		require.ErrorContains(t, err, "dependency 3 mismatch")
		require.Equal(t, []string{"", "", "", ""}, types())
		_, err = db.Exec(ctx, `update manifest_dependencies d set raw='u' from scan_manifests sm where sm.id=d.scan_manifest_id and sm.scan_id=$1 and d.position=3`, id)
		require.NoError(t, err)
	})
	t.Run("conflicting nonempty type", func(t *testing.T) {
		reset()
		_, err := db.Exec(ctx, `update manifest_dependencies d set package_type='pip' from scan_manifests sm where sm.id=d.scan_manifest_id and sm.scan_id=$1 and d.position=0`, id)
		require.NoError(t, err)
		require.ErrorContains(t, run(ctx, url, root, tenant, id, true, &out), "package type conflict")
		require.Equal(t, []string{"pip", "", "", ""}, types())
	})
	t.Run("missing dependency", func(t *testing.T) {
		tx, err := db.Begin(ctx)
		require.NoError(t, err)
		var deleted []byte
		require.NoError(t, tx.QueryRow(ctx, `delete from manifest_dependencies d using scan_manifests sm where sm.id=d.scan_manifest_id and sm.scan_id=$1 and d.position=3 returning to_jsonb(d)`, id).Scan(&deleted))
		require.NoError(t, tx.Commit(ctx))
		reset()
		require.ErrorContains(t, run(ctx, url, root, tenant, id, true, &out), "dependency count mismatch")
		_, err = db.Exec(ctx, `insert into manifest_dependencies select * from jsonb_populate_record(null::manifest_dependencies,$1::jsonb)`, string(deleted))
		require.NoError(t, err)
	})
	t.Run("artifact evidence", func(t *testing.T) {
		var key, hash string
		require.NoError(t, db.QueryRow(ctx, `select artifact_key,artifact_sha256 from scans where id=$1`, id).Scan(&key, &hash))
		saved, err := artifacts.Get(ctx, key)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, key), []byte(`{}`), 0600))
		require.ErrorContains(t, run(ctx, url, root, tenant, id, true, &out), "artifact SHA mismatch")
		require.NoError(t, os.Remove(filepath.Join(root, key)))
		require.ErrorContains(t, run(ctx, url, root, tenant, id, true, &out), "read artifact")
		require.NoError(t, os.WriteFile(filepath.Join(root, key), saved, 0600))
		for _, report := range []string{`{}`, `{"report":{"schema_version":2}}`} {
			newKey, newHash, err := artifacts.Put(ctx, []byte(report))
			require.NoError(t, err)
			_, err = db.Exec(ctx, `update scans set artifact_key=$2,artifact_sha256=$3 where id=$1`, id, newKey, newHash)
			require.NoError(t, err)
			out.Reset()
			require.NoError(t, run(ctx, url, root, tenant, id, true, &out))
			require.Contains(t, out.String(), "skipped:")
		}
		_, err = db.Exec(ctx, `update scans set artifact_key=$2,artifact_sha256=$3 where id=$1`, id, key, hash)
		require.NoError(t, err)
	})
	t.Run("repository lock serializes upload", func(t *testing.T) {
		reset()
		locker, err := db.Begin(ctx)
		require.NoError(t, err)
		defer locker.Rollback(ctx)
		_, err = locker.Exec(ctx, `select pg_advisory_xact_lock(772007)`)
		require.NoError(t, err)
		_, err = db.Exec(ctx, `create function pause_type_repair() returns trigger language plpgsql as $$ begin perform pg_advisory_xact_lock(772007); return new; end $$`)
		require.NoError(t, err)
		_, err = db.Exec(ctx, `create trigger pause_type_repair before update of package_type on manifest_dependencies for each row execute function pause_type_repair()`)
		require.NoError(t, err)
		repairDone := make(chan error, 1)
		go func() { repairDone <- run(ctx, url, root, tenant, id, true, &bytes.Buffer{}) }()
		require.Eventually(t, func() bool {
			var waiting bool
			err := db.QueryRow(ctx, `select exists(select 1 from pg_stat_activity where wait_event='advisory' and query like 'update manifest_dependencies d set package_type%')`).Scan(&waiting)
			return err == nil && waiting
		}, 5*time.Second, 20*time.Millisecond)
		input.Source.ScannedAt = "2026-11-03T12:00:00Z"
		uploadDone := make(chan error, 1)
		go func() { _, err := service.Upload(ctx, tenant, input); uploadDone <- err }()
		require.Eventually(t, func() bool {
			var waiting bool
			err := db.QueryRow(ctx, `select exists(select 1 from pg_stat_activity where wait_event_type='Lock' and query like '%insert into repositories%')`).Scan(&waiting)
			return err == nil && waiting
		}, 5*time.Second, 20*time.Millisecond)
		require.NoError(t, locker.Commit(ctx))
		require.NoError(t, <-repairDone)
		require.NoError(t, <-uploadDone)
		_, err = db.Exec(ctx, `drop trigger pause_type_repair on manifest_dependencies`)
		require.NoError(t, err)
	})
}
