# deplens-platform

`deplens-platform` is the self-hosted backend and customer-facing web UI for `deplens` scan snapshots.

## Local startup

1. `cp .env.example .env`
2. `docker compose up -d postgres`
3. `export $(grep -v '^#' .env | xargs)`
4. `go run ./cmd/deplens-platform`

In another terminal:

1. `pnpm install`
2. `pnpm dev:web`

The web app serves the authenticated product UI at `http://localhost:3000` and talks to the API on `http://localhost:8080`.

## First login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"change-me-now"}'
```

The session stores the user ID and tenant memberships. If the user belongs to one tenant, it also stores the active tenant and role.

## Create an API token

```bash
curl -X POST http://localhost:8080/api/v1/tokens \
  -H 'Content-Type: application/json' \
  -d '{"label":"scanner","scopes":["scan:write","scan:read","scan:metadata:write"]}'
```

This endpoint requires an authenticated owner or admin session. The plaintext token is returned once and should be used as a bearer token by scanners and CI.

## Inviting a user

V1 reserves tenant invite storage for a simple flow:

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
    "repository":{"name":"Repo","url":"https://example.com/repo.git","default_branch":"main"},
    "source":{"commit_sha":"abc123","ref":"refs/heads/main","scanned_at":"2026-05-04T10:00:00Z"},
    "snapshot":{"root":".","manifests":[]}
  }'
```

## Raw deplens upload

The upload API also accepts a raw `deplens -json` payload when repository and source metadata are supplied in headers:

```bash
deplens -json /path/to/repo > /tmp/deplens.json

curl -X POST http://localhost:8080/api/v1/scans \
  -H 'Authorization: Bearer <token-from-/api/v1/tokens>' \
  -H 'Content-Type: application/json' \
  -H 'X-Deplens-Repository-Name: juice-shop' \
  -H 'X-Deplens-Repository-URL: https://github.com/juice-shop/juice-shop' \
  -H 'X-Deplens-Default-Branch: master' \
  -H 'X-Deplens-Commit-SHA: <commit-sha>' \
  -H 'X-Deplens-Ref: refs/heads/master' \
  -H 'X-Deplens-Scanned-At: 2026-05-05T18:00:00Z' \
  --data-binary @/tmp/deplens.json
```

## Read scan manifests

`GET /api/v1/scans/{scan_id}` stays summary-only. Use the manifest detail endpoint when you need nested dependencies:

```bash
curl -s \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/scans/$SCAN_ID/manifests" | jq
```

## Compose deployment

For a single-server self-hosted install:

1. `cp deploy/compose/.env.example .env`
2. Update `APP_HOST` and the bootstrap credentials in `.env`
3. `docker compose up -d --build`

That starts `caddy`, `web`, `api`, and `postgres` behind a single origin.
