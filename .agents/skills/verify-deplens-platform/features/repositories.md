# Repositories

Users search imported repositories, open their manifest files, and navigate to scan history.

## Sub-features

- `repo-search`: matching, no-match and clear states.
- `repo-manifests`: inspect imported files.
- `repo-scans`: open repository scan history.
- `repo-pages`: navigate paginated results.

## How to get to it (user POV)

Sign-in lands on `/app/repositories`. Navigation `Repositories` and the direct URL reach the same list. Each row exposes `Manifest files` and `Open scans`.

## Driving it with Playwright

Preconditions: upload the `Verification repo` fixture from [scans](scans.md).

Click `getByRole('link', { name: 'Repositories', exact: true })`. Fill `getByRole('searchbox', { name: 'Search repositories' })` with `Verification` and click `Search`. Require the fixture row. Search for `no-such-verification-repo` and require `No repositories match this search.` Click the `Clear` link and require the fixture row again.

Within `getByRole('row').filter({ hasText: 'Verification repo' })`, click `Manifest files`. Require `package.json` in the displayed inventory. Return to Repositories, click the same row's `Open scans`, and require its `abc123` scan. Capture each navigation and resulting URL plus inventory. Repeat via the direct list URL to cover both list entry points.

## Gotchas

One fixture cannot prove `Next`/`Previous` behavior. Pagination needs at least 26 distinct uploaded repositories at the default page size. Manifest comparison needs at least two relevant inventory versions; the one-scan fixture does not meet that precondition. An empty inventory is not a substitute for either journey.
