# Deplens Platform V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first usable `deplens-platform` release: a self-hostable Go service that accepts authenticated `deplens` scan uploads, stores each upload as an immutable artifact plus normalized metadata, and exposes authenticated APIs for scan history and scan-level overlays for both browser users and machine clients.

**Architecture:** Use a modular monolith with clear packages for config, HTTP, auth, storage, and scan orchestration. Persist tenant-scoped relational data in PostgreSQL, store raw scan payloads through a blob interface backed by the local filesystem for v1, and keep the first normalized read model intentionally small: tenants, users, auth providers, auth identities, tenant memberships, API tokens, projects, repositories, scans, and scan metadata overlays.

**Tech Stack:** Go 1.24, `net/http` with `ServeMux`, PostgreSQL, `pgx/v5`, `pressly/goose`, `github.com/alexedwards/scs/v2`, `github.com/alexedwards/scs/postgresstore`, `golang.org/x/crypto/argon2`, `testcontainers-go`, OpenAPI 3.1, Docker Compose

**Migration Strategy:** Use versioned SQL migrations in `db/migrations/` and apply them with `pressly/goose`. Do not keep a hand-rolled migration runner that reads a single SQL file. V1 should support the same migration directory for tests, local startup, and container deployment.

**Auth Strategy:** V1 supports two authentication modes. Browser users authenticate through a provider-backed login flow and receive an HTTP-only session cookie managed by `scs`. Machine clients authenticate with bearer API tokens. The first provider is `local`, using email/password hashed with Argon2id via a local wrapper around `golang.org/x/crypto/argon2` that stores self-describing encoded hashes. The core model must remain provider-neutral so future `oidc`, `saml`, and other enterprise auth methods can attach identities to the same internal users and memberships without changing session or authorization behavior. A single human user must be able to belong to multiple tenants and hold a different role in each tenant. Authorization stays intentionally simple in v1: tenant-scoped user roles are limited to `owner`, `admin`, and `viewer`, and API tokens use scopes (`scan:write`, `scan:read`, `scan:metadata:write`). Session auth must carry both the authenticated `user_id` and the currently selected `active_tenant_id`; if a user belongs to multiple tenants, the UI/API should let them list memberships and switch the active tenant without reauthenticating. First-run bootstrap must create the default tenant, the first owner user, a `local` auth provider, and the first owner identity, then allow that owner to mint API tokens for scanners and CI. V1 should also support inviting users into a tenant with default `viewer` access, with role changes handled later through simple admin workflows rather than a more complex policy engine.

---

### Task 1: Bootstrap the Go service and configuration model

**Files:**
- Create: `go.mod`
- Create: `cmd/deplens-platform/main.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/app/app.go`
- Create: `Makefile`
- Create: `docker-compose.yml`

- [ ] **Step 1: Write the failing configuration tests**

```go
package config

import (
	"testing"
)

func TestLoadUsesDefaultHTTPAddressAndTenantMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/deplens?sslmode=disable")
	t.Setenv("BLOB_BACKEND", "filesystem")
	t.Setenv("BLOB_FILESYSTEM_ROOT", "./var/blobs")
	t.Setenv("BOOTSTRAP_OWNER_EMAIL", "admin@example.com")
	t.Setenv("BOOTSTRAP_OWNER_PASSWORD", "change-me-now")
	t.Setenv("SESSION_COOKIE_SECRET", "replace-me")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("HTTPAddress = %q, want %q", cfg.HTTPAddress, ":8080")
	}
	if cfg.Mode != "self-hosted" {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, "self-hosted")
	}
}

func TestLoadFailsWithoutDatabaseURL(t *testing.T) {
	t.Setenv("BLOB_BACKEND", "filesystem")
	t.Setenv("BLOB_FILESYSTEM_ROOT", "./var/blobs")
	t.Setenv("BOOTSTRAP_OWNER_EMAIL", "admin@example.com")
	t.Setenv("BOOTSTRAP_OWNER_PASSWORD", "change-me-now")
	t.Setenv("SESSION_COOKIE_SECRET", "replace-me")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestLoad -v`
Expected: FAIL with `no required module provides package` or `undefined: Load`

- [ ] **Step 3: Write the minimal bootstrap implementation**

```go
module github.com/ferretsecurity/deplens-platform

go 1.24.0

require (
	github.com/jackc/pgx/v5 v5.7.6
)
```

```go
package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddress            string
	Mode                   string
	DatabaseURL            string
	BlobBackend            string
	BlobFilesystemRoot     string
	BootstrapOwnerEmail    string
	BootstrapOwnerPassword string
	SessionCookieSecret    string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:            getEnv("HTTP_ADDRESS", ":8080"),
		Mode:                   getEnv("DEPLOYMENT_MODE", "self-hosted"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		BlobBackend:            os.Getenv("BLOB_BACKEND"),
		BlobFilesystemRoot:     os.Getenv("BLOB_FILESYSTEM_ROOT"),
		BootstrapOwnerEmail:    os.Getenv("BOOTSTRAP_OWNER_EMAIL"),
		BootstrapOwnerPassword: os.Getenv("BOOTSTRAP_OWNER_PASSWORD"),
		SessionCookieSecret:    os.Getenv("SESSION_COOKIE_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.BlobBackend == "" {
		return Config{}, errors.New("BLOB_BACKEND is required")
	}
	if cfg.BlobBackend == "filesystem" && cfg.BlobFilesystemRoot == "" {
		return Config{}, errors.New("BLOB_FILESYSTEM_ROOT is required when BLOB_BACKEND=filesystem")
	}
	if cfg.BootstrapOwnerEmail == "" {
		return Config{}, errors.New("BOOTSTRAP_OWNER_EMAIL is required")
	}
	if cfg.BootstrapOwnerPassword == "" {
		return Config{}, errors.New("BOOTSTRAP_OWNER_PASSWORD is required")
	}
	if cfg.SessionCookieSecret == "" {
		return Config{}, errors.New("SESSION_COOKIE_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
```

```go
package app

import "github.com/ferretsecurity/deplens-platform/internal/config"

type App struct {
	Config config.Config
}

func New(cfg config.Config) *App {
	return &App{Config: cfg}
}
```

```go
package main

import (
	"log/slog"
	"os"

	"github.com/ferretsecurity/deplens-platform/internal/app"
	"github.com/ferretsecurity/deplens-platform/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	_ = app.New(cfg)
	logger.Info("deplens-platform configured", "mode", cfg.Mode, "http_address", cfg.HTTPAddress)
}
```

