package snyk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/socket"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type fixtureTransport struct{ target *url.URL }

func (t fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.URL.Scheme = t.target.Scheme
	copy.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(copy)
}
func fixture(t *testing.T, fail *atomic.Bool) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token fixture-secret" && r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Error("missing authorization")
			w.WriteHeader(401)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/targets"):
			fmt.Fprint(w, `{"data":[{"id":"target-1","attributes":{"display_name":"owner/repo","url":"https://github.com/owner/repo"}},{"id":"target-2","attributes":{"display_name":"owner/other","url":"https://github.com/owner/other"}}],"links":{}}`)
		case strings.HasSuffix(r.URL.Path, "/projects"):
			if r.URL.Query().Get("cursor") == "two" {
				fmt.Fprint(w, `{"data":[{"id":"project-2","attributes":{"name":"second manifest","type":"npm","target_file":"nested/package.json","target_reference":"main"},"relationships":{"target":{"data":{"id":"target-1"}}}},{"id":"project-3","attributes":{"name":"other repo","type":"npm","target_file":"package.json"},"relationships":{"target":{"data":{"id":"target-2"}}}}],"links":{}}`)
			} else {
				fmt.Fprintf(w, `{"data":[{"id":"project-1","attributes":{"name":"first manifest","type":"npm","target_file":"package.json","target_reference":"main"},"relationships":{"target":{"data":{"id":"target-1"}}}}],"links":{"next":%q}}`, r.URL.Path+"?version="+apiVersion+"&cursor=two")
			}
		case strings.HasSuffix(r.URL.Path, "/dep-graph"):
			if r.Header.Get("Accept") != "application/json" {
				w.WriteHeader(http.StatusNotAcceptable)
				return
			}
			if fail.Load() {
				w.WriteHeader(403)
				return
			}
			fmt.Fprint(w, `{"depGraph":{"pkgManager":{"name":"npm"},"pkgs":[{"id":"root","info":{"name":"root","version":"1"}},{"id":"dep","info":{"name":"example","version":"2"}},{"id":"transitive","info":{"name":"child","version":"3"}}],"graph":{"rootNodeId":"root-node","nodes":[{"nodeId":"root-node","pkgId":"root","deps":[{"nodeId":"dep-node"}]},{"nodeId":"dep-node","pkgId":"dep","deps":[{"nodeId":"transitive-node"}]},{"nodeId":"transitive-node","pkgId":"transitive","deps":[]}]}}}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	client := NewClient()
	client.HTTP.Transport = fixtureTransport{target}
	return client
}
func TestClient(t *testing.T) {
	var fail atomic.Bool
	c := fixture(t, &fail)
	ctx := context.Background()
	repos, e := c.Repositories(ctx, uuid.NewString(), "us", "fixture-secret")
	if e != nil {
		t.Fatal(e)
	}
	if len(repos) != 2 || len(repos[0].Projects) != 2 {
		t.Fatalf("projects not grouped: %+v", repos)
	}
	snap, e := c.Snapshot(ctx, uuid.NewString(), "us", "fixture-secret", repos[0])
	if e != nil {
		t.Fatal(e)
	}
	if len(snap.Files) != 2 || len(snap.Artifacts) != 4 {
		t.Fatalf("lost files/packages: %+v", snap)
	}
	if !*snap.Artifacts[0].Direct || *snap.Artifacts[1].Direct {
		t.Fatal("wrong direct dependency classification")
	}
	if _, e = nextPage("https://evil.test/rest/orgs/org/projects", "/rest/orgs/org/projects"); e == nil {
		t.Fatal("accepted external pagination")
	}
	if _, e = nextPage("/rest/orgs/another/projects?cursor=x", "/rest/orgs/org/projects"); e == nil {
		t.Fatal("accepted pagination from another org")
	}
}
func TestServiceIntegration(t *testing.T) {
	ctx := context.Background()
	container, e := postgres.Run(ctx, "postgres:17", postgres.WithDatabase("snyk_test"), postgres.WithUsername("postgres"), postgres.WithPassword("postgres"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	dsn, e := container.ConnectionString(ctx, "sslmode=disable")
	if e != nil {
		t.Fatal(e)
	}
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for i := 0; i < 50; i++ {
		if db.Ping(ctx) == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if e = store.Migrate(dsn); e != nil {
		t.Fatal(e)
	}
	var tenant, other, cli string
	for i, dst := range []*string{&tenant, &other} {
		if e = db.QueryRow(ctx, `insert into tenants(slug,name)values($1,'Fixture')returning id`, fmt.Sprint(i)).Scan(dst); e != nil {
			t.Fatal(e)
		}
	}
	if e = db.QueryRow(ctx, `insert into repositories(tenant_id,name,url,default_branch)values($1,'CLI','git@github.com:owner/repo.git','main')returning id`, tenant).Scan(&cli); e != nil {
		t.Fatal(e)
	}
	vault, e := socket.NewVault(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if e != nil {
		t.Fatal(e)
	}
	var fail atomic.Bool
	s := Service{DB: db, Client: fixture(t, &fail), Vault: vault}
	token := "fixture-secret"
	org := uuid.NewString()
	if e = s.Save(ctx, tenant, org, "us", "token", &token, 60); e != nil {
		t.Fatal(e)
	}
	if e = s.Save(ctx, tenant, org, "us", "token", nil, 0); e != nil {
		t.Fatal(e)
	}
	cfg, e := s.Configuration(ctx, tenant)
	if e != nil || !cfg.TokenConfigured {
		t.Fatalf("config: %+v %v", cfg, e)
	}
	raw, _ := json.Marshal(cfg)
	if strings.Contains(string(raw), token) {
		t.Fatal("token exposed")
	}
	var encrypted []byte
	if e = db.QueryRow(ctx, `select token_cipher from snyk_connections where tenant_id=$1`, tenant).Scan(&encrypted); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(encrypted), token) {
		t.Fatal("token unencrypted")
	}
	if _, e = vault.Open(encrypted, "snyk:"+other); e == nil {
		t.Fatal("wrong tenant decrypted")
	}
	var socketConnection, socketRepo string
	if e = db.QueryRow(ctx, `insert into socket_connections(tenant_id,organization,token_cipher)values($1,'fixture',$2)returning id`, tenant, []byte("cipher")).Scan(&socketConnection); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow(ctx, `insert into socket_repositories(connection_id,remote_id,name,scm_url)values($1,'socket-other','Socket other','https://github.com/owner/other')returning id`, socketConnection).Scan(&socketRepo); e != nil {
		t.Fatal(e)
	}
	if e = s.Test(ctx, tenant); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Start(ctx, tenant); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Start(ctx, tenant); e != ErrConflict {
		t.Fatalf("duplicate run: %v", e)
	}
	s.workOnce(ctx)
	cfg, e = s.Configuration(ctx, tenant)
	if e != nil || cfg.LastRun == nil || cfg.LastRun.Status != "completed" || cfg.LastRun.Total != 2 {
		t.Fatalf("run: %+v %v", cfg.LastRun, e)
	}
	inv, e := s.Inventory(ctx, tenant, cli)
	if e != nil || len(inv.Sources) != 1 || !inv.Repository.Snyk || len(inv.Sources[0].Snapshot.Files) != 2 {
		t.Fatalf("inventory: %+v %v", inv, e)
	}
	if _, e = s.Inventory(ctx, other, cli); e == nil {
		t.Fatal("cross tenant inventory")
	}
	var otherCLI string
	if e = db.QueryRow(ctx, `insert into repositories(tenant_id,name,url,default_branch)values($1,'Later CLI','https://github.com/owner/other','main')returning id`, tenant).Scan(&otherCLI); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, `update socket_repositories set cli_repository_id=$1 where id=$2`, otherCLI, socketRepo); e != nil {
		t.Fatal(e)
	}
	otherInv, e := s.Inventory(ctx, tenant, otherCLI)
	if e != nil || len(otherInv.Sources) != 1 || !otherInv.Repository.Snyk || !otherInv.Repository.Socket {
		t.Fatalf("arrival order hid Snyk inventory: %+v %v", otherInv, e)
	}
	var duplicateCount int
	if e = db.QueryRow(ctx, `select count(*) from inventory_repositories where tenant_id=$1 and url like '%owner/other%'`, tenant).Scan(&duplicateCount); e != nil || duplicateCount != 1 {
		t.Fatalf("arrival order duplicated repository: %d %v", duplicateCount, e)
	}
	fail.Store(true)
	if _, e = s.Start(ctx, tenant); e != nil {
		t.Fatal(e)
	}
	s.workOnce(ctx)
	cfg, e = s.Configuration(ctx, tenant)
	if e != nil || cfg.LastRun.Status != "partial" {
		t.Fatalf("partial status: %+v %v", cfg.LastRun, e)
	}
	inv, e = s.Inventory(ctx, tenant, cli)
	if e != nil || inv.Sources[0].Snapshot == nil || inv.Sources[0].Error == "" {
		t.Fatal("failed import erased snapshot or error")
	}
	if e = s.Disconnect(ctx, tenant); e != nil {
		t.Fatal(e)
	}
	var remains bool
	if e = db.QueryRow(ctx, `select exists(select 1 from repositories where id=$1)`, cli).Scan(&remains); e != nil || !remains {
		t.Fatal("disconnect erased CLI")
	}
}
