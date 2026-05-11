# API Token Management Design

## Goal

Add a full-stack API token management page to the authenticated web UI. Owner and admin users should be able to view token metadata, issue a new token, edit token label and scopes, and delete tokens for the active tenant.

API token secrets are immutable and are shown only once at creation time. Rotation is an explicit operational workflow: create a replacement token, update CI systems or scanners to use it, then delete the old token.

## Scope

This feature includes:

- Backend endpoints for list, create, update, and delete token metadata.
- Store methods for active-tenant token operations.
- OpenAPI documentation for the token endpoints.
- Web API helpers and types.
- A new `/app/tokens` UI page and app-shell navigation entry.
- Tests for backend authorization, tenant scoping, metadata behavior, and frontend token workflows.

This feature does not include:

- Secret rotation in place.
- Last-used tracking.
- Soft delete, revoke state, or audit history.
- Displaying token hashes, token prefixes, or plaintext secrets after creation.

## Backend API

Extend `/api/v1/tokens` from create-only to full CRUD:

- `GET /api/v1/tokens`
  - Returns token metadata for the active tenant.
  - Response items include `id`, `label`, `scopes`, and `created_at`.
  - Does not return token hashes or plaintext token secrets.

- `POST /api/v1/tokens`
  - Creates a token for the active tenant.
  - Request body includes `label` and `scopes`.
  - Response includes token metadata plus the plaintext `token`.
  - The plaintext token is returned only in this response.

- `PATCH /api/v1/tokens/{id}`
  - Updates token metadata for the active tenant.
  - Request body includes `label` and `scopes`.
  - Does not change the secret or token hash.
  - Response includes updated metadata only.

- `DELETE /api/v1/tokens/{id}`
  - Hard-deletes the token row for the active tenant.
  - Returns `204 No Content` on success.

All token-management endpoints require an authenticated browser session with role `owner` or `admin`. Viewer users and anonymous requests are rejected. Token IDs are always scoped by `active_tenant_id`; a token ID from another tenant is treated as not found.

## Store

Use the existing `api_tokens` table. No database migration is needed for this version because the table already stores:

- `id`
- `tenant_id`
- `label`
- `token_hash`
- `scopes`
- `created_at`

Add store methods for:

- Listing token metadata by tenant ID.
- Updating `label` and `scopes` by token ID and tenant ID.
- Deleting by token ID and tenant ID.

The store must never return `token_hash` to HTTP response models.

## Web UI

Add an `API Tokens` navigation item to the authenticated app shell and add a `/app/tokens` page.

The page shows a table with:

- Label.
- Scopes.
- Created date.
- Row actions for edit and delete.

The issue flow:

- Opens a form for label and scopes.
- Creates the token through `POST /api/v1/tokens`.
- Shows the plaintext secret in a one-time reveal panel after success.
- Makes clear that the secret cannot be shown again.
- Refreshes the token list after creation.

The edit flow:

- Allows changing only label and scopes.
- Sends `PATCH /api/v1/tokens/{id}`.
- Updates the table after success.

The delete flow:

- Uses an explicit confirmation state.
- Explains the rotation workflow: create a replacement, update CI/scanners, then delete the old token.
- Sends `DELETE /api/v1/tokens/{id}` and removes the row after success.

Use the current app shell, shadcn-style UI components, Tailwind styling, and lucide icons. The design should remain compact and operational, consistent with the existing repositories and scans pages.

## Validation

Token labels must be non-empty after trimming. Scopes must contain at least one allowed scope. The initial allowed scopes are:

- `scan:read`
- `scan:write`
- `scan:metadata:write`

Validation should be enforced server-side. The UI should use the same scope set for form controls so users cannot accidentally submit unsupported scopes.

## Error Handling

Backend errors:

- Invalid JSON returns `400 Bad Request`.
- Empty labels or invalid scopes return `400 Bad Request`.
- Anonymous, viewer, or missing active-tenant sessions return `401 Unauthorized`.
- Token IDs outside the active tenant return `404 Not Found`.

UI errors:

- Form validation errors are shown inline.
- Mutation failures keep the current table visible.
- The one-time secret panel remains visible after a successful creation until the user dismisses it or leaves the page.

## Testing

Backend tests should cover:

- Listing token metadata without leaking secrets or hashes.
- Creating a token returns plaintext once.
- Updating label and scopes does not change the secret.
- Deleting removes token access.
- Viewer and anonymous sessions are rejected.
- Cross-tenant token IDs cannot be updated or deleted.
- Invalid labels and scopes are rejected.

Store tests should cover list, update, and delete SQL behavior against the existing schema.

Frontend tests should cover:

- The page renders token metadata.
- The issue flow submits label/scopes and displays the one-time token.
- The edit flow submits only label/scopes.
- The delete flow requires confirmation and removes the token from the UI after success.
- The app shell links to `/app/tokens`.

OpenAPI tests should be updated so the documented paths include `GET`, `POST`, `PATCH`, and `DELETE` token operations.