```make
test:
	go test ./...

run:
	go run ./cmd/deplens-platform

migrate-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up
```

```yaml
services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: deplens
      POSTGRES_PASSWORD: postgres
      POSTGRES_USER: postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data

volumes:
  postgres-data:
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestLoad -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/deplens-platform/main.go internal/config/config.go internal/config/config_test.go internal/app/app.go Makefile docker-compose.yml
git commit -m "chore: bootstrap service configuration"
```

### Task 2: Freeze the upload contract and OpenAPI surface

**Files:**
- Create: `api/openapi.yaml`
- Create: `internal/api/spec_test.go`
- Create: `internal/scans/payload.go`
- Modify: `go.mod`

- [ ] **Step 1: Write the failing API contract test**

```go
package api

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAPIDefinesScanUploadAndHistoryPaths(t *testing.T) {
	specBytes, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	spec := string(specBytes)
	for _, want := range []string{
		"/api/v1/scans:",
		"/api/v1/scans/{scan_id}:",
		"/api/v1/scans/{scan_id}/metadata:",
		"/api/v1/projects:",
		"/api/v1/repositories:",
	} {
		if !strings.Contains(spec, want) {
			t.Fatalf("OpenAPI spec missing path %q", want)
		}
	}
	if !strings.Contains(spec, "ScanUploadRequest") {
		t.Fatal("OpenAPI spec missing ScanUploadRequest schema")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api -run TestOpenAPIDefinesScanUploadAndHistoryPaths -v`
Expected: FAIL with `open ../../../api/openapi.yaml: no such file or directory`

- [ ] **Step 3: Write the minimal OpenAPI and payload model**

```go
require (
	github.com/jackc/pgx/v5 v5.7.6
	gopkg.in/yaml.v3 v3.0.1
)
```

```go
package scans

type UploadRequest struct {
	SchemaVersion string            `json:"schema_version"`
	Project       ProjectInput      `json:"project"`
	Repository    RepositoryInput   `json:"repository"`
	Source        SourceInput       `json:"source"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotation    string            `json:"annotation,omitempty"`
	Snapshot      SnapshotInput     `json:"snapshot"`
}

type ProjectInput struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type RepositoryInput struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	DefaultBranch string `json:"default_branch"`
}

type SourceInput struct {
	CommitSHA string `json:"commit_sha"`
	Ref       string `json:"ref"`
	ScannedAt string `json:"scanned_at"`
}

type SnapshotInput struct {
	Root      string           `json:"root"`
	Manifests []ManifestInput  `json:"manifests"`
	Warnings  []string         `json:"warnings,omitempty"`
}

type ManifestInput struct {
	Type            string            `json:"type"`
	Path            string            `json:"path"`
	HasDependencies *bool             `json:"has_dependencies"`
	Dependencies    []DependencyInput `json:"dependencies,omitempty"`
	Warnings        []string          `json:"warnings,omitempty"`
}

type DependencyInput struct {
	Raw        string   `json:"raw"`
	Name       string   `json:"name,omitempty"`
	Version    string   `json:"version,omitempty"`
	Constraint string   `json:"constraint,omitempty"`
	Section    string   `json:"section,omitempty"`
	Source     string   `json:"source,omitempty"`
	Extras     []string `json:"extras,omitempty"`
}
```

```yaml
openapi: 3.1.0
info:
  title: Deplens Platform API
  version: 0.1.0
servers:
  - url: http://localhost:8080
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
  schemas:
    ScanUploadRequest:
      type: object
      required: [schema_version, project, repository, source, snapshot]
      properties:
        schema_version:
          type: string
          const: v1alpha1
        project:
          $ref: "#/components/schemas/ProjectInput"
        repository:
          $ref: "#/components/schemas/RepositoryInput"
        source:
          $ref: "#/components/schemas/SourceInput"
        labels:
          type: object
          additionalProperties:
            type: string
        annotation:
          type: string
        snapshot:
          $ref: "#/components/schemas/SnapshotInput"
    ProjectInput:
      type: object
      required: [slug, name]
      properties:
        slug:
          type: string
        name:
          type: string
    RepositoryInput:
      type: object
      required: [slug, name, url, default_branch]
      properties:
        slug:
          type: string
        name:
          type: string
        url:
          type: string
        default_branch:
          type: string
    SourceInput:
      type: object
      required: [commit_sha, ref, scanned_at]
      properties:
        commit_sha:
          type: string
        ref:
          type: string
        scanned_at:
          type: string
          format: date-time
    SnapshotInput:
      type: object
      required: [root, manifests]
      properties:
        root:
          type: string
        manifests:
          type: array
          items:
            $ref: "#/components/schemas/ManifestInput"
        warnings:
          type: array
          items:
            type: string
    ManifestInput:
      type: object
      required: [type, path, has_dependencies]
      properties:
        type:
          type: string
        path:
          type: string
        has_dependencies:
          type: [boolean, "null"]
        dependencies:
          type: array
          items:
            $ref: "#/components/schemas/DependencyInput"
        warnings:
          type: array
          items:
            type: string
    DependencyInput:
      type: object
      required: [raw]
      properties:
        raw:
          type: string
        name:
          type: string
        version:
          type: string
        constraint:
          type: string
        section:
          type: string
        source:
          type: string
        extras:
          type: array
          items:
            type: string
security:
  - bearerAuth: []
