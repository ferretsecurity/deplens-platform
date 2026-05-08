# Repository and Manifest Lifecycle Design

**Date:** 2026-05-08

**Goal**

Simplify the v1 data model by removing `projects`, making `repositories` the top-level scanned asset, and introducing a durable `manifests` table that tracks manifest-file lifecycle across immutable scans.

**Current State**

The schema currently models scans as belonging to `tenants`, `projects`, and `repositories`. Scan detail is normalized into `scan_manifests` and `manifest_dependencies`, but manifest identity is still scan-scoped because `scan_manifests` stores `path` directly. There is no durable manifest record that survives across scans for the same repository.

This creates two modeling problems:

1. `projects` add complexity that is not needed for v1.
2. Manifest files do not have their own lifecycle state such as first seen, last seen, active/inactive, and user-managed labels.

**Scope**

This change will:

- Remove `projects` from the v1 schema and API model
- Make `repositories` the top-level scan target under a tenant
- Add a durable `manifests` table keyed by `(repository_id, path)`
- Keep `scans` immutable
- Make `scan_manifests` point to `manifests` instead of storing `path`
- Keep dependencies immutable and scan-scoped
- Track whether a manifest is currently active in the latest full-repository scan

This change will not:

- Add a manifest event log or separate history table
- Add project-level grouping or many-to-many project/repository modeling
- Make dependencies durable across scans
- Support partial-repository scan ingestion semantics

## Design Principles

1. `repositories` are the top-level scanned object in v1.
2. `scans` are immutable historical facts.
3. `manifests` are mutable current-state records for repository files.
4. `scan_manifests` are immutable observations of manifests in a specific scan.
5. `manifest_dependencies` are immutable observations attached to one `scan_manifest`.
6. Manifest labels are user-managed metadata and must never be overwritten by scan ingestion.

## Data Model

### `repositories`

Keep `repositories`, but remove `project_id`.

Fields:

- `id uuid primary key default gen_random_uuid()`
- `tenant_id uuid not null references tenants(id) on delete cascade`
- `slug text not null`
- `name text not null`
- `url text not null`
- `default_branch text not null`
- `created_at timestamptz not null default now()`

Constraints and indexes:

- unique `(tenant_id, slug)`

Notes:

- For v1, repository identity is tenant-scoped slug.
- This keeps repository upsert logic simple and matches the simplified top-level model.

### `manifests`

Add a durable manifest lifecycle table.

Fields:

- `id uuid primary key default gen_random_uuid()`
- `repository_id uuid not null references repositories(id) on delete cascade`
- `path text not null`
- `first_seen_at timestamptz not null`
- `last_seen_at timestamptz not null`
- `is_active boolean not null`
- `labels jsonb not null default '{}'::jsonb`

Constraints and indexes:

- unique `(repository_id, path)`
- index on `(repository_id, is_active, path)`

Notes:

- Manifest identity is based on `repository_id + path`.
- `first_seen_at` is set when the manifest is first created.
- `last_seen_at` is updated whenever the manifest appears in a new scan.
- `is_active` reflects whether the manifest was present in the latest full scan for that repository.
- `labels` are user-managed and are never touched by scan ingestion.

### `scans`

Keep `scans` immutable, but remove `project_id`.

Fields retained:

- `id`
- `tenant_id`
- `repository_id`
- `artifact_key`
- `artifact_sha256`
- `schema_version`
- `root_path`
- `commit_sha`
- `source_ref`
- `scanned_at`
- summary count fields
- `labels`
- `annotation`
- `created_at`

Constraints and indexes:

- keep uniqueness on `artifact_key`
- keep the repository/time index shape, updated to `(tenant_id, repository_id, scanned_at desc)`

Notes:

- Scan-level `labels` and `annotation` remain immutable metadata attached to that uploaded scan.

### `scan_manifests`

Keep scan-scoped manifest observations, but remove `path` and point to `manifests`.

Fields:

- `id uuid primary key default gen_random_uuid()`
- `scan_id uuid not null references scans(id) on delete cascade`
- `manifest_id uuid not null references manifests(id) on delete cascade`
- `position integer not null`
- `type text not null`
- `has_dependencies boolean null`
- `warnings jsonb not null default '[]'::jsonb`
- `created_at timestamptz not null default now()`

Constraints and indexes:

- unique `(scan_id, position)`
- unique `(scan_id, manifest_id)`
- index on `(scan_id, position)`

Notes:

- `type` stays here because it is an observed property of a specific scan result, not part of durable manifest identity.
- `has_dependencies` remains nullable because the scanner distinguishes `false` from unknown.

### `manifest_dependencies`

Keep dependency observations immutable and attached to one scan manifest.

Fields:

- `id uuid primary key default gen_random_uuid()`
- `scan_manifest_id uuid not null references scan_manifests(id) on delete cascade`
- `position integer not null`
- `raw text not null`
- `name text not null default ''`
- `version text not null default ''`
- `constraint text not null default ''`
- `section text not null default ''`
- `source text not null default ''`
- `extras jsonb not null default '{}'::jsonb`
- `created_at timestamptz not null default now()`

