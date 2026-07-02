# Current Status

Last updated: 2026-07-01.

Librarry is an early alpha Readarr replacement. It is useful for validating the
metadata-first workflow and exercising acquisition integrations, but it is not
yet ready to replace a production Readarr instance unattended.

## Verified Working

- The Go API, React web UI, Postgres migrations, and Docker/TrueNAS custom-app
  shape build and run together.
- Public distribution files are present for generic Docker Compose, TrueNAS
  Custom Apps, and Unraid Docker Compose Manager, including example env files,
  path/permission guidance, backup notes, and upgrade instructions.
- The live TrueNAS deployment was verified at `http://192.168.1.221:30200/`.
- The Cosmos route `https://librarry.borchetta.xyz/` was verified after the
  route was added, and the app rendered through the proxy.
- The Cosmos route was re-tested on 2026-06-29 after proper React routing was
  added. Direct loads for `/dashboard`, `/library`, `/search`, `/wanted`,
  `/downloads`, `/imports`, `/providers`, and `/settings` all returned the live
  `index-BYM1I52w.js` bundle.
- Open Library metadata search works and returns normalized book results with
  provider provenance.
- Metadata search, manual release search, and wanted-item release search now
  default to English through the library settings preference.
- Local OPF/embedded metadata support is wired for import evidence.
- Metadata result merging deduplicates equivalent provider candidates by ISBN
  or compatible title/author evidence while retaining provider aliases.
- Manual override persistence exists for wanted-item bibliographic corrections.
- Prowlarr integration is configured in the live deployment and returned real
  ebook release results through Librarry.
- Prowlarr release grabs now resolve the Prowlarr download URL inside Librarry
  before handing off to qBittorrent or Transmission. Torrent files are uploaded
  to the client, magnet redirects are handed off as magnets, and qBittorrent
  infohash IDs are normalized to lowercase so live queue rows reconcile with
  stored rows. This was verified against the live TrueNAS deployment on
  2026-07-01 with a public-domain Moby Dick EPUB candidate:
  `740b73b9bd31325f178b216cc43b4e735a6dca47` appeared in qBittorrent and was
  then removed through Librarry cleanup.
- A deployed browser E2E through `https://librarry.borchetta.xyz/` successfully
  searched ISBN `9781250313195`, created a temporary wanted item for Gideon the
  Ninth, searched Prowlarr releases, persisted four approved release decisions,
  rendered the wanted queue without browser errors or 4xx/5xx API calls, and
  removed the temporary wanted item afterward.
- qBittorrent integration is configured in the live deployment. A paused legal
  Ubuntu torrent smoke test was added, listed through Librarry with
  `librarry-smoke`, deleted with files, and verified absent afterward.
- A deployed browser E2E on 2026-07-01 searched for Moby-Dick, selected a
  normalized Open Library result, searched Prowlarr releases, and successfully
  queued a qBittorrent torrent through the Librarry UI. A public-domain
  Internet Archive Pride and Prejudice torrent was then added through the
  deployed manual-add UI, refreshed to the real qBittorrent hash, started from
  the Librarry queue, and observed through the API as `downloading` at roughly
  98% before qBittorrent reported `stalledDL` from peer availability.
- A deterministic CC0 EPUB fixture in `docs/fixtures/e2e/` was added through the
  LAN-deployed manual-add UI, started from the Librarry queue, completed in
  qBittorrent as hash `25fa3db023fd85445bfc49adca0387991839d562`, and rendered
  in Librarry with `progress: 1` and `importStatus: ready`. The completed file
  was verified on the `media-stack` filesystem at
  `/mnt/HDD_pool/vault/media-stack/torrents/books/librarry-public-domain-e2e-book.epub`
  with SHA-256
  `3ae24570da25451931c1b2cb62c4cd205eb387801a8dd5099045c2f15ec07eb2`.
- Queue actions no longer merge stored placeholder download rows back into the
  UI after an ID-specific live client lookup returns no matching hash. The UI
  also disables Start/Stop/details and qBittorrent manager actions for
  persisted `pending` placeholder rows until the external client exposes a real
  download ID.
- The public Cosmos hostname was re-tested after the deployment:
  `/api/v1/downloads?tag=librarry` returned live qBittorrent rows from both curl
  and Chromium, and one valid browser POST to `/api/v1/grabs` reached the API
  and returned 200. A later Chromium POST to `/api/v1/grabs` intermittently
  reproduced `net::ERR_ECH_FALLBACK_CERTIFICATE_INVALID`; the LAN URL remains
  the reliable deployed E2E path until the Cosmos/Cloudflare route is fixed.
- Removed qBittorrent downloads are hidden from active download listings.
- The UI tolerates nullable list payloads from the API, including empty
  `files`, `authors`, `downloads`, `releases`, `profiles`, `reviews`, and
  `events` arrays.