paths:
  /api/v1/scans:
    post:
      summary: Upload a scan snapshot
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/ScanUploadRequest"
      responses:
        "201":
          description: Created
  /api/v1/scans/{scan_id}:
    get:
      summary: Get a scan by ID
      parameters:
        - in: path
          name: scan_id
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK
  /api/v1/scans/{scan_id}/metadata:
    patch:
      summary: Update scan labels and annotation
      parameters:
        - in: path
          name: scan_id
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK
  /api/v1/projects:
    get:
      summary: List projects for the current tenant
      responses:
        "200":
          description: OK
  /api/v1/repositories:
    get:
      summary: List repositories for the current tenant
      responses:
        "200":
          description: OK
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api -run TestOpenAPIDefinesScanUploadAndHistoryPaths -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add api/openapi.yaml internal/api/spec_test.go internal/scans/payload.go go.mod
git commit -m "feat: define initial upload contract and api surface"
```

### Task 3: Add the PostgreSQL schema, tenant bootstrap, user model, and token model

**Implementation note:** This task establishes the migration workflow and foundational auth model. Every schema change after `001_initial.sql` should be added as a new versioned SQL file and applied through `goose`, both in tests and at application startup. The schema must support both human users for the future web UI and machine API tokens for uploads and automation. Session persistence should use the schema expected by `scs/postgresstore` instead of a hand-rolled session table. Local email/password is only the first auth provider; the schema must leave room for future OIDC and SAML providers without changing the `users` or `tenant_memberships` model. Do not assume one user maps to one tenant. `tenant_memberships` is the source of truth for per-tenant roles, and later auth code should resolve the full membership set for a user instead of selecting a single tenant implicitly. V1 should also reserve room for tenant invites, even if the first pass is a minimal invite table and accept flow.

**Files:**
- Create: `db/migrations/001_initial.sql`
- Create: `internal/store/bootstrap.go`
- Create: `internal/store/bootstrap_test.go`
- Create: `internal/store/models.go`
- Modify: `go.mod`

- [ ] **Step 1: Write the failing bootstrap integration test**

```go
package store

import (
	"context"
	"testing"
)

func TestBootstrapDefaultTenantCreatesTenantOwnerAndAdminToken(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)

	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	input := BootstrapInput{
		OwnerEmail:    "admin@example.com",
		OwnerPassword: "change-me-now",
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
}
```

```go
package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func openTestDatabase(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

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

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	return pool, connString
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store -run TestBootstrapDefaultTenantCreatesTenantOwnerAndAdminToken -v`
Expected: FAIL with `undefined: BootstrapDefaultTenant` or missing migration file

- [ ] **Step 3: Write the schema and bootstrap implementation**

```go
require (
	github.com/jackc/pgx/v5 v5.7.6
	github.com/jackc/pgx/v5/stdlib v5.7.6
	github.com/pressly/goose/v3 v3.24.3
	github.com/testcontainers/testcontainers-go v0.39.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.39.0
	gopkg.in/yaml.v3 v3.0.1
)
```

```sql
create extension if not exists pgcrypto;

create table if not exists tenants (
    id uuid primary key default gen_random_uuid(),
    slug text not null unique,
    name text not null,
    created_at timestamptz not null default now()
);

create table if not exists users (
    id uuid primary key default gen_random_uuid(),
    display_name text not null,
    primary_email text,
    created_at timestamptz not null default now()
);

create table if not exists auth_providers (
    id uuid primary key default gen_random_uuid(),
    provider_type text not null check (provider_type in ('local', 'oidc', 'saml')),
    slug text not null unique,
    display_name text not null,
    issuer text,
    metadata_url text,
    saml_metadata xml,
    is_enabled boolean not null default true,
    created_at timestamptz not null default now()
);

create table if not exists auth_identities (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id) on delete cascade,
    provider_id uuid not null references auth_providers(id) on delete cascade,
    subject text not null,
    email text,
    password_hash text,
    created_at timestamptz not null default now(),
    unique (provider_id, subject)
);

create table if not exists tenant_memberships (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    user_id uuid not null references users(id) on delete cascade,
    role text not null check (role in ('owner', 'admin', 'viewer')),
    created_at timestamptz not null default now(),
    unique (tenant_id, user_id)
);

create table if not exists tenant_invites (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    email text not null,
    role text not null check (role in ('owner', 'admin', 'viewer')),
    invite_token_hash text not null unique,
    invited_by_user_id uuid not null references users(id) on delete cascade,
    accepted_at timestamptz,
    expires_at timestamptz not null,
    created_at timestamptz not null default now()
);

create unique index if not exists tenant_invites_active_email_idx
    on tenant_invites (tenant_id, email)
    where accepted_at is null;

create table if not exists http_sessions (
    token text primary key,
    data bytea not null,
    expiry timestamptz not null
);

create index if not exists http_sessions_expiry_idx
    on http_sessions (expiry);

create table if not exists api_tokens (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    label text not null,
    token_hash text not null unique,
    scopes text[] not null,
    created_at timestamptz not null default now()
);

create table if not exists projects (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    slug text not null,
    name text not null,
    created_at timestamptz not null default now(),
    unique (tenant_id, slug)
);

create table if not exists repositories (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    project_id uuid not null references projects(id) on delete cascade,
    slug text not null,
    name text not null,
    url text not null,
    default_branch text not null,
    created_at timestamptz not null default now(),
    unique (tenant_id, project_id, slug)
);

create table if not exists scans (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    project_id uuid not null references projects(id) on delete cascade,
    repository_id uuid not null references repositories(id) on delete cascade,
    artifact_key text not null unique,
    artifact_sha256 text not null,
    schema_version text not null,
    root_path text not null,
    commit_sha text not null,
    source_ref text not null,
    scanned_at timestamptz not null,
    manifest_count integer not null,
    manifests_with_dependencies_count integer not null,
    manifests_without_dependencies_count integer not null,
    manifests_unknown_count integer not null,
    dependency_count integer not null,
    labels jsonb not null default '{}'::jsonb,
    annotation text not null default '',
    created_at timestamptz not null default now()
);

create index if not exists scans_tenant_repository_scanned_at_idx
    on scans (tenant_id, repository_id, scanned_at desc);
```

```go
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type BootstrapResult struct {
	TenantID   string
	TenantSlug string
	OwnerEmail string
}

type BootstrapInput struct {
	OwnerEmail       string
	OwnerPassword    string
	OwnerDisplayName string
}

