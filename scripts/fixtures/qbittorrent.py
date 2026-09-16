#!/usr/bin/env python3
"""Isolated qBittorrent contract fixture. Simulated adds never download data."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import json
from urllib.parse import urlsplit, parse_qs
from email.parser import BytesParser
from email.policy import default
from threading import Lock

ACQUISITION_HASH = "0123456789abcdef0123456789abcdef01234567"
accepted = []
add_count = 0
lock = Lock()

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def send(self, value, status=200):
        data = value.encode() if isinstance(value, str) else json.dumps(value).encode()
        self.send_response(status)
        self.send_header("Content-Type", "text/plain" if isinstance(value, str) else "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_POST(self):
        global add_count
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        if self.path == "/api/v2/torrents/add":
            message = BytesParser(policy=default).parsebytes(("Content-Type: " + self.headers["Content-Type"] + "\r\n\r\n").encode() + body)
            fields = {part.get_param("name", header="content-disposition"): part.get_content() for part in message.iter_parts()}
            if fields.get("urls") != "magnet:?xt=urn:btih:" + ACQUISITION_HASH:
                self.send({"error": "only the controlled acquisition fixture is accepted"}, 400)
                return
            with lock:
                add_count += 1
                accepted.append({"hash": ACQUISITION_HASH, "name": "Acquisition fixture", "state": "downloading", "progress": 0, "category": fields.get("category", ""), "tags": fields.get("tags", "")})
            self.send({"error": "simulated acknowledgement loss after acceptance"}, 502)
            return
        if self.path in ("/api/v2/auth/login", "/api/v2/torrents/createCategory"):
            self.send("Ok.")
        else:
            self.send({"error": "fixture does not allow download mutations"}, 405)

    def do_GET(self):
        url = urlsplit(self.path)
        params = parse_qs(url.query)
        config = Path("/fixture/client.json")
        rows = json.loads(config.read_text()) if config.exists() else []
        if url.path == "/api/v2/torrents/info":
            hashes = params.get("hashes", [""])[0].split("|")
            with lock:
                statuses = [row["status"] for row in rows] + list(accepted)
            self.send([row for row in statuses if hashes == [""] or row["hash"] in hashes])
        elif url.path == "/fixture/stats":
            with lock:
                self.send({"adds": add_count})
        elif url.path == "/api/v2/torrents/files":
            target = params.get("hash", [""])[0]
            row = next((row for row in rows if row["status"]["hash"] == target), None)
            self.send(row["files"] if row else [])
        elif url.path == "/api/v2/torrents/trackers":
            self.send([])
        elif url.path in ("/api/v2/torrents/properties", "/api/v2/transfer/info", "/api/v2/torrents/categories"):
            self.send({})
        elif url.path == "/api/v2/sync/torrentPeers":
            self.send({"peers": {}})
        elif url.path == "/api/v2/app/version":
            self.send("v5.0.4-contract-fixture")
        else:
            self.send({"error": "unsupported fixture route"}, 404)

ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
