# Readarr Parity Plan

> **Status: executed in v0.2.0 (2026-07-01)** — all six milestones landed in
> one release (delay profiles skipped by owner decision; CDH-remove and
> auto-grab defaults set to arr parity). Verified: full Go test suite, web
> build, and a local integration environment (disposable Postgres 16 + live
> API + UI) exercising migrations 0001–0026, the scheduler registry, tag CRUD,
> the forms-auth login flow end to end, health checks, and task run-now.
> Remaining follow-ups live in docs/status.md (live-deployment verification of
> Hardcover list sync, ICS consumption by a real calendar app, and backup
> restore drill).

Last updated: 2026-07-01. Executes the gaps in
[readarr-parity.md](readarr-parity.md). Six milestones, each independently
deployable and verified against the live TrueNAS deployment using the
established E2E pattern (search → add → grab → download → import).

Conventions that bind every item:

- Migrations are append-only Postgres (`backend/migrations/00NN_*.sql`).
- Native API under `/api/v1/`; compat routes get wired to real stores where
  they exist as stubs today.
- Workers follow the `runX(ctx, wg, logger, service, cfg)` pattern in
  `backend/cmd/librarry/main.go` with `LIBRARRY_*` env config.
- Frontend features live in `web/src/features/<name>/`, compose
  `components/ui.tsx` primitives, fetch through `lib/queries.ts` hooks, and
  toast every mutation. Route paths are stable API.
- Release evaluation stays explainable: every new rejection path carries a
  visible reason.
- Sizes: S ≈ hours, M ≈ a day-ish, L ≈ multiple days of agent time.

---

## M1 — Automation correctness

Goal: the automated loop never repeats a known failure and cleans up after
itself. Highest priority because every other automation builds on it.

### 1.1 Blocklist (M)

- **Schema** (`0019_blocklist.sql`): `blocklist` table — id uuid, wanted_item_id
  uuid null fk, title, indexer, protocol, download_url_hash, infohash null,
  reason, source (`auto-failed` | `queue-remove` | `history-mark-failed`),
  created_at. Indexes on infohash and download_url_hash.
- **Backend** (`backend/internal/wanted/`): blocklist store CRUD;
  `evaluate.go` rejects candidates whose infohash/url-hash (fallback:
  title+indexer) match, reason `blocklisted`; failed-download recovery worker
  blocklists before searching replacements; new handler for queue-remove with
  blocklist option; history "mark failed" endpoint (marks download failed +
  blocklists + optional re-search).
- **API**: native `GET/DELETE /api/v1/librarry/blocklist[...]`; rewire compat
  `/api/v1/blocklist` stubs to the real store.
- **Frontend** (`features/activity/`): third tab **Blocklist** (title,
  indexer, protocol, reason, source, date, remove/bulk-remove); queue Remove
  modal gains "Blocklist release" checkbox; History rows get "Mark as failed".
- **Tests**: evaluation rejection, store CRUD, recovery-worker blocklisting.
- **Live proof**: grab a public-domain release, mark failed, re-search, watch
  it appear rejected with reason `blocklisted`.

### 1.2 Remove completed downloads after seeding (S/M)

- Extend the completed-import worker: after `importStatus: imported`, if the
  client reports seeding goals met (qBittorrent `stoppedUP`/`pausedUP`,
  Transmission seed-idle equivalents), issue the existing delete-with-data
  action. Safe because import used hardlink-or-copy.
- Config `LIBRARRY_COMPLETED_REMOVE_ENABLED` — **decision needed**: arr
  defaults this ON; proposal: ship default OFF for one release, flip after
  live observation.
- Tests: eligibility predicate per client/state matrix.

---

## M2 — Cheap familiarity wins

Goal: close the small deltas a Readarr user trips on in the first hour.

### 2.1 Cutoff Unmet view (S/M)

- Backend: `GET /api/v1/wanted?view=cutoff-unmet` — wanted items with a
  tracked file whose `currentReleaseScore < profile.cutoffScore` and
  `upgradeAllowed`.
- Frontend: new segmented filter (or tab) on Wanted Books; bulk
  "Upgrade Search Selected" using the existing upgrade-search API.

### 2.2 Search on add (S)

- Add New: "Start search for missing book" checkbox in the add flow
  (persist last choice in localStorage). On add → `searchWantedReleases`,
  toast the outcome. Review-first rule intact: search ≠ grab.

### 2.3 Full monitor modes (M)

- Extend `AuthorMissingBookPolicy` with `missing`, `existing`, `first`,
  `latest` (migration to relax the check constraint from 0017 if present).
- Monitor logic in `wanted/service.go` maps modes to book selection; UI
  selects gain the new options (Add New author flow + Wanted Authors tab).

---

## M3 — Library experience (the arr look)

Goal: the Library stops being a list and becomes the arr-style browsing
surface. Pure frontend except one bulk endpoint.

### 3.1 Author and book detail pages (M/L)

- Routes `/library/author/:id` and `/library/book/:wantedId` (new paths, no
  existing-path changes). Author page: header (policy, profile, stats),
  books table with per-book monitor toggle, search, presence. Book page:
  routable version of the Wanted detail panel — extract the provenance/
  releases/edit components from `features/wanted/` into shared components
  first.

### 3.2 Poster / Overview view modes (M)

- Library books pane view toggle: Table | Posters | Overview (cover grid via
  `coverUrl` with fallback, overview rows with description). Persist choice.

### 3.3 Mass editor (M)

- Backend: `POST /api/v1/wanted/bulk` (ids + patch: monitored, quality
  profile, format, delete) — one transaction, per-item results.
