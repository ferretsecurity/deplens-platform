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

The session stores the user ID and tenant memberships. If the user belongs to one tenant, it also stores the active tenant and role.

## Inviting a user

V1 reserves tenant invite storage for a simple flow:

- tenant `owner` or `admin` invites a user by email
- the invite defaults to `viewer`
- once accepted, the user gets a `tenant_membership` for that tenant
- role changes can be handled later through a simple admin workflow

## First upload

```bash
curl -X POST http://localhost:8080/api/v1/scans \
  -H 'Authorization: Bearer change-me' \
  -H 'Content-Type: application/json' \
  -d '{
    "schema_version":"v1alpha1",
    "project":{"slug":"core","name":"Core"},
    "repository":{"slug":"repo","name":"Repo","url":"https://example.com/repo.git","default_branch":"main"},
    "source":{"commit_sha":"abc123","ref":"refs/heads/main","scanned_at":"2026-05-04T10:00:00Z"},
    "snapshot":{"root":".","manifests":[]}
  }'
```
