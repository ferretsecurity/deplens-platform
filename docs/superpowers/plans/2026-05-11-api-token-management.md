# API Token Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-stack API token management page where owner/admin users can list, issue, edit, and delete active-tenant API tokens.

**Architecture:** Keep the Go backend authoritative for token management and session authorization. Add tenant-scoped store methods and HTTP endpoints that expose token metadata while returning plaintext secrets only from create. Add a Next.js authenticated route that server-loads token metadata and uses a focused client component for create/edit/delete mutations.

**Tech Stack:** Go `net/http`, `scs`, `pgx`, PostgreSQL, Next.js App Router, React 19, TypeScript, Tailwind, shadcn-style components, Vitest, Go tests.

---

## File Structure

- Modify `internal/store/models.go`: add `APITokenMetadata` response/storage model.
- Modify `internal/store/uploads.go`: add list, update, and delete token methods next to existing token persistence.
- Create `internal/store/api_tokens_test.go`: store integration tests for token metadata CRUD.
- Modify `internal/http/token_handlers.go`: extend service interfaces, add validation, add `GET`, `PATCH`, and `DELETE` handlers.
- Modify `internal/http/token_handlers_test.go`: handler tests for list/create/update/delete, validation, role rejection, and metadata-only responses.
- Modify `internal/app/app.go`: route all `/api/v1/tokens` and `/api/v1/tokens/{id}` requests to the token router.
- Modify `api/openapi.yaml`: document token list, create, update, and delete.
- Modify `apps/web/src/lib/types.ts`: add API token types.
- Modify `apps/web/src/lib/queries.ts`: add `listAPITokensServer`.
- Create `apps/web/src/app/app/tokens/token-management-client.tsx`: client-side issue/edit/delete UI and mutations.
- Create `apps/web/src/app/app/tokens/page.tsx`: server route that renders the token page.
- Create `apps/web/src/app/app/tokens/page.test.tsx`: route rendering test.
- Create `apps/web/src/app/app/tokens/token-management-client.test.tsx`: client workflow tests.
- Modify `apps/web/src/components/app-shell.tsx`: add an `API Tokens` nav item.
- Create or modify `apps/web/src/components/app-shell.test.tsx`: nav link coverage.

---

### Task 1: Store Token Metadata CRUD

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/uploads.go`
- Create: `internal/store/api_tokens_test.go`

- [ ] **Step 1: Write failing store tests**

Create `internal/store/api_tokens_test.go`:

```go
package store

