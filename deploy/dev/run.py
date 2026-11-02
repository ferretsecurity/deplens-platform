#!/usr/bin/env python3
"""Run the local web server or rebuild the API when its inputs change."""

import os
from pathlib import Path
import signal
import subprocess
import sys
import time

from dotenv import load_dotenv

ROOT = Path(__file__).resolve().parents[2]
RUNTIME = ROOT / "var/service"
PORT = os.environ.get("PORT", "9005")
stopping = False


def environment():
    load_dotenv(ROOT / ".env", override=True)
    return {
        **os.environ,
        "DATABASE_URL": os.environ.get("DATABASE_URL", "postgres://postgres:postgres@127.0.0.1:5432/deplens?sslmode=disable"),
        "BLOB_FILESYSTEM_ROOT": str(ROOT / "var/blobs"),
        "HTTP_ADDRESS": "127.0.0.1:8080",
        "API_INTERNAL_BASE_URL": "http://127.0.0.1:8080",
        "APP_BASE_URL": f"http://localhost:{PORT}",
        "NEXT_PUBLIC_APP_BASE_URL": f"http://localhost:{PORT}",
        "SESSION_COOKIE_SECURE": "false",
        "TRUST_PROXY": "false",
    }


def snapshot():
    paths = [ROOT / name for name in ("go.mod", "go.sum", ".env")]
    for directory in ("cmd", "internal", "db"):
        paths.extend(path for path in (ROOT / directory).rglob("*") if path.is_file())
    return {str(path): (path.stat().st_mtime_ns, path.stat().st_size)
            for path in paths if path.exists()}


def stop(child):
    if child is not None and child.poll() is None:
        child.terminate()
        try:
            child.wait(timeout=15)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait()


def shutdown(_signum, _frame):
    global stopping
    stopping = True


def api():
    signal.signal(signal.SIGTERM, shutdown)
    signal.signal(signal.SIGINT, shutdown)
    RUNTIME.mkdir(parents=True, exist_ok=True)
    child = None
    previous = None
    try:
        while not stopping:
            current = snapshot()
            if current != previous:
                previous = current
                print("Building API after source/config change", flush=True)
                result = subprocess.run(
                    ["go", "build", "-o", str(RUNTIME / "api.next"), "./cmd/deplens-platform"],
                    cwd=ROOT,
                )
                if result.returncode == 0 and not stopping:
                    stop(child)
                    (RUNTIME / "api.next").replace(RUNTIME / "api")
                    child = subprocess.Popen([str(RUNTIME / "api")], cwd=ROOT, env=environment())
                elif result.returncode != 0:
                    print("Build failed; waiting for the next edit", flush=True)
            if child is not None and child.poll() is not None:
                raise SystemExit(child.returncode or 1)
            time.sleep(1)
    finally:
        stop(child)


if __name__ == "__main__":
    os.chdir(ROOT)
    if sys.argv[1:] == ["web"]:
        os.execvpe("pnpm", ["pnpm", "--dir", "apps/web", "dev", "--hostname", "0.0.0.0", "--port", PORT], environment())
    elif sys.argv[1:] == ["api"]:
        api()
    else:
        raise SystemExit("Usage: run.py api|web")
