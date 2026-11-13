#!/usr/bin/env node
import { createRequire } from 'node:module';
import { writeFile, readFile, copyFile, mkdir, mkdtemp, rm } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { execFileSync } from 'node:child_process';
import assert from 'node:assert/strict';
const require = createRequire(`${process.env.VERIFY_ROOT}/apps/web/package.json`);
const { chromium, expect } = require('@playwright/test');
const browser = await chromium.launch();
const context = await browser.newContext({ baseURL: process.env.VERIFY_WEB, viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
const evidence = process.env.VERIFY_EVIDENCE;
await context.tracing.start({ screenshots: true, snapshots: true, sources: true });
const fixtureRoot = await mkdtemp(join(tmpdir(), "deplens-assessment-"));
try {
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill('verify@example.com');
  await page.getByLabel('Password', { exact: true }).fill('verification-only-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/repositories$/);
  const issued = await context.request.post('/api/v1/tokens', { data: { label: 'Package verification', scopes: ['scan:read', 'scan:write'] } });
  assert.equal(issued.status(), 201);
  const { token } = await issued.json();
  assert.ok(process.env.VERIFY_SCANNER, 'VERIFY_SCANNER must point to a built deplens binary');
  const sourceState = {};
  for (const [name, directory, prefixes] of [
    ['platform', process.env.VERIFY_ROOT, ['go.mod', 'go.sum', 'Dockerfile.api', 'internal/', 'cmd/', 'db/', 'apps/web/src/', 'apps/web/package.json', 'package.json', 'pnpm-lock.yaml', '.agents/skills/verify-deplens-platform/scripts/']],
    ['scanner', process.env.VERIFY_SCANNER_SOURCE || join(process.env.VERIFY_ROOT, '../deplens'), ['go.mod', 'go.sum', 'internal/', 'cmd/']]
  ]) {
    const revision = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: directory, encoding: 'utf8' }).trim();
    const status = execFileSync('git', ['status', '--short'], { cwd: directory, encoding: 'utf8' });
    const files = execFileSync('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard'], { cwd: directory, encoding: 'utf8' }).split('\0').filter(file => file && prefixes.some(prefix => file === prefix || prefix.endsWith('/') && file.startsWith(prefix)));
    const hashes = {};
    for (const file of files) {
      const content = await readFile(join(directory, file));
      hashes[file] = createHash('sha256').update(content).digest('hex');
      const target = join(evidence, 'source', name, file);
      await mkdir(dirname(target), { recursive: true });
      await copyFile(join(directory, file), target);
    }
    sourceState[name] = { revision, status, hashes };
  }
  sourceState.scanner_binary_sha256 = createHash('sha256').update(await readFile(process.env.VERIFY_SCANNER)).digest('hex');
  await writeFile(`${evidence}/source-state.json`, JSON.stringify(sourceState, null, 2));
  const manifest = JSON.stringify({ dependencies: { widget: '^2.3.0' } });
  const lock = version => JSON.stringify({ name: 'fixture', lockfileVersion: 3, packages: { '': { dependencies: { widget: '^2.3.0' } }, 'node_modules/widget': { version, resolved: `https://registry.npmjs.org/widget/-/widget-${version}.tgz` } } });
  const yarn = version => `# yarn lockfile v1\n\nwidget@^2.3.0:\n  version "${version}"\n  resolved "https://registry.yarnpkg.com/widget/-/widget-${version}.tgz"\n`;
  const scenarios = [
    ['unlocked', { 'package.json': manifest }, ['warning']],
    ['fixed', { 'package.json': manifest, 'package-lock.json': lock('2.3.5') }, []],
    ['affected', { 'package.json': manifest, 'package-lock.json': lock('2.3.3') }, ['issue']],
    ['competing', { 'package.json': manifest, 'package-lock.json': lock('2.3.5'), 'yarn.lock': yarn('2.3.4') }, ['issue']],
    ['broken', { 'package.json': manifest, 'package-lock.json': '{ broken' }, ['warning']],
    ['workspace', { 'package.json': JSON.stringify({ private: true, workspaces: ['apps/*'] }), 'apps/web/package.json': manifest, 'package-lock.json': lock('2.3.3') }, ['issue', 'warning']]
  ];
  const expected = new Map();
  for (const [name, files, statuses] of scenarios) {
    const root = join(fixtureRoot, name);
    for (const [file, content] of Object.entries(files)) {
      await mkdir(dirname(join(root, file)), { recursive: true });
      await writeFile(join(root, file), content);
    }
    const report = JSON.parse(execFileSync(process.env.VERIFY_SCANNER, ['--json', root], { encoding: 'utf8' }));
    const payload = { schema_version: 'v1alpha1', repository: { name, url: `https://example.com/${name}.git`, default_branch: 'main' }, source: { commit_sha: name, ref: 'refs/heads/main', scanned_at: new Date().toISOString() }, report };
    const upload = await context.request.post('/api/v1/scans', { headers: { Authorization: 'Bearer ' + token }, data: payload });
    assert.equal(upload.status(), 201, await upload.text());
    await writeFile(`${evidence}/${name}-report.json`, JSON.stringify(report, null, 2));
    expected.set(name, statuses);
  }
  const query = new URLSearchParams({ q: 'widget', ecosystem: 'npm', affected_range: '>0.0.0 <=2.3.4' });
  const response = await context.request.get(`/api/v1/packages?${query}`);
  assert.equal(response.status(), 200, await response.text());
  const result = await response.json();
  for (const [name, statuses] of expected) {
    assert.deepEqual(result.items.filter(item => item.repository_name === name).map(item => item.assessment.status).sort(), [...statuses].sort(), name);
    console.log(`assessment-${name}: ${statuses.join(', ') || 'no findings'}`);
  }
  const workspaceIssue = result.items.find(item => item.repository_name === 'workspace' && item.assessment.status === 'issue');
  assert.equal(workspaceIssue.assessment.scope, 'workspace');
  assert.equal(workspaceIssue.manifest_path, 'package-lock.json');
  await writeFile(`${evidence}/assessments.json`, JSON.stringify(result, null, 2));
  const exact = await context.request.get('/api/v1/packages?q=widg&ecosystem=npm&affected_range=%3C3');
  assert.equal((await exact.json()).pagination.total, 0);
  const firstPage = await context.request.get(`/api/v1/packages?${query}&page_size=2`);
  const first = await firstPage.json();
  assert.equal(first.items.length, 2);
  assert.equal(first.pagination.total, result.items.length);
  assert.equal(first.pagination.has_next, true);
  const paginated = [...first.items];
  for (let number = 2; number <= first.pagination.total_pages; number++) {
    const next = await context.request.get(`/api/v1/packages?${query}&page_size=2&page=${number}`);
    assert.equal(next.status(), 200);
    paginated.push(...(await next.json()).items);
  }
  assert.deepEqual(paginated, result.items);
  assert.equal((await context.request.get('/api/v1/packages?q=widget&ecosystem=npm&affected_range=latest')).status(), 400);
  await page.goto('/app/packages');
  await page.getByRole('link', { name: 'Assess a vulnerable version range' }).click();
  await page.getByLabel('Exact package name').fill('widget');
  await page.getByLabel('Affected range').fill('>0.0.0 <=2.3.4');
  await page.getByRole('button', { name: 'Assess', exact: true }).click();
  await expect(page.getByRole('status')).toHaveText(`${result.items.length} findings`);
  await expect(page.getByRole('heading', { name: 'Issue: widget@2.3.4', exact: true })).toBeVisible();
  await expect(page.getByText('fixed', { exact: true })).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole('status')).toHaveText(`${result.items.length} findings`);
  await page.screenshot({ path: `${evidence}/assessments.png`, fullPage: true });
  await writeFile(`${evidence}/assessments.aria.txt`, await page.locator('body').ariaSnapshot());
  console.log('assessment-ui: submitted form, persisted results after reload, exact identity and pagination verified through API');
} finally {
  await context.tracing.stop({ path: `${evidence}/trace.zip` });
  await browser.close();
  await rm(fixtureRoot, { recursive: true, force: true });
}
