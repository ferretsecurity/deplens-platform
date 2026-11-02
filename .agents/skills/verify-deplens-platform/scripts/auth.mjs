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
  console.log('auth-protected: open /app/repositories anonymously');
  await page.goto('/app/repositories');
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel('Email', { exact: true }).fill('verify@example.com');
  await page.getByLabel('Password', { exact: true }).fill('wrong-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page.getByText('Login failed. Check your credentials and try again.')).toBeVisible();
  console.log('auth-invalid: rejected wrong password');
  await page.getByLabel('Password', { exact: true }).fill('verification-only-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/repositories$/);
  await expect(page.getByText('No repositories have been imported yet.')).toBeVisible();
  await page.reload();
  await expect(page.getByRole('link', { name: 'API Tokens', exact: true })).toBeVisible();
  const me = await context.request.get('/auth/me');
  assert.equal(me.status(), 200);
  const user = await me.json();
  assert.equal(user.role, 'owner');
  await writeFile(`${evidence}/auth-me.json`, JSON.stringify(user, null, 2));
  await writeFile(`${evidence}/signed-in.aria.txt`, await page.locator('body').ariaSnapshot());
  await page.screenshot({ path: `${evidence}/signed-in.png`, fullPage: true });
  console.log('auth-login: persisted owner session confirmed through /auth/me and reload');
  await page.locator('header').getByRole('button').filter({ hasText: user.display_name || user.user_id }).click();
  await page.getByRole('menuitem', { name: 'Sign out' }).click();
  await expect(page).toHaveURL(/\/login$/);
  assert.equal((await context.request.get('/auth/me')).status(), 401);
  await page.goto('/app/repositories');
  await expect(page).toHaveURL(/\/login$/);
  await page.screenshot({ path: `${evidence}/signed-out.png`, fullPage: true });
  console.log('auth-logout: session rejected by API and protected page redirects');
} finally {
  await context.tracing.stop({ path: `${evidence}/trace.zip` });
  await browser.close();
}
