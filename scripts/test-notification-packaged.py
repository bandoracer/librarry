#!/usr/bin/env python3
"""Durable native delivery with disposable Postgres/API and a local receiver only."""
import json
import os
import subprocess
import sys
import time
import urllib.request
import uuid

DOCKER = ["docker"] + (["--context", os.environ["DOCKER_CONTEXT"]] if os.environ.get("DOCKER_CONTEXT") else [])
PREFIX = "librarry-notification-test-" + uuid.uuid4().hex[:10]
PG, API, RECEIVER = [PREFIX + suffix for suffix in ("-pg", "-api", "-receiver")]
IMAGE = sys.argv[1] if len(sys.argv) > 1 else "librarry-api:stabilization"
containers = []
RECEIVER_CODE = '''
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
import json, time
messages=[]
class Receiver(BaseHTTPRequestHandler):
 def do_GET(self):
  body=json.dumps(messages).encode(); self.send_response(200); self.end_headers(); self.wfile.write(body)
 def do_POST(self):
  body=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
  messages.append({'id':self.headers.get('X-Librarry-Delivery-ID'),'event':body,'path':self.path,'method':self.command,'authorization':self.headers.get('Authorization')})
  if self.path=='/slow': time.sleep(30)
  self.send_response(204); self.end_headers()
 do_PUT=do_POST
 def log_message(self,*args): pass
ThreadingHTTPServer(('0.0.0.0',8080),Receiver).serve_forever()
'''

def docker(*args):
    return subprocess.run(DOCKER + list(args), check=True, capture_output=True, text=True).stdout.strip()

def sql(query):
    return docker("exec", PG, "psql", "-U", "postgres", "-d", "fixture", "-At", "-v", "ON_ERROR_STOP=1", "-c", query)

