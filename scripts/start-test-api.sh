#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
: "${LIBRARRY_TEST_DATABASE_URL:?Set a disposable Postgres URL for browser tests}"
# The fixture server must not inherit live integrations, credentials, or jobs.
exec env -i PATH="$PATH" HOME="$HOME" TMPDIR="${TMPDIR:-/tmp}" \
 LIBRARRY_DATABASE_URL="$LIBRARRY_TEST_DATABASE_URL" \
 LIBRARRY_LISTEN_ADDR=127.0.0.1:18182 LIBRARRY_AUTH_METHOD=none \
 LIBRARRY_WEB_ORIGIN=http://127.0.0.1:15173 \
 LIBRARRY_MONITOR_ENABLED=false LIBRARRY_AUTHOR_MONITOR_ENABLED=false \
 LIBRARRY_FEED_SYNC_ENABLED=false LIBRARRY_FAILED_DOWNLOAD_ENABLED=false \
 LIBRARRY_UPGRADE_SEARCH_ENABLED=false LIBRARRY_CALIBRE_REFRESH_ENABLED=false \
 LIBRARRY_COMPLETED_IMPORT_ENABLED=false LIBRARRY_COMPLETED_REMOVE_ENABLED=false \
 LIBRARRY_BACKUP_ENABLED=false \
 go run ./backend/cmd/librarry
