# Configure Snyk imports

1. Set `SNYK_ENCRYPTION_KEY` to a base64 encoded, random 32-byte key. Keep it outside PostgreSQL and preserve it across restarts.
2. Run the database migrations and start the API and web app.
3. Sign in as an owner or administrator and open **Snyk integration**.
4. Enter your organization UUID, select its Snyk region, and enter an API token with permission to read targets, projects, and dependency graphs.
5. Save the configuration, test the connection, and select **Sync now**. Choose an interval to enable scheduled imports.
6. Open **Repositories** and select a repository to compare Deplens, Socket, and Snyk manifest paths and inspect imported dependencies.

For a temporary OAuth access token, choose bearer authentication. OAuth tokens expire and the integration does not refresh them. Use an API token for unattended synchronization.

Find the organization UUID in Snyk organization settings. The organization slug used in browser URLs is a different value. See the official [REST API setup guide](https://docs.snyk.io/snyk-api/rest-api/getting-started-with-the-rest-api) and [API authentication guide](https://docs.snyk.io/snyk-api/authentication-for-api).

An omitted token preserves the saved credential. API responses show whether a token is configured and never return its value. Disconnect removes this tenant's Snyk configuration and imports. Deplens scans and Socket imports remain.

Snyk projects represent individual manifests or analysis types. The importer groups projects under their Snyk target rather than creating one repository for every project. The number of projects can therefore exceed the number of repositories. A repository with no successful Snyk import cannot appear as covered merely because it was in the original selection.

Imports list all target and project pages through Snyk REST API version `2024-10-15`. Package graphs use the Snyk V1 dependency graph endpoint. Code and infrastructure analysis projects do not have package graphs. Many CLI-imported projects have no reported manifest path. The comparison marks their coverage as unknown rather than treating an absent path as a missed file. Imported file paths come from project metadata and do not represent an archive of repository files. Different providers may have scanned different commits, branches, and files.

Manual and scheduled imports use a persisted queue with renewable leases. A repository snapshot publishes atomically. Failed repository imports retain their previous snapshot and produce a partial run. A complete successful listing marks missing repositories unavailable. Credentials use AES-256-GCM encryption with tenant and integration identity as authenticated data.

Run the project checks from the repository root:

```sh
go test ./cmd/... ./internal/...
pnpm test:web
pnpm lint:web
pnpm build:web
```
