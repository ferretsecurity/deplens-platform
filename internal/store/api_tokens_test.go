package store

import (
	"context"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAPITokenMetadataCRUDIsTenantScoped(t *testing.T) {
	ctx := context.Background()
	db, databaseURL := openTestDatabase(t)

	if err := Migrate(databaseURL); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	store := Store{DB: db}
	tenantID := mustCreateTenant(t, ctx, db)
	otherTenantID := mustCreateTenantWithSlug(t, ctx, db, "other")

	if _, _, err := store.CreateAPIToken(ctx, tenantID, "scanner", []string{"scan:write"}); err != nil {
		t.Fatalf("CreateAPIToken() tenant error = %v", err)
	}
	if _, _, err := store.CreateAPIToken(ctx, otherTenantID, "other scanner", []string{"scan:read"}); err != nil {
		t.Fatalf("CreateAPIToken() other tenant error = %v", err)
	}

	items, err := store.ListAPITokens(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListAPITokens() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("token count = %d, want 1", len(items))
	}
	if items[0].ID == "" {
		t.Fatal("token ID is empty")
	}
	if items[0].Label != "scanner" {
		t.Fatalf("label = %q, want scanner", items[0].Label)
	}
	if !reflect.DeepEqual(items[0].Scopes, []string{"scan:write"}) {
		t.Fatalf("scopes = %#v, want scan:write", items[0].Scopes)
	}
	if items[0].CreatedAt.IsZero() {
		t.Fatal("created_at is zero")
	}

	updated, err := store.UpdateAPIToken(ctx, tenantID, items[0].ID, "ci scanner", []string{"scan:read", "scan:metadata:write"})
	if err != nil {
		t.Fatalf("UpdateAPIToken() error = %v", err)
	}
	if updated.Label != "ci scanner" {
		t.Fatalf("updated label = %q, want ci scanner", updated.Label)
	}
	if !reflect.DeepEqual(updated.Scopes, []string{"scan:read", "scan:metadata:write"}) {
		t.Fatalf("updated scopes = %#v, want read and metadata write", updated.Scopes)
	}

	if _, err := store.UpdateAPIToken(ctx, otherTenantID, items[0].ID, "cross tenant", []string{"scan:read"}); err == nil {
		t.Fatal("UpdateAPIToken() cross tenant error = nil, want error")
	}

	if err := store.DeleteAPIToken(ctx, otherTenantID, items[0].ID); err == nil {
		t.Fatal("DeleteAPIToken() cross tenant error = nil, want error")
	}
	if err := store.DeleteAPIToken(ctx, tenantID, items[0].ID); err != nil {
		t.Fatalf("DeleteAPIToken() error = %v", err)
	}
	items, err = store.ListAPITokens(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListAPITokens() after delete error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("token count after delete = %d, want 0", len(items))
	}
}

func mustCreateTenantWithSlug(t *testing.T, ctx context.Context, db *pgxpool.Pool, slug string) string {
	t.Helper()

	var tenantID string
	if err := db.QueryRow(ctx, `insert into tenants (slug, name) values ($1, $2) returning id`, slug, slug).Scan(&tenantID); err != nil {
		t.Fatalf("insert tenant %q error = %v", slug, err)
	}
	return tenantID
}
