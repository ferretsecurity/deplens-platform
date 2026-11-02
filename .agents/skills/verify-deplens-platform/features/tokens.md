# API tokens

Owners and admins manage scanner tokens; plaintext is shown only when issued.

## Sub-features

- `token-issue`: create a scoped token.
- `token-edit`: change label and scopes and persist them.
- `token-cancel`: abandon editing or deletion.
- `token-delete`: revoke a token.

## How to get to it (user POV)

Click `API Tokens` in navigation or visit `/app/tokens`. API clients can also use `/api/v1/tokens` with an owner/admin session.

## Driving it with Playwright

Preconditions: logged-in owner and an unused label `Verification scanner`.

Click `getByRole('link', { name: 'API Tokens', exact: true })`. Fill `getByLabel('Label', { exact: true })` with `Verification scanner`, check `getByLabel('scan:read', { exact: true })`, and click the `Issue token` button. Require a row containing that label and save the returned plaintext privately for a public bearer read of `/api/v1/repositories`.

Click `getByRole('button', { name: 'Edit Verification scanner', exact: true })`, change Label to `Updated scanner`, and click `Save token`. Reload and require the updated row. Repeat an edit and choose `Cancel`; the persisted label must stay unchanged.

Click `Delete Updated scanner`, then `Cancel`; require the row still exists. Repeat and click `Confirm delete Updated scanner`. Reload, require the row absent, and require the saved bearer token's repository request to fail with 401. Capture issuance/edit/deletion actions and readbacks. Repeat through the direct page entry before claiming both entry points.

## Gotchas

Plaintext appears once; a page reload cannot recover it. Keep it out of screenshots shared externally. The baseline has no viewer account, so permission denial needs an additional fixture. Public API operations are separate entry points; UI proof alone does not cover API validation errors.
