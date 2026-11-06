#!/usr/bin/env node
import { createRequire } from 'node:module';
import { writeFile } from 'node:fs/promises';
import assert from 'node:assert/strict';
const require = createRequire(`${process.env.VERIFY_ROOT}/apps/web/package.json`);
const { chromium, expect } = require('@playwright/test');
const browser = await chromium.launch();
const context = await browser.newContext({ baseURL: process.env.VERIFY_WEB, viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
const evidence = process.env.VERIFY_EVIDENCE;
await context.tracing.start({ screenshots: true, snapshots: true, sources: true });
try {
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill('verify@example.com');
  await page.getByLabel('Password', { exact: true }).fill('verification-only-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/repositories$/);
  const issued = await context.request.post('/api/v1/tokens', { data: { label: 'Package verification', scopes: ['scan:read', 'scan:write'] } });
  assert.equal(issued.status(), 201);
  const { token } = await issued.json();
  const dependencies = [
    { constraint: '>=1.103.0,<2' },
    { constraint: '==1.103.1' },
    { constraint: '>=1.103.1,<2.0.0,!=1.104.0rc1,!=1.107.0' },
    { constraint: '>=1.95,<2' },
    { version: '1.100.4', constraint: '>=1.95,<2' },
    { version: '1.103.0' }, { version: '1.103.1' },
    { version: '1.103.2' }, { version: '1.106.6' }
  ].map((fields) => ({ name: 'litellm', package_type: 'pypi', ...fields }));
  const payload = {
    schema_version: 'v1alpha1',
    repository: { name: 'LiteLLM verification', url: 'https://example.com/litellm.git', default_branch: 'main' },
    source: { commit_sha: 'package-versions', ref: 'refs/heads/main', scanned_at: new Date().toISOString() },
    snapshot: { root: '.', manifests: [{ type: 'requirements', path: 'requirements.txt', has_dependencies: true, dependencies }] }
  };
  const upload = await context.request.post('/api/v1/scans', { headers: { Authorization: 'Bearer ' + token }, data: payload });
  assert.equal(upload.status(), 201, await upload.text());
  await writeFile(`${evidence}/package-fixture.json`, JSON.stringify(payload, null, 2));
  await page.goto('/app/packages?q=litellm');
  await expect(page.getByText('Showing 1-9 of 9 package usages')).toBeVisible();
  await page.screenshot({ path: `${evidence}/packages-unfiltered.png`, fullPage: true });
  for (const [version, total] of [['1.103.1', 5], ['1.100.4', 2], ['1.107.0', 2], ['1.104.0rc1', 0], ['2.0.0', 0]]) {
    await page.getByRole('textbox', { name: 'Version', exact: true }).fill(version);
    await page.getByRole('button', { name: 'Search', exact: true }).click();
    await expect(page.getByText(`Showing ${total ? 1 : 0}-${total} of ${total} package usages`)).toBeVisible();
    const result = await context.request.get(`/api/v1/packages?q=litellm&version=${version}`);
    assert.equal(result.status(), 200);
    const body = await result.json();
    assert.equal(body.pagination.total, total);
    assert.equal(body.filters.version, version);
    await writeFile(`${evidence}/packages-${version}.json`, JSON.stringify(body, null, 2));
    await page.screenshot({ path: `${evidence}/packages-${version}.png`, fullPage: true });
    console.log(`packages-version: ${version} returns ${total} matches`);
  }
  await page.goto('/app/packages?q=litellm&version=1.103.1&page_size=2');
  await expect(page.getByText('Showing 1-2 of 5 package usages')).toBeVisible();
  await page.getByRole('link', { name: 'Next', exact: true }).click();
  await expect(page).toHaveURL(/version=1.103.1&page=2&page_size=2$/);
  await expect(page.getByText('Showing 3-4 of 5 package usages')).toBeVisible();
  await page.screenshot({ path: `${evidence}/packages-pagination.png`, fullPage: true });
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByText('Showing 1-2 of 5 package usages')).toBeVisible();
  await page.getByRole('link', { name: 'Clear', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/packages$/);
  await expect(page.getByRole('textbox', { name: 'Version', exact: true })).toHaveValue('');
  assert.equal((await context.request.get('/api/v1/packages?version=latest')).status(), 400);
  await writeFile(`${evidence}/packages.aria.txt`, await page.locator('body').ariaSnapshot());
  console.log('packages-pagination-clear: version preserved, search resets page, clear removes filters');

} finally {
  await context.tracing.stop({ path: `${evidence}/trace.zip` });
  await browser.close();
}
