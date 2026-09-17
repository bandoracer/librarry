#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
for variant in deploy/docker-compose.yml deploy/docker-compose.build.yml deploy/unraid/docker-compose.yml; do
  LIBRARRY_COMPLETED_REMOVE_ENABLED=false LIBRARRY_COMPLETED_IMPORT_MODE=copy \
  docker compose --env-file deploy/.env.example -f "$variant" config --format json |
  python3 -c 'import sys,json; env=json.load(sys.stdin)["services"]["api"]["environment"]; assert env["LIBRARRY_COMPLETED_REMOVE_ENABLED"]=="false"; assert env["LIBRARRY_COMPLETED_IMPORT_MODE"]=="copy"'
done
go test ./backend/internal/config
