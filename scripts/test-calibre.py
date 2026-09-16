#!/usr/bin/env python3
"""Exercise the real Go client against a disposable Calibre library only.
Build scripts/fixtures/calibre/Dockerfile first. No homelab credentials required.
"""
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[1]
command = ["docker"]
if os.environ.get("DOCKER_CONTEXT"):
    command += ["--context", os.environ["DOCKER_CONTEXT"]]

def docker(*args):
    result = subprocess.run(command + list(args), capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(f"Docker {args[0]} failed: {result.stderr.strip()}")
    return result.stdout.strip()

name = "librarry-calibre-test-" + uuid.uuid4().hex[:12]
try:
    docker("run", "-d", "--name", name, "-p", "127.0.0.1::8080", "-e", "LIBRARRY_CALIBRE_AUTH_MODE=" + os.environ.get("LIBRARRY_CALIBRE_AUTH_MODE", "auto"), "librarry-calibre:fixture")
    port = docker("port", name, "8080/tcp").rsplit(":", 1)[1]
    url = "http://127.0.0.1:" + port
    for _ in range(60):
        try:
            with urllib.request.urlopen(url, timeout=2):
                break
        except urllib.error.HTTPError as error:
            if error.code == 401:
                break
            time.sleep(1)
        except OSError:
            time.sleep(1)
    else:
        raise RuntimeError("disposable Calibre did not become ready")
    print(docker("exec", name, "calibre-server", "--version"), flush=True)
    env = dict(os.environ, LIBRARRY_CALIBRE_FIXTURE_URL=url)
    subprocess.run(["go", "test", "-race", "./backend/internal/calibre", "-run", "^TestDisposableCalibre", "-count=1", "-v"], cwd=ROOT, env=env, check=True)
    books = docker("exec", name, "python3", "-c", "import sqlite3; db=sqlite3.connect('file:/library/metadata.db?mode=ro',uri=True); print(db.execute('select count(*) from books').fetchone()[0])")
    assert books == "0", books
    print("Disposable Calibre: authenticated upload, library ID, metadata, conversion, deletion and final empty library verified")
except Exception:
    try:
        print(docker("logs", name))
    except Exception:
        pass
    raise
finally:
    subprocess.run(command + ["rm", "-f", name], capture_output=True)
