package socket

import (
	"archive/tar"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type redirectTransport struct{ target *url.URL }

func (t redirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}
func fixtureClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	client := NewClient()
	client.HTTP.Transport = redirectTransport{target}
	return client
}
func TestIntegration(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:17", postgres.WithDatabase("socket_test"), postgres.WithUsername("postgres"), postgres.WithPassword("postgres"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 50; i++ {
		if db.Ping(ctx) == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err = store.Migrate(dsn); err != nil {
		t.Fatal(err)
	}
	var tenant, other, cli string
	if err = db.QueryRow(ctx, `insert into tenants(slug,name)values('test','Test')returning id`).Scan(&tenant); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `insert into tenants(slug,name)values('other','Other')returning id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `insert into repositories(tenant_id,name,url,default_branch)values($1,'CLI fixture','git@github.com:fixture/repo-1.git','main')returning id`, tenant).Scan(&cli); err != nil {
		t.Fatal(err)
	}
	cliScan, err := (store.Store{DB: db}).CreateScan(ctx, store.UploadScanParams{TenantID: tenant, RepositoryName: "CLI fixture", URL: "git@github.com:fixture/repo-1.git", DefaultBranch: "main", ArtifactKey: "fixture-cli", ScannedAt: time.Now(), Labels: map[string]string{}, Manifests: []store.UploadManifestParams{{Path: "cli-only.json", Type: "fixture", Warnings: []string{}, Dependencies: []store.UploadDependencyParams{{Name: "cli-only", Version: "1", Extras: map[string]string{}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	cliBefore, err := (store.ScanStore{DB: db}).ListScanManifests(ctx, tenant, cliScan)
	if err != nil {
		t.Fatal(err)
	}
	var current, peak atomic.Int32
	var fail, hide, lastPageFails atomic.Bool
	client := fixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := current.Add(1)
		defer current.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
		if r.Header.Get("Authorization") != "Bearer disposable-test-token" {
			t.Error("missing authorization")
		}
		if strings.HasSuffix(r.URL.Path, "/repos") {
			page := r.URL.Query().Get("page")
			if page == "2" && lastPageFails.Load() {
				http.Error(w, "private-body", 400)
				return
			}
			if page == "1" {
				io.WriteString(w, `{"results":[{"id":"1","name":"Fixture 1","head_full_scan_id":"scan-1","html_url":"https://socket.dev/dashboard/org/fixture/repo/1"},{"id":"2","name":"Fixture 2","head_full_scan_id":"scan-2"}],"nextPage":2}`)
			} else {
				if hide.Load() {
					io.WriteString(w, `{"results":[{"id":"3","name":"Fixture 3","head_full_scan_id":"scan-3"}],"nextPage":null}`)
				} else {
					io.WriteString(w, `{"results":[{"id":"3","name":"Fixture 3","head_full_scan_id":"scan-3"},{"id":"4","name":"Fixture 4","head_full_scan_id":"scan-4"}],"nextPage":null}`)
				}
			}
			return
		}
		if strings.HasSuffix(r.URL.Path, "/metadata") {
			io.WriteString(w, `{"branch":"main","commit_hash":"fixture-commit","integration_repo_url":"https://github.com/fixture/repo-1"}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/files/tar") {
			tw := tar.NewWriter(w)
			for _, name := range []string{"package.json", "empty.lock"} {
				_ = tw.WriteHeader(&tar.Header{Name: name, Size: 2, Mode: 0600})
				_, _ = tw.Write([]byte("{}"))
			}
			_ = tw.Close()
			return
		}
		if fail.Load() && strings.HasSuffix(r.URL.Path, "scan-1") {
			http.Error(w, "confidential upstream body", 400)
			return
		}
		io.WriteString(w, `{"id":"pkg1","type":"npm","name":"debug","version":"4.4.3","dev":true,"manifestFiles":[{"file":"package.json","start":{"line":2}}]}`+"\n"+`{"id":"pkg2","type":"pypi","name":"debug","version":"4.4.3","manifestFiles":[]}`+"\n"+`{"_type":"scores","value":{}}`+"\n")
	}))
	vault, err := NewVault(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db, Client: client, Vault: vault}
	token := "disposable-test-token"
	if err = s.Save(ctx, tenant, "fixture", &token, 60); err != nil {
		t.Fatal(err)
	}
	if err = s.Save(ctx, tenant, "fixture", nil, 360); err != nil {
		t.Fatal(err)
	}
	org, recovered, err := s.credentials(ctx, tenant)
	if err != nil || org != "fixture" || recovered != token {
		t.Fatalf("token preservation: %v", err)
	}
	if _, _, err = s.credentials(ctx, other); err == nil {
		t.Fatal("cross tenant credentials")
	}
	var cipher []byte
	_ = db.QueryRow(ctx, `select token_cipher from socket_connections where tenant_id=$1`, tenant).Scan(&cipher)
	if strings.Contains(string(cipher), token) {
		t.Fatal("plaintext token stored")
	}
	if _, err = vault.Open(cipher, other); err == nil {
		t.Fatal("cipher not tenant bound")
	}
	var wg sync.WaitGroup
	var starts atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Start(ctx, tenant); e == nil {
				starts.Add(1)
			}
		}()
	}
	wg.Wait()
	if starts.Load() != 1 {
		t.Fatalf("duplicate starts %d", starts.Load())
	}
	if err = s.Save(ctx, tenant, "fixture", nil, 60); err != ErrConflict {
		t.Fatalf("save during run %v", err)
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.workOnce(ctx) }()
	}
	wg.Wait()
	cfg, err := s.Configuration(ctx, tenant)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LastRun.Status != "completed" || cfg.LastRun.Completed != 4 || cfg.LastRun.Failed != 0 {
		t.Fatalf("run %#v", cfg.LastRun)
	}
	if peak.Load() <= 1 || peak.Load() > 4 {
		t.Fatalf("concurrent requests %d", peak.Load())
	}
	inventory, err := s.Inventory(ctx, tenant, cli)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Sources) != 4 {
		t.Fatalf("matching sources %d", len(inventory.Sources))
	}
	snapshot := inventory.Sources[0].Snapshot
	if len(snapshot.Files) != 2 || len(snapshot.Artifacts) != 2 {
		t.Fatalf("snapshot %#v", snapshot)
	}
	if _, err = s.Inventory(ctx, other, cli); err == nil {
		t.Fatal("tenant inventory leaked")
	}
	page, err := (store.ScanStore{DB: db}).ListRepositories(ctx, store.RepositoryListFilter{TenantID: tenant})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || !page.Items[0].CLI || !page.Items[0].Socket {
		t.Fatalf("provenance %#v", page)
	}
	deps, err := (store.ScanStore{DB: db}).ListDependencies(ctx, tenant)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 3 {
		t.Fatalf("ecosystems merged %#v", deps)
	}
	var sourceID string
	_ = db.QueryRow(ctx, `select id from socket_repositories where remote_id='1'`).Scan(&sourceID)
	before, err := s.Inventory(ctx, tenant, cli)
	if err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	if _, err = s.Start(ctx, tenant); err != nil {
		t.Fatal(err)
	}
	s.workOnce(ctx)
	cfg, _ = s.Configuration(ctx, tenant)
	if cfg.LastRun.Status != "partial" || cfg.LastRun.Failed != 1 {
		t.Fatalf("failed run %#v", cfg.LastRun)
	}
	after, _ := s.Inventory(ctx, tenant, cli)
	var beforeJSON, afterJSON []byte
	for _, source := range before.Sources {
		if source.ID == sourceID {
			beforeJSON, _ = json.Marshal(source.Snapshot)
		}
	}
	for _, source := range after.Sources {
		if source.ID == sourceID {
			afterJSON, _ = json.Marshal(source.Snapshot)
			if source.Error == "" {
				t.Fatal("failed repository has no error")
			}
		}
	}
	if string(beforeJSON) != string(afterJSON) {
		t.Fatal("failed import replaced snapshot")
	}
	// A stale owner cannot publish, even if it still has a decoded snapshot.
	if err = s.publish(ctx, cfg.LastRun.ID, uuid.NewString(), uuid.NewString(), tenant, Repository{ID: "1"}, *snapshot); err != ErrLease {
		t.Fatalf("stale publication %v", err)
	}
	if err = s.MapRepository(ctx, other, sourceID, &cli); err == nil {
		t.Fatal("cross tenant mapping")
	}
	if err = s.MapRepository(ctx, tenant, sourceID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Inventory(ctx, tenant, sourceID); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `insert into repositories(tenant_id,name,url,default_branch)select $1,'Bulk '||i,'','' from generate_series(1,110)i`, tenant)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Inventory(ctx, tenant, sourceID); err != nil {
		t.Fatal("direct lookup beyond first page", err)
	}
	_, _ = db.Exec(ctx, `update socket_connections set next_run_at=now()-interval '3 days' where tenant_id=$1`, tenant)
	s.workOnce(ctx)
	cfg, _ = s.Configuration(ctx, tenant)
	if cfg.NextRun == nil || !cfg.NextRun.After(time.Now()) {
		t.Fatal("schedule not advanced")
	}
	if err = s.Save(ctx, tenant, "fixture", nil, 0); err != nil {
		t.Fatal(err)
	}
	cfg, _ = s.Configuration(ctx, tenant)
	if cfg.NextRun != nil {
		t.Fatal("disabled schedule has next run")
	}
	lastPageFails.Store(true)
	if _, err = s.Start(ctx, tenant); err != nil {
		t.Fatal(err)
	}
	s.workOnce(ctx)
	var available bool
	_ = db.QueryRow(ctx, `select available from socket_repositories where remote_id='4'`).Scan(&available)
	if !available {
		t.Fatal("incomplete listing marked a repository unavailable")
	}
	lastPageFails.Store(false)
	hide.Store(true)
	if _, err = s.Start(ctx, tenant); err != nil {
		t.Fatal(err)
	}
	s.workOnce(ctx)
	_ = db.QueryRow(ctx, `select available from socket_repositories where remote_id='4'`).Scan(&available)
	if available {
		t.Fatal("missing upstream repository not marked unavailable")
	}
	// Recover a run left behind by a dead server.
	id, err := s.Start(ctx, tenant)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(ctx, `update socket_runs set status='running',owner=$2,lease_until=now()-interval '1 second' where id=$1`, id, uuid.NewString())
	s.workOnce(ctx)
	cfg, _ = s.Configuration(ctx, tenant)
	if cfg.LastRun.Status != "partial" {
		t.Fatal("expired lease not recovered")
	}

	t.Run("progress failure stops a full queue", func(t *testing.T) {
		s.Client = fixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			repos := make([]Repository, 12)
			for i := range repos {
				repos[i] = Repository{ID: fmt.Sprint(i + 1), Name: "Queue fixture"}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"results": repos, "nextPage": nil})
		}))
		_, err := db.Exec(ctx, `create function fail_socket_progress() returns trigger language plpgsql as $$ begin if new.completed > old.completed then raise exception 'injected progress failure'; end if; return new; end $$; create trigger fail_socket_progress before update on socket_runs for each row execute function fail_socket_progress()`)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Exec(ctx, `drop trigger fail_socket_progress on socket_runs; drop function fail_socket_progress()`)
		run, err := s.Start(ctx, tenant)
		if err != nil {
			t.Fatal(err)
		}
		owner := uuid.NewString()
		var connection string
		err = db.QueryRow(ctx, `update socket_runs set status='running',owner=$2,lease_until=now()+interval '60 seconds' where id=$1 returning connection_id`, run, owner).Scan(&connection)
		if err != nil {
			t.Fatal(err)
		}
		deadline, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		err = s.sync(deadline, run, owner, connection, tenant, "fixture", "disposable-test-token")
		if err == nil || deadline.Err() != nil {
			t.Fatalf("progress failure did not stop queue promptly: %v", err)
		}
	})
	if err = s.Disconnect(ctx, tenant); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = db.QueryRow(ctx, `select count(*) from repositories where id=$1`, cli).Scan(&count)
	if count != 1 {
		t.Fatal("disconnect removed CLI")
	}
	cliAfter, err := (store.ScanStore{DB: db}).ListScanManifests(ctx, tenant, cliScan)
	if err != nil {
		t.Fatal(err)
	}
	beforeCLI, _ := json.Marshal(cliBefore)
	afterCLI, _ := json.Marshal(cliAfter)
	if string(beforeCLI) != string(afterCLI) {
		t.Fatal("Socket altered CLI scan manifests")
	}

}
func TestClientSafety(t *testing.T) {
	t.Run("processing preserves distinction", func(t *testing.T) {
		client := fixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202); w.Header().Set("Retry-After", "0") }))
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if _, err := client.Snapshot(ctx, "fixture", "token", "scan"); err == nil {
			t.Fatal("processing became empty snapshot")
		}
	})
	t.Run("pagination loop", func(t *testing.T) {
		client := fixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"results":[],"nextPage":1}`) }))
		if _, err := client.Repositories(context.Background(), "fixture", "token"); err == nil {
			t.Fatal("pagination loop accepted")
		}
	})
	t.Run("bounded retry", func(t *testing.T) {
		calls := 0
		client := fixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			io.WriteString(w, "secret-body")
		}))
		_, err := client.Repositories(context.Background(), "fixture", "secret-token")
		if calls != 4 || err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("retry error %v calls %d", err, calls)
		}
	})
	t.Run("real response decoding", func(t *testing.T) {
		paths, _ := filepath.Glob("../../var/socket-popular-100/repositories/*/verification/results.json")
		if len(paths) == 0 {
			t.Skip("private fixtures unavailable")
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		var wrapper struct {
			Data []json.RawMessage `json:"data"`
		}
		if err = json.Unmarshal(data, &wrapper); err != nil {
			t.Fatal(err)
		}
		for _, raw := range wrapper.Data {
			var artifact Artifact
			if err = json.Unmarshal(raw, &artifact); err != nil {
				t.Fatal(err)
			}
			if artifact.ID == "" || artifact.Ecosystem == "" {
				t.Fatal("real artifact shape not decoded")
			}
		}
	})
	for _, raw := range []string{"https://socket.dev.evil.test/x", "javascript:alert(1)", "http://socket.dev/x", "https://socket.dev@evil.test"} {
		if DashboardURL(raw) != "" {
			t.Fatal("unsafe dashboard accepted")
		}
	}
	if NormalizeSCM("git@github.com:Fixture/Repo.git") != NormalizeSCM("https://github.com/fixture/repo") {
		t.Fatal("SCM normalization")
	}
	if _, err := cleanFile("../../secret"); err == nil {
		t.Fatal("unsafe archive path accepted")
	}
}

func TestInterruptedStreamRetriesWithoutDuplicateArtifacts(t *testing.T) {
	var attempts atomic.Int32
	client := fixtureClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/metadata"):
			io.WriteString(w, `{}`)
		case strings.HasSuffix(r.URL.Path, "/files/tar"):
			tw := tar.NewWriter(w)
			tw.Close()
		default:
			if attempts.Add(1) == 1 {
				w.Header().Set("Content-Length", "1000")
			}
			io.WriteString(w, `{"id":"one","type":"npm","name":"test","manifestFiles":[]}`+"\n")
		}
	}))
	snapshot, err := client.Snapshot(context.Background(), "fixture", "token", "scan")
	if err != nil || attempts.Load() != 2 || len(snapshot.Artifacts) != 1 {
		t.Fatalf("retry result: %v, attempts %d, artifacts %d", err, attempts.Load(), len(snapshot.Artifacts))
	}
}
