# UI Surface Backlog & Phantom-UI Ledger

Last updated: 2026-07-01 (frontend migration pass).

This document tracks every UI surface that needs more work under the hood.
Rule from the migration pass: **no phantom UI element goes undocumented** — if
something renders but is demo-only, backed by a stub, or weaker than it looks,
it must be listed here.

Status legend: **real** = full backend behavior · **partial** = works with
caveats · **demo-only** = renders seeded data, no persistence · **stub** =
endpoint exists but returns placeholders.

## 1. API alignment summary (audited 2026-07-01)

All 60 exported functions in `web/src/lib/api.ts` map to registered backend
routes; the audit found **zero UI calls without a backend route**. The
under-the-hood risk is concentrated in demo seeds, partial schemas, and
compat-layer stubs the UI does not call.

## 2. Demo-mode surfaces (fake data by design)

With `VITE_LIBRARRY_DEMO=true`, these surfaces render seed data from
`web/src/lib/seed.ts` when the backend is unreachable. They are honest in live
deployments; the ledger exists so nobody mistakes a demo screenshot for
verified behavior:

| Surface | Seed | Notes |
| --- | --- | --- |
| Add New results | `seedResults` (2 books) | Query boxes prefill "Project Hail Mary" / "Andy Weir" |
| Wanted list + provenance | `seedWantedItems`, `seedWantedMetadataByID` | Provenance seeds exist only for seeded item IDs |
| Activity queue (+details, categories/tags, client prefs) | `seedDownloads`, `seedDownloadDetailsByKey`, `seedDownloadResourcesByClient`, `seedDownloadPreferencesByClient` | Queue actions in demo mode appear to succeed but persist nothing |
| Provider / integration health | `seedProviders`, `seedIntegrations` | Seed marks all four download integrations "ready" — a demo-mode lie by design |
| Readarr compatibility report | `seedReadarrCompatibility` | |
| Metadata review queue | `seedWantedMetadataReview` | |

No seeds exist for: author subscriptions, author metadata reviews, import
reviews, library files, history, acquisition queue → these show real empty
states in demo mode.

## 3. Backend gaps behind real UI (needs work under the hood)

| Surface | Status | Gap |
| --- | --- | --- |
| Rich metadata (Hardcover) | partial | UI shows provider card; live deploys still need `LIBRARRY_HARDCOVER_TOKEN` (docs/status.md) |
| Google Books fallback | partial | Needs `LIBRARRY_GOOGLE_BOOKS_API_KEY` in live deploys |
| Readarr migration (Settings → Import) | partial | Full API+UI coverage, but never dry-run against a real Readarr instance |
| Library scan/import on real roots | partial | Real `media-stack` roots still need a controlled full scan + review |
| Completed download → import → missing-state clears | partial | Proven once E2E; needs repeated proof on real data |
| Author monitoring auto-grab | partial | Policies exist; needs longer observation before auto-grab is trusted |
| Calibre handoff | partial | Add/convert/status/delete work; edition sync, embedded writes, rename-refresh, rollback are future work |
| Settings → Connections validation | partial | Saving credentials does not fully validate against live clients in all cases; System checklist is the source of truth |

## 4. Readarr-compat endpoints that are stubs (not used by this UI)

The compat layer exposes ~90 routes the modern UI never calls; these exist for
external Readarr-compatible clients. Known stubs/partials (evidence:
`backend/internal/api/compat.go`):

- `GET /api/v1/system/routes/duplicate`, `GET /api/v1/system/backup`,
  `GET /api/v1/update` → return `[]`.
- Generic `compatDeleteResource` DELETE handlers (tag, customformat,
  notification, importlist, indexer, downloadclient, delayprofile,
  languageprofile, metadataprofile, restriction, remotepathmapping,
  importlistexclusion, metadata) → no-ops that return success.
- `/api/v1/metadata/schema`, `/api/v1/notification/schema`,
  `/api/v1/importlist/schema` → single-option schemas (Calibre / Webhook /
  ReadarrImportList only).

Risk: an external tool pointed at Librarry's compat API can "delete" resources
that silently survive. Consider returning `501 Not Implemented` instead.

## 5. Findings from the 2026-07-01 feature ports

(Additions from the per-feature migration reports.)

**System (/providers):**
- `system/status` and `readiness` have no demo seeds, so the Status card and
  Setup checklist render an informational "needs a live API" notice in demo
  mode. Seed them if demo coverage matters.
- `IntegrationHealth` lacks a `checkedAt` field (providers have one) — the UI
  cannot show freshness for download/indexer integrations. Backend addition
  would close the gap.