import (
	"context"
	"reflect"
	"testing"
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
```

Append this helper to an existing store test helper file, or place it in `api_tokens_test.go`:

```go
func mustCreateTenantWithSlug(t *testing.T, ctx context.Context, db *pgxpool.Pool, slug string) string {
	t.Helper()

	var tenantID string
	if err := db.QueryRow(ctx, `insert into tenants (slug, name) values ($1, $2) returning id`, slug, slug).Scan(&tenantID); err != nil {
		t.Fatalf("insert tenant %q error = %v", slug, err)
	}
	return tenantID
}
```

- [ ] **Step 2: Run store test and verify red**

Run:

```bash
go test ./internal/store -run TestAPITokenMetadataCRUDIsTenantScoped -count=1
```

Expected: FAIL because `ListAPITokens`, `UpdateAPIToken`, and `DeleteAPIToken` are undefined.

- [ ] **Step 3: Add token metadata model**

Add to `internal/store/models.go`:

```go
type APITokenMetadata struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 4: Add store methods**

Replace the existing `CreateAPIToken` in `internal/store/uploads.go` and add the new metadata methods after it:

```go
func (s Store) CreateAPIToken(ctx context.Context, tenantID string, label string, scopes []string) (string, APITokenMetadata, error) {
	token, err := randomToken()
	if err != nil {
		return "", APITokenMetadata{}, err
	}

	var item APITokenMetadata
	err = s.DB.QueryRow(ctx, `
		insert into api_tokens (tenant_id, label, token_hash, scopes)
		values ($1, $2, $3, $4)
		returning id, label, scopes, created_at
	`, tenantID, label, hashToken(token), scopes).Scan(&item.ID, &item.Label, &item.Scopes, &item.CreatedAt)
	if err != nil {
		return "", APITokenMetadata{}, err
	}

	return token, item, nil
}

func (s Store) ListAPITokens(ctx context.Context, tenantID string) ([]APITokenMetadata, error) {
	rows, err := s.DB.Query(ctx, `
		select id, label, scopes, created_at
		from api_tokens
		where tenant_id = $1
		order by created_at desc, label asc
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]APITokenMetadata, 0)
	for rows.Next() {
		var item APITokenMetadata
		if err := rows.Scan(&item.ID, &item.Label, &item.Scopes, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Store) UpdateAPIToken(ctx context.Context, tenantID string, tokenID string, label string, scopes []string) (APITokenMetadata, error) {
	var item APITokenMetadata
	err := s.DB.QueryRow(ctx, `
		update api_tokens
		set label = $3, scopes = $4
		where tenant_id = $1 and id = $2
		returning id, label, scopes, created_at
	`, tenantID, tokenID, label, scopes).Scan(&item.ID, &item.Label, &item.Scopes, &item.CreatedAt)
	return item, err
}

func (s Store) DeleteAPIToken(ctx context.Context, tenantID string, tokenID string) error {
	tag, err := s.DB.Exec(ctx, `
		delete from api_tokens
		where tenant_id = $1 and id = $2
	`, tenantID, tokenID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
```

- [ ] **Step 5: Run store tests and verify green**

Run:

```bash
go test ./internal/store -run 'TestAPITokenMetadataCRUDIsTenantScoped|TestBootstrapDefaultTenantCreatesTenantOwnerAndAdminToken' -count=1
```

Expected: PASS, or Docker-dependent store tests skip cleanly if Docker is unavailable.

- [ ] **Step 6: Commit store task**

```bash
git add internal/store/models.go internal/store/uploads.go internal/store/api_tokens_test.go
git commit -m "feat: add api token metadata store operations"
```

---

### Task 2: Backend Token CRUD API

**Files:**
- Modify: `internal/http/token_handlers.go`
- Modify: `internal/http/token_handlers_test.go`

- [ ] **Step 1: Replace handler tests with CRUD coverage**

Update `internal/http/token_handlers_test.go` to cover:

```go
func TestListTokensReturnsMetadataOnlyForOwnerSession(t *testing.T)
func TestCreateTokenReturnsPlaintextOnceForOwnerSession(t *testing.T)
func TestUpdateTokenChangesLabelAndScopes(t *testing.T)
func TestDeleteTokenReturnsNoContent(t *testing.T)
func TestTokenRouterRejectsViewerSession(t *testing.T)
func TestTokenRouterRejectsInvalidScopes(t *testing.T)
func TestTokenRouterReturnsNotFoundForMissingTenantToken(t *testing.T)
```

Use this fake service shape:

```go
type fakeTokenService struct {
	items []store.APITokenMetadata
}

func (f fakeTokenService) ListTokens(_ context.Context, tenantID string, role string) ([]store.APITokenMetadata, error) {
	if tenantID != "tenant-123" || role == "viewer" {
		return nil, errUnauthorized
	}
	return f.items, nil
}

func (fakeTokenService) CreateToken(_ context.Context, tenantID string, role string, label string, scopes []string) (string, store.APITokenMetadata, error) {
	if tenantID != "tenant-123" || role == "viewer" {
		return "", store.APITokenMetadata{}, errUnauthorized
	}
	if label != "scanner" {
		return "", store.APITokenMetadata{}, errUnauthorized
	}
	return "plain-token", store.APITokenMetadata{ID: "token-1", Label: label, Scopes: scopes}, nil
}

func (fakeTokenService) UpdateToken(_ context.Context, tenantID string, role string, tokenID string, label string, scopes []string) (store.APITokenMetadata, error) {
	if tenantID != "tenant-123" || role == "viewer" {
		return store.APITokenMetadata{}, errUnauthorized
	}
	if tokenID == "missing" {
		return store.APITokenMetadata{}, errNotFound
	}
	return store.APITokenMetadata{ID: tokenID, Label: label, Scopes: scopes}, nil
}

func (fakeTokenService) DeleteToken(_ context.Context, tenantID string, role string, tokenID string) error {
	if tenantID != "tenant-123" || role == "viewer" {
		return errUnauthorized
	}
	if tokenID == "missing" {
		return errNotFound
	}
	return nil
}
```

Assert list response does not contain `plain-token` or `token_hash`:

```go
body := rr.Body.String()
if strings.Contains(body, "plain-token") || strings.Contains(body, "token_hash") {
	t.Fatalf("response leaked secret material: %s", body)
}
```

- [ ] **Step 2: Run handler tests and verify red**

Run:

```bash
go test ./internal/http -run 'TestListTokens|TestCreateToken|TestUpdateToken|TestDeleteToken|TestTokenRouter' -count=1
```

Expected: FAIL because the service interface and routes do not yet support list, update, or delete.

- [ ] **Step 3: Extend token service and validation**

In `internal/http/token_handlers.go`, update interfaces and add validation:

```go
type TokenService interface {
	ListTokens(ctx context.Context, tenantID string, role string) ([]store.APITokenMetadata, error)
	CreateToken(ctx context.Context, tenantID string, role string, label string, scopes []string) (string, store.APITokenMetadata, error)
	UpdateToken(ctx context.Context, tenantID string, role string, tokenID string, label string, scopes []string) (store.APITokenMetadata, error)
	DeleteToken(ctx context.Context, tenantID string, role string, tokenID string) error
}

type TokenStore interface {
	CreateAPIToken(ctx context.Context, tenantID string, label string, scopes []string) (string, store.APITokenMetadata, error)
	ListAPITokens(ctx context.Context, tenantID string) ([]store.APITokenMetadata, error)
	UpdateAPIToken(ctx context.Context, tenantID string, tokenID string, label string, scopes []string) (store.APITokenMetadata, error)
	DeleteAPIToken(ctx context.Context, tenantID string, tokenID string) error
}

var errBadRequest = errors.New("bad request")
var errNotFound = errors.New("not found")

var allowedTokenScopes = map[string]struct{}{
	"scan:read":           {},
	"scan:write":          {},
	"scan:metadata:write": {},
}

func authorizeTokenManagement(tenantID string, role string) error {
	if tenantID == "" {
		return errUnauthorized
	}
	if role != "owner" && role != "admin" {
		return errUnauthorized
	}
	return nil
}

func validateTokenPayload(label string, scopes []string) (string, []string, error) {
	label = strings.TrimSpace(label)
	if label == "" || len(scopes) == 0 {
		return "", nil, errBadRequest
	}
	for _, scope := range scopes {
		if _, ok := allowedTokenScopes[scope]; !ok {
			return "", nil, errBadRequest
		}
	}
	return label, scopes, nil
}
```

Import `strings` and `github.com/ferretsecurity/deplens-platform/internal/store`.

- [ ] **Step 4: Implement production service methods**

Update `ProductionTokenService` methods:

```go
type ProductionTokenService struct {
	Store TokenStore
}

func (s ProductionTokenService) ListTokens(ctx context.Context, tenantID string, role string) ([]store.APITokenMetadata, error) {
	if err := authorizeTokenManagement(tenantID, role); err != nil {
		return nil, err
	}
	return s.Store.ListAPITokens(ctx, tenantID)
}

func (s ProductionTokenService) CreateToken(ctx context.Context, tenantID string, role string, label string, scopes []string) (string, store.APITokenMetadata, error) {
	if err := authorizeTokenManagement(tenantID, role); err != nil {
		return "", store.APITokenMetadata{}, err
	}
	label, scopes, err := validateTokenPayload(label, scopes)
	if err != nil {
		return "", store.APITokenMetadata{}, err
	}
	token, item, err := s.Store.CreateAPIToken(ctx, tenantID, label, scopes)
	if err != nil {
		return "", store.APITokenMetadata{}, err
	}
	return token, item, nil
}

func (s ProductionTokenService) UpdateToken(ctx context.Context, tenantID string, role string, tokenID string, label string, scopes []string) (store.APITokenMetadata, error) {
	if err := authorizeTokenManagement(tenantID, role); err != nil {
		return store.APITokenMetadata{}, err
	}
	label, scopes, err := validateTokenPayload(label, scopes)
	if err != nil {
		return store.APITokenMetadata{}, err
	}
	item, err := s.Store.UpdateAPIToken(ctx, tenantID, tokenID, label, scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.APITokenMetadata{}, errNotFound
	}
	return item, err
}

func (s ProductionTokenService) DeleteToken(ctx context.Context, tenantID string, role string, tokenID string) error {
	if err := authorizeTokenManagement(tenantID, role); err != nil {
		return err
	}
	err := s.Store.DeleteAPIToken(ctx, tenantID, tokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	return err
}
```

Import `errors` and `github.com/jackc/pgx/v5`.

- [ ] **Step 5: Implement token routes**

Replace `NewTokenRouter` body with these route registrations and response behavior:

```go
mux.HandleFunc("GET /api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
	items, err := service.ListTokens(r.Context(), sessions.GetString(r.Context(), "active_tenant_id"), sessions.GetString(r.Context(), "role"))
	if writeTokenError(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
})

mux.HandleFunc("POST /api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
	var payload tokenPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	token, item, err := service.CreateToken(r.Context(), sessions.GetString(r.Context(), "active_tenant_id"), sessions.GetString(r.Context(), "role"), payload.Label, payload.Scopes)
	if writeTokenError(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createdTokenResponse{
		APITokenMetadata: item,
		Token:            token,
	})
})

mux.HandleFunc("PATCH /api/v1/tokens/{tokenID}", func(w http.ResponseWriter, r *http.Request) {
	var payload tokenPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	item, err := service.UpdateToken(r.Context(), sessions.GetString(r.Context(), "active_tenant_id"), sessions.GetString(r.Context(), "role"), r.PathValue("tokenID"), payload.Label, payload.Scopes)
	if writeTokenError(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
})

mux.HandleFunc("DELETE /api/v1/tokens/{tokenID}", func(w http.ResponseWriter, r *http.Request) {
	err := service.DeleteToken(r.Context(), sessions.GetString(r.Context(), "active_tenant_id"), sessions.GetString(r.Context(), "role"), r.PathValue("tokenID"))
	if writeTokenError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
})
```

Add these helper types and function in `internal/http/token_handlers.go`:

```go
type tokenPayload struct {
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
}

type createdTokenResponse struct {
	store.APITokenMetadata
	Token string `json:"token"`
}

func writeTokenError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, errBadRequest):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, errNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, errUnauthorized):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	return true
}
```

- [ ] **Step 6: Run handler tests and verify green**

Run:

```bash
go test ./internal/http -run 'TestListTokens|TestCreateToken|TestUpdateToken|TestDeleteToken|TestTokenRouter' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit backend API task**

```bash
git add internal/http/token_handlers.go internal/http/token_handlers_test.go
git commit -m "feat: expose api token management endpoints"
```

---

### Task 3: Route Wiring and OpenAPI

**Files:**
- Modify: `internal/app/app.go`
- Modify: `api/openapi.yaml`
- Modify: `internal/api/spec_test.go`

- [ ] **Step 1: Write or extend routing/spec tests**

In `internal/api/spec_test.go`, extend the token path assertions so the test requires these snippets:

```go
required := []string{
	"/api/v1/tokens:",
	"get:",
	"post:",
	"/api/v1/tokens/{token_id}:",
	"patch:",
	"delete:",
}
```

In `internal/app/app_test.go`, add a route assertion that a `PATCH /api/v1/tokens/token-1` request is handled by the token router. Use the existing app test setup and assert the response is not the generic API router 404.

- [ ] **Step 2: Run route/spec tests and verify red**

Run:

```bash
go test ./internal/api ./internal/app -run 'TestOpenAPISpec|TestApp' -count=1
```

Expected: FAIL because `/api/v1/tokens/{token_id}` is not documented and `routeAuthAndAPI` only routes the collection path.

- [ ] **Step 3: Route token item paths**

In `internal/app/app.go`, change:

```go
if r.URL.Path == "/api/v1/tokens" {
	tokenRouter.ServeHTTP(w, r)
	return
}
```

to:

```go
if r.URL.Path == "/api/v1/tokens" || strings.HasPrefix(r.URL.Path, "/api/v1/tokens/") {
	tokenRouter.ServeHTTP(w, r)
	return
}
```

Import `strings`.

- [ ] **Step 4: Update OpenAPI token paths**

In `api/openapi.yaml`, expand `/api/v1/tokens` with `get` and `post`, and add `/api/v1/tokens/{token_id}` with `patch` and `delete`. The documented metadata schema must include `id`, `label`, `scopes`, and `created_at`; only the create response includes `token`.

- [ ] **Step 5: Run route/spec tests and verify green**

Run:

```bash
go test ./internal/api ./internal/app -run 'TestOpenAPISpec|TestApp' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit route/docs task**

```bash
git add internal/app/app.go internal/app/app_test.go internal/api/spec_test.go api/openapi.yaml
git commit -m "docs: document api token management endpoints"
```

---

### Task 4: Web Token Management Page

**Files:**
- Modify: `apps/web/src/lib/types.ts`
- Modify: `apps/web/src/lib/queries.ts`
- Create: `apps/web/src/app/app/tokens/page.tsx`
- Create: `apps/web/src/app/app/tokens/token-management-client.tsx`
- Create: `apps/web/src/app/app/tokens/page.test.tsx`
- Create: `apps/web/src/app/app/tokens/token-management-client.test.tsx`
- Modify: `apps/web/src/components/app-shell.tsx`
- Create: `apps/web/src/components/app-shell.test.tsx`

- [ ] **Step 1: Write failing frontend tests**

Create `apps/web/src/app/app/tokens/page.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import TokensPage from "./page";

vi.mock("@/lib/queries", () => ({
  listAPITokensServer: vi.fn().mockResolvedValue([
    {
      id: "token-1",
      label: "CI scanner",
      scopes: ["scan:read", "scan:write"],
      created_at: "2026-05-11T10:00:00Z"
    }
  ])
}));

describe("TokensPage", () => {
  it("renders token metadata without a secret", async () => {
    render(await TokensPage());

    expect(screen.getByText("API Tokens")).toBeInTheDocument();
    expect(screen.getByText("CI scanner")).toBeInTheDocument();
    expect(screen.getByText("scan:read")).toBeInTheDocument();
    expect(screen.queryByText("plain-token")).not.toBeInTheDocument();
  });
});
```

Create `apps/web/src/app/app/tokens/token-management-client.test.tsx` with tests that mock `global.fetch` and verify:

```tsx
it("creates a token and shows the one-time secret", async () => {})
it("edits label and scopes without sending a secret", async () => {})
it("requires delete confirmation before deleting", async () => {})
```

Create `apps/web/src/components/app-shell.test.tsx` and mock `next/navigation` so `usePathname` returns `/app/tokens`. Assert the link named `API Tokens` points to `/app/tokens`.

- [ ] **Step 2: Run frontend tests and verify red**

Run:

```bash
pnpm --dir apps/web test -- --run src/app/app/tokens/page.test.tsx src/app/app/tokens/token-management-client.test.tsx src/components/app-shell.test.tsx
```

Expected: FAIL because the token page, client component, types, query helper, and app-shell link do not exist yet.

- [ ] **Step 3: Add token types and server query**

Add to `apps/web/src/lib/types.ts`:

```ts
export type APITokenMetadata = {
  id: string;
  label: string;
  scopes: string[];
  created_at: string;
};

export type CreatedAPIToken = APITokenMetadata & {
  token: string;
};
```

Add to `apps/web/src/lib/queries.ts`:

```ts
export async function listAPITokensServer() {
  return serverApiFetch<APITokenMetadata[]>(`${env.API_INTERNAL_BASE_URL}/api/v1/tokens`, {
    headers: cookieHeaders()
  });
}
```

Import `APITokenMetadata`.

- [ ] **Step 4: Add token page**

Create `apps/web/src/app/app/tokens/page.tsx`:

```tsx
import { listAPITokensServer } from "@/lib/queries";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

import { TokenManagementClient } from "./token-management-client";

export default async function TokensPage() {
  const tokens = await listAPITokensServer();

  return (
    <section className="space-y-6">
      <Card className="border-slate-200/80 bg-white/80 shadow-sm backdrop-blur">
        <CardHeader>
          <CardTitle>API Tokens</CardTitle>
          <CardDescription>
            Issue scanner tokens, adjust token scopes, and remove old credentials after rotation.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <TokenManagementClient initialTokens={tokens} />
        </CardContent>
      </Card>
    </section>
  );
}
```

- [ ] **Step 5: Add client component**

Create `apps/web/src/app/app/tokens/token-management-client.tsx` as a client component. It must:

- Define `allowedScopes = ["scan:read", "scan:write", "scan:metadata:write"]`.
- Keep local token list state from `initialTokens`.
- Keep form state for `label` and selected scopes.
- Use `clientApiFetch<CreatedAPIToken>("/api/v1/tokens", { method: "POST", body: JSON.stringify({ label, scopes }) })` for issue.
- Use `clientApiFetch<APITokenMetadata>(\`/api/v1/tokens/${editing.id}\`, { method: "PATCH", body: JSON.stringify({ label, scopes }) })` for edit.
- Use `clientApiFetch<void>(\`/api/v1/tokens/${token.id}\`, { method: "DELETE" })` for delete.
- Render the one-time plaintext `created.token` after successful issue until dismissed.
- Never render or accept a token secret in the edit form.

Use existing `Button`, `Input`, `Label`, and `Table` components. Use compact inline forms instead of adding a dialog dependency.

- [ ] **Step 6: Add app-shell nav link**

In `apps/web/src/components/app-shell.tsx`, import `KeyRound` from `lucide-react` and add:

```tsx
{ href: "/app/tokens", label: "API Tokens", icon: KeyRound }
```

to `navItems`.

- [ ] **Step 7: Run frontend tests and verify green**

Run:

```bash
pnpm --dir apps/web test -- --run src/app/app/tokens/page.test.tsx src/app/app/tokens/token-management-client.test.tsx src/components/app-shell.test.tsx
```

Expected: PASS.

- [ ] **Step 8: Commit frontend task**

```bash
git add apps/web/src/lib/types.ts apps/web/src/lib/queries.ts apps/web/src/app/app/tokens apps/web/src/components/app-shell.tsx apps/web/src/components/app-shell.test.tsx
git commit -m "feat: add api token management page"
```

---

### Task 5: Full Verification

**Files:**
- No new files.

- [ ] **Step 1: Run backend tests**

```bash
go test ./...
```

Expected: PASS, or Docker-dependent store integration tests skip cleanly if Docker is unavailable.

- [ ] **Step 2: Run frontend tests**

```bash
pnpm --dir apps/web test
```

Expected: PASS.

- [ ] **Step 3: Run frontend lint**

```bash
pnpm --dir apps/web lint
```

Expected: PASS.

- [ ] **Step 4: Run OpenAPI spec test directly**

```bash
go test ./internal/api -count=1
```

Expected: PASS.

- [ ] **Step 5: Review git diff**

```bash
git diff --stat HEAD~4..HEAD
git status --short
```

Expected: only token-management backend, docs, routing, and frontend files are changed; working tree is clean after the task commits.

---

## Self-Review Notes

- Spec coverage: store metadata CRUD, HTTP CRUD, role checks, tenant scoping, OpenAPI, page UI, one-time secret display, edit label/scopes only, hard delete, and tests are covered by Tasks 1-5.
- Scope intentionally excludes last-used tracking, soft delete, in-place secret rotation, token prefixes, and post-create secret display.
- Type consistency: backend metadata uses `id`, `label`, `scopes`, `created_at`; frontend mirrors these JSON names in `APITokenMetadata`.
