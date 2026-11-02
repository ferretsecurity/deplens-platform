---
name: verify-deplens-platform
description: Verify Deplens Platform web login, tokens, repositories, scans, and dependency inventory against a disposable real API and Postgres. Use after user-facing changes or to reproduce a product failure locally.
---

# Verify Deplens Platform

Read [the feature index](features/README.md), then the feature being changed. The primary interface is the Next.js web app. Scanner uploads also use the public HTTP API. This recipe verifies local source; it does not prove packaged images, Caddy, TLS, or a deployed server.

## Launch

From the repository root, run:

```sh
.agents/skills/verify-deplens-platform/scripts/verify.py
```

Requires Python 3, Go, Node, Corepack and a working Docker daemon. The helper installs locked pnpm dependencies and Playwright Chromium. Missing Chromium system libraries must be installed before retrying. It builds the API, copies the web source into scratch storage, links installed dependencies, and runs Next.js dev there. It never loads or edits `.env`.

Each run creates its own Postgres 17 container, database, blob directory, API process and web process. Docker assigns the database port; the API and web use free loopback ports. A bind race fails the run rather than reusing another server. Concurrent runs have separate `.next` output and browser contexts; dependency installation is shared, so finish installation before starting concurrent runs.

The API applies `db/migrations` and bootstraps tenant `default` with owner `verify@example.com`, password `verification-only-password`. The database starts empty. Repository, scan and dependency journeys need the additional fixture described in [scans](features/scans.md). There is no Socket credential in this stack.

Readiness requires a TCP SQL query after Postgres initialization, API `/readyz`, the web login page, and the doctor below. The authentication drive then proves bootstrap credentials really work. Evidence prints before launch. The helper tears down automatically after success, failure, SIGTERM or Ctrl-C.

## Doctor

The helper performs one read-only doctor pass before driving: both owned child processes must be alive, the API must return success from `/readyz`, and anonymous `/app/repositories` must redirect to `/login`. It writes the URLs, PIDs, container name and source revision to `doctor.json`. The binary was built from this checkout and the web source copied from it; `working-tree.diff` records tracked local changes.

When something looks off, repeat those checks on the URLs in that run's `doctor.json` while the helper is running. Require that its PIDs are still the owned children; an old evidence directory describes a stopped instance. `/readyz` alone only proves database connectivity, not authentication. After sign-in, `GET /auth/me` must return the expected owner. An HTTP error or process exit fails verification; inspect `api.log`, `web.log`, and `drive.log` before retrying.

## Drive

The executable `scripts/auth.mjs` uses the repo's installed Playwright library with an isolated Chromium context. The default helper runs it against the real stack. It checks anonymous redirect, rejected credentials, successful login, session persistence after reload, owner identity, logout, and rejection of the old session.

For another mapped journey, copy `auth.mjs` to a scratch `.mjs` file and replace its action block with that feature's Playwright steps. Keep the browser/context setup, tracing and `finally` cleanup. Invoke it through the same lifecycle:

```sh
VERIFY_DRIVER=/absolute/path/to/journey.mjs .agents/skills/verify-deplens-platform/scripts/verify.py
```

The driver receives `VERIFY_WEB`, `VERIFY_ROOT` and `VERIFY_EVIDENCE`. Use `page`, `context.request`, and `expect` as in the shipped driver. Record each action in `drive.log`. Existing `apps/web/e2e/auth.spec.ts` covers anonymous behavior only. Socket specs mock network responses and cannot prove real imports.

## Evidence

Proof survives in `var/verification/<timestamp>-<id>/`, ignored by git. Default artifacts include `doctor.json`, logs, `auth-me.json`, ARIA snapshot, before/after screenshots, `trace.zip`, `result.json` and `cleanup.json`. A missing `result.json` means the drive failed. Read the logs even when cleanup succeeded.

Exercise real user paths. Capture actions and their resulting state, using the trace and explicit assertions, rather than a final screenshot alone. Confirm mutations through another public read or page reload. Uploads should also leave blobs and persisted scan data; inspect both when verifying upload storage. Use mocks only for an already isolated external boundary and label that limitation. There is no dry-run mode: this creates real disposable database rows and downloads dependencies/images as needed.

The trace contains disposable credentials and session cookies. Keep artifacts local; do not publish raw traces. Record feature IDs and entry points covered. A missing fixture is a failed precondition, not a passing empty-state substitute.

## Cleanup

The helper stops only its owned process groups, waits for exit, removes its named container and anonymous volumes, and deletes its scratch directory. It retains evidence and installed dependency/browser caches. `cleanup.json` confirms cleanup; verify the trace and screenshots still exist afterward.

After an uncatchable termination, use that run's `doctor.json` and process command lines to confirm ownership before stopping its PIDs and removing its named container. Never kill by process name or remove the development Compose volume. The scratch path is visible in the owned API command line. A cleanup error must be resolved before starting another run.

## Helpers

- `scripts/verify.py` is the executable launch, doctor, drive and cleanup supervisor. Invocation is above.
- `scripts/auth.mjs` is the executable default Playwright journey. Run it through the supervisor so its environment and target are isolated.

Use `/maintain-verification-skill` when routes, selectors or fixture requirements change.
