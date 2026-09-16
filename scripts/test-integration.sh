#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ -n "${LIBRARRY_TEST_DATABASE_URL:-}" ]]; then
  exec go test -race ./...
fi
# A unique disposable container; never touches an existing database or volume.
container="librarry-test-$(date +%s)-$$"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --name "$container" -e POSTGRES_PASSWORD=librarry-test -e POSTGRES_DB=librarry_test -p 127.0.0.1::5432 postgres:16-alpine >/dev/null
for attempt in {1..60}; do
  if docker exec "$container" pg_isready -h 127.0.0.1 -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
port=$(docker port "$container" 5432/tcp | awk -F: '{print $NF}')
export LIBRARRY_TEST_DATABASE_URL="postgres://postgres:librarry-test@127.0.0.1:${port}/librarry_test?sslmode=disable"
go test -race ./...
