// Throwaway, read-only sample API. Only the child dev server uses it.
import { createServer } from 'node:http';
import { spawn } from 'node:child_process';

const names = ['storefront', 'payments-api', 'design-system', 'legacy-worker', 'new-service'];
const repositories = names.map((name, index) => ({ id: `repo-${index}`, name, url: '', default_branch: 'main' }));
const scans = repositories.slice(0, 4).map((repository, index) => ({ id: `00000000-0000-4000-8000-00000000000${index}`, repository_id: repository.id, commit_sha: ['a71f890', 'bc823a1', 'e91d237', 'f32a811'][index], scanned_at: '2026-11-15T10:30:00Z', manifest_count: 1, dependency_count: 42, labels: {}, annotation: '' }));
const samples = [
  { check_id: 'lockfile-required', severity: 'high', summary: 'Lockfile is missing', remediation: 'Generate and commit a lockfile so installs use the same dependency versions.' },
  { check_id: 'pinned-dependencies', severity: 'medium', summary: 'Dependency uses an unpinned Git reference', remediation: 'Replace the Git branch reference with a reviewed commit SHA.' },
  { check_id: 'lockfile-consistency', severity: 'medium', summary: 'Manifest and lockfile disagree', remediation: 'Install dependencies with the project package manager, then commit the updated lockfile.' },
  { check_id: 'pinned-dependencies', severity: 'high', summary: 'Dependency points to a mutable URL', remediation: 'Use a versioned package release or pin the URL to an immutable artifact.' }
];
function report(index) {
  if (index === 3) return null;
  const selected = index === 0 ? [samples[0], samples[1], samples[2]] : index === 1 ? [samples[0], samples[3], samples[2]] : [samples[1]];
  return { schema_version: 1, sources: [{ detector: 'npm', path: 'package.json', form: 'manifest', analysis: { presence: 'present', extraction: index === 1 ? 'partial' : 'complete' }, diagnostics: [] }], check_runs: samples.map(sample => ({ check_id: sample.check_id, subject: { project_root: '.' }, status: index === 1 && sample.check_id === 'lockfile-consistency' ? 'skipped' : 'completed' })), findings: selected.map((sample, position) => ({ ...sample, subject: { project_root: '.' }, locations: [{ path: index === 0 ? 'apps/web/package.json' : 'package.json' }], evidence: { package: index === 0 ? '@example/ui' : index === 1 ? '@example/payments' : '@example/tokens' }, fingerprint: `${index}-${position}` })) };
}
const api = createServer((request, response) => {
  const url = new URL(request.url, 'http://localhost');
  let result;
  if (url.pathname === '/auth/me') result = { user_id: 'prototype', display_name: 'Sample data', memberships: [] };
  else if (url.pathname === '/api/v1/repositories') result = { items: repositories, pagination: { page: 1, page_size: 100, total: 5, total_pages: 1, has_previous: false, has_next: false }, filters: { q: '' } };
  else if (url.pathname === '/api/v1/scans') result = scans.filter(scan => scan.repository_id === url.searchParams.get('repository_id'));
  else if (url.pathname.endsWith('/report')) result = { report: report(scans.findIndex(scan => url.pathname.includes(scan.id))) };
  else result = scans.find(scan => url.pathname.endsWith(scan.id));
  response.writeHead(result === undefined ? 404 : 200, { 'Content-Type': 'application/json' });
  response.end(JSON.stringify(result ?? {}));
});
api.listen(0, '127.0.0.1', () => {
  const address = api.address();
  if (!address || typeof address === 'string') throw new Error('Missing sample API port');
  const child = spawn(process.env.npm_execpath ? process.execPath : 'pnpm', [...(process.env.npm_execpath ? [process.env.npm_execpath] : []), '--dir', 'apps/web', 'dev', '--port', '4318'], { stdio: 'inherit', env: { ...process.env, API_INTERNAL_BASE_URL: `http://127.0.0.1:${address.port}` } });
  console.log('Checks prototype: http://localhost:4318/app/checks?variant=D');
  child.on('exit', () => api.close());
  for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => { child.kill(signal); api.close(); });
});
