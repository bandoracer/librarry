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
import zipfile
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
DOCKER = ["docker"]
if os.environ.get("DOCKER_CONTEXT"):
    DOCKER += ["--context", os.environ["DOCKER_CONTEXT"]]
API_IMAGE = sys.argv[1] if len(sys.argv) > 1 else "librarry-api:stabilization"
WEB_IMAGE = sys.argv[2] if len(sys.argv) > 2 else "librarry-web:stabilization"
PREFIX = "librarry-qualification-" + uuid.uuid4().hex[:12]
PG, API, WEB, CLIENT = [PREFIX + suffix for suffix in ("-pg", "-api", "-web", "-client")]
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
    for name in ("Disc 1/01.mp3", "Disc 1/02.mp3", "Disc 2/01.mp3", "cover.jpg"):
        target = chapters / name
        target.parent.mkdir(exist_ok=True)
        target.write_bytes(b"controlled chapter fixture " + name.encode())
    with zipfile.ZipFile(source) as epub:
        opf = ET.fromstring(epub.read(next(name for name in epub.namelist() if name.endswith(".opf"))))
        book_title = next(node.text for node in opf.iter() if node.tag.endswith("}title"))
        book_author = next(node.text for node in opf.iter() if node.tag.endswith("}creator"))
    docker("network", "create", PREFIX)
    try:
        start(PG, "-e", "POSTGRES_PASSWORD=fixture-only", "-e", "POSTGRES_DB=librarry_test", "postgres:16-alpine")
        wait_for(lambda: docker("exec", PG, "pg_isready", "-U", "postgres"))
        start(CLIENT, "--network-alias", "fixture-client", "-v", f"{media}:/fixture:ro",
              "-v", f"{ROOT / 'scripts/fixtures/qbittorrent.py'}:/server.py:ro",
              "python:3.13-alpine", "python", "/server.py")
        env = {
            "LIBRARRY_QBITTORRENT_URL": "http://fixture-client:8080",
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
        assert status["authentication"] == "none" and status["migrationVersion"] >= 42, status
        if expected_commit:
            assert status["commit"] == expected_commit, status
        print("Packaged status:", json.dumps({key: status[key] for key in
              ("version", "commit", "buildTime", "migrationVersion", "runtimeVersion", "authentication")}))
        provider_status = {p["name"]: p for p in request("/api/v1/providers/health")["providers"]}
        assert provider_status["Open Library"]["status"] == "configured", provider_status
        assert "lastCheckedAt" not in provider_status["Open Library"], provider_status
        checked = request("/api/v1/providers/Hardcover/check", {}, method="POST")
        assert checked["status"] == "missing_credentials" and "lastCheckedAt" not in checked, checked
        print("Packaged provider health: configuration does not invent reachability/authentication; missing-token check records no request")
        for route in ("/library", "/activity", "/settings"):
            page = request(route)
            assert b'<div id="root">' in page, route
        asset = re.search(rb'src="(/assets/[^"]+\.js)"', page).group(1).decode()
        assert len(request(asset)) > 1000
        assert request("/api/v1/library/files")["files"] == []
        wanted = request("/api/v1/wanted", {"result": {"provider": "fixture", "kind": "book",
            "work": {"id": "packaged-fixture", "title": book_title,
                     "authors": [{"id": "fixture-author", "name": book_author}]}}, "format": "ebook"})
        wanted_id = str(uuid.UUID(wanted["id"]))
        audio = request("/api/v1/wanted", {"result": {"provider": "fixture", "kind": "book",
            "work": {"id": "packaged-audio", "title": "Chapters",
                     "authors": [{"id": "audio-author", "name": "Audio Author"}]}}, "format": "audiobook"})
        audio_id = str(uuid.UUID(audio["id"]))
        client_rows = []
        for external, name, category, linked_id in (("single", "Fixture.epub", "books-ebook", wanted_id),
                                                    ("missing", "Missing.epub", "books-ebook", wanted_id),
                                                    ("chapters", "Chapters", "books-audiobook", audio_id)):
            sql("insert into downloads(client,external_id,name,category,save_path,state,progress,tags) values"
                f"('qBittorrent','{external}','{name}','{category}','/fixture/downloads','pausedUP',1,"
                f"'librarry,wanted:{linked_id}')")
            payload = media / "downloads" / name
            paths = sorted(payload.rglob("*")) if payload.is_dir() else [payload]
            files = [{"name": str(path.relative_to(media / "downloads")), "size": path.stat().st_size if path.exists() else 42,
                      "progress": 1, "priority": 1} for path in paths if not path.is_dir()]
            client_rows.append({"status": {"hash": external, "name": name, "category": category,
                "save_path": "/fixture/downloads", "state": "pausedUP", "progress": 1,
                "tags": f"librarry,wanted:{linked_id}"}, "files": files})
        (media / "client.json").write_text(json.dumps(client_rows))
        result = request("/api/v1/library/import-completed", {"downloadIds": ["missing"], "importMode": "copy"})
        assert result["imported"] == 0 and result["reviewQueued"] == 1, result
        sql("alter table downloads add constraint inject_commit_failure check(import_status <> 'imported')")
        failed = request("/api/v1/library/import-completed", {"downloadIds": ["single"], "importMode": "copy"})
        assert failed["errored"] == 1 and failed["imported"] == 0, failed
        pending = request("/api/v1/library/import-recovery")
        assert pending["unfinished"] == 1 and pending["operations"][0]["state"] == "failed", pending
        assert request("/api/v1/library/files")["files"] == []
        assert request("/api/v1/library/scan", {"format": "ebook"})["upserted"] == 0
        planned_destination = pending["operations"][0]["files"][0]["destinationPath"]
        assert (media / Path(planned_destination).relative_to("/fixture")).exists()
        sql("alter table downloads drop constraint inject_commit_failure")
        # Model a process dying with a journaled temporary copy. The new process
        # must reclaim that exact stage and preserve unrelated temporary files.
        manifest_file = pending["operations"][0]["files"][0]
        file_id = str(uuid.UUID(manifest_file["id"]))
        stage_token = str(uuid.uuid4())
        stage_path = str(Path(planned_destination).parent / f".librarry-stage-{file_id}-{stage_token}")
        local_stage = media / Path(stage_path).relative_to("/fixture")
        unrelated_stage = local_stage.parent / ".librarry-stage-unowned"
        # Destination directories belong to the API uid; Linux CI's host uid
        # may differ. Seed the interrupted bytes as the same container user.
        docker("exec", API, "sh", "-ec", 'printf "%s" "interrupted staged copy" > "$1"; printf "%s" "retain unowned bytes" > "$2"',
               "fixture-stage", stage_path, str(Path(stage_path).parent / ".librarry-stage-unowned"))
        sql(f"update import_operation_files set stage_path='{stage_path}',stage_lease_token='{stage_token}' where id='{file_id}'")
        docker("restart", API)
        wait_for(lambda: request("/api/v1/system/status"))
        result = request("/api/v1/library/import-completed", {"downloadIds": ["single"], "importMode": "copy"})
        assert result["imported"] == 1 and result["errored"] == 0, result
        record = result["results"][0]["import"]["file"]
        operation_id = result["results"][0]["import"]["operationId"]
        recovery = request("/api/v1/library/import-recovery")
        assert recovery["unfinished"] == 0 and len(recovery["operations"]) == 1, recovery
        assert recovery["operations"][0]["id"] == operation_id
        assert recovery["operations"][0]["state"] == "committed"
        assert not local_stage.exists() and unrelated_stage.read_bytes() == b"retain unowned bytes"
        assert "stagePath" not in recovery["operations"][0]["files"][0]
        assert recovery["operations"][0]["cleanupState"] == "blocked"
        assert request(f"/api/v1/library/import-operations/{operation_id}/retry", {}, method="POST")["skipped"] is True

        destination = media / Path(record["path"]).relative_to("/fixture")
        digest = hashlib.sha256(source.read_bytes()).hexdigest()
        assert hashlib.sha256(destination.read_bytes()).hexdigest() == digest
        assert record["metadata"]["verifiedDownload"]["sha256"] == digest
        assert record["path"] == planned_destination
        for _ in range(2):
            scan = request("/api/v1/library/scan", {"format": "ebook"})
            assert scan["upserted"] == 1, scan
        records = request("/api/v1/library/files")["files"]
        assert len(records) == 1 and records[0]["id"] == record["id"], records
        assert records[0]["metadata"]["wantedId"] == wanted_id
        assert records[0]["metadata"]["verifiedDownload"]["sha256"] == digest
        assert source.exists() and all((chapters / name).exists() for name in ("Disc 1/01.mp3", "Disc 1/02.mp3", "Disc 2/01.mp3"))
        repeat = request("/api/v1/library/import-completed", {"downloadIds": ["single"], "importMode": "copy"})
        assert repeat["imported"] == 0 and repeat["skipped"] == 1, repeat
        audio_result = request("/api/v1/library/import-completed", {"downloadIds": ["chapters"], "importMode": "copy"})
        assert audio_result["imported"] == 1 and audio_result["errored"] == 0, audio_result
        audio_files = audio_result["results"][0]["import"]["files"]
        assert len(audio_files) == 3, audio_files
        for file in audio_files:
            src = media / Path(file["sourcePath"]).relative_to("/fixture")
            dest = media / Path(file["path"]).relative_to("/fixture")
            assert src.exists() and hashlib.sha256(src.read_bytes()).digest() == hashlib.sha256(dest.read_bytes()).digest()
        assert (Path(audio_files[0]["path"]).parent.name == "Disc 1")
        assert (media / Path(audio_files[0]["path"]).relative_to("/fixture").parent.parent / "cover.jpg").exists()
        print("Packaged imports: database failure rolled back, unfinished publication hidden, process restart reclaimed journaled stage and resumed exact plan, unowned stage retained, missing payload reviewed, multi-disc chapters and cover imported, source retained, rescans preserve links")
        # Complete the missing-file review through the real HTTP preview/resolve
        # contract once the client and filesystem both prove the file is present.
        missing_source = media / "downloads" / "Missing.epub"
        shutil.copyfile(source, missing_source)
        client_rows[1]["files"][0]["size"] = missing_source.stat().st_size
        (media / "client.json").write_text(json.dumps(client_rows))
        reviews = request("/api/v1/library/import-reviews?status=pending")["reviews"]
        review = next(row for row in reviews if row["downloadId"] == "missing")
        mapping = {"action": "import", "confirmIdentity": True, "importMode": "copy", "conflictAction": "rename",
                   "mapping": [{"relativePath": "Missing.epub", "wantedId": wanted_id}]}
        preview = request(f"/api/v1/library/import-reviews/{review['id']}/preview", mapping)
        preview_destination = media / Path(preview["operation"]["files"][0]["destinationPath"]).relative_to("/fixture")
        assert not preview_destination.exists()
        try:
            request(f"/api/v1/library/import-reviews/{review['id']}/resolve", {**mapping, "previewToken": "stale"})
            raise AssertionError("review accepted a stale preview")
        except urllib.error.HTTPError as error:
            assert error.code == 409, error.code
        resolved = request(f"/api/v1/library/import-reviews/{review['id']}/resolve", {**mapping, "previewToken": preview["fingerprint"]})
        assert resolved["review"]["status"] == "imported", resolved
        assert preview_destination.exists() and missing_source.exists()
        assert hashlib.sha256(preview_destination.read_bytes()).hexdigest() == digest
        print("Packaged review: read-only destination preview, stale-token rejection, explicit mapping import and source retention verified")

        manual_source = media / "downloads" / "Manual.epub"
        shutil.copyfile(source, manual_source)
        manual_request = {"sourcePath": "/fixture/downloads/Manual.epub", "wantedId": wanted_id, "importMode": "move", "conflictAction": "rename"}
        before_manual = request("/api/v1/library/files")["files"]
        sql("alter table import_operations add constraint inject_manual_commit_failure check(source_kind<>'manual' or state<>'committed')")
        try:
            request("/api/v1/library/import", manual_request)
            raise AssertionError("manual import hid a failed database commit")
        except urllib.error.HTTPError as error:
            assert error.code >= 400
        manual_report = request("/api/v1/library/import-recovery")
        manual_operation = next(operation for operation in manual_report["operations"] if operation.get("sourceKind") == "manual")
        assert manual_operation["state"] == "failed" and manual_source.exists()
        assert len(request("/api/v1/library/files")["files"]) == len(before_manual)
        sql("alter table import_operations drop constraint inject_manual_commit_failure")
        docker("restart", API)
        wait_for(lambda: request("/api/v1/system/status"))
        manual_result = request(f"/api/v1/library/import-operations/{manual_operation['id']}/retry", {})
        assert manual_result["moved"] is True and not manual_source.exists(), manual_result
        assert manual_result["operationId"] == manual_operation["id"]
        assert request("/api/v1/library/import", manual_request)["skipped"] is True
        manual_target = media / Path(manual_result["destinationPath"]).relative_to("/fixture")
        assert hashlib.sha256(manual_target.read_bytes()).hexdigest() == digest
        print("Packaged manual import: failed commit retains source and hides destination; process restart resumes original plan; move cleanup and idempotent retry verified")
        acquisition_request = {"client": "qBittorrent", "title": "Acquisition fixture",
                               "releaseUrl": "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567",
                               "tags": ["librarry", "librarry-smoke"]}
        try:
            request("/api/v1/grabs", acquisition_request)
            raise AssertionError("simulated acknowledgement loss reported success")
        except urllib.error.HTTPError as error:
            assert error.code == 502, error.code
        intents = request("/api/v1/acquisition-recovery")["intents"]
        assert len(intents) == 1 and intents[0]["state"] == "uncertain", intents
        intent_id = str(uuid.UUID(intents[0]["id"]))
        docker("restart", API)
        wait_for(lambda: request("/api/v1/system/status"))
        sql(f"update acquisition_intents set next_check_at=null where id='{intent_id}'")
        sql("alter table history_events add constraint fail_grab_history check(event_type<>'release_grabbed')")
        try:
            request(f"/api/v1/acquisition-recovery/{intent_id}", {"action": "check"})
            raise AssertionError("history persistence failure reported success")
        except urllib.error.HTTPError as error:
            assert error.code == 502, error.code
        pending = request("/api/v1/acquisition-recovery")["intents"]
        assert len(pending) == 1 and pending[0]["state"] == "accepted", pending
        assert sql(f"select count(*) from acquisition_intents where id='{intent_id}' and bookkeeping_at is null and state='accepted'") == "1"
        sql("alter table history_events drop constraint fail_grab_history")
        docker("restart", API)
        wait_for(lambda: request("/api/v1/system/status"))
        reconciled = request(f"/api/v1/acquisition-recovery/{intent_id}", {"action": "check"})
        assert reconciled["download"]["deduplicated"] and reconciled["download"]["acquisitionId"] == intent_id, reconciled
        repeated = request("/api/v1/grabs", acquisition_request)
        assert repeated["deduplicated"] and repeated["acquisitionId"] == intent_id, repeated
        stats = json.loads(docker("exec", CLIENT, "python", "-c", "import urllib.request; print(urllib.request.urlopen('http://127.0.0.1:8080/fixture/stats').read().decode())"))
        assert stats["adds"] == 1, stats
        assert request("/api/v1/acquisition-recovery")["intents"] == []
        assert sql(f"select count(*) from history_events where event_type='release_grabbed' and data->>'acquisitionId'='{intent_id}'") == "1"
        assert sql(f"select count(*) from downloads where acquisition_intent_id='{intent_id}'") == "1"
        print("Packaged acquisition: acknowledgement loss reconciled; history failure retained accepted receipt; process restart repaired bookkeeping exactly once without another add")
        scan_root = media / "scan-library"
        scan_root.mkdir()
        for index in range(1201):
            (scan_root / f"Book-{index:05}.mp3").write_bytes(f"controlled scan fixture {index}".encode())
        scan = request("/api/v1/library/scan", {"root": "/fixture/scan-library", "limit": 10})
        scan_id = str(uuid.UUID(scan["jobId"]))
        assert scan["hasMore"] and scan["scanned"] < 1201, scan
        docker("kill", "--signal", "KILL", API)
        # Model the lease expiry after a crash without spending two minutes idle.
        sql(f"update library_scan_jobs set lease_expires_at=now()-interval '1 second' where id='{scan_id}'")
        docker("start", API)
        wait_for(lambda: request("/api/v1/system/status"))
        def completed_scan(scan_job_id):
            job = next(row for row in request("/api/v1/library/scans")["scans"] if row["id"] == scan_job_id)
            if job["state"] == "failed":
                raise AssertionError(job)
            if job["state"] != "completed":
                raise OSError("scan still running")
            return job
        scan = wait_for(lambda: completed_scan(scan_id))
        assert scan["scanned"] == 1201 and scan["missing"] == 0, scan
        (scan_root / "Book-00000.mp3").unlink()
        second = request("/api/v1/library/scans", {"root": "/fixture/scan-library"})
        second_id = str(uuid.UUID(second["id"]))
        second = wait_for(lambda: completed_scan(second_id))
        assert second["missing"] == 1, second
        assert sql("select count(*) from files where scan_root='/fixture/scan-library'") == "1201"
        assert sql("select count(*) from files where scan_root='/fixture/scan-library' and presence_state='missing'") == "1"
        scan_root.rename(media / "scan-offline")
        try:
            request("/api/v1/library/scan", {"root": "/fixture/scan-library"})
            raise AssertionError("unavailable scan root reported success")
        except urllib.error.HTTPError as error:
            assert error.code == 502, error.code
        assert sql("select count(*) from files where scan_root='/fixture/scan-library' and presence_state='missing'") == "1"
        print("Packaged scans: 1,201 files resume through scheduler after process kill; missing file confirmed only after completion; unavailable root retains prior presence")
        # Reviewed completed replacement must retain all old bytes until commit.
        replacement_root = media / "downloads" / "Replacement"
        replacement_root.mkdir()
        replacement_files = []
        for name in ("Disc 1/01.mp3", "Disc 1/02.mp3", "Disc 2/01.mp3", "cover.jpg"):
            target = replacement_root / name
            target.parent.mkdir(exist_ok=True)
            target.write_bytes(b"controlled replacement chapter " + name.encode())
            replacement_files.append({"name": "Replacement/" + name, "size": target.stat().st_size, "progress": 1, "priority": 1})
        client_rows.append({"status": {"hash": "replacement-chapters", "name": "Replacement", "category": "books-audiobook", "save_path": "/fixture/downloads", "state": "pausedUP", "progress": 1, "tags": f"librarry,wanted:{audio_id}"}, "files": replacement_files})
        (media / "client.json").write_text(json.dumps(client_rows))
        replacement_download = sql(f"insert into downloads(client,external_id,name,category,save_path,state,progress,tags) values('qBittorrent','replacement-chapters','Replacement','books-audiobook','/fixture/downloads','pausedUP',1,'librarry,wanted:{audio_id}') returning id")
        replacement_download = str(uuid.UUID(replacement_download.splitlines()[0]))
        queued = request("/api/v1/library/import-completed", {"downloadIds": ["replacement-chapters"], "importMode": "copy", "conflictAction": "replace"})
        assert queued["reviewQueued"] == 1, queued
        replacement_review = next(row for row in request("/api/v1/library/import-reviews?status=pending")["reviews"] if row["downloadId"] == "replacement-chapters")
        replacement_request = {"action": "import", "wantedId": audio_id, "confirmIdentity": True, "importMode": "copy", "conflictAction": "replace", "mapping": [{"relativePath": file["relativePath"], "wantedId": audio_id} for file in replacement_review["metadata"]["payload"]["files"] if file["format"] != "excluded"]}
        replacement_preview = request(f"/api/v1/library/import-reviews/{replacement_review['id']}/preview", replacement_request)
        assert all(file["previousPath"] for file in replacement_preview["operation"]["files"]), replacement_preview
        sql(f"alter table import_operations add constraint inject_completed_replace check(download_record_id<>'{replacement_download}' or state<>'committed')")
        try:
            request(f"/api/v1/library/import-reviews/{replacement_review['id']}/resolve", {**replacement_request, "previewToken": replacement_preview["fingerprint"]})
            raise AssertionError("replacement hid failed commit")
        except urllib.error.HTTPError as error:
            assert error.code >= 400
        replacement_operation = next(row for row in request("/api/v1/library/import-recovery")["operations"] if row["downloadId"] == "replacement-chapters")
        assert replacement_operation["state"] == "failed", replacement_operation
        for file in replacement_operation["files"]:
            previous = media / Path(file["previousPath"]).relative_to("/fixture")
            assert hashlib.sha256(previous.read_bytes()).hexdigest() == file["previousSha256"]
            assert (media / Path(file["sourcePath"]).relative_to("/fixture")).exists()
        sql("alter table import_operations drop constraint inject_completed_replace")
        docker("restart", API)
        wait_for(lambda: request("/api/v1/system/status"))
        replaced = request(f"/api/v1/library/import-operations/{replacement_operation['id']}/retry", {})
        assert replaced["replaced"] is True and {file["id"] for file in replaced["files"]} == {file["id"] for file in audio_files}, replaced
        completed_replacement = next(row for row in request("/api/v1/library/import-recovery")["operations"] if row["id"] == replacement_operation["id"])
        assert completed_replacement["replacementCleanupState"] == "cleaned" and completed_replacement["cleanupState"] == "blocked", completed_replacement
        for file in completed_replacement["files"]:
            assert not (media / Path(file["previousPath"]).relative_to("/fixture")).exists()
            assert (media / Path(file["sourcePath"]).relative_to("/fixture")).exists()
        request(f"/api/v1/library/import-reviews/{replacement_review['id']}/resolve", {**replacement_request, "previewToken": replacement_preview["fingerprint"]})
        print("Packaged completed replacement: reviewed chapter/sidecar replacements retain old bytes after failed commit; restart/retry preserves file IDs; backup cleanup never removes download sources")
        # Move a unique imported chapter outside the app, then interrupt the
        # reconciliation commit. The same original ID must survive restart/retry.
        request("/api/v1/library/scan", {"format": "audiobook"})
        chapter_id = str(uuid.UUID(audio_files[0]["id"]))
        old_chapter_path = audio_files[0]["path"]
        new_chapter_path = str(Path(old_chapter_path).with_name("renamed-chapter.mp3"))
        docker("exec", API, "mv", old_chapter_path, new_chapter_path)
        sql("alter table library_scan_moves add constraint inject_move_commit_failure check(false)")
        try:
            request("/api/v1/library/scan", {"format": "audiobook"})
            raise AssertionError("scan concealed move commit failure")
        except urllib.error.HTTPError as error:
            assert error.code >= 400
        failed_move = next(row for row in request("/api/v1/library/scans")["scans"] if row["state"] == "failed" and row["format"] == "audiobook")
        assert sql(f"select path from files where id='{chapter_id}'") == old_chapter_path
        assert sql("select count(*) from library_scan_moves") == "0"
        sql("alter table library_scan_moves drop constraint inject_move_commit_failure")
        docker("restart", API)
        wait_for(lambda: request("/api/v1/system/status"))
        request(f"/api/v1/library/scans/{failed_move['id']}", {"action": "retry"})
        reconciled = wait_for(lambda: completed_scan(failed_move["id"]))
        assert reconciled["moved"] == 1 and reconciled["missing"] == 0, reconciled
        assert sql(f"select path from files where id='{chapter_id}'") == new_chapter_path
        assert sql(f"select count(*) from file_wanted_links where file_id='{chapter_id}' and wanted_item_id='{audio_id}'") == "1"
        assert sql(f"select count(*) from file_download_links where file_id='{chapter_id}'") == "2"
        assert sql(f"select count(*) from import_operation_files where file_id='{chapter_id}' and destination_path<>'{old_chapter_path}'") == "0"
        history = request(f"/api/v1/library/scans/{failed_move['id']}/moves")
        assert len(history["moves"]) == 1 and history["moves"][0]["fileId"] == chapter_id, history
        assert history["moves"][0]["previousPath"] == old_chapter_path and history["moves"][0]["currentPath"] == new_chapter_path
        assert not (media / Path(old_chapter_path).relative_to("/fixture")).exists()
        assert (media / Path(new_chapter_path).relative_to("/fixture")).exists()
        assert (media / Path(audio_files[0]["sourcePath"]).relative_to("/fixture")).exists()
        print("Packaged move reconciliation: failed commit rolled back; process restart/retry retained original chapter ID, book/download links and historical manifest; sources retained")
        # A read-only legacy report must inspect all pages, including clean ones.
        sql("insert into files(media_format,path,size_bytes,checksum,import_status,metadata) values"
            "('audiobook','/fixture/legacy-one.mp3',42,repeat('a',64),'imported','{\"wantedId\":\"missing-book\"}'),"
            "('audiobook','/fixture/legacy-two.mp3',42,repeat('a',64),'imported','{}')")
        snapshot_query = "select md5(jsonb_agg(to_jsonb(f) order by id)::text) from files f"
        before_preview = sql(snapshot_query)
        cursor, findings, checked = "", [], 0
        for _ in range(100):
            preview = request("/api/v1/library/repair-preview" + ("?cursor=" + cursor if cursor else ""))
            assert preview["readOnly"] is True and isinstance(preview["findings"], list), preview
            checked += preview["checked"]
            findings.extend(preview["findings"])
            cursor = preview.get("nextCursor", "")
            if not cursor:
                break
        else:
            raise AssertionError("repair preview did not finish")
        assert checked > 1201 and {"wanted_link", "duplicate_content", "legacy_audio_file"}.issubset({f["kind"] for f in findings}), findings
        assert sql(snapshot_query) == before_preview
        print("Packaged repair preview: all pages inspected; legacy link, duplicate and unverified audiobook evidence explained; file records unchanged")
        roots = request("/api/v1/library/root-folders")["rootFolders"]
        author_root = next((root for root in roots if root["path"] == "/fixture/ebooks"), None)
        if author_root is None:
            author_root = request("/api/v1/library/root-folders", {
                "name": "Author fixture", "path": "/fixture/ebooks", "mediaFormat": "ebook"})["rootFolder"]
        author = request("/api/v1/authors", {"authorName": "Packaged Author", "provider": "Hardcover",
            "providerKey": "hardcover-author:7", "format": "ebook", "missingBookPolicy": "none",
            "rootFolderId": author_root["id"], "qualityProfile": "fixture-profile", "tags": ["fixture-author"]})
        assert author["rootFolderId"] == author_root["id"] and author["qualityProfile"] == "fixture-profile", author
        docker("restart", API)
        wait_for(lambda: request("/api/v1/system/status"))
        saved_author = next(item for item in request("/api/v1/authors?status=all")["authors"] if item["id"] == author["id"])
        assert saved_author["rootFolderId"] == author_root["id"] and saved_author["tags"] == ["fixture-author"], saved_author
        print("Packaged author defaults: selected root/profile/tags survive process restart; monitoring disabled and no provider request")
        empty_author = request("/api/v1/library/authors/" + author["id"])
        assert empty_author["books"] == [] and empty_author["subscriptions"][0]["id"] == author["id"], empty_author
        linked_author = request("/api/v1/wanted/" + wanted_id)["authors"][0]
        author_page = request("/api/v1/library/authors/" + linked_author["id"] + "?limit=100")
        assert wanted_id in {book["id"] for book in author_page["books"]}, author_page
        assert author_page["choices"] == [] and author_page["totalBooks"] >= 1, author_page
        print("Packaged author detail: direct subscription and recorded-author links retain imported books after restart")
        # Native collection counts are based on the recorded writer identity.
        linked_author_id = str(uuid.UUID(linked_author["id"]))
        linked_subscription_id = sql(f"insert into author_subscriptions(provider,provider_key,author_name,wanted_format,status,monitor_new_items,missing_book_policy) select provider,provider_key,'Packaged linked author','ebook','unmonitored',false,'none' from provider_records where entity_type='author' and entity_id='{linked_author_id}' limit 1 returning id").splitlines()[0]
        author_first = request("/api/v1/library/authors?status=all&limit=1")
        author_collection = author_first
        author_ids = set()
        while True:
            assert author_collection["total"] == 2 and author_collection["filtered"] == 2, author_collection
            for item in author_collection["authors"]:
                assert item["id"] not in author_ids, item
                author_ids.add(item["id"])
                if item["id"] == linked_subscription_id:
                    assert item["identityLinked"] and item["totalBooks"] == author_page["totalBooks"], item
                    assert sum(item["counts"].values()) == item["totalBooks"], item
                else:
                    assert item["id"] == author["id"] and not item["identityLinked"] and item["totalBooks"] == 0, item
            if not author_collection.get("nextCursor"):
                break
            author_collection = request("/api/v1/library/authors?status=all&limit=1&cursor=" + author_collection["nextCursor"])
        assert author_ids == {author["id"], linked_subscription_id}, author_ids
        # Prove native presence against real packaged import/scan observations.
        imported_book = request("/api/v1/wanted/" + wanted_id)
        assert imported_book["stateEvidence"]["files"]["state"] == "present", imported_book
        audio_book_id = audio_files[0]["metadata"]["wantedId"]
        audio_book = request("/api/v1/wanted/" + audio_book_id)
        assert audio_book["stateEvidence"]["files"]["state"] == "present", audio_book
        current_audio = next(file for file in request("/api/v1/library/files")["files"] if file["id"] == audio_files[0]["id"])
        lost_chapter = media / Path(current_audio["path"]).relative_to("/fixture")
        chapter_bytes = lost_chapter.read_bytes()
        docker("exec", "--user", "0", API, "rm", "--", current_audio["path"])
        loss_scan = request("/api/v1/library/scans", {"root": "/fixture/audiobooks"})
        wait_for(lambda: completed_scan(loss_scan["id"]))
        incomplete = request("/api/v1/wanted/" + audio_book_id)
        assert incomplete["derivedState"] == "incomplete", incomplete
        assert incomplete["stateEvidence"]["files"]["state"] == "incomplete", incomplete
        incomplete_page = request("/api/v1/library/books?state=incomplete")
        assert audio_book_id in {book["id"] for book in incomplete_page["books"]}, incomplete_page
        assert incomplete_page["filtered"] == incomplete_page["counts"]["incomplete"], incomplete_page
        docker("exec", "--user", "0", "-i", API, "tee", current_audio["path"], binary=True, input=chapter_bytes)
        restored_scan = request("/api/v1/library/scans", {"root": "/fixture/audiobooks"})
        wait_for(lambda: completed_scan(restored_scan["id"]))
        restored_book = request("/api/v1/wanted/" + audio_book_id)
        assert restored_book["stateEvidence"]["files"]["state"] == "present", restored_book
        print("Packaged book evidence: complete import, missing chapter after scan, and restored complete audiobook verified through native book API")
        expected_books = int(sql("select count(*) from wanted_items where status not in ('removed','ignored')"))
        native_first = request("/api/v1/library/books?limit=1&sort=title")
        native_page = native_first
        native_ids = set()
        while True:
            assert native_page["total"] == expected_books and native_page["filtered"] == expected_books, native_page
            assert len(native_page["books"]) <= 1, native_page
            for book in native_page["books"]:
                assert book["id"] not in native_ids, native_page
                native_ids.add(book["id"])
            if not native_page.get("nextCursor"):
                break
            native_page = request("/api/v1/library/books?limit=1&sort=title&cursor=" + native_page["nextCursor"])
        assert len(native_ids) == expected_books and wanted_id in native_ids and audio_book_id in native_ids
        # Checks advance durably even when file evidence makes a search unnecessary.
        # The fixture has no configured indexer, and auto-grab remains disabled.
        first_checks = request("/api/v1/wanted/monitor", {"limit": 1, "autoGrab": False})
        assert first_checks["wantedChecked"] == 1, first_checks
        first_checked_id = str(uuid.UUID(first_checks["items"][0]["wantedItem"]["id"]))
        assert sql(f"select last_monitor_checked_at is not null from wanted_items where id='{first_checked_id}'") == "t"
        docker("restart", API)
        wait_for(lambda: request("/api/v1/system/status"))
        second_checks = request("/api/v1/wanted/monitor", {"limit": 1, "autoGrab": False})
        assert second_checks["wantedChecked"] == 1, second_checks
        assert second_checks["items"][0]["wantedItem"]["id"] != first_checked_id, second_checks
        print("Packaged worker fairness: checked book retained scheduling progress across restart; next batch advanced without a real indexer or acquisition")
        if native_first.get("nextCursor"):
            after_restart = request("/api/v1/library/books?limit=1&sort=title&cursor=" + native_first["nextCursor"])
            assert after_restart["total"] == expected_books and after_restart["books"], after_restart
            assert after_restart["books"][0]["id"] != native_first["books"][0]["id"], after_restart
        print("Packaged book collection: exact counts, complete keyset traversal, incomplete-audio filtering and cursor continuation after restart verified")
        author_after_restart = request("/api/v1/library/authors?status=all&limit=1&cursor=" + author_first["nextCursor"])
        assert author_after_restart["total"] == 2 and author_after_restart["authors"][0]["id"] != author_first["authors"][0]["id"], author_after_restart
        print("Packaged author collection: identity counts, unlinked identities, complete traversal and cursor continuation after restart verified")
        dump = docker("exec", PG, "pg_dump", "-U", "postgres", "-Fc", "librarry_test", binary=True)
        docker("exec", PG, "createdb", "-U", "postgres", "librarry_restore")
        docker("exec", "-i", PG, "pg_restore", "-U", "postgres", "-d", "librarry_restore", "--exit-on-error", binary=True, input=dump)
        for query in ("select count(*) from schema_migrations", "select count(*) from wanted_items",
                      "select count(*) from downloads", "select count(*) from files",
                      "select count(*) from file_wanted_links", "select count(*) from file_download_links",
                      "select * from librarry_book_file_evidence(null) order by wanted_id",
                      "select id,root_folder_id,quality_profile,tags from author_subscriptions order by id",
                      "select id,root_folder_id from author_metadata_reviews order by id",
                      "select id,state,cleanup_state,source_kind,request_key,replacement_cleanup_state,replacement_cleanup_error from import_operations order by id", "select operation_id,sha256,file_id,stage_path,stage_lease_token,previous_path,previous_sha256,source_removed from import_operation_files order by id",
                      "select id,metadata->'verifiedDownload' from files order by id",
                      "select id,scope_key,request_key,state,external_id,result,selection,bookkeeping_required,bookkeeping_at from acquisition_intents order by id",
                      "select id,acquisition_intent_id,release_id from downloads order by id",
                      "select id,event_type,entity_id,data from history_events order by id",
                      "select id,current_release_id,current_release_score from wanted_items order by id",
                      "select id,state,phase,scanned,missing,moved from library_scan_jobs order by id",
                      "select * from library_scan_moves order by job_id,file_id",
                      "select * from library_scan_discoveries order by file_id",
                      "select path,identity,completed_job_id from library_scan_roots order by path",
                      "select id,presence_state,scan_root,last_seen_scan_id,scan_file_stamp from files order by id"):
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
        try:
            request("/api/v1/library/repair-preview")
            raise AssertionError("repair preview bypassed forms authentication")
        except urllib.error.HTTPError as error:
            assert error.code == 401
        try:
            request("/api/v1/providers/Hardcover/check", {}, method="POST")
            raise AssertionError("provider check bypassed forms authentication")
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