Constraints and indexes:

- unique `(scan_manifest_id, position)`
- index on `(scan_manifest_id, position)`

Notes:

- Dependencies stay scan-scoped in v1.
- `extras` remains JSON because scanner output contains structured per-ecosystem metadata.

## Write Path Design

The upload flow remains synchronous and transactional. Every upload is treated as a full snapshot of one repository.

Transaction flow:

1. Decode wrapped upload payload or raw Deplens payload
2. Store the full upload artifact in blob storage
3. Compute scan summary counters
4. Open a DB transaction
5. Upsert the repository
6. Insert the immutable `scans` row
7. For each manifest in input order:
   - upsert `manifests(repository_id, path)`
   - on first insert, set `first_seen_at = scanned_at`
   - set `last_seen_at = scanned_at`
   - set `is_active = true`
   - leave `labels` unchanged on conflict
   - insert one `scan_manifests` row pointing to the durable manifest
   - insert dependency rows for that `scan_manifest`
8. Mark every previously known manifest in the repository that was not present in this scan as `is_active = false`
9. Commit the transaction

Important write-path rule:

- Inactive detection is only correct if each upload represents the full repository snapshot.
- Partial scans are out of scope for this design and should not be silently treated as authoritative current state.

## Read Path Design

### Existing scan summary endpoint

`GET /api/v1/scans/{scan_id}` remains a summary endpoint. It should stop returning any project fields.

### Scan manifest detail endpoint

`GET /api/v1/scans/{scan_id}/manifests` remains the detail endpoint for immutable scan contents.

Response behavior:

- read `scan_manifests`
- join `manifests` to get durable `path`
- nest `manifest_dependencies` under each scan manifest
- order manifests by `position`
- order dependencies by `position`

This endpoint should return scan-time observation fields from `scan_manifests`, not mutable lifecycle state from `manifests`, except for `path`, which is now stored only on the durable manifest row.

### Future repository manifest endpoint

This schema enables a future repository-level endpoint such as `GET /api/v1/repositories/{id}/manifests` or `GET /api/v1/repositories/{slug}/manifests` without reconstructing current state from scan history.

That future endpoint can expose:

- `path`
- `first_seen_at`
- `last_seen_at`
- `is_active`
- `labels`

This is a direct consequence of making manifest lifecycle first-class.

## API Contract Changes

The upload contract should stop requiring project metadata.

Required repository/source metadata remains:

- repository slug
- repository name
- repository URL
- default branch
- commit SHA
- source ref
- scanned-at timestamp

For wrapped uploads:

- remove the top-level `project` object
- keep `repository`, `source`, and `snapshot`

For raw Deplens uploads:

- remove all `X-Deplens-Project-*` headers
- keep repository and source headers only

Read APIs should stop returning project fields.

## Migration Strategy

This redesign is a schema break, not a purely additive migration.

Recommended migration approach for v1:

1. Create a new migration that:
   - creates `manifests`
   - reshapes `repositories`
   - reshapes `scans`
   - reshapes `scan_manifests`
   - reshapes `manifest_dependencies` foreign keys if needed
2. Update application code and tests to target only the new model
3. Accept that local/dev data may need to be reset

Because this is still v1, it is reasonable to prefer a clean schema over a complex compatibility layer.

## Testing Strategy

### Store tests

Verify:

- repository upsert works without projects
- first upload creates repository, scan, manifest, scan manifest, and dependency rows
- repeated upload for the same path preserves one durable manifest row
- `first_seen_at` stays stable across scans
- `last_seen_at` advances on later scans
- absent manifests become `is_active = false`
- labels survive scan ingestion unchanged

### HTTP/API tests

Verify:

- upload endpoints accept repository-only metadata
- project headers and fields are no longer required
- scan summary responses no longer include project fields
- `GET /api/v1/scans/{scan_id}/manifests` still returns path and nested dependencies correctly through the new join

### Behavioral tests

Verify:

- scan immutability is preserved
- manifest lifecycle mutates only in `manifests`
- dependency rows remain immutable and scan-scoped

## Risks

1. Current code and docs assume project fields exist
   This redesign touches schema, upload decoding, store logic, query handlers, tests, OpenAPI, and README.

2. Inactive detection depends on full snapshots
   If partial scans are introduced later, this lifecycle model needs an explicit scan-completeness flag or a different reconciliation model.

3. Schema migration may be easier to implement as a reset in development
   That is acceptable at this stage of the product.

## Recommendation

Implement the repository-only v1 model with a durable `manifests` lifecycle table, immutable `scans`, immutable `scan_manifests` that reference durable manifests, and immutable scan-scoped dependencies. This removes an unnecessary layer from the current schema and creates the right foundation for current-state manifest APIs later.
