# Dependencies

Users inspect active dependencies and their repository and manifest usage counts.

## Sub-features

- `dependency-empty`: empty inventory message.
- `dependency-list`: populated dependency name and usage counts.
- `dependency-pages`: navigate larger inventories.

## How to get to it (user POV)

Choose `Dependencies` in navigation or visit `/app/dependencies` directly.

## Driving it with Playwright

Preconditions: owner session. Use a fresh database for empty-state proof; upload the [scan fixture](scans.md) for populated proof.

Click `getByRole('link', { name: 'Dependencies', exact: true })` and require `No active dependencies found.` Upload the fixture through the public API, reload this page, and require `getByRole('row').filter({ hasText: 'react@19.1.0' })` to show one repository and one manifest file. Compare with the public `/api/v1/dependencies` response and save it alongside a screenshot and ARIA snapshot. Visit `/app/dependencies` directly and assert the same persisted row.

## Gotchas

Pagination uses 100 rows per page. The fixture provides only one dependency; proving another page requires at least 101 distinct active dependencies. A single scan cannot prove that removed dependencies stop appearing after later uploads. Add a later scan without React before claiming that lifecycle behavior.
