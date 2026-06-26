# Agent Guide

This repository contains Librarry, a self-hosted book manager intended to become
a modern Readarr replacement. Treat it as an early alpha product with real
working integrations, not as a finished Readarr-compatible appliance.

## Product Boundaries

- Build Readarr replacement workflows: metadata search, provider provenance,
  wanted books, author monitoring, release evaluation, imports, library files,
  Readarr-compatible APIs, and operator review.
- Do not turn Librarry into a full qBittorrent, Transmission, or SABnzbd
  replacement. Download-client controls should stay scoped to book acquisition,
  recovery, and import handoff.
- Manual overrides always win over provider data.
- Hardcover is the intended rich primary metadata source, Open Library is the
  open-data backbone, Google Books is exact-match fallback only, and local OPF
  or embedded file metadata is import evidence.
- Do not add Goodreads, Amazon, or Audible scraping to core.

## Current Status

Read [docs/status.md](docs/status.md) before making roadmap, parity, or
deployment claims. It records what has been verified, what is still risky, and
the current live homelab shape.

Important current facts:

- Live homelab deployments use the `media-stack` dataset, not the older `data`
  dataset.
- TrueNAS media mount: `/mnt/HDD_pool/vault/media-stack:/data`.
- Local images are `librarry-api:local` and `librarry-web:local`.
- The live portal, when available, is `http://192.168.1.221:30200/`.
- The Cosmos hostname, when configured, is `https://librarry.borchetta.xyz/`.
- Prowlarr and qBittorrent have been verified through the live app, but
  Hardcover and Google Books credentials still need to be supplied for rich
  metadata coverage.

Never print or commit live API keys, provider tokens, database passwords, or
download-client credentials.

## Repository Map

- `backend/cmd/librarry`: API process entrypoint and worker wiring.
- `backend/internal/api`: native and Readarr-compatible HTTP routes.
- `backend/internal/metadata`: provider adapters, matching, merging, and
  normalization.
- `backend/internal/wanted`: wanted items, release evaluation, monitoring, feed
  sync, upgrades, history, and metadata review.
- `backend/internal/acquisition`: Prowlarr, qBittorrent, Transmission, and
  SABnzbd integrations.
- `backend/internal/library`: scanning, import, file tracking, rename, local
  metadata extraction, and Calibre handoff.
- `backend/internal/compat`: Readarr-compatible resource persistence and
  restrictions.
- `backend/migrations`: append-only Postgres migrations.
- `web/src`: Vite React UI.
- `deploy`: Docker Compose, Dockerfiles, nginx config, and TrueNAS template.
- `docs`: human-facing architecture, setup, metadata, provider, and status docs.

## Development Commands

Use these before committing relevant changes:

```bash
go test ./...
cd web && npm run build
git diff --check
```

Useful local run commands:

```bash
LIBRARRY_DATABASE_URL=postgres://librarry:librarry@127.0.0.1:5432/librarry?sslmode=disable \
  go run ./backend/cmd/librarry

cd web
npm run dev
```

Docker Compose:

```bash
cd deploy
cp .env.example .env
docker compose up --build
```

## Implementation Rules

- Keep database migrations append-only. Do not rewrite existing migrations after
  they have been used.
- Normalize empty API list payloads to `[]` at API boundaries or in the web API
  client. The UI must not crash when a persisted table has zero rows.
- Preserve provider provenance when merging metadata records. Avoid losing
  provider IDs or raw record links when choosing a canonical result.
- Keep release evaluation explainable. Rejections should carry reasons that the
  UI and history can show.
- Scheduled automation should default to review/search-first behavior. Auto-grab
  must remain explicit.
- Prefer focused tests for matching, provider normalization, API handlers,
  settings validation, release scoring, import matching, and download-client
  actions.

## Real Integration Testing

Use safe, controlled data. Do not grab arbitrary indexer results while testing.

Known safe qBittorrent smoke pattern:

- Add a paused legal magnet through `POST /api/v1/grabs`.
- Tag it with `librarry` and `librarry-smoke`.
- Verify it appears through `GET /api/v1/downloads?tag=librarry-smoke`.
- Delete it through `POST /api/v1/downloads/actions` with `deleteFiles: true`.
- Verify the final list count is zero.

For Prowlarr, release search can be verified with a book query through
`POST /api/v1/releases/search`, but do not grab a result unless the test plan
explicitly calls for it.

## Documentation Expectations

When adding or changing product behavior, update the relevant docs:

- README for public-facing feature and positioning changes.
- `docs/status.md` for verified status, known gaps, and deployment reality.
- `docs/local-dev.md` for configuration, environment, and operator flows.
- `docs/architecture.md` for API surfaces or system design changes.
- `docs/metadata-strategy.md` for metadata policy changes.
- `docs/provider-setup.md` for provider credential or behavior changes.

