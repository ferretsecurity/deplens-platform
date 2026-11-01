package app

import (
	"archive/tar"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/config"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type socketFixtureTransport struct{ target *url.URL }

func (t socketFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}
func socketFixtureApp(t *testing.T, slow bool) *App {
	t.Helper()
	ctx := context.Background()
	db, dsn := openTestDatabase(t)
	if err := store.Migrate(dsn); err != nil {
		t.Fatal(err)
	}
	boot, err := store.BootstrapDefaultTenant(ctx, db, store.BootstrapInput{OwnerEmail: "socket-admin@example.test", OwnerPassword: "fixture-password", OwnerDisplayName: "Fixture administrator"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `with viewer as (insert into users(display_name,primary_email)values('Fixture viewer','socket-viewer@example.test')returning id), identity as (insert into auth_identities(user_id,provider_id,subject,email,password_hash) select v.id,a.provider_id,'socket-viewer@example.test','socket-viewer@example.test',a.password_hash from viewer v cross join auth_identities a where a.email='socket-admin@example.test') insert into tenant_memberships(tenant_id,user_id,role)select $1,id,'viewer'from viewer`, boot.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `insert into repositories(tenant_id,name,url,default_branch)values($1,'Fixture CLI plus Socket','git@github.com:fixture/project-1.git','main')`, boot.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `insert into repositories(tenant_id,name,url,default_branch)select $1,'Bulk fixture '||i,'https://example.test/fixture/'||i,'main' from generate_series(1,105)i`, boot.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	application, err := New(config.Config{DatabaseURL: dsn, BlobBackend: "filesystem", BlobFilesystemRoot: t.TempDir(), AppBaseURL: "http://127.0.0.1:3100", SocketEncryptionKey: base64.StdEncoding.EncodeToString(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(application.DB.Close)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/repos") {
			repos := make([]map[string]string, 0, 12)
			for i := 1; i <= 12; i++ {
				repos = append(repos, map[string]string{"id": fmt.Sprint(i), "name": fmt.Sprintf("Fixture Socket project %02d", i), "head_full_scan_id": fmt.Sprintf("scan-%d", i), "default_branch": "main", "html_url": fmt.Sprintf("https://socket.dev/dashboard/org/fixture/repo/project-%d", i)})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": repos, "nextPage": nil})
			return
		}
		if slow {
			time.Sleep(900 * time.Millisecond)
		}
		if strings.HasSuffix(r.URL.Path, "/metadata") {
			scan := strings.Split(r.URL.Path, "/")[5]
			_ = json.NewEncoder(w).Encode(map[string]string{"branch": "main", "commit_hash": "abc123def456fixture", "integration_repo_url": "https://github.com/fixture/project-" + strings.TrimPrefix(scan, "scan-"), "html_report_url": "https://socket.dev/dashboard/org/fixture"})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/files/tar") {
			tw := tar.NewWriter(w)
			for _, name := range []string{"package.json", "pnpm-lock.yaml", "services/api/requirements.txt", "empty/package.json"} {
				_ = tw.WriteHeader(&tar.Header{Name: name, Size: 2, Mode: 0600})
				_, _ = tw.Write([]byte("{}"))
			}
			_ = tw.Close()
			return
		}
		io.WriteString(w, `{"id":"pkg-react","type":"npm","name":"react","version":"19.1.0","direct":true,"dev":false,"manifestFiles":[{"file":"package.json","start":{"line":12},"end":{"line":12}},{"file":"pnpm-lock.yaml"}]}`+"\n"+`{"id":"pkg-fastapi","type":"pypi","name":"fastapi","version":"0.115.0","direct":true,"dev":null,"manifestFiles":[{"file":"services/api/requirements.txt"}]}`+"\n"+`{"id":"pkg-unreferenced","type":"npm","namespace":"@fixture","name":"unreferenced","version":"1.0.0","dev":true,"manifestFiles":[]}`+"\n")
	}))
	t.Cleanup(upstream.Close)
	target, _ := url.Parse(upstream.URL)
	application.Socket.Client.HTTP.Transport = socketFixtureTransport{target}
	return application
}
func TestSocketHTTPIntegration(t *testing.T) {
	application := socketFixtureApp(t, false)
	server := httptest.NewServer(application.Handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	call := func(method, path, body, origin, content string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", content)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(data)
	}
	status, _ := call("GET", "/api/v1/integrations/socket", "", "", "application/json")
	if status != 401 {
		t.Fatalf("anonymous %d", status)
	}
	status, _ = call("POST", "/auth/login", `{"email":"socket-admin@example.test","password":"fixture-password"}`, "", "application/json")
	if status != 200 {
		t.Fatalf("login %d", status)
	}
	payload := `{"organization":"fixture","token":"disposable-fixture-token","interval_minutes":60}`
	status, _ = call("PUT", "/api/v1/integrations/socket", payload, "https://evil.test", "application/json")
	if status != 403 {
		t.Fatalf("cross origin %d", status)
	}
	status, _ = call("PUT", "/api/v1/integrations/socket", payload, "", "text/plain")
	if status != 415 {
		t.Fatalf("CSRF content type %d", status)
	}
	status, _ = call("PUT", "/api/v1/integrations/socket", payload, "http://127.0.0.1:3100", "application/json")
	if status != 200 {
		t.Fatalf("save %d", status)
	}
	status, body := call("GET", "/api/v1/integrations/socket", "", "", "application/json")
	if status != 200 || strings.Contains(body, "disposable-fixture-token") || strings.Contains(body, "token_cipher") {
		t.Fatalf("safe config response %d", status)
	}
	status, _ = call("POST", "/api/v1/integrations/socket/sync", "", "", "application/json")
	if status != 200 {
		t.Fatalf("sync %d", status)
	}
	workerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); application.Socket.Work(workerCtx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, body = call("GET", "/api/v1/integrations/socket/status", "", "", "application/json")
		if strings.Contains(body, `"status":"completed"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sync did not complete", body)
		}
		time.Sleep(50 * time.Millisecond)
	}
	var repositoryID string
	err := application.DB.QueryRow(context.Background(), `select id from socket_repositories where remote_id='12'`).Scan(&repositoryID)
	if err != nil {
		t.Fatal(err)
	}
	status, body = call("GET", "/api/v1/repositories/"+repositoryID+"/inventory", "", "", "application/json")
	if status != 200 || !strings.Contains(body, "empty/package.json") || !strings.Contains(body, "unreferenced") {
		t.Fatalf("inventory %d", status)
	}
	// Session still says owner. The endpoint must use the current database membership.
	_, err = application.DB.Exec(context.Background(), `update tenant_memberships set role='viewer' where role='owner'`)
	if err != nil {
		t.Fatal(err)
	}
	status, _ = call("GET", "/api/v1/integrations/socket", "", "", "application/json")
	if status != 403 {
		t.Fatalf("stale role accepted %d", status)
	}
	status, _ = call("GET", "/api/v1/repositories/"+repositoryID+"/inventory", "", "", "application/json")
	if status != 200 {
		t.Fatalf("viewer inventory %d", status)
	}
	status, _ = call("POST", "/auth/login", `{"email":"socket-viewer@example.test","password":"fixture-password"}`, "", "application/json")
	if status != 200 {
		t.Fatalf("viewer login %d", status)
	}
	status, _ = call("POST", "/api/v1/integrations/socket/sync", "", "", "application/json")
	if status != 403 {
		t.Fatalf("viewer sync %d", status)
	}
	_, err = application.DB.Exec(context.Background(), `delete from tenant_memberships where user_id in (select id from users where primary_email='socket-viewer@example.test')`)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/repositories", "/api/v1/dependencies", "/api/v1/repositories/" + repositoryID + "/inventory"} {
		status, _ = call("GET", path, "", "", "application/json")
		if status != 401 && status != 403 {
			t.Fatalf("revoked member read %s: %d", path, status)
		}
	}
}
func TestSocketBrowserFixture(t *testing.T) {
	if os.Getenv("SOCKET_BROWSER_FIXTURE") != "1" {
		t.Skip("opt-in isolated browser fixture")
	}
	application := socketFixtureApp(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go application.Socket.Work(ctx)
	t.Log("Synthetic Socket API fixture listening on 127.0.0.1:18081. PostgreSQL is disposable and separate from production.")
	server := &http.Server{Addr: "127.0.0.1:18081", Handler: application.Handler, ReadHeaderTimeout: 10 * time.Second}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
