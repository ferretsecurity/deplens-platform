# Scans

Scanners upload snapshots; users inspect runs by repository and date and open their details.

## Sub-features

- `scan-upload`: accept and persist a public upload.
- `scan-filter`: select repository/date window.
- `scan-details`: inspect uploaded inventory and metadata.

## How to get to it (user POV)

Upload to `POST /api/v1/scans`. In the UI, choose `Scans`, choose a repository's `Open scans`, or visit `/app/scans?repository_id=<returned repository ID>`. Open a run with `Details`.

## Driving it with Playwright

Preconditions: owner session plus a token with `scan:write` and `scan:read`, issued through the token UI or public token API. The fresh baseline has no scans; create this fixture before inventory journeys.

Use `context.request.post('/api/v1/scans', { headers: { Authorization: 'Bearer ' + token }, data: payload })` with this payload, replacing `scanned_at` with `new Date().toISOString()`:

```json
{
  "schema_version": "v1alpha1",
  "repository": {"name": "Verification repo", "url": "https://example.com/verification.git", "default_branch": "main"},
  "source": {"commit_sha": "abc123", "ref": "refs/heads/main", "scanned_at": "2026-11-01T12:00:00Z"},
  "snapshot": {"root": ".", "manifests": [{"type": "npm", "path": "package.json", "has_dependencies": true, "dependencies": [{"raw": "react@19.1.0", "name": "react", "version": "19.1.0"}]}]}
}
```

Require successful upload and save response IDs. Read the scan and `/api/v1/scans/{scan_id}/manifests` back and check `package.json` and `react`. Capture the request without its Authorization header and the response. For storage changes, inspect the run's blob directory before teardown as well.

Click navigation `Scans`. Select the fixture through `getByLabel('Repository', { exact: true }).selectOption({ label: 'Verification repo' })`, fill `From` and `To` to include today, and click `Apply`. Require `abc123` in a row. Click its `Details` link and require the commit and uploaded inventory. Repeat through `Open scans` in the repository row and direct URL. Set a date window before the upload and require `No scans found for this window.`

## Gotchas

The default window is the last 30 days; stale fixture dates hide valid scans. A legacy snapshot has no native report, so it cannot prove native diagnostics, findings or report-download fidelity. Those need a native scanner report fixture. Upload success alone does not prove extraction was complete.
