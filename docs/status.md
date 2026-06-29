# Current Status

Last updated: 2026-06-29.

Librarry is an early alpha Readarr replacement. It is useful for validating the
metadata-first workflow and exercising acquisition integrations, but it is not
yet ready to replace a production Readarr instance unattended.

## Verified Working

- The Go API, React web UI, Postgres migrations, and Docker/TrueNAS custom-app
  shape build and run together.
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
- Removed qBittorrent downloads are hidden from active download listings.
- The UI tolerates nullable list payloads from the API, including empty
  `files`, `authors`, `downloads`, `releases`, `profiles`, `reviews`, and
  `events` arrays.

## Available But Still Needs Real-World Proving

- Readarr migration preview/import has broad API and UI coverage, but needs a
  full dry run against a real Readarr instance and a row-by-row comparison.
- Library scanning and import can process ebook/audiobook roots, but the real
  `media-stack` book roots still need a controlled full scan and import review.
- The end-to-end book loop needs more proof on real data:
  wanted item -> Prowlarr decision -> download client grab -> completed
  download -> import -> tracked file -> missing state clears.
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
- Published, versioned container images are not available yet; the TrueNAS
  deployment currently uses local images.
- The default local and TrueNAS examples are operator-facing and should be put
  behind `LIBRARRY_API_KEY`, Cosmos auth, Cloudflare Access, or another trusted
  boundary before WAN exposure.
- Librarry intentionally does not replace qBittorrent, Transmission, or SABnzbd
  as full download-client administration UIs.

## Current Live Deployment Notes

The local homelab deployment, when present, uses:

- TrueNAS custom app name: `librarry`
- Web portal: `http://192.168.1.221:30200/`
- Cosmos hostname: `https://librarry.borchetta.xyz/`
- API and web images: `librarry-api:local` and `librarry-web:local`
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
