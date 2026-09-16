#!/usr/bin/env python3
"""Qualify locally built images with disposable Postgres/media. No live credentials.

DOCKER_CONTEXT=... python3 scripts/test-packaged.py [api-image] [web-image]
Images default to librarry-api:stabilization and librarry-web:stabilization.
"""
import base64
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[1]
DOCKER = ["docker"]
if os.environ.get("DOCKER_CONTEXT"):
    DOCKER += ["--context", os.environ["DOCKER_CONTEXT"]]
API_IMAGE = sys.argv[1] if len(sys.argv) > 1 else "librarry-api:stabilization"
WEB_IMAGE = sys.argv[2] if len(sys.argv) > 2 else "librarry-web:stabilization"
PREFIX = "librarry-qualification-" + uuid.uuid4().hex[:12]
PG, API, WEB = [PREFIX + suffix for suffix in ("-pg", "-api", "-web")]
CONTAINERS = []
OPENER = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def docker(*args, binary=False, input=None):
    return subprocess.run(DOCKER + list(args), check=True, capture_output=True,
                          text=not binary, input=input).stdout


def start(name, *args):
    CONTAINERS.append(name)
    docker("run", "-d", "--name", name, "--network", PREFIX, *args)


def sql(query, db="librarry_test"):
    return docker("exec", PG, "psql", "-U", "postgres", "-d", db,
                  "-v", "ON_ERROR_STOP=1", "-Atc", query).strip()


