# Configure Socket imports

1. Set `SOCKET_ENCRYPTION_KEY` in your deployment secret store. Use a base64 encoded, cryptographically random 32-byte key. Keep the same key across replicas and restarts. Keep it outside PostgreSQL and back it up separately. The application does not generate or store this deployment key.
2. Set `APP_BASE_URL` to the browser-facing application origin. This allows same-origin JSON mutations through the web proxy.
3. Run the normal database migrations and start the API. The API process runs the synchronization worker and shuts it down on SIGINT or SIGTERM.
4. Sign in as an owner or administrator. Open **Socket integration**.
5. Enter the Socket organization slug and an organization API token with `repo:list` and `full-scans:list`. No write permissions are needed. See [Socket API keys](https://docs.socket.dev/docs/api-keys).
6. Choose manual synchronization or an interval. Save the configuration, test the connection, and select **Sync now**.
7. Open **Repositories** to browse imported files and dependency references. The source badges distinguish CLI inventory and Socket inventory.

An omitted API token preserves the saved token. Disconnecting removes the saved token, Socket inventory, and synchronization history for that tenant. CLI repositories and scans remain. Disconnect before changing the Socket organization.

Without `SOCKET_ENCRYPTION_KEY`, configuration and imports are disabled. Existing inventory remains readable. A malformed key prevents startup. If you replace a valid key, save the Socket API token again before syncing.

Repositories link automatically to CLI inventory only when one CLI repository has the same normalized SCM URL. Names never establish identity. For other cases, open the repository's **Manage CLI repository mapping** control. Search for and select the matching CLI repository, or keep the Socket repository separate.

# Socket synchronization reference

The integration uses the official `https://api.socket.dev/v0` API. It lists all repository pages, including archived repositories and all workspaces. It imports each head full scan, its metadata, all artifacts and manifest references, and the names of submitted files from the tar archive. It never extracts the archive or stores file contents. Files with no package references and artifacts with no manifest references remain visible.

The latest snapshot for each Socket repository publishes atomically. An import failure keeps that repository's previous snapshot and produces a partial run. A successful complete listing marks missing repositories unavailable. Missing repositories never remove CLI data. Unavailable Socket inventory remains browsable but does not contribute to the active dependency totals.

Manual and scheduled runs use the same persisted queue. A partial unique index permits one queued or running synchronization per connection. A 60-second renewable lease coordinates replicas. Snapshot publication checks the run owner under a row lock. A new worker retries abandoned runs after lease expiry. The worker coalesces missed schedule intervals and computes the next interval after each attempt finishes. A browser can close without stopping the run.

A process handles one organization run at a time with four repository workers. Each client permits at most four concurrent requests. Requests have a 90-second timeout and at most four attempts. Retries cover HTTP 202, HTTP 429, server failures, and transport failures. `Retry-After` is honored up to 60 seconds. Longer waits stop the attempt so a later synchronization can retry. A run has a 30-minute deadline. Explicit server shutdown cancels the current run. Crashes leave it recoverable after lease expiry.

Each repository import limits responses and decompressed archive data to 128 MiB, a stream row to 8 MiB, archive entries to 100,000, and artifacts to 500,000. Exceeding a limit fails that repository and retains its old snapshot. All head scans are fetched on each synchronization because scan analysis may change without a new scan ID. Inventories store one current JSON snapshot per repository rather than snapshot history. File and dependency browsing displays 50 items per page. Global dependency groups include the ecosystem. CLI dependencies retain an unspecified ecosystem because the CLI upload schema does not supply one.

The configuration and mapping endpoints require a session and a current owner or admin database membership. Repository inventory requires a current tenant membership. Mutations require JSON and reject cross-origin browser requests. Tokens use AES-256-GCM with the tenant ID as authenticated data. API responses expose only `token_configured`. Raw upstream response bodies and credentials are not returned or logged.

# Reproduce the browser verification

The screenshots in `docs/pr/screenshots/socket-*.png` use synthetic fixture data. They do not show company repositories or a live Socket organization. Docker and the installed Playwright Chromium browser are required.

Start the disposable API fixture in one terminal from the repository root:

```sh
SOCKET_BROWSER_FIXTURE=1 go test ./internal/app -run '^TestSocketBrowserFixture$' -v -timeout 2h
```

Start the web app in another terminal:

```sh
API_INTERNAL_BASE_URL=http://127.0.0.1:18081 \
NEXT_PUBLIC_APP_BASE_URL=http://127.0.0.1:3100 \
pnpm --dir apps/web dev --hostname 127.0.0.1 --port 3100
```

Run the browser verification:

```sh
SOCKET_BROWSER_FIXTURE=1 PLAYWRIGHT_BASE_URL=http://127.0.0.1:3100 \
pnpm --dir apps/web exec playwright test e2e/socket.spec.ts --workers=1
```

The test uses `page.screenshot` through the Playwright CLI. It covers administrator login, configuration, saved token masking, a connection test, live synchronization progress, reload recovery, both source badges, Socket dashboard links, imported files, referenced and unreferenced dependencies, global dependency inventory, and viewer access. It also opens a Socket repository beyond the first 100 CLI rows. Stop both terminal processes after inspection. PostgreSQL runs in a dedicated disposable container.

Run the automated checks:

```sh
go test ./cmd/... ./internal/...
pnpm test:web
pnpm lint:web
pnpm build:web
```

The Socket Go tests use `httptest` and real PostgreSQL. A decoding check also reads existing private response fixtures if they are available under `var/socket-popular-100`. Those private fixtures are never copied into source control.

API references are [repository listing](https://docs.socket.dev/reference/getorgrepolist), [full scan streaming](https://docs.socket.dev/reference/getorgfullscan), and [submitted file archives](https://docs.socket.dev/reference/downloadorgfullscanfilesastar). The vendored [official Socket logomark](https://github.com/SocketDev/socket-cli/blob/main/assets/repo/logomark.svg) has attribution beside the SVG in `apps/web/public/icons`.

`go test ./...` also walks unrelated source checkouts under the local ignored `var` directory. If those checkouts contain incomplete Go examples, use the explicit project package paths above.
