package app

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/config"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/google/uuid"
)

func TestSnykHTTPAuthorization(t *testing.T) {
	db, dsn := openTestDatabase(t)
	if e := store.Migrate(dsn); e != nil {
		t.Fatal(e)
	}
	boot, e := store.BootstrapDefaultTenant(context.Background(), db, store.BootstrapInput{OwnerEmail: "snyk-owner@example.test", OwnerPassword: "fixture-password"})
	if e != nil {
		t.Fatal(e)
	}
	application, e := New(config.Config{DatabaseURL: dsn, BlobBackend: "filesystem", BlobFilesystemRoot: t.TempDir(), AppBaseURL: "http://localhost:3100", SnykEncryptionKey: base64.StdEncoding.EncodeToString(make([]byte, 32))})
	if e != nil {
		t.Fatal(e)
	}
	defer application.DB.Close()
	server := httptest.NewServer(application.Handler)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	call := func(method, path, body, origin, content string) (int, string) {
		t.Helper()
		r, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		r.Header.Set("Content-Type", content)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		resp, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(data)
	}
	if code, _ := call("GET", "/api/v1/integrations/snyk", "", "", "application/json"); code != 401 {
		t.Fatalf("anonymous: %d", code)
	}
	if code, _ := call("POST", "/auth/login", `{"email":"snyk-owner@example.test","password":"fixture-password"}`, "", "application/json"); code != 200 {
		t.Fatalf("login: %d", code)
	}
	body := `{"organization":"` + uuid.NewString() + `","region":"us","authentication":"token","token":"fixture-secret","interval_minutes":0}`
	if code, _ := call("PUT", "/api/v1/integrations/snyk", body, "https://evil.test", "application/json"); code != 403 {
		t.Fatalf("cross origin: %d", code)
	}
	if code, _ := call("PUT", "/api/v1/integrations/snyk", body, "", "text/plain"); code != 415 {
		t.Fatalf("content type: %d", code)
	}
	if code, _ := call("PUT", "/api/v1/integrations/snyk", body, "http://localhost:3100", "application/json"); code != 200 {
		t.Fatalf("save: %d", code)
	}
	if code, result := call("GET", "/api/v1/integrations/snyk", "", "", "application/json"); code != 200 || strings.Contains(result, "fixture-secret") || strings.Contains(result, "token_cipher") {
		t.Fatalf("configuration leaks token or failed: %d", code)
	}
	var repository string
	if e = db.QueryRow(context.Background(), `insert into repositories(tenant_id,name,url,default_branch)values($1,'Fixture','https://github.com/fixture/repo','main')returning id`, boot.TenantID).Scan(&repository); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(context.Background(), `update tenant_memberships set role='viewer' where tenant_id=$1`, boot.TenantID); e != nil {
		t.Fatal(e)
	}
	if code, _ := call("POST", "/api/v1/integrations/snyk/sync", "", "", "application/json"); code != 403 {
		t.Fatalf("stale role: %d", code)
	}
	if code, _ := call("GET", "/api/v1/repositories/"+repository+"/snyk-inventory", "", "", "application/json"); code != 200 {
		t.Fatalf("viewer inventory: %d", code)
	}
	if _, e = db.Exec(context.Background(), `delete from tenant_memberships where tenant_id=$1`, boot.TenantID); e != nil {
		t.Fatal(e)
	}
	if code, _ := call("GET", "/api/v1/repositories/"+repository+"/snyk-inventory", "", "", "application/json"); code != 403 {
		t.Fatalf("revoked member: %d", code)
	}
}