func Migrate(databaseURL string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()

	if err := goose.Up(db, "../../db/migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func BootstrapDefaultTenant(ctx context.Context, db *pgxpool.Pool, input BootstrapInput) (BootstrapResult, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return BootstrapResult{}, err
	}
	defer tx.Rollback(ctx)

	var tenantID string
	err = tx.QueryRow(ctx, `
		insert into tenants (slug, name)
		values ('default', 'Default Tenant')
		on conflict (slug) do update set name = excluded.name
		returning id
	`).Scan(&tenantID)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("upsert tenant: %w", err)
	}

	passwordHash, err := hashPassword(input.OwnerPassword)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("hash owner password: %w", err)
	}

	var userID string
	err = tx.QueryRow(ctx, `
		insert into users (display_name, primary_email)
		values ($1, $2)
		on conflict (primary_email) do update set display_name = excluded.display_name
		returning id
	`, input.OwnerDisplayName, input.OwnerEmail).Scan(&userID)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("upsert owner user: %w", err)
	}

	var providerID string
	err = tx.QueryRow(ctx, `
		insert into auth_providers (provider_type, slug, display_name)
		values ('local', 'local', 'Local Password')
		on conflict (slug) do update set display_name = excluded.display_name
		returning id
	`).Scan(&providerID)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("upsert local auth provider: %w", err)
	}

	_, err = tx.Exec(ctx, `
		insert into auth_identities (user_id, provider_id, subject, email, password_hash)
		values ($1, $2, $3, $4, $5)
		on conflict (provider_id, subject) do nothing
	`, userID, providerID, input.OwnerEmail, input.OwnerEmail, passwordHash)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("insert local auth identity: %w", err)
	}

	_, err = tx.Exec(ctx, `
		insert into tenant_memberships (tenant_id, user_id, role)
		values ($1, $2, 'owner')
		on conflict (tenant_id, user_id) do nothing
	`, tenantID, userID)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("insert tenant membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return BootstrapResult{}, err
	}

	return BootstrapResult{
		TenantID:   tenantID,
		TenantSlug: "default",
		OwnerEmail: input.OwnerEmail,
	}, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) (string, error) {
	// Implement Argon2id hashing using golang.org/x/crypto/argon2 with a random salt
	// and an encoded hash format that stores parameters alongside the hash so values
	// can be verified and upgraded over time.
	return password, nil
}
```

```go
package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenRecord struct {
	TenantID string
	Scopes   []string
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store -run TestBootstrapDefaultTenantCreatesTenantOwnerAndAdminToken -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add db/migrations/001_initial.sql internal/store/bootstrap.go internal/store/bootstrap_test.go internal/store/models.go go.mod
git commit -m "feat: add tenant bootstrap and relational schema"
```

### Task 4: Implement blob storage and immutable artifact persistence

**Files:**
- Create: `internal/blob/blob.go`
- Create: `internal/blob/filesystem.go`
- Create: `internal/blob/filesystem_test.go`
- Create: `internal/scans/summary.go`

- [ ] **Step 1: Write the failing blob storage test**

```go
package blob

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSystemStorePutPersistsBytesAtArtifactKey(t *testing.T) {
	root := t.TempDir()
	store := NewFileSystemStore(root)

	key, sha, err := store.Put(context.Background(), []byte(`{"root":".","manifests":[]}`))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if key == "" {
		t.Fatal("Put() key = empty, want non-empty")
	}
	if sha == "" {
		t.Fatal("Put() sha = empty, want non-empty")
	}

	if _, err := os.Stat(filepath.Join(root, key)); err != nil {
		t.Fatalf("stored artifact missing: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/blob -run TestFileSystemStorePutPersistsBytesAtArtifactKey -v`
Expected: FAIL with `undefined: NewFileSystemStore`

- [ ] **Step 3: Write the blob interface and summary helper**

```go
package blob

import "context"

type Store interface {
	Put(ctx context.Context, payload []byte) (artifactKey string, sha256 string, err error)
}
```

```go
package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

type FileSystemStore struct {
	root string
}

func NewFileSystemStore(root string) *FileSystemStore {
	return &FileSystemStore{root: root}
}

func (s *FileSystemStore) Put(_ context.Context, payload []byte) (string, string, error) {
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	key := filepath.Join(hash[:2], hash[2:4], hash+".json")
	fullPath := filepath.Join(s.root, key)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(fullPath, payload, 0o644); err != nil {
		return "", "", err
	}

	return key, hash, nil
}
```

```go
package scans

type Summary struct {
	ManifestCount                  int
	ManifestsWithDependenciesCount int
	ManifestsWithoutDependencies   int
	ManifestsUnknownCount          int
	DependencyCount                int
}

func BuildSummary(input UploadRequest) Summary {
	summary := Summary{
		ManifestCount: len(input.Snapshot.Manifests),
	}

	for _, manifest := range input.Snapshot.Manifests {
		switch {
		case manifest.HasDependencies == nil:
			summary.ManifestsUnknownCount++
		case *manifest.HasDependencies:
			summary.ManifestsWithDependenciesCount++
		default:
			summary.ManifestsWithoutDependencies++
		}
		summary.DependencyCount += len(manifest.Dependencies)
	}

	return summary
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/blob -run TestFileSystemStorePutPersistsBytesAtArtifactKey -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/blob/blob.go internal/blob/filesystem.go internal/blob/filesystem_test.go internal/scans/summary.go
git commit -m "feat: add filesystem artifact storage"
```

### Task 5: Implement user auth, SCS-backed session handling, bearer-token auth, and the scan upload workflow

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/passwords.go`
- Create: `internal/auth/sessions.go`
- Create: `internal/http/auth_handlers.go`
- Create: `internal/http/auth_handlers_test.go`
- Create: `internal/http/router.go`
- Create: `internal/http/upload_handler.go`
- Create: `internal/http/upload_handler_test.go`
- Create: `internal/scans/service.go`
- Create: `internal/store/uploads.go`
- Modify: `internal/store/models.go`

- [ ] **Step 1: Write the failing auth and upload handler tests**

```go
package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadScanReturnsCreatedForValidBearerToken(t *testing.T) {
	handler := NewRouter(fakeUploadService{
		allowedToken: "bootstrap-token",
	})

	body := []byte(`{
	  "schema_version": "v1alpha1",
	  "project": {"slug":"core","name":"Core"},
	  "repository": {"slug":"repo","name":"Repo","url":"https://example.com/repo.git","default_branch":"main"},
	  "source": {"commit_sha":"abc123","ref":"refs/heads/main","scanned_at":"2026-05-04T10:00:00Z"},
	  "snapshot": {"root":".","manifests":[]}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestLoginSetsSessionCookieForValidUser(t *testing.T) {
	handler := NewAuthRouter(fakeAuthService{})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"admin@example.com","password":"change-me-now"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Fatal("expected session cookie to be set")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/http -run 'TestUploadScanReturnsCreatedForValidBearerToken|TestLoginSetsSessionCookieForValidUser' -v`
Expected: FAIL with `undefined: NewRouter` or `undefined: NewAuthRouter`

- [ ] **Step 3: Write the auth, session, and upload path**

Implementation notes:
- Use `scs/v2` as the session manager and `postgresstore` as the backing store.
- Store authenticated principal context in the session data, at minimum `user_id`, `active_tenant_id`, and `role`.
- On successful login, resolve all tenant memberships for the user. If exactly one membership exists, set it as `active_tenant_id`. If multiple memberships exist, return them in the login response and require the client to select or confirm the active tenant.
- Regenerate the session token on successful login.
- Configure the session cookie as `HttpOnly`, `Secure` in non-local environments, and `SameSite=Lax` by default.
- Keep upload endpoints bearer-token only; allow read endpoints to accept either session auth or API tokens.
- Model local email/password login as one auth provider implementation. The service boundary should be shaped so future OIDC and SAML handlers can resolve or provision `auth_identities` without changing the rest of the request pipeline.
- Keep the authorization surface intentionally simple in v1: `owner`, `admin`, and `viewer` only. Add tenant invite and membership listing/switching flows before adding more granular permissions.

```go
package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenRecord struct {
	TenantID string
	Scopes   []string
}

type UploadScanParams struct {
	TenantID             string
	ProjectSlug          string
	ProjectName          string
	RepositorySlug       string
	RepositoryName       string
	URL                  string
	DefaultBranch        string
	ArtifactKey          string
	ArtifactSHA256       string
	SchemaVersion        string
	RootPath             string
	CommitSHA            string
	SourceRef            string
	ScannedAt            time.Time
	ManifestCount        int
	WithDependencies     int
	WithoutDependencies  int
	UnknownDependencies  int
	DependencyCount      int
	Labels               map[string]string
	Annotation           string
}

type Store struct {
	DB *pgxpool.Pool
}

func (s Store) FindToken(ctx context.Context, tokenHash string) (string, []string, error) {
	var record TokenRecord
	err := s.DB.QueryRow(ctx, `
		select tenant_id, scopes
		from api_tokens
		where token_hash = $1
	`, tokenHash).Scan(&record.TenantID, &record.Scopes)
	if err != nil {
		return "", nil, err
	}
	return record.TenantID, record.Scopes, nil
}

func (s Store) FindLocalIdentityByEmail(ctx context.Context, email string) (string, string, error) {
	var userID string
	var passwordHash string
	err := s.DB.QueryRow(ctx, `
		select u.id, ai.password_hash
		from auth_identities ai
		join auth_providers ap on ap.id = ai.provider_id
		join users u on u.id = ai.user_id
		where ap.slug = 'local' and ai.email = $1
		limit 1
	`, email).Scan(&userID, &passwordHash)
	if err != nil {
		return "", "", err
	}
	return userID, passwordHash, nil
}

func (s Store) CreateScan(ctx context.Context, params UploadScanParams) (string, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var projectID string
	err = tx.QueryRow(ctx, `
		insert into projects (tenant_id, slug, name)
		values ($1, $2, $3)
		on conflict (tenant_id, slug) do update set name = excluded.name
		returning id
	`, params.TenantID, params.ProjectSlug, params.ProjectName).Scan(&projectID)
	if err != nil {
		return "", err
	}

	var repositoryID string
	err = tx.QueryRow(ctx, `
		insert into repositories (tenant_id, project_id, slug, name, url, default_branch)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (tenant_id, project_id, slug)
		do update set name = excluded.name, url = excluded.url, default_branch = excluded.default_branch
		returning id
	`, params.TenantID, projectID, params.RepositorySlug, params.RepositoryName, params.URL, params.DefaultBranch).Scan(&repositoryID)
	if err != nil {
		return "", err
	}

	labelsJSON, err := json.Marshal(params.Labels)
	if err != nil {
		return "", err
	}

	var scanID string
	err = tx.QueryRow(ctx, `
		insert into scans (
			tenant_id, project_id, repository_id, artifact_key, artifact_sha256, schema_version,
			root_path, commit_sha, source_ref, scanned_at, manifest_count,
			manifests_with_dependencies_count, manifests_without_dependencies_count,
			manifests_unknown_count, dependency_count, labels, annotation
		)
		values (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16::jsonb, $17
		)
		returning id
	`, params.TenantID, projectID, repositoryID, params.ArtifactKey, params.ArtifactSHA256, params.SchemaVersion, params.RootPath, params.CommitSHA, params.SourceRef, params.ScannedAt, params.ManifestCount, params.WithDependencies, params.WithoutDependencies, params.UnknownDependencies, params.DependencyCount, string(labelsJSON), params.Annotation).Scan(&scanID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return scanID, nil
}
```

```go
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type TokenLookup interface {
	FindToken(ctx context.Context, tokenHash string) (tenantID string, scopes []string, err error)
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func BearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}
```

```go
package scans

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type Repository interface {
	CreateScan(ctx context.Context, params store.UploadScanParams) (string, error)
}

type Service struct {
	Blob  blob.Store
	Store Repository
}

func (s Service) Upload(ctx context.Context, tenantID string, input UploadRequest) (string, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	artifactKey, artifactSHA, err := s.Blob.Put(ctx, payload)
	if err != nil {
		return "", err
	}

	summary := BuildSummary(input)
	scannedAt, err := time.Parse(time.RFC3339, input.Source.ScannedAt)
	if err != nil {
		return "", err
	}

	return s.Store.CreateScan(ctx, store.UploadScanParams{
		TenantID:            tenantID,
		ProjectSlug:         input.Project.Slug,
		ProjectName:         input.Project.Name,
		RepositorySlug:      input.Repository.Slug,
		RepositoryName:      input.Repository.Name,
		URL:                 input.Repository.URL,
		DefaultBranch:       input.Repository.DefaultBranch,
		ArtifactKey:         artifactKey,
		ArtifactSHA256:      artifactSHA,
		SchemaVersion:       input.SchemaVersion,
		RootPath:            input.Snapshot.Root,
		CommitSHA:           input.Source.CommitSHA,
		SourceRef:           input.Source.Ref,
		ScannedAt:           scannedAt,
		ManifestCount:       summary.ManifestCount,
		WithDependencies:    summary.ManifestsWithDependenciesCount,
		WithoutDependencies: summary.ManifestsWithoutDependencies,
		UnknownDependencies: summary.ManifestsUnknownCount,
		DependencyCount:     summary.DependencyCount,
		Labels:              input.Labels,
		Annotation:          input.Annotation,
	})
}
```

```go
package httpapi

import (
	"context"
	"errors"
	"encoding/json"
	"net/http"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
)

var errUnauthorized = errors.New("unauthorized")

type UploadService interface {
	Upload(r *http.Request, token string, input scans.UploadRequest) (string, error)
}

type AuthService interface {
	Login(r *http.Request, email string, password string) (sessionToken string, err error)
}

type fakeUploadService struct {
	allowedToken string
}

type fakeAuthService struct{}

func (fakeAuthService) Login(_ *http.Request, email string, password string) (string, error) {
	if email != "admin@example.com" || password != "change-me-now" {
		return "", errUnauthorized
	}
	return "session-token", nil
}

func (f fakeUploadService) Upload(_ *http.Request, token string, _ scans.UploadRequest) (string, error) {
	if token != f.allowedToken {
		return "", errUnauthorized
	}
	return "scan-123", nil
}

type TokenLookup interface {
	FindToken(ctx context.Context, tokenHash string) (tenantID string, scopes []string, err error)
}

type UploadProcessor interface {
	Upload(ctx context.Context, tenantID string, input scans.UploadRequest) (string, error)
}

type ProductionUploadService struct {
	Lookup  TokenLookup
	Uploads UploadProcessor
}

func NewProductionUploadService(lookup TokenLookup, uploads UploadProcessor) ProductionUploadService {
	return ProductionUploadService{
		Lookup:  lookup,
		Uploads: uploads,
	}
}

func (s ProductionUploadService) Upload(r *http.Request, token string, input scans.UploadRequest) (string, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil {
		return "", errUnauthorized
	}
	if !hasScope(scopes, "scan:write") {
		return "", errUnauthorized
	}
	return s.Uploads.Upload(r.Context(), tenantID, input)
}

func NewAuthRouter(service AuthService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		sessionToken, err := service.Login(r, payload.Email, payload.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// In the production implementation, delegate cookie issuance and persistence
		// to scs. The handler should authenticate the user, renew the session token,
		// and store principal fields in the server-side session before returning 200.
		http.SetCookie(w, &http.Cookie{
			Name:     "deplens_session",
			Value:    sessionToken,
			HttpOnly: true,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// Read APIs should accept either a valid session cookie or a bearer API token.
// Upload APIs should accept bearer API tokens only.

func NewRouter(service UploadService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/scans", func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		var input scans.UploadRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		scanID, err := service.Upload(r, token, input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"scan_id": scanID})
	})
	return mux
}

func hasScope(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/http -run TestUploadScanReturnsCreatedForValidBearerToken -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/auth.go internal/http/router.go internal/http/upload_handler.go internal/http/upload_handler_test.go internal/scans/service.go internal/store/models.go internal/store/uploads.go
git commit -m "feat: add authenticated scan upload workflow"
```

### Task 6: Implement scan history queries and scan-level metadata overlays

**Implementation note:** Read and metadata endpoints should continue to authorize against the current tenant context, whether it comes from an API token or a browser session. Add a minimal tenant membership flow in this phase or the next adjacent task: `GET /auth/me` should return the authenticated user plus tenant memberships, and the API should expose a simple way to switch `active_tenant_id` in the session. Tenant invite flows should default invited users to `viewer`, with later manual role edits through admin APIs or SQL-backed operator tooling.

**Files:**
- Create: `internal/http/query_handlers.go`
- Create: `internal/http/query_handlers_test.go`
- Create: `internal/store/scans.go`
- Modify: `api/openapi.yaml`

- [ ] **Step 1: Write the failing query and metadata tests**

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListScansSupportsRepositoryAndTimeFilters(t *testing.T) {
	handler := NewQueryRouter(fakeQueryService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans?repository_slug=repo&from=2026-05-01T00:00:00Z&to=2026-05-05T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestGetScanReturnsOK(t *testing.T) {
	handler := NewQueryRouter(fakeQueryService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scans/scan-123", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestPatchScanMetadataReturnsOK(t *testing.T) {
	handler := NewQueryRouter(fakeQueryService{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/scans/scan-123/metadata", strings.NewReader(`{"labels":{"env":"prod"},"annotation":"blessed build"}`))
	req.Header.Set("Authorization", "Bearer bootstrap-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/http -run 'TestListScansSupportsRepositoryAndTimeFilters|TestGetScanReturnsOK|TestPatchScanMetadataReturnsOK' -v`
Expected: FAIL with `undefined: NewQueryRouter`

- [ ] **Step 3: Write the query store and HTTP handlers**

```go
package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ScanListItem struct {
	ID              string            `json:"id"`
	ProjectSlug     string            `json:"project_slug"`
	RepositorySlug  string            `json:"repository_slug"`
	CommitSHA       string            `json:"commit_sha"`
	ScannedAt       time.Time         `json:"scanned_at"`
	ManifestCount   int               `json:"manifest_count"`
	DependencyCount int               `json:"dependency_count"`
	Labels          map[string]string `json:"labels"`
	Annotation      string            `json:"annotation"`
}

type ProjectListItem struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type RepositoryListItem struct {
	ProjectSlug    string `json:"project_slug"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	DefaultBranch  string `json:"default_branch"`
}

type ScanFilter struct {
	TenantID       string
	RepositorySlug string
	From           time.Time
	To             time.Time
}

type ScanStore struct {
	DB *pgxpool.Pool
}

func (s ScanStore) ListProjects(ctx context.Context, tenantID string) ([]ProjectListItem, error) {
	rows, err := s.DB.Query(ctx, `
		select slug, name
		from projects
		where tenant_id = $1
		order by slug asc
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ProjectListItem, 0)
	for rows.Next() {
		var item ProjectListItem
		if err := rows.Scan(&item.Slug, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s ScanStore) ListRepositories(ctx context.Context, tenantID string) ([]RepositoryListItem, error) {
	rows, err := s.DB.Query(ctx, `
		select p.slug, r.slug, r.name, r.url, r.default_branch
		from repositories r
		join projects p on p.id = r.project_id
		where r.tenant_id = $1
		order by p.slug asc, r.slug asc
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RepositoryListItem, 0)
	for rows.Next() {
		var item RepositoryListItem
		if err := rows.Scan(&item.ProjectSlug, &item.Slug, &item.Name, &item.URL, &item.DefaultBranch); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s ScanStore) ListScans(ctx context.Context, filter ScanFilter) ([]ScanListItem, error) {
	rows, err := s.DB.Query(ctx, `
		select s.id, p.slug, r.slug, s.commit_sha, s.scanned_at, s.manifest_count, s.dependency_count, s.labels, s.annotation
		from scans s
		join projects p on p.id = s.project_id
		join repositories r on r.id = s.repository_id
		where s.tenant_id = $1 and r.slug = $2 and s.scanned_at between $3 and $4
		order by s.scanned_at desc
	`, filter.TenantID, filter.RepositorySlug, filter.From, filter.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ScanListItem, 0)
	for rows.Next() {
		var item ScanListItem
		var labelsJSON []byte
		if err := rows.Scan(&item.ID, &item.ProjectSlug, &item.RepositorySlug, &item.CommitSHA, &item.ScannedAt, &item.ManifestCount, &item.DependencyCount, &labelsJSON, &item.Annotation); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s ScanStore) GetScan(ctx context.Context, tenantID string, scanID string) (ScanListItem, error) {
	var item ScanListItem
	var labelsJSON []byte
	err := s.DB.QueryRow(ctx, `
		select s.id, p.slug, r.slug, s.commit_sha, s.scanned_at, s.manifest_count, s.dependency_count, s.labels, s.annotation
		from scans s
		join projects p on p.id = s.project_id
		join repositories r on r.id = s.repository_id
		where s.tenant_id = $1 and s.id = $2
	`, tenantID, scanID).Scan(&item.ID, &item.ProjectSlug, &item.RepositorySlug, &item.CommitSHA, &item.ScannedAt, &item.ManifestCount, &item.DependencyCount, &labelsJSON, &item.Annotation)
	if err != nil {
		return ScanListItem{}, err
	}
	if err := json.Unmarshal(labelsJSON, &item.Labels); err != nil {
		return ScanListItem{}, err
	}
	return item, nil
}

func (s ScanStore) UpdateScanMetadata(ctx context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error {
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(ctx, `
		update scans
		set labels = $3::jsonb, annotation = $4
		where tenant_id = $1 and id = $2
	`, tenantID, scanID, string(labelsJSON), annotation)
	return err
}
```

```go
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ferretsecurity/deplens-platform/internal/auth"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type QueryService interface {
	ListProjects(r *http.Request, token string) (any, error)
	ListRepositories(r *http.Request, token string) (any, error)
	ListScans(r *http.Request, token string) (any, error)
	GetScan(r *http.Request, token string) (any, error)
	UpdateScanMetadata(r *http.Request, token string) error
}

type QueryStore interface {
	ListProjects(ctx context.Context, tenantID string) ([]store.ProjectListItem, error)
	ListRepositories(ctx context.Context, tenantID string) ([]store.RepositoryListItem, error)
	ListScans(ctx context.Context, filter store.ScanFilter) ([]store.ScanListItem, error)
	GetScan(ctx context.Context, tenantID string, scanID string) (store.ScanListItem, error)
	UpdateScanMetadata(ctx context.Context, tenantID string, scanID string, labels map[string]string, annotation string) error
}

type fakeQueryService struct{}

func (fakeQueryService) ListProjects(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return []map[string]string{{"slug": "core", "name": "Core"}}, nil
}

func (fakeQueryService) ListRepositories(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return []map[string]string{{"project_slug": "core", "slug": "repo", "name": "Repo"}}, nil
}

func (fakeQueryService) ListScans(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return []map[string]string{{"id": "scan-123"}}, nil
}

func (fakeQueryService) GetScan(_ *http.Request, token string) (any, error) {
	if token != "bootstrap-token" {
		return nil, errUnauthorized
	}
	return map[string]string{"id": "scan-123"}, nil
}

func (fakeQueryService) UpdateScanMetadata(_ *http.Request, token string) error {
	if token != "bootstrap-token" {
		return errUnauthorized
	}
	return nil
}

type ProductionQueryService struct {
	Lookup TokenLookup
	Reads  QueryStore
}

func NewProductionQueryService(lookup TokenLookup, reads QueryStore) ProductionQueryService {
	return ProductionQueryService{
		Lookup: lookup,
		Reads:  reads,
	}
}

func (s ProductionQueryService) ListProjects(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}
	return s.Reads.ListProjects(r.Context(), tenantID)
}

func (s ProductionQueryService) ListRepositories(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}
	return s.Reads.ListRepositories(r.Context(), tenantID)
}

func (s ProductionQueryService) ListScans(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}

	from, err := parseTime(r.URL.Query().Get("from"))
	if err != nil {
		return nil, err
	}
	to, err := parseTime(r.URL.Query().Get("to"))
	if err != nil {
		return nil, err
	}

	return s.Reads.ListScans(r.Context(), store.ScanFilter{
		TenantID:       tenantID,
		RepositorySlug: r.URL.Query().Get("repository_slug"),
		From:           from,
		To:             to,
	})
}

func (s ProductionQueryService) GetScan(r *http.Request, token string) (any, error) {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:read") {
		return nil, errUnauthorized
	}
	return s.Reads.GetScan(r.Context(), tenantID, r.PathValue("scan_id"))
}

func (s ProductionQueryService) UpdateScanMetadata(r *http.Request, token string) error {
	tenantID, scopes, err := s.Lookup.FindToken(r.Context(), auth.HashToken(token))
	if err != nil || !hasScope(scopes, "scan:metadata:write") {
		return errUnauthorized
	}

	var payload struct {
		Labels     map[string]string `json:"labels"`
		Annotation string            `json:"annotation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return err
	}

	return s.Reads.UpdateScanMetadata(r.Context(), tenantID, r.PathValue("scan_id"), payload.Labels, payload.Annotation)
}

func NewServer(upload UploadService, query QueryService) http.Handler {
	uploadMux := NewRouter(upload)
	queryMux := NewQueryRouter(query)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/scans" {
			uploadMux.ServeHTTP(w, r)
			return
		}
		queryMux.ServeHTTP(w, r)
	})
}

func NewQueryRouter(service QueryService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		result, err := service.ListProjects(r, token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("GET /api/v1/repositories", func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		result, err := service.ListRepositories(r, token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("GET /api/v1/scans", func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		result, err := service.ListScans(r, token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("GET /api/v1/scans/{scan_id}", func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		result, err := service.GetScan(r, token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("PATCH /api/v1/scans/{scan_id}/metadata", func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if err := service.UpdateScanMetadata(r, token); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}
```

```yaml
  /api/v1/scans:
    get:
      summary: List scans for the current tenant
      parameters:
        - in: query
          name: repository_slug
          required: true
          schema:
            type: string
        - in: query
          name: from
          required: true
          schema:
            type: string
            format: date-time
        - in: query
          name: to
          required: true
          schema:
            type: string
            format: date-time
      responses:
        "200":
          description: OK
    post:
      summary: Upload a scan snapshot
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/ScanUploadRequest"
      responses:
        "201":
          description: Created
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/http -run 'TestListScansSupportsRepositoryAndTimeFilters|TestGetScanReturnsOK|TestPatchScanMetadataReturnsOK' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/http/query_handlers.go internal/http/query_handlers_test.go internal/store/scans.go api/openapi.yaml
git commit -m "feat: add scan history queries and metadata overlays"
```

### Task 7: Wire the production app, add bootstrap startup, and verify end-to-end locally

**Files:**
- Modify: `internal/app/app.go`
- Modify: `cmd/deplens-platform/main.go`
- Create: `README.md`
- Create: `.env.example`

- [ ] **Step 1: Write the failing startup smoke test**

```go
package app

import (
	"testing"

	"github.com/ferretsecurity/deplens-platform/internal/config"
)

func TestNewInitializesBlobStoreForFilesystemBackend(t *testing.T) {
	cfg := config.Config{
		Mode:                   "self-hosted",
		DatabaseURL:            "postgres://postgres:postgres@localhost:5432/deplens?sslmode=disable",
		BlobBackend:            "filesystem",
		BlobFilesystemRoot:     "./var/blobs",
		BootstrapOwnerEmail:    "admin@example.com",
		BootstrapOwnerPassword: "change-me-now",
		SessionCookieSecret:    "replace-me",
	}

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if application.BlobStore == nil {
		t.Fatal("BlobStore = nil, want initialized store")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app -run TestNewInitializesBlobStoreForFilesystemBackend -v`
Expected: FAIL with `too many arguments in call to New` or missing `BlobStore`

- [ ] **Step 3: Write the production wiring and operator docs**

```go
package app

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ferretsecurity/deplens-platform/internal/blob"
	"github.com/ferretsecurity/deplens-platform/internal/config"
	"github.com/ferretsecurity/deplens-platform/internal/httpapi"
	"github.com/ferretsecurity/deplens-platform/internal/scans"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

type App struct {
	Config    config.Config
	DB        *pgxpool.Pool
	BlobStore blob.Store
	Handler   http.Handler
}

func New(cfg config.Config) (*App, error) {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	blobStore := blob.NewFileSystemStore(cfg.BlobFilesystemRoot)
	persistence := store.Store{DB: db}
	uploadService := httpapi.NewProductionUploadService(persistence, scans.Service{
		Blob:  blobStore,
		Store: persistence,
	})
	handler := httpapi.NewServer(
		uploadService,
		httpapi.NewProductionQueryService(persistence, store.ScanStore{DB: db}),
	)

	return &App{
		Config:    cfg,
		DB:        db,
		BlobStore: blobStore,
		Handler:   handler,
	}, nil
}
```

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/ferretsecurity/deplens-platform/internal/app"
	"github.com/ferretsecurity/deplens-platform/internal/config"
	"github.com/ferretsecurity/deplens-platform/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	if err := store.Migrate(cfg.DatabaseURL); err != nil {
		logger.Error("run migrations", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		logger.Error("build app", "error", err)
		os.Exit(1)
	}
	defer application.DB.Close()

	bootstrapInput := store.BootstrapInput{
		OwnerEmail:    cfg.BootstrapOwnerEmail,
		OwnerPassword: cfg.BootstrapOwnerPassword,
	}
	if _, err := store.BootstrapDefaultTenant(context.Background(), application.DB, bootstrapInput); err != nil {
		logger.Error("bootstrap default tenant", "error", err)
		os.Exit(1)
	}

	logger.Info("http server starting", "http_address", cfg.HTTPAddress)
	if err := http.ListenAndServe(cfg.HTTPAddress, application.Handler); err != nil {
		logger.Error("http server stopped", "error", err)
		os.Exit(1)
	}
}
```

```dotenv
DATABASE_URL=postgres://postgres:postgres@localhost:5432/deplens?sslmode=disable
HTTP_ADDRESS=:8080
DEPLOYMENT_MODE=self-hosted
BLOB_BACKEND=filesystem
BLOB_FILESYSTEM_ROOT=./var/blobs
BOOTSTRAP_OWNER_EMAIL=admin@example.com
BOOTSTRAP_OWNER_PASSWORD=change-me-now
SESSION_COOKIE_SECRET=replace-me
```

```md
# deplens-platform

`deplens-platform` is the self-hosted backend for `deplens` scan snapshots.

## Local startup

1. `cp .env.example .env`
2. `docker compose up -d postgres`
3. `export $(grep -v '^#' .env | xargs)`
4. `go run ./cmd/deplens-platform`

## First login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"change-me-now"}'
```

The login response should include the user's tenant memberships. If the user belongs to more than one tenant, the client should select the active tenant for subsequent session-backed requests.

## Inviting a user

V1 should support a simple tenant invite flow:

- tenant `owner` or `admin` invites a user by email
- the invite defaults to `viewer`
- once accepted, the user gets a `tenant_membership` for that tenant
- role changes can be handled later through a simple admin workflow

## First upload

```bash
curl -X POST http://localhost:8080/api/v1/scans \
  -H 'Authorization: Bearer <token-from-/api/v1/tokens>' \
  -H 'Content-Type: application/json' \
  -d '{
    "schema_version":"v1alpha1",
    "project":{"slug":"core","name":"Core"},
    "repository":{"slug":"repo","name":"Repo","url":"https://example.com/repo.git","default_branch":"main"},
    "source":{"commit_sha":"abc123","ref":"refs/heads/main","scanned_at":"2026-05-04T10:00:00Z"},
    "snapshot":{"root":".","manifests":[]}
  }'
```
```

- [ ] **Step 4: Run full verification**

Run: `go test ./...`
Expected: PASS

Run: `docker compose up -d postgres`
Expected: PostgreSQL container starts on port 5432

Run: `export $(grep -v '^#' .env.example | xargs) && go run ./cmd/deplens-platform`
Expected: service starts on `:8080`, bootstraps the default tenant, and creates the first owner user

- [ ] **Step 5: Commit**

```bash
git add internal/app/app.go cmd/deplens-platform/main.go README.md .env.example
git commit -m "feat: wire startup bootstrap and local operator flow"
```
