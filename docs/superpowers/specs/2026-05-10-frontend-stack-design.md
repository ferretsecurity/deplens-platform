# Frontend Stack and Delivery Design

**Date:** 2026-05-10

**Scope:** Customer-facing logged-in web application for exploring repositories, scans, and dependency details in `deplens-platform`, plus Compose-first self-hosted delivery. Helm is intentionally deferred, but the runtime contract must remain compatible with adding it later.

## Summary

The frontend should be built as a Next.js App Router application inside this repository under `apps/web`, with the existing Go service remaining the source of truth for authentication, tenancy, tokens, and scan data. The repo becomes a light monorepo, not a split-repo system.

The browser should see a single origin:

- `/` served by the Next.js web app
- `/api/*` proxied to the Go API
- `/auth/*` proxied to the Go API

This preserves secure cookie-based auth without CORS complexity and keeps the future Helm shape aligned with the Compose shape.

## Recommended Stack

- `Next.js` App Router
- `React 19`
- `TypeScript`
- `pnpm` workspaces
- `Tailwind CSS v4`
- `shadcn/ui`
- `TanStack Query`
- `Zod`
- `React Hook Form`
- `Vitest` for client-side unit tests
- `Playwright` for end-to-end coverage
- `Docker` and `docker compose`
- `Caddy` as the initial reverse proxy

## Architecture Decisions

### 1. Use Next.js App Router, not a plain SPA

The current best-fit frontend is a Next.js App Router app, not a Vite SPA:

- React’s current guidance explicitly recommends a framework such as Next.js for full applications.
- The App Router is the most complete implementation of React Server Components today.
- The project needs authenticated app flows, nested layouts, loading boundaries, and container-friendly self-hosting more than it needs SPA minimalism.

### 2. Keep the backend authoritative for auth

The Go backend already owns:

- login
- session cookies
- tenant memberships
- roles
- API tokens

The frontend should not add a second auth authority. Instead:

- login posts to `/auth/login`
- current session is read from `/auth/me`
- logout posts to `/auth/logout`
- the backend owns cookie issuance and invalidation

### 3. Server-first data loading for authenticated routes

For the logged-in app, the first render of route data should be loaded in Server Components where practical. TanStack Query should be used selectively for:

- client-side refetching
- interactive filters
- optimistic updates
- mutation invalidation

This avoids turning TanStack Query into a universal data layer when App Router already gives strong server-side primitives.

### 4. Default authenticated routes to dynamic rendering

Because the app is session-based and user-specific, authenticated route segments should default to dynamic behavior initially:

- use explicit `cache: "no-store"` server fetches for user-specific API calls
- or mark authenticated route segments `dynamic = "force-dynamic"` where appropriate

This is the safe default for the first implementation. More aggressive caching can be added later, once the data boundaries are proven.

### 5. Use `proxy.ts` only for optimistic redirects

In newer Next.js documentation, `middleware` is now documented as `Proxy`. For this app:

- use `proxy.ts` only for cheap request-time checks and redirects
- do not make Proxy the source of truth for authorization
- do not do slow session or API work in Proxy

The route layout and backend session endpoints remain authoritative.

### 6. Tailwind CSS v4, not legacy v3 setup

Use the current Tailwind setup:

- `@tailwindcss/postcss`
- `postcss.config.mjs`
- `@import "tailwindcss";` in `globals.css`

Do not start from older Tailwind v3-style config patterns unless a dependency forces it.

### 6a. Use `shadcn/ui` as the component baseline, not as the entire design system

The frontend should use `shadcn/ui` for common interactive primitives:

- buttons
- inputs
- form fields
- dialogs
- dropdown menus
- tabs
- sheets
- tables

This does not replace Tailwind. It sits on top of Tailwind and provides accessible component scaffolding built around Radix primitives.

For this product:

- use `shadcn/ui` to accelerate delivery and consistency
- install only the components needed by the first product surfaces
- keep the component code in-repo and customize it as needed
- do not treat the default `shadcn/ui` appearance as the final visual identity of the product

### 7. Standalone Next.js output for containers

For self-hosting, use `output: "standalone"` in `next.config.ts` and build a minimal runtime image from `.next/standalone`. This keeps the web image smaller and better aligned with modern Next.js Docker guidance.

### 8. Compose first, Helm later

The supported deployment target for now is:

- `docker compose` on a single server

The runtime contract should still be Helm-friendly later:

- separate `web` and `api` images
- config via environment variables
- Postgres as the only required persistent service
- no browser-visible split between frontend and API origins

### 9. Testing strategy should match App Router realities

Next.js currently recommends preferring E2E coverage for async Server Components. That means:

- `Vitest` for client utilities and interactive client components
- `Playwright` for login, navigation, and key authenticated flows
- avoid over-investing in unit tests for async route components

## Delivery Shape

Initial Compose topology:

- `caddy`
- `web`
- `api`
- `postgres`

The reverse proxy must stay compatible with App Router streaming behavior. Compose and future Helm ingress configuration should avoid buffering behavior that breaks streaming.

## Changes Incorporated Into The Plan

The implementation plan has been updated to reflect this research:

- scaffold `apps/web` from current `create-next-app` defaults instead of hand-rolling outdated package baselines
- use Tailwind v4 setup
- use `shadcn/ui` for the initial component primitives
- add `Playwright` as first-class test coverage
- replace `middleware.ts` planning with `proxy.ts`
- keep authenticated routes dynamic by default
- use `output: "standalone"` for the web container
- keep TanStack Query scoped to client interactivity rather than all initial route data

## Sources

- [React: Creating a React App](https://react.dev/learn/creating-a-react-app)
- [Next.js App Router docs](https://nextjs.org/docs/app)
- [Next.js production checklist](https://nextjs.org/docs/app/guides/production-checklist)
- [Next.js self-hosting guide](https://nextjs.org/docs/app/guides/self-hosting)
- [Next.js `output: "standalone"`](https://nextjs.org/docs/app/api-reference/config/next-config-js/output)
- [Next.js Proxy docs](https://nextjs.org/docs/app/getting-started/proxy)
- [Next.js `fetch` and caching docs](https://nextjs.org/docs/app/api-reference/functions/fetch)
- [Next.js testing guide](https://nextjs.org/docs/app/guides/testing)
- [Tailwind CSS with Next.js](https://tailwindcss.com/docs/installation/framework-guides/nextjs)
- [TanStack Query advanced SSR guide](https://tanstack.com/query/latest/docs/framework/react/guides/advanced-ssr)
- [TanStack Query important defaults](https://tanstack.com/query/latest/docs/framework/react/guides/important-defaults)
- [Playwright introduction](https://playwright.dev/docs/intro)
