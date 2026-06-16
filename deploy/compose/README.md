# Compose Deployment

This directory contains the self-hosted deployment contract for a single-node install.

## Start

1. `cp deploy/compose/.env.example .env`
2. Update `APP_HOST`, `DEPLENS_VERSION`, and the bootstrap credentials in `.env`
3. `docker compose pull`
4. `docker compose up -d`

The stack starts:

- `postgres`
- `api`
- `web`
- `caddy`

## First Login

Open `https://$APP_HOST/login` and sign in with the bootstrap owner credentials from `.env`.

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

Use the build override when developing locally or testing a private fork from source:

```bash
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## Notes

- `api` reads migrations from the bundled `/src/db/migrations` path at startup.
- Compose uses `API_DATABASE_URL` for the backend container. Do not reuse a local-only `DATABASE_URL=...@localhost...` value here, because `localhost` inside the container is not the `postgres` service.
- `caddy` needs `APP_HOST` in its environment so the Caddyfile can render the site address.
- `web` talks to `api` over the private Compose network and serves the browser UI from a single origin.