- Frontend: Library "Edit Mode" toggle → checkboxes + bulk bar, mirroring the
  import-review bulk pattern.

### 3.4 Sort & filters (S)

- Client-side sort (title/author/added/status) + monitored/presence filters
  on the Library and Wanted lists.

---

## M4 — Roots & file management

Goal: real multi-root libraries and complete file lifecycle.

### 4.1 Multiple root folders (L)

- The compat layer already persists root folders
  (`compat_root_folders`, consumed by `library.ConfigWithRootFolders`);
  promote to a native model: name, path, format affinity, default quality
  profile, default monitor policy, default tags.
- Library scan/import pick destination by root; Add flows gain a root
  selector; Settings → Media Management gets root CRUD cards (replacing the
  two fixed root fields, which become the migration defaults).
- Largest structural item in the plan — schedule first in its milestone,
  agents fan out UI/backend/tests.

### 4.2 Rename UI over existing endpoints (S)

- Backend already ships `POST /api/v1/library/files/rename[/preview]` with no
  callers. Add "Preview Rename" to Library (and post-naming-change prompt in
  Settings → Media): old→new table with per-row checkboxes, execute, toast.

### 4.3 Remote path mappings (M)

- Table (host, remote prefix, local prefix) + dumb prefix rewrite where
  download-client paths are consumed (`locateDownloadSource`, details panel);
  Settings → Connections card. Unblocks split-host/docker setups.

### 4.4 Recycle bin (M)

- `LIBRARRY_RECYCLE_BIN` path + retention; library delete/replace moves into
  the bin; cleanup pass added to an existing worker tick; Settings field +
  ledger note.

### 4.5 Naming token expansion (M)

- Add `{Series}`, `{SeriesPosition}`, `{Year}`, `{SubtitleNoSub}`-style
  tokens to backend naming + frontend preview (series/year already exist in
  metadata records; persist onto wanted items where missing).

---

## M5 — Operability: tasks, notifications, health

Goal: the automation becomes visible and reports outward.

### 5.1 Tasks view (M)

- Introduce a small scheduler registry wrapping each worker (name, interval,
  last run, last outcome summary, next run) + `POST /api/v1/system/tasks/{id}/run`.
- System page Tasks card with run-now buttons. Closes the "invisible workers"
  gap and gives support a first question to ask.

### 5.2 Notifications, native (M/L)

- Move dispatch out of the API handler layer into a service that workers can
  call (fixes: background grabs/imports currently never notify).
- Settings → Connect tab: target CRUD with triggers checklist + test button.
  Providers: webhook (exists), then ntfy, Discord, Telegram (schema-driven so
  more follow cheaply). Wire compat `/api/v1/notification` to it.

### 5.3 Continuous health checks (M)

- Expand readiness into recurring health: client unreachable N times, indexer
  errors, root missing/read-only, completed-import disabled, low disk.
  Surface on System + a dashboard badge; feed the notification triggers.

### 5.4 Disk space card (S)

- statfs per root/torrent path; System card + low-space health check.

---

## M6 — Long tail

### 6.1 Calendar + iCal (L)

- Needs release-date confidence on wanted/monitored books (publish dates
  already arrive from providers; persist them onto wanted items).
- Month/agenda UI + `GET /feed/v1/calendar.ics` (tag filter, past/future
  windows, auth token in URL). Wire compat `/api/v1/calendar` to real data.

### 6.2 Import lists, Hardcover-native (L)

- Sync worker for Hardcover lists/shelves (token already a first-class
  provider credential), exclusions table, auto-add options (monitor mode,
  quality profile, root, tags, search-on-add). UI under Settings → Import
  Lists. Goodreads stays out of core per product rule.

### 6.3 Author add filters (metadata-profile analog) (M)

- Per-subscription filters applied before the review queue: language
  allowlist, must-not-contain, skip-no-ISBN, min page count where the
  provider supplies it. Cuts review noise without abandoning review-first.

### 6.4 Tags system (M/L)

- Native tags table + links (wanted items, author subscriptions, notification
  targets, release restrictions, future import lists/root defaults);
  Settings → Tags management; chips across Library/Wanted.

### 6.5 Delay profiles (M) — **decision needed**

- Real value is protocol preference (usenet-over-torrent). If wanted:
  evaluation delay gate + queue "pending" state. Books get few simultaneous
  releases, so propose deferring unless usenet becomes primary.

### 6.6 Auth & backups (L) — **decision needed**

- Options: (a) in-app forms auth + sessions, arr-style; (b) declare the
  reverse-proxy (Cosmos/CF Access) the supported auth boundary and ship only
  API-key management UI + docs. Backups: scheduled `pg_dump` to a configured
  path + retention + System card (restore stays documented-manual).

### 6.7 Small items (S each)

- Import extra files (`.cue`) alongside imports (audiobook chapters).
- Per-format size definitions in profiles (or a global format table) if the
  score model proves insufficient.
- Filesystem browser endpoint + picker modal for path inputs.

---

## Standing decisions for Ryan

1. **CDH-Remove default** (1.2): ship OFF then flip, or arr-parity ON now?
2. **Auto-grab defaults**: feed/monitor auto-grab remain OFF (product rule)
   or flip to arr behavior now that blocklist (M1) will backstop failures?
3. **Auth approach** (6.6): in-app forms vs proxy-boundary-official.
4. **Delay profiles** (6.5): build or skip.

## Verification cadence

Every milestone: `go test ./...`, `cd web && npm run build`,
`git diff --check`, deploy images, then a live E2E through the deployed UI
covering the new surface (public-domain fixtures only), recorded in
`docs/status.md`. Parity doc rows get flipped as items land.
