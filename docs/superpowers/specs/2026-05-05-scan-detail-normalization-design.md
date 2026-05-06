# Scan Detail Normalization Design

**Date:** 2026-05-05

**Goal**

Persist manifest and dependency detail from uploaded Deplens scan payloads so the platform can return per-scan manifest/dependency data instead of only scan-level summary counters.

**Current State**

The upload pipeline accepts either the wrapped platform payload or raw `deplens -json` output plus metadata headers. The service stores the full JSON artifact in blob storage and persists only scan-level metadata and summary counters into `scans`. There are no relational tables for manifest or dependency detail, so the read APIs cannot return parsed scan contents.

**Scope**

This change will:

- Persist manifests for every uploaded scan
- Persist dependencies for every manifest in that scan
- Preserve manifest and dependency input order
- Keep scan artifact blob storage unchanged
- Keep `GET /api/v1/scans/{scan_id}` as a lightweight summary endpoint
- Add a new detail endpoint for scan manifests and dependencies

This change will not:

- Add cross-scan dependency search
- Add project-level or repository-level dependency analytics
- Add asynchronous parsing or background jobs
- Deduplicate manifests or dependencies across scans

## API Design

### Existing endpoint kept stable

`GET /api/v1/scans/{scan_id}` remains a scan summary endpoint. It should continue returning only scan-level fields such as project slug, repository slug, commit SHA, timestamps, summary counts, labels, and annotation.

### New endpoint

Add `GET /api/v1/scans/{scan_id}/manifests`.

Response shape:

```json
{
  "items": [
    {
      "id": "manifest-uuid",
      "type": "npm-package-lock",
      "path": "frontend/package-lock.json",
      "has_dependencies": true,
      "warnings": [],
      "dependencies": [
        {
          "id": "dependency-uuid",
          "raw": "react@19.1.0",
          "name": "react",
          "version": "19.1.0",
          "constraint": "",
          "section": "dependencies",
          "source": "npm",
          "extras": []
        }
      ]
    }
  ]
}
```

Ordering rules:

- manifests ordered by input position ascending
- dependencies ordered by input position ascending

Authorization rules:

- same bearer-token model as existing read APIs
- require `scan:read`

## Data Model

Add two new tables.

### `scan_manifests`

- `id uuid primary key default gen_random_uuid()`
- `scan_id uuid not null references scans(id) on delete cascade`
- `position integer not null`
- `type text not null`
- `path text not null`
- `has_dependencies boolean null`
- `warnings jsonb not null default '[]'::jsonb`
- `created_at timestamptz not null default now()`

Constraints and indexes:

- unique `(scan_id, position)` to preserve deterministic ordering
- index on `(scan_id, position)`

### `manifest_dependencies`

- `id uuid primary key default gen_random_uuid()`
- `manifest_id uuid not null references scan_manifests(id) on delete cascade`
- `position integer not null`
- `raw text not null`
- `name text not null default ''`
- `version text not null default ''`
- `constraint text not null default ''`
- `section text not null default ''`
- `source text not null default ''`
- `extras jsonb not null default '[]'::jsonb`
- `created_at timestamptz not null default now()`

Constraints and indexes:

- unique `(manifest_id, position)` to preserve deterministic ordering
- index on `(manifest_id, position)`

Notes:

- `has_dependencies` must stay nullable because the input model distinguishes between `false` and unknown
- `warnings` and `extras` stay JSON arrays because their structure is already array-like and simple

## Write Path Design

The upload flow remains synchronous and transactional.

1. Decode wrapped upload payload or raw Deplens payload
2. Store the full upload artifact in blob storage
3. Compute summary counters as done today
4. Open a DB transaction
5. Upsert project and repository
6. Insert the `scans` row
7. Insert one `scan_manifests` row per manifest
8. Insert one `manifest_dependencies` row per dependency
9. Commit the transaction

If any database insert fails, rollback the entire upload. Blob storage remains best-effort as it is today; this design does not try to garbage-collect an artifact written before a failed transaction.

### Parsing boundary

There is no separate parser stage beyond the existing decoded upload payload. Deplens already sends manifests and dependencies in structured JSON. The platform should normalize that already-parsed structure into relational storage rather than reparsing manifest files from source.

## Read Path Design

Add a store read method that returns manifest rows with nested dependencies for one scan scoped to one tenant.

Suggested shape:

- query manifests for `scan_id` joined through `scans` to enforce `tenant_id`
- query dependencies for those manifest IDs ordered by `position`
- assemble nested response in Go

This is preferable to returning one wide join result directly from the HTTP layer because:

- the nested output shape is clearer to build in the store/service layer
- it avoids repeating manifest fields for every dependency row in the handler
- it leaves room for future filters such as `include=dependencies`

## Code Organization

Expected code changes:

- `db/migrations/`: add migration for `scan_manifests` and `manifest_dependencies`
- `internal/store/uploads.go`: extend transactional upload persistence to insert normalized detail rows
- `internal/store/models.go`: add parameter and response structs for manifest/dependency persistence and reads
- `internal/store/scans.go`: add scan manifest read methods and response mapping
- `internal/http/query_handlers.go`: add `GET /api/v1/scans/{scan_id}/manifests`
- `api/openapi.yaml`: document the new endpoint and schemas
- `README.md`: add a usage example for reading scan manifests

## Testing Strategy

### Upload persistence tests

Add or extend store integration tests to verify:

- upload inserts the scan row
- upload inserts all manifest rows
- upload inserts all dependency rows
- nullable `has_dependencies` is preserved
- warnings and extras are preserved
- cascading delete semantics are provided by foreign keys

### HTTP handler tests

Add handler tests to verify:

- `GET /api/v1/scans/{scan_id}/manifests` returns `200` with nested detail
- missing or invalid auth is rejected consistently with existing read APIs

### Upload shape coverage

Retain coverage for both upload forms:

- wrapped platform payload
- raw `deplens -json` payload plus headers

At least one test should prove a raw Deplens upload results in persisted manifest/dependency detail.

## Migration and Compatibility

- Existing scan records will continue to exist without normalized manifest/dependency rows
- The new endpoint should return an empty `items` array for such older scans
- No existing endpoint shape needs to change

## Risks

1. Upload latency increases with scan size
   This is acceptable for now because there is no background job system and the product needs correctness more than throughput.

2. Large manifest/dependency scans may increase transaction time
   Keep inserts simple and ordered. Revisit batching only if real scans show it is necessary.

3. Old scans have no normalized detail
   The API should handle that cleanly rather than trying to backfill immediately.

## Recommendation

Implement normalized persistence on write with a dedicated `GET /api/v1/scans/{scan_id}/manifests` endpoint. This is the simplest design that materially improves the product without introducing asynchronous processing, dual-state behavior, or premature aggregation features.
