# Deplens verification map

Use the parent skill to launch a fresh stack. Baseline is one owner, one default tenant, and no inventory. All browser recipes use the Playwright objects from `scripts/auth.mjs` and a desktop viewport. Start each journey with UI sign-in unless testing authentication itself.

- [Authentication](auth.md): protected routes, credentials, persisted sessions and logout. Executed by the default driver.
- [API tokens](tokens.md): issue, edit, cancel and delete scanner credentials.
- [Repositories](repositories.md): search, clear, manifest files and scan entry points.
- [Scans](scans.md): public upload, repository/date filters and scan details.
- [Package search](packages.md): concrete versions, constraints, pagination and clear. Executable LiteLLM driver.
- [Vulnerability assessments](assessments.md): manifest and lockfile evidence, affected ranges, and the scanner-to-browser journey.
- [Dependencies](dependencies.md): active inventory and pagination.

The token, repository, scan and dependency journeys are source-grounded recipes, not claims of executed coverage. Each proof records the feature ID and entry point, action trace, resulting screenshot/ARIA snapshot, and public readback for mutations. A path listed in a feature remains unverified until driven, even if another entry point passed.

Socket integration is additional coverage outside this seed map. Real imports need an encryption key, authorized Socket credentials and populated organization. Consult `docs/socket-integration.md`; mocked Socket browser tests do not establish that those preconditions hold. Mobile navigation and non-owner permissions also need separate coverage.
