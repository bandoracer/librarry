#!/usr/bin/env python3
"""Prove shared worker ownership with two disposable API processes and SIGKILL."""
import json
import os
import subprocess
import sys
import time
import urllib.request
import urllib.error
import uuid

DOCKER = ["docker"] + (["--context", os.environ["DOCKER_CONTEXT"]] if os.environ.get("DOCKER_CONTEXT") else [])
PREFIX = "librarry-worker-test-" + uuid.uuid4().hex[:10]
PG, FIRST, SECOND = [PREFIX + suffix for suffix in ("-pg", "-first", "-second")]
IMAGE = sys.argv[1] if len(sys.argv) > 1 else "librarry-api:stabilization"
containers = []
blocker = None

def docker(*args):
    return subprocess.run(DOCKER + list(args), check=True, capture_output=True, text=True).stdout.strip()

def sql(query):
    return docker("exec", PG, "psql", "-U", "postgres", "-d", "fixture", "-At", "-v", "ON_ERROR_STOP=1", "-c", query)

def request(base, path, post=False, body=None):
    with urllib.request.urlopen(urllib.request.Request(base + path, data=json.dumps(body or {}).encode() if post else None, headers={"Content-Type": "application/json"}), timeout=10) as response:
        return json.load(response)

def wait(check):
    for _ in range(100):
        try:
            result = check()
            if result:
                return result
        except (OSError, subprocess.CalledProcessError):
            pass
        time.sleep(0.2)
    raise RuntimeError("disposable worker condition did not become true")

def start_api(name, monitor=False):
    containers.append(name)
    flags = ["-e", f"LIBRARRY_DATABASE_URL=postgres://postgres:fixture-only@{PG}:5432/fixture?sslmode=disable"]
    for worker in ("MONITOR", "AUTHOR_MONITOR", "FEED_SYNC", "FAILED_DOWNLOAD", "UPGRADE_SEARCH", "CALIBRE_REFRESH", "COMPLETED_IMPORT", "COMPLETED_REMOVE", "BACKUP", "IMPORT_LIST_SYNC"):
        flags += ["-e", f"LIBRARRY_{worker}_ENABLED={str(worker == 'MONITOR' and monitor).lower()}"]
    docker("run", "-d", "--name", name, "--network", PREFIX, "-p", "127.0.0.1::8080", *flags, IMAGE)
    port = docker("port", name, "8080/tcp").rsplit(":", 1)[1]
    base = "http://127.0.0.1:" + port
    wait(lambda: request(base, "/healthz"))
    return base