def request(path, body=None, method=None, headers=None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(BASE + path, data=data,
                                 headers={"Content-Type": "application/json", **(headers or {})}, method=method)
    with OPENER.open(req, timeout=15) as response:
        raw = response.read()
        return json.loads(raw) if "application/json" in response.headers.get("Content-Type", "") else raw


def wait_for(check):
    for _ in range(60):
        try:
            return check()
        except (OSError, subprocess.CalledProcessError):
            time.sleep(1)
    raise RuntimeError("disposable service did not become ready")


(ROOT / "output").mkdir(exist_ok=True)
with tempfile.TemporaryDirectory(prefix=PREFIX, dir=ROOT / "output") as temp:
    media = Path(temp)
    for folder in (media, media / "downloads", media / "ebooks", media / "audiobooks"):
        folder.mkdir(exist_ok=True)
        folder.chmod(0o777)
    source = media / "downloads" / "Fixture.epub"
    shutil.copyfile(ROOT / "docs/fixtures/e2e/librarry-public-domain-e2e-book.epub", source)
    source.chmod(0o644)
    chapters = media / "downloads" / "Chapters"
    chapters.mkdir()
    for name in ("01.mp3", "02.mp3"):
        (chapters / name).write_bytes(b"ID3 controlled chapter fixture " + name.encode())
    docker("network", "create", PREFIX)
    try:
        start(PG, "-e", "POSTGRES_PASSWORD=fixture-only", "-e", "POSTGRES_DB=librarry_test", "postgres:16-alpine")
        wait_for(lambda: docker("exec", PG, "pg_isready", "-U", "postgres"))
        env = {
            "LIBRARRY_DATABASE_URL": f"postgres://postgres:fixture-only@{PG}:5432/librarry_test?sslmode=disable",
            "LIBRARRY_EBOOK_LIBRARY_ROOT": "/fixture/ebooks",
            "LIBRARRY_AUDIOBOOK_LIBRARY_ROOT": "/fixture/audiobooks",
        }
        for worker in ("MONITOR", "AUTHOR_MONITOR", "FEED_SYNC", "FAILED_DOWNLOAD", "UPGRADE_SEARCH",
                       "CALIBRE_REFRESH", "COMPLETED_IMPORT", "COMPLETED_REMOVE", "BACKUP", "IMPORT_LIST_SYNC"):
            env[f"LIBRARRY_{worker}_ENABLED"] = "false"
        flags = [flag for key, value in env.items() for flag in ("-e", f"{key}={value}")]
        start(API, "--network-alias", "api", "-v", f"{media}:/fixture", *flags, API_IMAGE)
        start(WEB, "-p", "127.0.0.1::80", WEB_IMAGE)
        port = json.loads(docker("inspect", WEB))[0]["NetworkSettings"]["Ports"]["80/tcp"][0]["HostPort"]
        BASE = "http://127.0.0.1:" + port
        status = wait_for(lambda: request("/api/v1/system/status"))
        expected_commit = os.environ.get("EXPECTED_COMMIT")
        assert status["authentication"] == "none" and status["migrationVersion"] >= 29, status
        if expected_commit:
            assert status["commit"] == expected_commit, status
        print("Packaged status:", json.dumps({key: status[key] for key in
              ("version", "commit", "buildTime", "migrationVersion", "runtimeVersion", "authentication")}))
        for route in ("/library", "/activity", "/settings"):
            page = request(route)
            assert b'<div id="root">' in page, route
        asset = re.search(rb'src="(/assets/[^"]+\.js)"', page).group(1).decode()
        assert len(request(asset)) > 1000
        assert request("/api/v1/library/files")["files"] == []
        wanted = request("/api/v1/wanted", {"result": {"provider": "fixture", "kind": "book",
            "work": {"id": "packaged-fixture", "title": "Librarry Fixture Book",
                     "authors": [{"id": "fixture-author", "name": "Fixture Author"}]}}, "format": "ebook"})
        wanted_id = str(uuid.UUID(wanted["id"]))
        for external, name, category in (("single", "Fixture.epub", "books-ebook"),
                                         ("missing", "Missing.epub", "books-ebook"),
                                         ("chapters", "Chapters", "books-audiobook")):
            sql("insert into downloads(client,external_id,name,category,save_path,state,progress,tags) values"
                f"('qBittorrent','{external}','{name}','{category}','/fixture/downloads','pausedUP',1,"
                f"'librarry,wanted:{wanted_id}')")
        for external in ("missing", "chapters"):
            result = request("/api/v1/library/import-completed", {"downloadIds": [external], "importMode": "copy"})
            assert result["imported"] == 0 and result["errored"] == 1, result
        result = request("/api/v1/library/import-completed", {"downloadIds": ["single"], "importMode": "copy"})
        assert result["imported"] == 1 and result["errored"] == 0, result
        record = result["results"][0]["import"]["file"]
        destination = media / Path(record["path"]).relative_to("/fixture")
        digest = hashlib.sha256(source.read_bytes()).hexdigest()
        assert hashlib.sha256(destination.read_bytes()).hexdigest() == digest
        assert record["metadata"]["verifiedDownload"]["sha256"] == digest
        for _ in range(2):
            scan = request("/api/v1/library/scan", {"format": "ebook"})
            assert scan["upserted"] == 1, scan
        records = request("/api/v1/library/files")["files"]
        assert len(records) == 1 and records[0]["id"] == record["id"], records
        assert records[0]["metadata"]["wantedId"] == wanted_id
        assert records[0]["metadata"]["verifiedDownload"]["sha256"] == digest
        assert source.exists() and all((chapters / name).exists() for name in ("01.mp3", "02.mp3"))
        repeat = request("/api/v1/library/import-completed", {"downloadIds": ["single"], "importMode": "copy"})
        assert repeat["imported"] == 0 and repeat["skipped"] == 1, repeat
        print("Packaged imports: exact file verified, shared sibling/multipart rejected, source retained, rescans preserve links")
        dump = docker("exec", PG, "pg_dump", "-U", "postgres", "-Fc", "librarry_test", binary=True)
        docker("exec", PG, "createdb", "-U", "postgres", "librarry_restore")
        docker("exec", "-i", PG, "pg_restore", "-U", "postgres", "-d", "librarry_restore", "--exit-on-error", binary=True, input=dump)
        for query in ("select count(*) from schema_migrations", "select count(*) from wanted_items",
                      "select count(*) from downloads", "select count(*) from files",
                      "select metadata->'verifiedDownload' from files"):
            assert sql(query) == sql(query, "librarry_restore"), query
        print("Isolated database restore verified:", len(dump), "bytes; file/download/wanted counts and receipt preserved")
        # Exercise real persistence, cookies, restart and Basic auth in the image.
        request("/api/v1/auth/config", {"method": "forms", "username": "fixture", "password": "fixture-password"}, method="PUT")
        assert request("/api/v1/auth/status")["authenticated"] is False
        try:
            request("/api/v1/wanted?view=library")
            raise AssertionError("forms allowed an unauthenticated request")
        except urllib.error.HTTPError as error:
            assert error.code == 401
        assert request("/api/v1/login", {"username": "fixture", "password": "fixture-password"})["authenticated"] is True
        assert request("/api/v1/auth/status")["authenticated"] is True
        docker("restart", API)
        assert wait_for(lambda: request("/api/v1/auth/status"))["method"] == "forms"
        assert request("/api/v1/auth/status")["authenticated"] is True
        request("/api/v1/auth/config", {"method": "basic"}, method="PUT")
        assert request("/api/v1/auth/status")["authenticated"] is False
        basic = "Basic " + base64.b64encode(b"fixture:fixture-password").decode()
        assert request("/api/v1/auth/status", headers={"Authorization": basic})["authenticated"] is True
        request("/api/v1/auth/config", {"method": "none"}, method="PUT", headers={"Authorization": basic})
        assert request("/api/v1/auth/status")["method"] == "none"
        print("Packaged authentication: forms enforced, cookie/restart persisted, Basic enforced, explicit none restored")
    except Exception:
        for name in (API, WEB):
            if name in CONTAINERS:
                print(docker("logs", "--tail", "30", name), file=sys.stderr)
        raise
    finally:
        # Linux runners may have a different uid from the image's uid 1000.
        # Return only this unique fixture tree to the harness owner before cleanup.
        if API in CONTAINERS:
            subprocess.run(DOCKER + ["exec", "--user", "0", API, "chown", "-R",
                           f"{os.getuid()}:{os.getgid()}", "/fixture"], capture_output=True, check=False)
        for name in reversed(CONTAINERS):
            subprocess.run(DOCKER + ["rm", "-f", "-v", name], capture_output=True, check=False)
        subprocess.run(DOCKER + ["network", "rm", PREFIX], capture_output=True, check=False)
