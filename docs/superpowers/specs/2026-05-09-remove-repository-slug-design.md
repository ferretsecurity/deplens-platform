# Remove Repository Slug Design

## Goal

Remove repository slug from the platform entirely.

After this change:

- repository API identity is `id`
- ingest matches repositories by `name`
- ingest overwrites stored `url` and `default_branch` when a repository name matches
- no request, response, schema, query, header, or persistence path refers to repository slug

## Current State

Repositories currently carry a tenant-scoped `slug`.

That slug is used in several places:

- upload payloads require `repository.slug`
- raw upload headers accept `X-Deplens-Repository-Slug`
- scan queries filter by `repository_slug`
- scan responses return `repository_slug`
- repository persistence uses `unique (tenant_id, slug)` and upserts on slug

This creates a second repository identifier beyond the existing UUID primary key.

## Decision Summary

### Repository identity

Repository `id` becomes the only API identifier for repositories.

API consumers should refer to repositories by UUID, not by a human-assigned stable string.

### Ingest identity

Repository ingest matches on `(tenant_id, name)`.

If an upload arrives for an existing repository name:

- the existing repository row is reused
- `url` is overwritten from the upload
- `default_branch` is overwritten from the upload

Repository names therefore become tenant-scoped unique.

### Compatibility

There is no backward-compatibility layer for repository slug.

Clients must stop sending repository slug and must stop querying by `repository_slug` in the same rollout.

## API Changes

### Upload payloads

`RepositoryInput` removes `slug`.

Repository upload payloads contain:

- `name`
- `url`
- `default_branch`

### Raw upload headers

Remove `X-Deplens-Repository-Slug`.

Raw upload handling must not read, validate, or require that header.

### Query parameters

Replace `repository_slug` filters with `repository_id`.

This applies to scan-listing endpoints and any internal request parsing that still maps query parameters to repository slug.

### Responses

Remove `repository_slug` from API responses.

Where scan summaries currently expose repository identity, return `repository_id` instead.

Repository list responses should expose:

- `id`
- `name`
- `url`
- `default_branch`

No response body should contain repository slug after this change.

## Persistence Changes

### Repository schema

Update the `repositories` table:

- drop `slug`
- replace `unique (tenant_id, slug)` with `unique (tenant_id, name)`

The table continues to use the existing UUID `id` primary key.

### Repository upsert

Repository ingest upserts by `(tenant_id, name)`.

On conflict:

- keep the existing row identity
- update `name` from the incoming row
- update `url` from the incoming row
- update `default_branch` from the incoming row

Although `name` is also the match key, updating it in the upsert keeps the SQL shape explicit and aligned with the ingest payload.

### Scan reads

Queries that currently join repositories to fetch slug should instead fetch repository `id` when repository identity is needed in scan responses.

## Migration Strategy

Use a direct cutover.

Migration steps:

1. update code paths and tests to stop referencing repository slug
2. change repository ingest and query behavior to use name and id
3. migrate the database schema by dropping the slug column and adding the tenant/name uniqueness rule
4. update API documentation and examples so slug does not appear anywhere

No repository data backfill is required because repository rows already have UUID primary keys.

## Error Handling

The system should not attempt to infer repository identity from removed slug fields.

Expected behavior:

- upload requests containing only `name`, `url`, and `default_branch` decode successfully
- raw upload requests succeed without a repository slug header
- scan list requests using `repository_id` parse successfully
- old clients using `repository_slug` or `X-Deplens-Repository-Slug` are unsupported after rollout

## Testing Requirements

Tests should cover:

1. upload decoding without repository slug in both JSON and raw-upload paths
2. repository upsert by repository name with overwrite behavior for `url` and `default_branch`
3. scan query parsing and filtering by `repository_id`
4. API responses and OpenAPI output no longer containing repository slug fields or headers
5. repository list responses still returning stable repository metadata including `id`

## Non-Goals

- preserving repository slug as a deprecated alias
- adding alternate human-readable repository identifiers
- adding migration shims for old clients
- changing tenant slug or auth-provider slug behavior
