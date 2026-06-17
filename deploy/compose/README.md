# Compose Deployment

This directory contains the self-hosted deployment contract for a single-node
install. Use it for both server installs and local product evaluations where you
want to run the packaged service instead of the contributor development loop.

## Start

Run these commands from this directory:

1. `cp .env.example .env`
2. Update `APP_HOST`, `APP_BASE_URL`, `DEPLENS_VERSION`, and the secrets in `.env`
3. `docker compose pull`
4. `docker compose up -d`

The stack starts:

- `postgres`
- `api`
- `web`
- `caddy`

## First Login

Open `https://$APP_HOST/login` and sign in with the bootstrap owner credentials from `.env`.

For a local evaluation, keep:

```env
APP_HOST=localhost
APP_BASE_URL=https://localhost
```

For a server, set both values to the public HTTPS origin, for example:

```env
APP_HOST=deplens.example.com
APP_BASE_URL=https://deplens.example.com
```

## Image Versions

The default Compose file pulls published images from GitHub Container Registry:

- `ghcr.io/ferretsecurity/deplens-platform-api`
- `ghcr.io/ferretsecurity/deplens-platform-web`

Set `DEPLENS_VERSION` in `.env` to the image version you want to run. Production deployments should pin a released version such as `0.1.0`; `latest` is convenient for quick trials but makes upgrades implicit.

Release tags use the `vX.Y.Z` git tag format. Published image tags omit the leading `v`, so git tag `v0.1.0` publishes image tag `0.1.0`. Stable releases also publish moving `X.Y`, `X`, and `latest` tags.

The initial release workflow publishes `linux/amd64` images. Add multi-platform Dockerfile support before expanding the workflow to `linux/arm64`.

To verify a published image attestation with the GitHub CLI:

```bash
gh attestation verify \
  oci://ghcr.io/ferretsecurity/deplens-platform-api:$DEPLENS_VERSION \
  -R ferretsecurity/deplens-platform
```

To upgrade:

1. Change `DEPLENS_VERSION` in `.env`
2. `docker compose pull`
3. `docker compose up -d`

## Build From Source

Use the build override when testing a private fork from source with the same
self-hosted topology:

```bash
docker compose -f compose.yml -f compose.build.yml up -d --build
```

## Notes

- `api` reads migrations from the bundled `/src/db/migrations` path at startup.
- The root `docker-compose.yml` is for contributor development and only starts local infrastructure.
- Compose builds the backend container database URL from `POSTGRES_PASSWORD`. Do not reuse a local-only `DATABASE_URL=...@localhost...` value here, because `localhost` inside the container is not the `postgres` service.
- `caddy` needs `APP_HOST` in its environment so the Caddyfile can render the site address.
- `web` talks to `api` over the private Compose network and serves the browser UI from a single origin.
- Back up the `postgres-data`, `blobs-data`, `caddy_data`, and `caddy_config` volumes before upgrades.