def request(base, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    with urllib.request.urlopen(urllib.request.Request(base + path, data=data, headers={"Content-Type": "application/json"}), timeout=10) as response:
        return json.load(response)

def wait(check):
    for _ in range(150):
        try:
            result = check()
            if result:
                return result
        except (OSError, subprocess.CalledProcessError):
            pass
        time.sleep(0.2)
    raise RuntimeError("disposable notification condition did not become true")

def address(name):
    return "http://127.0.0.1:" + docker("port", name, "8080/tcp").rsplit(":", 1)[1]

def restart():
    docker("start", API)
    base = address(API)
    wait(lambda: request(base, "/healthz"))
    return base

def settle(base):
    wait(lambda: not next(t for t in request(base,"/api/v1/system/tasks")["tasks"] if t["id"]=="notification-delivery")["running"])

def trigger(base):
    settle(base)
    request(base, "/api/v1/system/tasks/notification-delivery/run", {})

try:
    docker("network", "create", PREFIX)
    containers.append(PG)
    docker("run", "-d", "--name", PG, "--network", PREFIX, "-e", "POSTGRES_PASSWORD=fixture-only", "-e", "POSTGRES_DB=fixture", "postgres:16-alpine")
    wait(lambda: docker("exec", PG, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"))
    containers.append(RECEIVER)
    docker("run", "-d", "--name", RECEIVER, "--network", PREFIX, "-p", "127.0.0.1::8080", "python:3.13-alpine", "python", "-u", "-c", RECEIVER_CODE)
    receiver = address(RECEIVER)
    wait(lambda: request(receiver,"/") == [])
    containers.append(API)
    flags = ["-e", f"LIBRARRY_DATABASE_URL=postgres://postgres:fixture-only@{PG}:5432/fixture?sslmode=disable"]
    for worker in ("MONITOR", "AUTHOR_MONITOR", "FEED_SYNC", "FAILED_DOWNLOAD", "UPGRADE_SEARCH", "CALIBRE_REFRESH", "COMPLETED_IMPORT", "COMPLETED_REMOVE", "BACKUP", "IMPORT_LIST_SYNC"):
        flags += ["-e", f"LIBRARRY_{worker}_ENABLED=false"]
    docker("run", "-d", "--name", API, "--network", PREFIX, "-p", "127.0.0.1::8080", *flags, IMAGE)
    base = address(API)
    wait(lambda: request(base,"/healthz"))
    target = request(base,"/api/v1/notifications", {"name":"Disposable receiver","type":"webhook","enabled":True,"settings":{"url":f"http://{RECEIVER}:8080/accepted"}})["target"]
    docker("stop", API)
    sql("insert into history_events(event_type,message,data) values('book_imported','Fixture imported','{\"paths\":[\"/fixture/Walden.epub\"]}')")
    assert sql("select state from notification_deliveries") == "pending"
    base = restart()
    trigger(base)
    wait(lambda: sql("select state from notification_deliveries") == "accepted")
    assert len(request(receiver,"/")) == 1
    trigger(base); settle(base)
    assert len(request(receiver,"/")) == 1
    # Receive the request but kill the API before the delayed response.
    sql(f"update notification_targets set settings=jsonb_build_object('url','http://{RECEIVER}:8080/slow'),updated_at=clock_timestamp() where id='{target['id']}'")
    sql("insert into history_events(event_type,message) values('release_grabbed','Fixture acquired')")
    trigger(base)
    wait(lambda: len(request(receiver,"/")) == 2)
    assert sql("select count(*) from notification_deliveries where state='sending'") == "1"
    docker("kill",API)
    base = restart(); trigger(base)
    wait(lambda: sql("select count(*) from notification_deliveries where state='uncertain'") == "1")
    settle(base)
    assert len(request(receiver,"/")) == 2
    page = request(base,"/api/v1/notification-deliveries")
    uncertain = next(d for d in page["items"] if d["state"] == "uncertain")
    request(base,f"/api/v1/notification-deliveries/{uncertain['id']}/resolve",{"action":"accepted","confirm":True,"expectedUpdatedAt":uncertain["updatedAt"]})
    assert sql("select count(*) from notification_deliveries where state='accepted'") == "2"
    assert len(request(receiver,"/")) == 2
    assert len({m['id'] for m in request(receiver,'/')}) == 2
    print("Packaged native outbox: pending restart, SIGKILL uncertainty and confirmed acceptance without resend verified")
    sql(f"update notification_targets set enabled=false,updated_at=clock_timestamp() where id='{target['id']}'")
    compat = request(base,"/api/v1/notification",{"name":"Disposable Readarr receiver","implementation":"Webhook","enable":True,"url":f"http://{RECEIVER}:8080/compat","method":"PUT","username":"fixture-user","password":"fixture-password"})
    compat_id = int(compat["id"])
    docker("stop",API)
    wanted = sql("insert into wanted_items(wanted_format,title,author_name) values('ebook','Walden fixture','Fixture author') returning id").splitlines()[0]
    download = sql("insert into downloads(client,external_id,name,category,save_path,state) values('qBittorrent','compat-fixture','Walden release','books','/fixture/downloads','paused') returning id").splitlines()[0]
    history = f"insert into history_events(event_type,entity_type,entity_id,message,data) values('release_grabbed','wanted_item','{wanted}','Fixture acquired',jsonb_build_object('downloadRecordId','{download}','title','Walden release'))"
    sql(history)
    sql(f"update wanted_items set title='Changed after capture' where id='{wanted}'")
    base=restart(); trigger(base)
    wait(lambda: sql("select count(*) from notification_deliveries where target_kind='compat' and state='accepted'")=="1")
    messages=request(receiver,"/"); assert len(messages)==3,messages
    saved=messages[-1]
    assert saved["method"]=="PUT" and saved["path"]=="/compat" and saved["authorization"].startswith("Basic "),saved
    assert saved["event"]["book"]["title"]=="Walden fixture" and saved["event"]["downloadId"]=="compat-fixture",saved
    sql(f"update compat_resources set payload=payload||jsonb_build_object('url','http://{RECEIVER}:8080/slow'),updated_at=clock_timestamp() where resource_type='notification' and compat_id={compat_id}")
    sql(history); trigger(base)
    wait(lambda: len(request(receiver,"/"))==4)
    docker("kill",API); base=restart(); trigger(base)
    wait(lambda: sql("select count(*) from notification_deliveries where target_kind='compat' and state='uncertain'")=="1")
    settle(base); assert len(request(receiver,"/"))==4
    pending=next(d for d in request(base,"/api/v1/notification-deliveries")["items"] if d["targetKind"]=="compat" and d["state"]=="uncertain")
    request(base,f"/api/v1/notification-deliveries/{pending['id']}/resolve",{"action":"cancel","confirm":True,"expectedUpdatedAt":pending["updatedAt"]})
    assert sql("select count(*) from notification_deliveries where target_kind='compat' and state='cancelled'")=="1"
    print("Packaged Readarr webhooks: API-created target, PUT/Basic, immutable book snapshot, restart recovery, SIGKILL uncertainty and cancellation without resend verified")

    # Compact resolved native + compatibility history, retaining one unresolved
    # old message, all event/source identities, and current worker diagnostics.
    docker("stop",API)
    sources = sql("select source_key from notification_events where id in(select event_id from notification_deliveries where resolved_at is not null) order by source_key").splitlines()
    sql(history)
    sql("update notification_deliveries set state='uncertain' where resolved_at is null")
    sql("update notification_events set created_at=now()-interval '100 days'")
    sql("update notification_deliveries set resolved_at=now()-interval '91 days' where resolved_at is not null")
    sql("insert into worker_tasks(task_id) values('disabled-fixture')")
    current = sql("insert into worker_task_runs(task_id,trigger,backend_pid,state,reviewed_at) values('disabled-fixture','fixture',0,'failed',now()-interval '100 days') returning id").splitlines()[0]
    sql(f"update worker_tasks set run_id='{current}' where task_id='disabled-fixture'")
    sql("insert into worker_task_runs(task_id,trigger,backend_pid,state,reviewed_at) values('disabled-fixture','fixture',0,'failed',now()-interval '100 days')")
    base=restart()
    request(base,"/api/v1/system/tasks/history-maintenance/run",{})
    def maintenance_finished():
        task=next(t for t in request(base,"/api/v1/system/tasks")["tasks"] if t["id"]=="history-maintenance")
        return task if task.get("runState")=="completed" else None
    maintenance=wait(maintenance_finished)
    assert maintenance["details"]["counts"]["deliveriesPruned"]==4,maintenance
    assert maintenance["details"]["counts"]["reviewedRunsPruned"]==1,maintenance
    assert sql("select count(*) from notification_deliveries")=="1"
    assert sql("select state from notification_deliveries")=="uncertain"
    assert sql("select count(*) from worker_task_runs where task_id='disabled-fixture'")=="1"
    archived_before=sql("select count(*) from notification_events where archived_at is not null")
    for source in sources:
        assert sql(f"select event='{{}}'::jsonb and compat_context='{{}}'::jsonb and archived_at is not null from notification_events where source_key='{source}'")=="t"
        sql(f"select enqueue_native_notification('{source}','{{\"type\":\"import\",\"title\":\"Old event replay\"}}')")
    docker("stop",API);base=restart();trigger(base);settle(base)
    assert sql("select count(*) from notification_deliveries")=="1"
    assert sql("select count(*) from notification_events where archived_at is not null")==archived_before
    assert len(request(receiver,"/"))==4
    print("Packaged retention: native/compat resolution, 90-day window, unresolved preservation, disabled-worker cleanup, restart and archived-event replay barriers verified")

finally:
    for container in reversed(containers):
        subprocess.run(DOCKER+["rm","-f",container],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    subprocess.run(DOCKER+["network","rm",PREFIX],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