- Legacy UI hard-capped Readarr-compat example endpoints at 3 per category and
  used an `integrations.length || 2` denominator hack; both removed.

**Dashboard:**
- In demo mode only metadata-review and downloads queues have seeds — the
  author-candidate, import-review, and blocked lanes of the legacy dashboard
  were permanently empty phantoms in demos. The new dashboard hides zero-count
  rows, making demo screenshots honest by construction.
- The acquisition-queue demo fallback is an all-zero summary (no seed).
- Legacy monitor-run result details were session-local state and never
  persisted; they are now condensed to outcome toasts + history events.

**Library Import (/imports):**
- No demo seeds for import reviews or library files → the whole page renders
  empty states in demo mode.
- "Scanned/Skipped" stats are session-local (populated only after a scan in
  the current session) — they look like persistent metrics but are not.
- Manual import no longer silently attaches the wanted-panel selection as
  `wantedId` (legacy cross-view coupling); server-side matching still applies.
- Review-status filter now exposes pending/resolved/all — the backend
  supported `all` but the legacy UI only ever fetched pending.

**Library (/library):**
- Demo seeds cover only wanted items: the Authors pane in demo mode is derived
  entirely from wanted items (all badged "manual"), the Files stat reads 0,
  and "present" status is unreachable.
- `runAuthorMonitor` in `lib/api.ts` throws a status-only error message (does
  not parse the response body), so its failure toasts cannot surface the
  database-persistence hint other endpoints provide. Small API-client fix.
- Monitor/search runs no longer merge results into local state; they
  invalidate shared queries (wanted, author subscriptions, acquisition queue).

**Add New (/search):**
- Demo mode now auto-runs the prefilled query through the seed fallback
  instead of injecting seed results at state-init — same screenshots, honest
  data flow.
- Subscribing to an author no longer auto-triggers an author-monitor run
  (legacy side effect); the run action lives on Wanted/Library toolbars.
- The tracked badge shows `status · format` — showing Missing/Grabbed/Present
  presence would need a library-files cross-join on this page.

**Wanted (/wanted):**
- Metadata-provenance demo seeds exist only for the seeded wanted item
  (`demo-wanted-project-hail-mary`); other items show empty provenance in demo
  mode.
- No release-decision seeds exist — the releases panel is always empty in demo
  mode and grab/search mutations error-toast without a backend.
- The Authors tab is fully seedless → EmptyStates in demo mode.
- `library settings` has no demo fallback, so the search language silently
  defaults to "English" in demo mode.

**Settings (/settings):**
- Without Postgres the backend reports `persisted: false` — library and
  integration saves apply to the running process only and vanish on restart.
  Surfacing hardened 2026-07-01: Media Management and Connections render a
  standing warning notice when persistence is absent, and runtime-only save
  toasts are warn-toned. (With Postgres configured, saves are durable via the
  compat resource store — real deployments are unaffected.)
- `integration settings` / `library settings` have no demo fallbacks: demo
  mode shows editable defaults whose saves fail with an error toast.
- Profiles tab now edits preferred score, required terms, and
  upgrade-allowed — legacy compared these fields for save-gating but provided
  no inputs for them (phantom-adjacent: silently uneditable data).
- Readarr import outcome now surfaces top-level `outcome.errors` — legacy
  silently dropped them.

**Activity (/downloads):**
- ~~`DownloadStatus` has no book/author fields~~ Closed 2026-07-01: the
  downloads API now annotates rows carrying a `wanted:<id>` tag with
  `wantedId`/`wantedTitle`/`wantedAuthor` (resolved at the API layer, never
  persisted), and the queue shows a book·author link to the wanted item.
  Downloads added outside a wanted grab (e.g. manual adds) have no tag and
  stay unannotated.
- The queue and the dashboard's failed-download triage now default to the
  Librarry-tagged scope; the full client view remains one click away
  ("All clients"). Matches the product boundary that Librarry is not a
  general torrent-client UI.
- Demo mode keeps seeded fallbacks for queue, details, categories/tags, and
  client preferences; all queue mutations in demo appear to succeed but
  persist nothing (legacy "Showing demo…" copy preserved as warn toasts).
- Removal is now always confirmed via modal offering "Remove" vs "Delete with
  data" (legacy deleted immediately on click — destructive without confirm).
- Pending-placeholder rows (stored grabs with no live client ID yet) still
  disable start/stop/details/qBittorrent-manager actions until the client
  reports a real hash.
