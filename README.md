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

Use the deployment Compose files for a single-machine evaluation or single-server
self-hosted install. This runs the packaged product: Caddy, web, API, and
Postgres.

1. `cd deploy/compose`
2. `cp .env.example .env`
3. Update `APP_HOST`, `APP_BASE_URL`, `DEPLENS_VERSION`, and the secrets in `.env`
4. `docker compose pull`
5. `docker compose up -d`

For local evaluation on one machine, keep:

```env
APP_HOST=localhost
APP_BASE_URL=https://localhost
```

For a server, set both values to the public HTTPS origin, for example:

```env
APP_HOST=deplens.example.com
APP_BASE_URL=https://deplens.example.com
```

That starts `caddy`, `web`, `api`, and `postgres` behind a single origin.

The deployment Compose stack pulls published images from GitHub Container
Registry:

- `ghcr.io/ferretsecurity/deplens-platform-api`
- `ghcr.io/ferretsecurity/deplens-platform-web`

Pin `DEPLENS_VERSION` to a released image tag such as `0.1.0` for production
deployments. Git release tags use `vX.Y.Z`; image tags omit the leading `v`.
`latest` is convenient for quick trials but makes upgrades implicit.

To build the API and web images from local source instead:

```bash
cd deploy/compose
docker compose -f compose.yml -f compose.build.yml up -d --build
```

The root `docker-compose.yml` is for contributor development and only starts
local infrastructure. Production and local product evaluation should use
`deploy/compose/compose.yml`.

The Compose stack builds its container database URL from `POSTGRES_PASSWORD`.
Use a host-local `.env` file and do not commit real deployment secrets.

## Release images

Pushing a `vX.Y.Z` tag publishes `linux/amd64` API and web images to GitHub Container Registry:

- `ghcr.io/ferretsecurity/deplens-platform-api`
- `ghcr.io/ferretsecurity/deplens-platform-web`

The workflow publishes the `X.Y.Z` tag for every release. Stable releases also publish moving `X.Y`, `X`, and `latest` tags. The build attaches Docker provenance and SBOM attestations and publishes a GitHub artifact attestation for each image digest.