- On 2026-07-01 the web UI was migrated from a single-file React app
  (7,445-line `App.tsx`) to a modular feature architecture with TanStack Query
  and lazy-loaded pages, and redesigned toward classic arr conventions (dark
  sidebar always reachable: full rail / icon rail / mobile drawer; page
  toolbars; queue-first Activity page; confirm-before-delete). Route paths are
  unchanged (`/dashboard`, `/library`, `/search`, `/wanted`, `/downloads`,
  `/imports`, `/providers`, `/settings`); nav labels changed (Search→Add New,
  Queue→Activity, Providers→System) and the default route is now `/library`.
  Verified in demo mode at 375/768/1280 px with zero console errors; `tsc -b`
  and `vite build` pass. See docs/frontend.md and docs/ui-backlog.md. The
  redesigned UI has not yet been re-verified against the live TrueNAS
  deployment.
- On 2026-07-01 the redesigned UI was verified read-only against the live
  TrueNAS API (dev server proxied to `http://192.168.1.221:30200`): library,
  wanted, activity queue, system checklist, and settings all rendered live
  data with zero console errors, and saved secrets stayed masked. Follow-up
  changes from that pass: the downloads API now annotates `wanted:<id>`-tagged
  rows with the wanted item's id/title/author and the Activity queue links
  them; the queue and dashboard failed-download triage default to the
  Librarry-tagged scope; and Settings shows a standing warning when saves are
  runtime-only because no database is configured. The redesigned UI was then
  deployed to the live TrueNAS app from local images
  `librarry-api:local` (`6d4f29274da4`) and `librarry-web:local`
  (`8e7ca39b82c6`) and verified through both
  `http://192.168.1.221:30200/library` and
  `https://librarry.borchetta.xyz/library`.
- A real-user E2E was run through the deployed redesigned UI on 2026-07-01
  (bundle `index-Cuji0PMa.js`, API build 2026-07-01T22:30:20Z): searched
  "Moby Dick" on Add New (9 normalized Open Library results), added it through
  the review-confirmation modal (medium-confidence gate fired with reasons),
  deep-linked to the wanted item via `/wanted?item=<id>`, ran a Prowlarr
  release search (18 found · 6 approved · 12 rejected, rejection reasons
  rendered), grabbed the top approved public-domain EPUB (score 88.5) paused,
  started it from the Activity queue, and watched it complete: progress 1,
  `stalledUP`, `importStatus: ready` at `/data/torrents/books`. The wanted
  item flipped to `grabbed` with release score 88.5, and the queue row showed
  the new "Moby Dick · Herman Melville" wanted-item link from the downloads
  annotation. Zero browser console errors. The test book and its wanted item
  were intentionally left in place as proof.
- Completed download handling now runs automatically (arr parity): a
  background worker imports completed librarry-tagged downloads every minute
  by default (`LIBRARRY_COMPLETED_IMPORT_*`, mode `hardlinkOrCopy`), matching
  wanted-linked downloads directly and routing unmatched files to the import
  review queue with store-level dedupe by source path. Covered by unit tests;
  needs live verification after the next API image deploy — the Moby Dick
  E2E download sitting at `importStatus: ready` should import on its own
  within ~a minute of startup.

- v0.2.0 (2026-07-01) implemented the full Readarr parity plan
  (docs/parity-plan.md, all six milestones; delay profiles skipped by owner
  decision): blocklist with evaluation rejection and auto-blocklisting,
  remove-after-seeding, cutoff-unmet view, search-on-add, seven monitor
  modes, author/book detail pages, poster/overview library views, mass
  editor, multiple root folders, remote path mappings, recycle bin,
  series/year naming tokens, rename UI, task scheduler with run-now, native
  notifications (webhook/ntfy/Discord/Telegram) dispatched from workers,
  continuous health checks, disk space, calendar + iCal feed, Hardcover
  import lists with exclusions, author add-filters, native tags,
  none/basic/forms authentication, and scheduled pg_dump backups. Auto-grab
  defaults flipped to arr parity. Verified locally against a disposable
  Postgres 16 (all 26 migrations, endpoint smoke across the new surface,
  forms-auth login through the real UI, task run-now with tracked outcomes);
  `go test ./...`, `go vet`, and the web build pass. The homelab was deployed
  to v0.2.0 from local images `librarry-api:local` (`22e241134431`) and
  `librarry-web:local` (`18ba4ec169e7`) on 2026-07-01. LAN health, the
  `/calendar` route, qBittorrent download listing, database migrations,
  completed-download import, and a manual pg_dump backup were verified live.
  Not yet verified live: Hardcover list sync against the real GraphQL schema
  (no token locally), ICS consumption by a calendar app, and backup restore.
- The first v0.2.0 live deploy exposed missing deployment directories. The
  `media-stack` ebook/audiobook roots were created for the API user, allowing
  Moby Dick to import to
  `/mnt/HDD_pool/vault/media-stack/media/books/ebooks/Herman Melville/Moby Dick/Moby Dick.epub`
  with SHA-256
  `db359a71d3f57af793cf1906a56da1309fce32e908a846229b1146cd2c32fde5`.
  The live custom app also now mounts
  `/mnt/HDD_pool/vault/app-config/librarry/config:/config`; a manual backup
  created `librarry-20260702-013251.dump`.
