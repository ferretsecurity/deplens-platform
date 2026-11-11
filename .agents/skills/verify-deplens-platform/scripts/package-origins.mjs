#!/usr/bin/env node
import { createRequire } from 'node:module';
import { writeFile } from 'node:fs/promises';
import assert from 'node:assert/strict';

for (const name of ['VERIFY_ROOT', 'VERIFY_WEB', 'VERIFY_EVIDENCE']) {
  assert.ok(process.env[name], `${name} must be supplied by verify.py`);
}
const require = createRequire(`${process.env.VERIFY_ROOT}/apps/web/package.json`);
const { chromium, expect } = require('@playwright/test');
const browser = await chromium.launch();
const context = await browser.newContext({ baseURL: process.env.VERIFY_WEB, viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
const evidence = process.env.VERIFY_EVIDENCE;
const save = (name, value) => writeFile(`${evidence}/${name}.json`, JSON.stringify(value, null, 2));
await context.tracing.start({ screenshots: true, snapshots: true, sources: true });

try {
  console.log('package-origins: sign in to the disposable stack');
  await page.goto('/login');
  await page.getByLabel('Email', { exact: true }).fill('verify@example.com');
  await page.getByLabel('Password', { exact: true }).fill('verification-only-password');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page).toHaveURL(/\/app\/repositories$/);
  const initial = await context.request.get('/api/v1/packages');
  assert.equal(initial.status(), 200);
  assert.equal((await initial.json()).pagination.total, 0, 'verification requires an empty disposable inventory');
  const issued = await context.request.post('/api/v1/tokens', {
    data: { label: 'Package origin verification', scopes: ['scan:read', 'scan:write'] }
  });
  assert.equal(issued.status(), 201, await issued.text());
  const { token } = await issued.json();

  const echoURL = 'https://example.test/modules/echo.ts';
  const wheelURL = 'https://example.test/dist/unnamed-1.0.whl#sha256=123';
  const gitURL = 'git+https://example.test/unnamed-repository.git';
  const aliasURLs = ['https://example.test/first/alias.ts', 'https://example.test/second/alias.ts'];
  const localPython = { raw: '-e .', origin_kind: 'path', attributes: { source_path: '.', editable: 'true' } };
  const localDeno = { raw: 'local-module@~/src/local.ts', name: 'local-module', origin_kind: 'path', attributes: { path: '~/src/local.ts' } };
  const localWorkspace = { raw: 'buf-workspace@proto', name: 'buf-workspace', origin_kind: 'workspace', attributes: { path: 'proto' } };
  const source = (detector, path, packageType, dependencies) => ({
    detector, path, form: 'manifest', analysis: { presence: 'present', extraction: 'complete' },
    dependencies: dependencies.map((dependency) => ({ package_type: packageType, ...dependency })), diagnostics: []
  });
  const sources = [
    source('py-requirements', 'requirements.txt', 'pypi', [
      localPython,
      { raw: 'requests==2.31.0', name: 'requests', version: '2.31.0', origin_kind: 'registry' },
      { raw: 'legacy-package==1.0.0', name: 'legacy-package', version: '1.0.0' },
      { raw: 'unknown-package==1.0.0', name: 'unknown-package', version: '1.0.0', origin_kind: 'unknown' },
      { raw: wheelURL, origin_kind: 'url', attributes: { source_url: wheelURL } },
      { raw: gitURL, origin_kind: 'git', attributes: { source_url: gitURL } },
      { raw: 'unnamed-url-without-destination', origin_kind: 'url' }
    ]),
    source('deno-json', 'deno.json', 'npm', [
      localDeno,
      { raw: `$echo@${echoURL}`, name: '$echo', origin_kind: 'url', attributes: { source_url: echoURL } },
      ...aliasURLs.map((url) => ({ raw: `alias@${url}`, name: 'alias', origin_kind: 'url', attributes: { source_url: url } }))
    ]),
    source('buf-yaml', 'buf.yaml', 'generic', [localWorkspace])
  ];
  const upload = async (commit, reportSources) => {
    const payload = {
      schema_version: 'v1alpha1',
      repository: { name: 'Package origins verification', url: 'https://example.test/package-origins.git', default_branch: 'main' },
      source: { commit_sha: commit, ref: 'refs/heads/main', scanned_at: new Date().toISOString() },
      report: { schema_version: 1, root: '.', sources: reportSources, check_runs: [], findings: [] }
    };
    await save(`${commit}-fixture`, payload);
    const response = await context.request.post('/api/v1/scans', { headers: { Authorization: `Bearer ${token}` }, data: payload });
    assert.equal(response.status(), 201, await response.text());
    const body = await response.json();
    assert.ok(body.scan_id);
    await save(`${commit}-upload`, body);
    return { scanID: body.scan_id, payload };
  };
  const packages = async (params = {}) => {
    const response = await context.request.get(`/api/v1/packages?${new URLSearchParams(params)}`);
    assert.equal(response.status(), 200, await response.text());
    return response.json();
  };

  const { scanID, payload } = await upload('origins-first', sources);
  const all = await packages({ page_size: '100' });
  await save('packages-origins-all', all);
  assert.equal(all.pagination.total, 8);
  assert.equal(all.items.length, 8);
  for (const item of all.items) {
    assert.equal(item.source, 'cli');
    assert.equal(typeof item.origin_kind, 'string');
    assert.equal(typeof item.destination, 'string');
    assert.ok(!['path', 'workspace'].includes(item.origin_kind));
  }
  assert.deepEqual(all.items.filter((item) => item.name === 'alias').map((item) => item.destination).sort(), aliasURLs);
  assert.deepEqual(all.items.filter((item) => item.name === '').map((item) => item.destination).sort(), [gitURL, wheelURL].sort());
  assert.equal(all.items.find((item) => item.name === 'requests').destination, '');
  assert.equal(all.items.find((item) => item.name === 'legacy-package').origin_kind, '');
  assert.equal(all.items.find((item) => item.name === 'unknown-package').origin_kind, 'unknown');
  console.log('package-origins: local/workspace excluded; registry, unknown and unnamed remote entries retained');

  const searches = [
    ['$echo', 1], ['alias', 2], ['second/alias.ts', 1], ['unnamed-repository.git', 1],
    ['unnamed-1.0.whl', 1], ['local-module', 0], ['buf-workspace', 0], ['unnamed-url-without-destination', 0]
  ];
  const searchResults = [];
  for (const [q, total] of searches) {
    const result = await packages({ q });
    assert.equal(result.pagination.total, total, `search ${q}`);
    searchResults.push({ q, result });
  }
  await save('packages-origin-searches', searchResults);
  const paginated = [];
  for (let number = 1; number <= 4; number++) {
    const result = await packages({ page: String(number), page_size: '2' });
    assert.equal(result.pagination.total, 8);
    assert.equal(result.pagination.total_pages, 4);
    paginated.push(...result.items);
  }
  assert.deepEqual(paginated, all.items);
  const version = await packages({ q: 'requests', version: '2.31.0', page_size: '1' });
  assert.equal(version.pagination.total, 1);
  assert.equal(version.items[0].origin_kind, 'registry');
  assert.equal(version.items[0].destination, '');
  await save('packages-origin-version', version);
  console.log('package-origins: name/destination search, stable pagination and version query readback passed');

  const manifestsResponse = await context.request.get(`/api/v1/scans/${scanID}/manifests`);
  assert.equal(manifestsResponse.status(), 200);
  const manifests = await manifestsResponse.json();
  await save('package-origin-manifests', manifests);
  for (const expectedSource of sources) {
    const manifest = manifests.items.find((item) => item.path === expectedSource.path);
    assert.ok(manifest, expectedSource.path);
    assert.equal(manifest.dependencies.length, expectedSource.dependencies.length);
    for (const expected of expectedSource.dependencies) {
      const dependency = manifest.dependencies.find((item) => item.raw === expected.raw);
      assert.ok(dependency, expected.raw);
      assert.equal(dependency.source, expected.origin_kind ?? '');
      assert.deepEqual(dependency.extras ?? {}, expected.attributes ?? {});
    }
  }
  const reportResponse = await context.request.get(`/api/v1/scans/${scanID}/report`);
  assert.equal(reportResponse.status(), 200);
  const report = await reportResponse.json();
  assert.deepEqual(report.report, payload.report);
  await save('package-origin-report', report);
  console.log('package-origins: persisted native report and raw declarations/path/editable attributes preserved');

  const captureLayout = async (name) => {
    const dimensions = await page.evaluate(() => ({
      viewport: window.innerWidth, document: document.documentElement.scrollWidth
    }));
    await save(`${name}-layout`, dimensions);
    await page.screenshot({ path: `${evidence}/${name}.png`, fullPage: true });
    await writeFile(`${evidence}/${name}.aria.txt`, await page.locator('body').ariaSnapshot());
    assert.ok(dimensions.document <= dimensions.viewport, `${name} overflows: ${JSON.stringify(dimensions)}`);
  };
  await page.goto('/app/packages');
  await expect(page.getByText('Showing 1-8 of 8 package usages')).toBeVisible();
  for (const label of ['Origin', 'Provider']) {
    await expect(page.getByRole('columnheader', { name: label, exact: true }).first()).toBeVisible();
  }
  await expect(page.getByRole('cell', { name: 'CLI', exact: true }).first()).toBeVisible();
  for (const label of ['URL', 'Git', 'Registry', 'Unknown']) {
    await expect(page.getByText(label, { exact: true }).first()).toBeVisible();
  }
  await expect(page.getByRole('heading', { name: '$echo', exact: true })).toBeVisible();
  for (const destination of [echoURL, wheelURL, gitURL, ...aliasURLs]) {
    await expect(page.getByText(destination, { exact: true }).last()).toBeVisible();
  }
  await captureLayout('package-origins-desktop');
  await page.setViewportSize({ width: 390, height: 844 });
  await captureLayout('package-origins-mobile');
  console.log('package-origins: desktop/mobile show origin, provider, aliases and destinations without document overflow');

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`/app/scans/${scanID}`);
  await expect(page.getByRole('heading', { name: 'Scan details', exact: true })).toBeVisible();
  for (const path of ['requirements.txt', 'deno.json', 'buf.yaml']) {
    await page.locator('summary').filter({ hasText: path }).click();
  }
  for (const text of ['Path: .', 'Path: ~/src/local.ts', 'Path: proto', 'Editable', 'Raw: -e .']) {
    await expect(page.getByText(text, { exact: true })).toBeVisible();
  }
  await expect(page.getByRole('cell', { name: 'Workspace', exact: true }).first()).toBeVisible();
  await captureLayout('package-origin-manifests-desktop');
  await page.setViewportSize({ width: 390, height: 844 });
  await captureLayout('package-origin-manifests-mobile');
  console.log('package-origins: scan details retain Python dot/editable, Deno home path, Buf workspace and raw declarations');

  const newURL = 'https://example.test/replacement/echo.ts';
  const replacement = sources.map((item) => item.path === 'deno.json' ? {
    ...item, dependencies: [{ package_type: 'npm', raw: `$echo@${newURL}`, name: '$echo', origin_kind: 'url', attributes: { source_url: newURL } }]
  } : item);
  await upload('origins-second', replacement);
  const after = await packages({ page_size: '100' });
  await save('packages-origins-replaced', after);
  assert.equal(after.pagination.total, 6);
  assert.equal(after.items.find((item) => item.name === '$echo').destination, newURL);
  for (const destination of [echoURL, ...aliasURLs]) {
    assert.equal((await packages({ q: destination })).pagination.total, 0);
  }
  await page.goto('/app/packages?q=%24echo');
  await expect(page.getByText(newURL, { exact: true })).toBeVisible();
  await expect(page.getByText(echoURL, { exact: true })).toHaveCount(0);
  await captureLayout('package-origins-replaced');
  console.log('package-origins: second native upload replaces prior manifest rows and removes old destinations');
  await save('package-origins-coverage', {
    features: ['scan-upload', 'scan-details', 'package-origins', 'package-search', 'package-pagination', 'manifest-replacement'],
    first_scan_id: scanID, initial_usages: 8, replaced_usages: 6, viewports: [1440, 390], result: 'passed'
  });
} finally {
  await context.tracing.stop({ path: `${evidence}/trace.zip` });
  await browser.close();
}
