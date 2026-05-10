# Compose Deployment

This directory contains the self-hosted deployment contract for a single-node install.

## Start

1. `cp deploy/compose/.env.example .env`
2. Update `APP_HOST` and the bootstrap credentials in `.env`
3. `docker compose up -d --build`

The stack starts:

- `postgres`
- `api`
- `web`
- `caddy`

## First Login

Open `https://$APP_HOST/login` and sign in with the bootstrap owner credentials from `.env`.

## Notes

- `api` reads migrations from the bundled `/src/db/migrations` path at startup.
- `caddy` needs `APP_HOST` in its environment so the Caddyfile can render the site address.
- `web` talks to `api` over the private Compose network and serves the browser UI from a single origin.
