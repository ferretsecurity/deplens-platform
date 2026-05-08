package store

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestBootstrapDefaultTenantCreatesTenantOwnerAndAdminToken(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)

	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	input := BootstrapInput{
		OwnerEmail:     "admin@example.com",
		OwnerPassword:  "change-me-now",
		BootstrapToken: "bootstrap-token",
	}
	result, err := BootstrapDefaultTenant(ctx, db, input)
	if err != nil {
		t.Fatalf("BootstrapDefaultTenant() error = %v", err)
	}
	if result.TenantSlug != "default" {
		t.Fatalf("TenantSlug = %q, want %q", result.TenantSlug, "default")
	}
	if result.OwnerEmail != "admin@example.com" {
		t.Fatalf("OwnerEmail = %q, want admin@example.com", result.OwnerEmail)
	}
	if result.CreatedTokenPlaintext != "bootstrap-token" {
		t.Fatalf("CreatedTokenPlaintext = %q, want bootstrap-token", result.CreatedTokenPlaintext)
	}
}

func openTestDatabase(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

	if os.Getenv("DOCKER_HOST") == "" && exec.Command("docker", "version").Run() != nil {
		t.Skip("Docker is required for store integration tests")
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
