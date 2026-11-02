#!/usr/bin/env python3
"""Run a real, disposable Deplens stack and retain its proof artifacts."""
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import tempfile
import time
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[4]
SCRIPTS = Path(__file__).resolve().parent

def run(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)

def port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]

def main():
    evidence = ROOT / 'var/verification' / (time.strftime('%Y%m%d-%H%M%S') + '-' + uuid.uuid4().hex[:8])
    evidence.mkdir(parents=True)
    evidence.chmod(0o700)
    print(f'Evidence: {evidence}', flush=True)
    scratch = Path(tempfile.mkdtemp(prefix='deplens-verify-'))
    container = 'deplens-verify-' + uuid.uuid4().hex[:12]
    processes = []
    logs = []
    created = False
    def start(args, cwd, env, name):
        log = (evidence / (name + '.log')).open('w')
        logs.append(log)
        proc = subprocess.Popen(args, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        processes.append(proc)
        return proc
    def wait_http(url):
        deadline = time.monotonic() + 120
        while time.monotonic() < deadline:
            if any(p.poll() is not None for p in processes):
                raise RuntimeError('Owned process exited; inspect evidence logs')
            try:
                with urllib.request.urlopen(url, timeout=3) as response:
                    if response.status in (200, 204):
                        return
            except (OSError, TimeoutError):
                pass
            time.sleep(0.5)
        raise RuntimeError(f'Not ready: {url}')
    try:
        run('corepack', 'pnpm', 'install', '--frozen-lockfile', cwd=ROOT)
        run('corepack', 'pnpm', '--dir', 'apps/web', 'exec', 'playwright', 'install', 'chromium', cwd=ROOT)
        run('go', 'build', '-o', str(scratch / 'api'), './cmd/deplens-platform', cwd=ROOT)
        shutil.copytree(ROOT / 'apps/web', scratch / 'web', ignore=shutil.ignore_patterns('node_modules', '.next', '.next-dev', 'test-results', 'playwright-report'))
        (scratch / 'web/node_modules').symlink_to(ROOT / 'apps/web/node_modules', target_is_directory=True)
        run('docker', 'run', '-d', '--name', container, '-e', 'POSTGRES_PASSWORD=verification', '-e', 'POSTGRES_DB=deplens', '-p', '127.0.0.1::5432', 'postgres:17', stdout=subprocess.DEVNULL)
        created = True
        dbport = subprocess.check_output(['docker', 'port', container, '5432/tcp'], text=True).strip().split(':')[-1]
        for _ in range(120):
            # SQL through TCP waits for the final server, not the temporary init server.
            result = subprocess.run(['docker', 'exec', container, 'psql', '-h', '127.0.0.1', '-U', 'postgres', '-d', 'deplens', '-c', 'SELECT 1'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if result.returncode == 0:
                break
            time.sleep(0.5)
        else:
            raise RuntimeError('Postgres did not become usable')
        apiport, webport = port(), port()
        api, web = f'http://127.0.0.1:{apiport}', f'http://127.0.0.1:{webport}'
        env = dict(os.environ, DATABASE_URL=f'postgres://postgres:verification@127.0.0.1:{dbport}/deplens?sslmode=disable', HTTP_ADDRESS=f'127.0.0.1:{apiport}', BLOB_BACKEND='filesystem', BLOB_FILESYSTEM_ROOT=str(scratch / 'blobs'), BOOTSTRAP_OWNER_EMAIL='verify@example.com', BOOTSTRAP_OWNER_PASSWORD='verification-only-password', APP_BASE_URL=web, DEPLOYMENT_MODE='self-hosted', SESSION_COOKIE_SECURE='false', TRUST_PROXY='false', MIGRATE_ON_START='true', SOCKET_ENCRYPTION_KEY='', API_INTERNAL_BASE_URL=api, NEXT_PUBLIC_APP_BASE_URL=web, DEV_LOGIN_PREFILL='false', NEXT_TELEMETRY_DISABLED='1', VERIFY_EVIDENCE=str(evidence), VERIFY_WEB=web, VERIFY_ROOT=str(ROOT))
        api_proc = start([str(scratch / 'api')], ROOT, env, 'api')
        wait_http(api + '/readyz')
        web_proc = start(['node', str(ROOT / 'apps/web/node_modules/next/dist/bin/next'), 'dev', '--hostname', '127.0.0.1', '--port', str(webport)], scratch / 'web', env, 'web')
        wait_http(web + '/login')
        # Read-only doctor: prove process identity, owned database and protected routing.
        assert api_proc.poll() is None and web_proc.poll() is None
        wait_http(api + '/readyz')
        with urllib.request.urlopen(web + '/app/repositories') as response:
            assert response.url == web + '/login', response.url
        metadata = dict(api=api, web=web, container=container, api_pid=api_proc.pid, web_pid=web_proc.pid, revision=subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(), doctor='passed')
        (evidence / 'doctor.json').write_text(json.dumps(metadata, indent=2))
        (evidence / 'working-tree.diff').write_bytes(subprocess.check_output(['git', 'diff'], cwd=ROOT))
        driver = Path(os.environ.get('VERIFY_DRIVER', str(SCRIPTS / 'auth.mjs'))).resolve()
        with (evidence / 'drive.log').open('w') as log:
            run('node', str(driver), cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
        (evidence / 'result.json').write_text(json.dumps({'feature': driver.name, 'result': 'passed'}))
    finally:
        for proc in reversed(processes):
            if proc.poll() is None:
                os.killpg(proc.pid, signal.SIGTERM)
                try:
                    proc.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    os.killpg(proc.pid, signal.SIGKILL)
                    proc.wait()
        for log in logs:
            log.close()
        if created:
            run('docker', 'rm', '-f', '-v', container, stdout=subprocess.DEVNULL)
        shutil.rmtree(scratch)
        (evidence / 'cleanup.json').write_text(json.dumps({'scratch_removed': not scratch.exists(), 'processes_stopped': all(p.poll() is not None for p in processes), 'container_removed': created}))
        print(f'Cleanup complete; evidence retained: {evidence}', flush=True)

if __name__ == '__main__':
    signal.signal(signal.SIGTERM, lambda *_: (_ for _ in ()).throw(KeyboardInterrupt()))
    main()
