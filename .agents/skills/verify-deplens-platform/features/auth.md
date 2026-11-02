# Authentication

An owner signs in, retains access across reloads, and signs out so protected data is no longer accessible.

## Sub-features

- `auth-protected`: anonymous protected routes redirect.
- `auth-invalid`: incorrect credentials show an error.
- `auth-login`: owner session survives reload.
- `auth-logout`: sign-out invalidates the session.

## How to get to it (user POV)

Visit `/login`, or visit `/app/repositories` without a session. Sign out through the account menu in the header.

## Driving it with Playwright

Preconditions: the fresh stack and bootstrap owner from the parent skill.

Run the default `scripts/verify.py`. Its `auth.mjs` opens the protected route, fills `getByLabel('Email', { exact: true })` and `getByLabel('Password', { exact: true })`, then clicks the `Sign in` button. Wrong credentials must show `Login failed. Check your credentials and try again.` Valid credentials must reach `/app/repositories` and show `No repositories have been imported yet.`

After reload, the API Tokens navigation remains visible and `context.request.get('/auth/me')` returns HTTP 200 with role `owner`. Open the header button matching the user's display name or ID and click `getByRole('menuitem', { name: 'Sign out' })`. `/auth/me` must then return 401 and a protected-page visit must redirect to `/login`.

## Gotchas

The baseline proves owner access only. The default run exercises the login form reached by redirect; separately visit `/login` to claim its direct entry. Login page rendering alone is not evidence that database bootstrap succeeded.