try:
    docker("network", "create", PREFIX)
    containers.append(PG)
    docker("run", "-d", "--name", PG, "--network", PREFIX, "-e", "POSTGRES_PASSWORD=fixture-only", "-e", "POSTGRES_DB=fixture", "postgres:16-alpine")
    wait(lambda: docker("exec", PG, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"))
    first = start_api(FIRST, monitor=True)
    second = start_api(SECOND)  # Schema is already migrated; this tests worker ownership.
    task = "library-scan"
    route = "/api/v1/system/tasks/" + task
    wait(lambda: sql("select count(*) from worker_tasks where task_id='library-scan'") == "1")
    wait(lambda: not next(t for t in request(first, "/api/v1/system/tasks")["tasks"] if t["id"] == task)["running"])
    sql("update worker_tasks set next_run_at=now()+interval '1 hour' where task_id='library-scan'")
    application = PREFIX + "-blocker"
    query = f"set application_name='{application}'; begin; lock table library_scan_jobs in access exclusive mode; select pg_sleep(90); commit;"
    blocker = subprocess.Popen(DOCKER + ["exec", PG, "psql", "-U", "postgres", "-d", "fixture", "-v", "ON_ERROR_STOP=1", "-c", query], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    wait(lambda: sql(f"select exists(select 1 from pg_locks l join pg_stat_activity a on a.pid=l.pid where a.application_name='{application}' and l.relation='library_scan_jobs'::regclass and l.granted)") == "t")
    assert request(first, route + "/run", True)["started"]
    sql("update worker_tasks set next_run_at=now()+interval '1 hour' where task_id='library-scan'")
    first_run = request(second, route + "/runs")["runs"][0]["id"]
    assert next(t for t in request(second, "/api/v1/system/tasks")["tasks"] if t["id"] == task)["running"]
    try:
        request(second, route + "/run", True)
        raise AssertionError("peer accepted concurrent worker")
    except urllib.error.HTTPError as error:
        assert error.code == 409, error.code
    docker("kill", FIRST)
    wait(lambda: next(t for t in request(second, "/api/v1/system/tasks")["tasks"] if t["id"] == task).get("runState") == "interrupted")
    sql(f"select pg_terminate_backend(pid) from pg_stat_activity where application_name='{application}'")
    blocker.wait(timeout=10)
    blocker = None
    assert request(second, route + "/run", True)["started"]
    wait(lambda: request(second, route + "/runs")["runs"][0]["state"] == "completed")
    history = request(second, route + "/runs")["runs"]
    assert next(r for r in history if r["id"] == first_run)["state"] == "interrupted", history
    interrupted = next(r for r in history if r["id"] == first_run)
    assert "finishedAt" not in interrupted and "durationMs" not in interrupted, interrupted
    completed = history[0]["id"]
    status = next(t for t in request(second, "/api/v1/system/tasks")["tasks"] if t["id"] == task)
    assert status["lastSuccessRunId"] == completed and status["lastSuccessAt"] and status["unreviewedFailures"] == 1, status
    assert history[0]["durationMs"] >= 0, history
    failures = request(second, route + "/runs?view=unreviewed&limit=1&offset=0")
    assert failures["total"] == 1 and failures["runs"][0]["id"] == first_run, failures
    assert request(second, route + "/runs/" + first_run + "/review", True, {"reviewed": True, "expectedState": "interrupted"})["ok"]
    assert request(second, route + "/runs?view=unreviewed")["total"] == 0
    health = "/api/v1/system/tasks/health-check"
    wait(lambda: not next(t for t in request(second, "/api/v1/system/tasks")["tasks"] if t["id"] == "health-check")["running"])
    assert request(second, health + "/run", True)["started"]
    wait(lambda: request(second, health + "/runs")["runs"][0]["state"] == "degraded")
    unhealthy = request(second, health + "/runs")["runs"][0]
    assert unhealthy["details"]["errors"] > 0 and unhealthy["details"]["counts"]["checked"] > 0, unhealthy
    docker("start", FIRST)
    first = "http://127.0.0.1:" + docker("port", FIRST, "8080/tcp").rsplit(":", 1)[1]
    wait(lambda: request(first, "/healthz"))
    assert any(r["id"] == completed and r["state"] == "completed" for r in request(first, route + "/runs")["runs"])
    status = next(t for t in request(first, "/api/v1/system/tasks")["tasks"] if t["id"] == task)
    assert status["lastSuccessRunId"] == completed and status["unreviewedFailures"] == 0, status
    assert next(r for r in request(first, route + "/runs")["runs"] if r["id"] == first_run)["reviewedAt"]
    # Local scheduling is distinct from the shared history: the disabled peer
    # can inspect the enabled peer's completion, but cannot trigger it here.
    disabled = next(t for t in request(second,"/api/v1/system/tasks")["tasks"] if t["id"]=="wanted-monitor")
    assert disabled["enabled"] is False and disabled["available"] is True and "nextRunAt" not in disabled,disabled
    assert "LIBRARRY_MONITOR_ENABLED" in disabled["disabledReason"]
    try:
        request(second,"/api/v1/system/tasks/wanted-monitor/run",True)
        raise AssertionError("disabled peer accepted a manual run")
    except urllib.error.HTTPError as error:
        assert error.code==409,error.code
    assert sql("select count(*) from wanted_items")=="0"  # no provider/search work
    request(first,"/api/v1/system/tasks/wanted-monitor/run",True)
    wait(lambda: next(t for t in request(second,"/api/v1/system/tasks")["tasks"] if t["id"]=="wanted-monitor").get("lastSuccessAt"))
    disabled = next(t for t in request(second,"/api/v1/system/tasks")["tasks"] if t["id"]=="wanted-monitor")
    assert disabled["enabled"] is False and disabled["lastFinishedAt"] and "nextRunAt" not in disabled,disabled
    compat = request(second,"/api/v1/system/task/2")
    assert compat["enabled"] is False and compat["lastExecution"]==disabled["lastFinishedAt"] and compat["nextExecution"]=="0001-01-01T00:00:00Z" and compat["librarryNextExecutionKnown"] is False and compat["librarryLastExecutionKnown"] is True,compat
    import_task = next(t for t in request(second,"/api/v1/system/tasks")["tasks"] if t["id"]=="import-list-sync")
    assert import_task["enabled"] is False and "IMPORT_LIST_SYNC_ENABLED" in import_task["disabledReason"],import_task
    # A fresh API without persistence lists unavailable workers instead of hiding
    # them or creating successful task timestamps.
    no_db = PREFIX + "-no-db"
    containers.append(no_db)
    docker("run","-d","--name",no_db,"--network",PREFIX,"-p","127.0.0.1::8080",IMAGE)
    empty = "http://127.0.0.1:"+docker("port",no_db,"8080/tcp").rsplit(":",1)[1]
    wait(lambda: request(empty,"/healthz"))
    tasks = request(empty,"/api/v1/system/tasks")["tasks"]
    assert len(tasks)==13,tasks
    for task_status in tasks:
        if task_status["id"]!="health-check":
            assert task_status["available"] is False and not task_status["running"] and "nextRunAt" not in task_status and "lastRunAt" not in task_status,task_status
    try:
        request(empty,"/api/v1/system/tasks/wanted-monitor/run",True)
        raise AssertionError("unavailable task ran without persistence")
    except urllib.error.HTTPError as error:
        assert error.code==503,error.code
    unknown = request(empty,"/api/v1/system/task/2")
    assert unknown["lastExecution"]=="0001-01-01T00:00:00Z" and unknown["librarryLastExecutionKnown"] is False and unknown["librarryLastDurationKnown"] is False,unknown
    print("Worker availability: disabled peer preserves history and refuses runs, import-list flag honored, database-free worker inventory and truthful compatibility timestamps verified")
    print("Diagnostic counts, degraded health, review, last success and restart readback verified")
    print("Two packaged APIs: shared running status, peer 409, SIGKILL interruption, peer recovery and persistent history verified")
except Exception:
    for name in (FIRST, SECOND):
        try:
            print(docker("logs", name))
        except subprocess.CalledProcessError:
            pass
    raise
finally:
    for name in reversed(containers):
        subprocess.run(DOCKER + ["rm", "-f", name], capture_output=True)
    if blocker is not None:
        blocker.wait(timeout=10)
    subprocess.run(DOCKER + ["network", "rm", PREFIX], capture_output=True)
