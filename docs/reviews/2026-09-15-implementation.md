# Stabilization implementation ledger

Execution started September 15, 2026 on `codex/stabilize-librarry`. This is a
progress record, not a claim that the full stabilization plan is complete.

## Implemented in the branch

- Community PR #2 is cherry-picked with Zak Strassberg's authorship preserved.
  Prowlarr JSON searches send repeated categories.
- Acquisition configuration swaps use immutable snapshots. Store mutations scope
  client identity; ambiguous legacy external IDs fail instead of updating several
  clients. Remote acceptance/deletion followed by failed persistence reports an
  explicit error.
- Completed imports inspect only the exact named payload. Missing names, path
  traversal, symlinks, format conflicts, and more than one supported book file
  are rejected. Completed move mode is rejected to preserve seeding sources.
- Scan observations preserve authoritative names, source paths, JSON associations,
  manual metadata, and Calibre evidence. A real Postgres regression verifies two
  rescans preserve an imported record's identity.
- Copies stage and sync data before atomic, non-overwriting publication. Failed
  replacement transfer keeps the original. Replacement originals remain at a
  recovery path until persistence succeeds; manual moves remove sources only
  after record persistence. Full crash/retry idempotency remains S09 work.
- New completed imports store a client/ID/content-hash receipt. Cleanup rechecks
  exact client inventory, file count/progress/size, source and destination hashes,
  destination separation, and seed-goal evidence. Old imports do not inherit
  eligibility. Calibre-only remote files cannot satisfy local verification yet.
- Forms/basic startup fails without persistence or a usable user; invalid auth
  modes and unreadable persisted auth settings fail. Failed settings persistence
  cannot silently switch the method. Method and credential updates now commit together, with rollback coverage for
  a deliberately failed second write. Changed credentials revoke sessions; stale
  concurrent logins cannot recreate them. Env-owned fields are locked in Settings.
- Every advertised runtime env key reaches every installer. Defaults agree with
  the owner's auto-grab/removal decision; nondefault completed settings are tested
  through actual Compose rendering.
- System status has stable build identity, actual auth mode and migration version.
  Go is 1.26.8 with updated pgx/crypto/text. Frontend/tooling dependencies are
  patched, including a security-motivated React Router 7 update. nginx moves from
  1.27 to the current stable 1.30 line.
- Hardcover HTTP-200 GraphQL errors and malformed data no longer masquerade as
  successful empty searches. Typesense documents are decoded; health reports
  configured without inventing token verification or rate limiting.
- Library has an explicit server view, preserves imported books, hides removed
  entries, and rejects invalid views. Query callback arguments cannot become
  filters. Empty lists normalize to arrays.
- Empty Library leads with adding a book; book totals include unmonitored books.
  Dialogs trap/restore focus and close on Escape. Closed mobile navigation is
  hidden from keyboard/screen-reader navigation; the open drawer traps focus.
  Route errors retain navigation, and auth probe failures offer retry.
- Added disposable Postgres tests, frontend tests, desktop/mobile browser smoke
  tests, and deployment checks. Image builds depend on the verification workflow.

## Qualification evidence so far

- Full Go suite passed with `-race` and disposable Postgres on Go 1.26.8.
- Go vet and deployment render checks passed. Frontend unit tests and production
  build passed; the expanding browser suite is rerun after UI changes.
- Final local browser run: 9 passed, one desktop-only inapplicable mobile test
  skipped, at 1440 and 390 pixels. Covered fresh persisted lists, eight core
  routes, no console/page errors, no page overflow, outage recovery, dialog focus
  containment/restoration, and mobile navigation accessibility.
- `govulncheck` on Go 1.26.8 reported zero reachable vulnerabilities, zero affected
  imported packages. The remaining module-only advisory, GO-2026-5932, concerns
  unmaintained `golang.org/x/crypto/openpgp`, which this project does not import.
  npm audit after the router
  update reported zero vulnerabilities. Initial OS scans found fixable OpenSSL/libuuid findings; package refresh and rescan are in progress.
- An ARM64 API candidate image built and started. It reported Go 1.26.8, schema
  29, the injected commit/build timestamp, and auth `none`. Configured forms with
  no database exited with code 1 before listening.
- The API+web image pair passed exact imports, sibling/multipart rejection, source retention, repeat-import skipping and two scans through nginx. A subsequent isolated dump/restore preserved wanted/download/file counts and the verification receipt (89,577-byte dump).
- An isolated Postgres custom-format dump restored 29 migrations and a file's
  fixture provenance (88,021-byte dump). This is fixture evidence, not a restore
  of the live September backup or a complete media rollback drill.
- Exploratory Chromium review confirmed the empty-state improvement and hidden
  mobile drawer. Before/after screenshots are local `output/playwright/` artifacts.

## Plan accounting

| Work | State | Remaining gate or scope |
|---|---|---|
| S01 | Substantially implemented | Live predeployment file/database inventory and backup-copy restore |
| S02 | Implemented locally | Initial remote verification passed; expanded packaged/scanning CI gate pending readback |
| S03 | Partial | Unified effective configuration/source/precedence view for settings beyond auth |
| S04 | Implemented safety guard | Packaged safety regressions passed; controlled live qualification and multipart review UI remain |
| S05 | Implemented | Read-only live audiobook search against the candidate |
| S06 | Partial | Both packaged architectures, OS image scan, upgrade/rollback, candidate deployment |
| S07 | Partial | Client isolation implemented; relational file/import-operation migrations not yet written |
| S08–S11 | Not complete | Multipart sets, durable recovery/leases, resumable scans and legacy repair |
| S12 | Partial | Error/shape handling fixed; rich provider traversal, caching, credentials and live qualification pending |
| S13 | Not complete | Matching corpus and full author monitoring policy qualification |
| S14 | Partial | Library view/query bug fixed; direct detail routes and verified lifecycle semantics remain |
| S15–S16 | Not complete | Pagination/counts/scale and full compatibility/migration contracts |
| S17–S19 | Not complete | Setup, search/add, activity/import-review redesign and workflow qualification |
| S20 | Partial | Empty state, recovery and keyboard fixes; populated/degraded/tablet/full accessibility review remains |
| S21–S25 | Not complete | Remaining release/security/integration/documentation gates and timed soak |

No production deployment, tag, release, live backup restore, arbitrary grab, or
72-hour soak has been completed by this work. The approved full plan remains the
scope for subsequent implementation; this ledger must not be used to mark it done.

## Primary references checked during implementation

- [qBittorrent 5.0 serializer](https://github.com/qbittorrent/qBittorrent/blob/release-5.0.0/src/webui/api/serialize/serialize_torrent.cpp)
  and [torrent implementation](https://github.com/qbittorrent/qBittorrent/blob/release-5.0.0/src/base/bittorrent/torrentimpl.cpp):
  effective maximum seeding time is in minutes, despite the API wiki's seconds
  label; elapsed seeding time is seconds. The adapter test covers this discrepancy.
- [Hardcover search guide](https://github.com/hardcoverapp/hardcover-docs/blob/main/src/content/docs/api/guides/Searching.mdx):
  Typesense search results and supported document fields.
- [Go downloads](https://go.dev/dl/) and [nginx stable release](https://nginx.org/en/download.html).
- [React Router navigation advisory](https://github.com/advisories/GHSA-wrjc-x8rr-h8h6)
  motivated upgrading the router rather than leaving its version finding unresolved.