- `https://librarry.borchetta.xyz/healthz` and direct HTML requests return the
  v0.2.0 deployment through Cosmos, but Chromium/Playwright still reproduced
  `ERR_ECH_FALLBACK_CERTIFICATE_INVALID` on
  `https://librarry.borchetta.xyz/calendar`. The LAN URL remains the reliable
  browser verification path until the Cosmos/Cloudflare certificate path is
  fixed.

- The deployed v0.2.0 instance passed a live E2E of the parity surface on
  2026-07-01/02: completed-download handling ran autonomously on deploy —
  Moby Dick auto-imported to
  `/data/media/books/ebooks/Herman Melville/Moby Dick/Moby Dick.epub`
  (wanted status `imported`) and its seeded torrent was auto-removed from
  qBittorrent; the unmatched CC0 fixture correctly queued an import review;
  failed-download recovery auto-grabbed a paused replacement for the old
  failed download (auto-grab defaults working live); search-on-add ran a
  release search during a real add (11 found · 3 approved · 8 rejected);
  queue-remove with "Blocklist release" produced a blocklist entry (infohash,
  reason, source) in the Blocklist tab; all 8 health checks passed; the task
  registry showed real last/next runs; disk space read the NAS filesystem;
  and Backup Now produced a real pg_dump in the deployed container alongside
  the scheduled dumps. Test artifacts were cleaned up (temporary wanted item
  deleted before the monitor tick, blocklist cleared, fixture removed with
  data). Wide-breakpoint layout fixes shipped as v0.2.1.
- Known deployment bug found during the E2E, fixed in-repo: nginx did not
  proxy `/feed/`, so the deployed iCal URL returned the SPA shell; the
  `/feed/` proxy block is now in deploy/nginx.conf and ships with the next
  web image. The live calendar is empty because existing wanted rows have
  year-only publish dates (full-date items will populate it).

## Available But Still Needs Real-World Proving

- Readarr migration preview/import has broad API and UI coverage, but needs a
  full dry run against a real Readarr instance and a row-by-row comparison.
- Library scanning and import can process ebook/audiobook roots, but the real
  `media-stack` book roots still need a controlled full scan and import review.
- The end-to-end book loop has proof through metadata search, release search,
  manual torrent add, qBittorrent grab/start, completed download, and filesystem
  verification. It still needs more proof on real data for completed download ->
  import -> tracked file -> missing state clears.
- Author monitoring exists, including all/future/none policies, but needs longer
  observation with real monitored authors before it should auto-grab.
- Calibre Content Server handoff exists for add-book, basic metadata fields,
  conversion starts, status refresh, and deletes, but richer edition sync,
  embedded metadata writes, path refresh after Calibre renames, and rollback are
  still future work.
- Readarr-compatible APIs cover many common routes, but this is compatibility
  surface area, not a promise of complete OpenAPI parity or identical side
  effects.

## Known Gaps

- Hardcover is the intended primary rich metadata provider, but the live
  deployment still needs `LIBRARRY_HARDCOVER_TOKEN`.
- Google Books exact-match fallback is implemented, but the live deployment
  still needs `LIBRARRY_GOOGLE_BOOKS_API_KEY`.
- Goodreads, Amazon, and Audible scraping are intentionally not in core.
- The GHCR image workflow is in-repo, but the first tagged release still needs
  to be cut and verified after the workflow runs from `main`.
- The default local, TrueNAS, and Unraid examples are operator-facing and should
  be put behind `LIBRARRY_API_KEY`, Cosmos auth, Cloudflare Access, or another
  trusted boundary before WAN exposure.
- Public Docker/NAS packaging is ready for trial installs, but the first
  versioned GHCR release still needs to be cut and smoke tested after the image
  workflow publishes from `main`.
- Librarry intentionally does not replace qBittorrent, Transmission, or SABnzbd
  as full download-client administration UIs.

## Current Live Deployment Notes

The local homelab deployment, when present, uses:

- TrueNAS custom app name: `librarry`
- Web portal: `http://192.168.1.221:30200/`
- Cosmos hostname: `https://librarry.borchetta.xyz/`
- API and web images: currently deployed from local images; public templates use
  `ghcr.io/bandoracer/librarry-api:latest` and
  `ghcr.io/bandoracer/librarry-web:latest`
- Postgres data: `/mnt/HDD_pool/vault/app-config/librarry/postgres`
- App config/backups: `/mnt/HDD_pool/vault/app-config/librarry/config:/config`
- Media mount: `/mnt/HDD_pool/vault/media-stack:/data`
- Ebook root: `/data/media/books/ebooks`
- Audiobook root: `/data/media/books/audiobooks`
- Book torrent root: `/data/torrents/books`
- Standard search language: `English`

Do not commit real API keys, database passwords, Prowlarr keys, download-client
passwords, or provider tokens.

## Readiness Assessment

Librarry is roughly alpha quality:

- Good enough to test metadata search, wanted items, release decisions,
  Prowlarr search, qBittorrent handoff, and UI review workflows.
- Not yet good enough to retire Readarr without a controlled migration/import
  test and several successful full acquisition/import loops.
