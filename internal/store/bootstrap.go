package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/argon2"
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

	if err := goose.Up(db, migrationsDir()); err != nil {
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

	displayName := input.OwnerDisplayName
	if displayName == "" {
		displayName = input.OwnerEmail
	}

	var userID string
	err = tx.QueryRow(ctx, `
		insert into users (display_name, primary_email)
		values ($1, $2)
		on conflict (primary_email) do update set display_name = excluded.display_name
		returning id
	`, displayName, input.OwnerEmail).Scan(&userID)
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
		on conflict (provider_id, subject) do update set email = excluded.email, password_hash = excluded.password_hash
	`, userID, providerID, input.OwnerEmail, input.OwnerEmail, passwordHash)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("insert local auth identity: %w", err)
	}

	_, err = tx.Exec(ctx, `
		insert into tenant_memberships (tenant_id, user_id, role)
		values ($1, $2, 'owner')
		on conflict (tenant_id, user_id) do update set role = excluded.role
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
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	const memory = 64 * 1024
	const iterations = 3
	const parallelism = 2
	const keyLength = 32

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory,
		iterations,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func migrationsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "db/migrations"
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations"))
}
