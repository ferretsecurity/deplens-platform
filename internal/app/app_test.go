package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/config"
	"github.com/ferretsecurity/deplens-platform/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestNewInitializesBlobStoreForFilesystemBackend(t *testing.T) {
	_, databaseURL := openTestDatabase(t)

	cfg := config.Config{
		Mode:                   "self-hosted",
		DatabaseURL:            databaseURL,
		BlobBackend:            "filesystem",
		BlobFilesystemRoot:     t.TempDir(),
		BootstrapOwnerEmail:    "admin@example.com",
		BootstrapOwnerPassword: "change-me-now",
	}

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer application.DB.Close()

	if application.BlobStore == nil {
		t.Fatal("BlobStore = nil, want initialized store")
	}
}

func TestSessionSurvivesAppRestart(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)
	if err := store.Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	_, err := store.BootstrapDefaultTenant(ctx, db, store.BootstrapInput{
		OwnerEmail:    "admin@example.com",
		OwnerPassword: "change-me-now",
	})
	if err != nil {
		t.Fatalf("BootstrapDefaultTenant() error = %v", err)
	}

	cfg := config.Config{
		Mode:                   "self-hosted",
		DatabaseURL:            databaseURL,
		BlobBackend:            "filesystem",
		BlobFilesystemRoot:     t.TempDir(),
		BootstrapOwnerEmail:    "admin@example.com",
		BootstrapOwnerPassword: "change-me-now",
	}

	firstApp, err := New(cfg)
	if err != nil {
		t.Fatalf("New() first app error = %v", err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"change-me-now"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	firstApp.Handler.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginResp.Code, http.StatusOK)
	}
	cookies := loginResp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set a session cookie")
	}
	var storedToken string
	if err := db.QueryRow(ctx, "select token from http_sessions").Scan(&storedToken); err != nil {
		t.Fatalf("query stored session token error = %v", err)
	}
	if storedToken == cookies[0].Value {
		t.Fatal("stored session token matches browser cookie, want hashed token")
	}
	firstApp.DB.Close()

	secondApp, err := New(cfg)
	if err != nil {
		t.Fatalf("New() second app error = %v", err)
	}
	defer secondApp.DB.Close()

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	for _, cookie := range cookies {
		meReq.AddCookie(cookie)
	}
	meResp := httptest.NewRecorder()
	secondApp.Handler.ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusOK {
		t.Fatalf("/auth/me status after app restart = %d, want %d", meResp.Code, http.StatusOK)
	}
}

func TestRouteAuthAndAPIForwardsTokenItemPathsToTokenRouter(t *testing.T) {
	handler := routeAuthAndAPI(
		http.NotFoundHandler(),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}),
	)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tokens/token-1", bytes.NewBufferString(`{"label":"scanner","scopes":["scan:read"]}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
}

func TestRouteHealthAndAppBypassesSessionWrappedHandler(t *testing.T) {
	handler := routeHealthAndApp(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Set-Cookie", "session=created")
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if got := rr.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("Set-Cookie = %q, want empty header", got)
	}
}

func openTestDatabase(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

	if os.Getenv("DOCKER_HOST") == "" && exec.Command("docker", "version").Run() != nil {
		t.Skip("Docker is required for app integration tests")
	}

	ctx := context.Background()
	container, err := postgres.Run(
		ctx,
		"postgres:17",
		postgres.WithDatabase("deplens"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
	)
	if err != nil {
		t.Fatalf("postgres.Run() error = %v", err)
	}
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	connString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("ConnectionString() error = %v", err)
	}

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	t.Cleanup(pool.Close)

	deadline := time.Now().Add(10 * time.Second)
	for {
		err = pool.Ping(ctx)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Ping() error = %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	return pool, connString
}
