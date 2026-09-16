#!/usr/bin/env python3
"""Read-only qBittorrent contract fixture. It never downloads or deletes data."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import json
from urllib.parse import urlsplit, parse_qs

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
        self.rfile.read(int(self.headers.get("Content-Length", 0)))
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
            self.send([row["status"] for row in rows if hashes == [""] or row["status"]["hash"] in hashes])
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
