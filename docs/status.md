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
- Queue actions no longer merge stored placeholder download rows back into the
  UI after an ID-specific live client lookup returns no matching hash. The UI
  also disables Start/Stop/details and qBittorrent manager actions for
  persisted `pending` placeholder rows until the external client exposes a real
  download ID.
- The public Cosmos hostname was re-tested from Chromium after the deployment:
  `/api/v1/downloads?tag=librarry` returned the live qBittorrent row, and a
  valid browser POST to `/api/v1/grabs` reached the API and returned 200.
- Removed qBittorrent downloads are hidden from active download listings.
- The UI tolerates nullable list payloads from the API, including empty
  `files`, `authors`, `downloads`, `releases`, `profiles`, `reviews`, and
  `events` arrays.

## Available But Still Needs Real-World Proving

- Readarr migration preview/import has broad API and UI coverage, but needs a
  full dry run against a real Readarr instance and a row-by-row comparison.
- Library scanning and import can process ebook/audiobook roots, but the real
  `media-stack` book roots still need a controlled full scan and import review.
- The end-to-end book loop has proof through metadata search, release search,
  qBittorrent grab, and active download initiation. It still needs more proof on
  real data for completed download -> import -> tracked file -> missing state
  clears.
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
