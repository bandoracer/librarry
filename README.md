# Librarry

Librarry is a self-hosted book manager for ebooks and audiobooks. It is designed
as a modern Readarr replacement with a stronger metadata foundation, explicit
provider provenance, and first-class support for manual correction when upstream
book data is incomplete or ambiguous.

> Status: early alpha. The current build has real metadata search, provider
> health, Prowlarr release search, qBittorrent category bootstrap, paused
> qBittorrent grabs, Readarr-compatible release search/grab endpoints, download
> polling, torrent actions, and simple active-queue rebalancing. SABnzbd
> NZB/Usenet grabs, queue/history polling, and start/stop/delete actions are
> implemented.
> Wanted items, author subscriptions, and release evaluation are implemented,
> with manual and scheduled wanted and author monitoring plus history. Feed-based
> indexer sync is implemented through Prowlarr-compatible RSS feeds. Library scan
> and manual file import are implemented. Completed qBittorrent downloads can be
> imported into organized library roots. Readarr-compatible manual import can
> list pending review items, scan a folder for candidates, and import selected
> files. Failed-download detection and replacement search/grab are implemented.
> Failed downloads and failed history events are exposed through
> Readarr-compatible blocklist/blacklist endpoints. Score-based upgrade
> search/grab is implemented. Pending import review for unlinked completed
> downloads and configurable naming templates are implemented. Common Arr
> resource catalog endpoints are present for tags, quality definitions, language
> profiles, metadata profiles, custom formats, restrictions, notifications,
> import lists, and remote path mappings. Host, UI, indexer, download-client,
> delay-profile, and system-task compatibility endpoints are implemented.
> Persisted quality profiles now drive release scoring, preferred and rejected
> terms, size limits, seeder minimums, and upgrade cutoffs. Librarry is not yet
> a complete Readarr-style torrent manager; conflict handling, advanced torrent
> queue arbitration, persistent compatibility config/resource editing, and broad
> download-client parity are still early.

![Librarry UI concept](docs/assets/librarry-ui-concept.png)

## Why Librarry?

Book automation is harder than movie or TV automation because metadata is
messy: works, editions, formats, narrators, translations, ISBNs, series order,
and publisher data frequently disagree across providers. Librarry treats that as
the core product problem instead of hiding it behind one fragile upstream source.

The project goals are:

- keep a local canonical model for authors, works, editions, series, files,
  releases, downloads, and manual overrides;
- preserve raw provider records so matches can be explained and corrected;
- support ebooks and audiobooks without collapsing them into one target;
- make manual overrides durable and higher priority than every provider;
- integrate with existing self-hosted media stacks instead of replacing them.

## Librarry vs. Readarr

Readarr is the obvious reference point, but it has been retired by the Servarr
team and its repository is archived. The official retirement note cites
unusable metadata, limited maintainer time, and a stalled Open Library
transition. Readarr remains much more complete as an automation app today;
Librarry is an early replacement focused first on fixing the metadata model.

| Capability | Readarr | Librarry today | Librarry direction |
| --- | --- | --- | --- |
| Project status | Retired and archived; existing installs may continue but upstream development has stopped. | Active early alpha. | Public, maintained replacement with a smaller but durable core. |
| Metadata source model | Historically depended on centralized metadata; the retirement announcement names metadata failure as the blocking issue. | Multi-provider abstraction with Hardcover, Open Library, Google Books fallback, and local metadata stubs. | Local canonical graph with provider provenance, explainable matches, and resilient fallback behavior. |
| Manual correction | Supports normal app-level editing workflows. | Schema includes manual overrides as first-class records. | Manual overrides always win and remain auditable across provider refreshes. |
| Ebook and audiobook handling | Supports ebooks and audiobooks, but Readarr notes that one type of a given book requires one instance; both formats require multiple instances. | Data model and acquisition categories are format-aware for ebooks and audiobooks. | One app should manage both formats without collapsing editions or file targets. |
| Library import | Mature library scan and missing-book detection. | Library roots can be scanned for ebook/audiobook files, tracked in Postgres, manually imported, fed by completed qBittorrent imports, and reviewed when completed downloads are not linked to wanted items. | Add OPF, EPUB, audio-tag extraction, missing-book detection, and stronger import matching. |
| Wanted automation | Mature author/book monitoring, RSS monitoring, automatic grabs, failed-download handling, upgrades, sorting, and renaming. | Wanted queue, author subscriptions, metadata search, persisted quality profiles, release evaluation, manual/interval wanted monitoring, scheduled author metadata sync, feed-based indexer sync, failed-download replacement search/grab, score-based upgrade search/grab, optional paused auto-grab, history, provider health, integration bootstrap, qBittorrent/SABnzbd grabs, download reconciliation, and bulk selected-download actions. | Add richer review flows, missing-book policy, and conflict handling. |
| Manual release search | Mature manual search with rejection reasons and direct send to download clients. | Prowlarr-backed wanted release search with score/rejection reasons, paused grab endpoint, Readarr-compatible `/api/v1/release` search/grab adapter, qBittorrent controls, and SABnzbd add/list/start/stop/delete support for NZB releases. | Add richer rejection explanations, quality scoring, and manual review queues. |
| Indexers | Native Readarr indexer support plus common Arr ecosystem patterns. | Prowlarr-compatible search client. | Keep Prowlarr as the preferred indexer aggregator instead of duplicating every indexer implementation. |
| Download clients | Supports SABnzbd, NZBGet, qBittorrent, Deluge, rTorrent, Transmission, uTorrent, and others. | qBittorrent add/list/start/stop/delete/recheck/priority controls and simple active-queue rebalancing are implemented. SABnzbd can add NZB/Usenet releases, list queue/history state, and start, stop, or delete jobs. This is still not a full multi-client torrent manager. | Add conflict-aware arbitration, per-client capability handling, and more clients only behind small interfaces once metadata and matching are stable. |
| API compatibility | Readarr exposes the standard Arr `/api/v1` API for clients and tooling. | Compatibility shim covers common probes and read paths: ping, system status, health, diskspace, host/UI/naming/media-management/indexer/download-client config, calendar, history, parse, root folders, queue, blocklist/blacklist, author/book list/create/lookup, manual import, missing wanted books, quality profiles, quality definitions, delay profiles, language/metadata profiles, tags, custom formats, restrictions, notifications, import lists, remote path mappings, system tasks, download clients, indexers, release search/grab, and basic commands. | Expand toward full OpenAPI compatibility, including durable author/book update/delete semantics, persistent config writes, rename/retag, Calibre endpoints, and persisted resource editing. |
| Calibre integration | Supports Calibre library integration and conversion through Calibre Content Server. | Not implemented. | Possible future integration, but not before import, matching, and organization are reliable. |
| Post-download organization | Mature sorting and renaming. | Completed Librarry-tagged qBittorrent downloads can be imported into format-aware ebook/audiobook roots, mark wanted items imported, use configurable naming templates, and queue unlinked files for review. | Add conflict policies, embedded metadata matching, bulk review, and per-profile organization rules. |
| Deployment | Windows, Linux, macOS, NAS, and Docker guidance; no official Docker image according to Readarr docs. | Docker Compose and TrueNAS custom-app templates. | Publish versioned container images and release artifacts. |
| License | GPL-3.0. | AGPL-3.0. | Keep network-service modifications available to users. |

Sources: [Readarr GitHub repository](https://github.com/Readarr/Readarr),
[Readarr website](https://readarr.com/), and
[Servarr Readarr wiki](https://wiki.servarr.com/readarr).

## Current Features

- Go backend with REST APIs for health, metadata search, settings validation,
  provider diagnostics, release search, integration bootstrap, paused grabs, and
  download status.
- React and TypeScript web UI for provider health, metadata search, release
  search, wanted queue management, download management, and integration
  settings.
- Postgres schema for authors, works, editions, series, provider records,
  manual overrides, files, wanted items, releases, and downloads.
- Postgres-backed download reconciliation from qBittorrent and SABnzbd state.
- Wanted-item persistence and release evaluation with approved/rejected
  decisions.
- Persisted quality profiles for ebooks and audiobooks, including minimum
  scores, upgrade cutoffs, seeder minimums, size limits, preferred terms,
  required terms, and rejected terms.
- Manual and scheduled wanted monitoring with monitor-run summaries and history
  events.
- Author subscriptions with manual and scheduled metadata refresh that creates
  or refreshes wanted items from monitored authors.
- Manual and scheduled feed sync for Prowlarr-compatible indexer RSS feeds,
  with release persistence, matching against wanted items, optional paused
  auto-grab, and history events.
- Library scanning for ebook/audiobook roots and manual file import into
  organized book folders.
- Completed-download import for Librarry-tagged qBittorrent items, with
  imported/error state persisted on download records.
- Pending import review queue for completed downloads that are not linked to a
  wanted item, with import/skip resolution from the UI.
- Readarr-compatible manual import endpoints for pending reviews, folder scans,
  and selected-file imports.
- Readarr-compatible calendar, history, and parse endpoints mapped from wanted
  items and Librarry history events.
- Configurable library naming templates for author folder, book folder, file
  name, and optional space replacement.
- Failed-download recovery for qBittorrent error/missing-file states and stale
  no-seed stalled downloads, with replacement search/grab and optional removal
  of failed torrents.
- Readarr-compatible blocklist and legacy blacklist endpoints populated from
  failed active downloads and failed Librarry history events.
- Readarr-compatible resource catalog endpoints for quality definitions,
  language profiles, metadata profiles, tags, custom formats, restrictions,
  notifications, import lists, and remote path mappings. These return useful
  defaults and echo create/update payloads until each resource has a persisted
  Librarry model.
- Readarr-compatible host, UI, download-client, indexer, delay-profile, and
  system-task endpoints derived from Librarry config and scheduler settings.
- Score-based upgrade search for grabbed/imported wanted items, with profile
  cutoffs, minimum score deltas, optional paused auto-grab, and history events.
- Metadata provider abstraction with initial adapters for Hardcover, Open
  Library, Google Books, and local OPF/embedded metadata.
- Prowlarr-compatible release search for book indexers, exposed through both
  native Librarry routes and Readarr-compatible `/api/v1/release` search/grab.
- qBittorrent integration for book categories, paused grabs, polling, simple
  active-queue rebalancing, single and bulk start, stop, delete, recheck,
  import, recovery, and priority actions.
- SABnzbd integration for NZB/Usenet grabs, queue/history polling, and start,
  stop, and delete actions.
- Readarr-compatible API shim for common client probes and status views,
  including `/ping`, `/api/v1/system/status`, `/api/v1/health`,
  `/api/v1/diskspace`, `/api/v1/rootfolder`, `/api/v1/queue`,
  `/api/v1/blocklist`, `/api/v1/blacklist`,
  `/api/v1/author`, `/api/v1/author/lookup`, `/api/v1/book`,
  `/api/v1/book/lookup`, `/api/v1/calendar`, `/api/v1/history`,
  `/api/v1/parse`, `/api/v1/manualimport`, `/api/v1/wanted/missing`,
  `/api/v1/qualityprofile`, `/api/v1/qualitydefinition`,
  `/api/v1/delayprofile`, `/api/v1/languageprofile`, `/api/v1/metadataprofile`,
  `/api/v1/customformat`, `/api/v1/tag`, `/api/v1/restriction`,
  `/api/v1/notification`, `/api/v1/importlist`,
  `/api/v1/remotepathmapping`, `/api/v1/downloadclient`, `/api/v1/indexer`,
  `/api/v1/release`, `/api/v1/command`, and `/api/v1/system/task`, plus
  read-compatible host, UI, naming, media-management, indexer, and
  download-client config.
- Docker Compose and TrueNAS custom-app deployment templates.

## Metadata Strategy

Librarry does not depend on a single book database.

Provider priority:

1. Hardcover is the preferred rich source for modern books, editions, series,
   ebook metadata, and audiobook metadata.
2. Open Library is the open-data backbone for works, authors, editions, ISBNs,
   and covers.
3. Google Books is an API-keyed exact-match fallback, not a primary graph.
4. Local OPF, EPUB, and audio tags are high-confidence import evidence.
5. Manual overrides always win.

Goodreads, Amazon, and Audible scraping are intentionally not part of core.

## Architecture

```text
backend/   Go API, worker foundation, metadata adapters, acquisition clients
web/       Vite, React, TypeScript UI
deploy/    Dockerfiles, Compose files, TrueNAS template, sample environment
docs/      Architecture, metadata policy, local dev, provider setup
```

The backend owns provider credentials and normalization. The browser only talks
to the Librarry API.

Important API surfaces:

- Readarr-compatible endpoints:
  - `GET /ping`
  - `HEAD /ping`
  - `GET /api/v1/system/status`
  - `GET /api/v1/system/routes`
  - `GET /api/v1/system/routes/duplicate`
  - `GET /api/v1/health`
  - `GET /api/v1/diskspace`
  - `GET /api/v1/config/naming`
  - `GET /api/v1/config/naming/{id}`
  - `PUT /api/v1/config/naming/{id}`
  - `GET /api/v1/config/naming/examples`
  - `GET /api/v1/config/mediamanagement`
  - `GET /api/v1/config/mediamanagement/{id}`
  - `PUT /api/v1/config/mediamanagement/{id}`
  - `GET /api/v1/config/host`
  - `GET /api/v1/config/host/{id}`
  - `PUT /api/v1/config/host/{id}`
  - `GET /api/v1/config/ui`
  - `GET /api/v1/config/ui/{id}`
  - `PUT /api/v1/config/ui/{id}`
  - `GET /api/v1/config/downloadclient`
  - `GET /api/v1/config/downloadclient/{id}`
  - `PUT /api/v1/config/downloadclient/{id}`
  - `GET /api/v1/config/indexer`
  - `GET /api/v1/config/indexer/{id}`
  - `PUT /api/v1/config/indexer/{id}`
  - `GET /api/v1/calendar`
  - `GET /api/v1/history`
  - `GET /api/v1/history/since`
  - `GET /api/v1/history/author`
  - `GET /api/v1/history/book`
  - `GET /api/v1/parse`
  - `GET /api/v1/rootfolder`
  - `GET /api/v1/queue`
  - `GET /api/v1/queue/details`
  - `GET /api/v1/queue/status`
  - `DELETE /api/v1/queue/{id}`
  - `DELETE /api/v1/queue/bulk`
  - `GET /api/v1/blocklist`
  - `DELETE /api/v1/blocklist/{id}`
  - `DELETE /api/v1/blocklist/bulk`
  - `GET /api/v1/blacklist`
  - `DELETE /api/v1/blacklist/{id}`
  - `DELETE /api/v1/blacklist/bulk`
  - `GET /api/v1/author`
  - `POST /api/v1/author`
  - `GET /api/v1/author/lookup`
  - `GET /api/v1/author/{id}`
  - `PUT /api/v1/author/{id}`
  - `DELETE /api/v1/author/{id}`
  - `GET /api/v1/book`
  - `POST /api/v1/book`
  - `GET /api/v1/book/lookup`
  - `GET /api/v1/book/{id}`
  - `GET /api/v1/book/{id}/overview`
  - `PUT /api/v1/book/{id}`
  - `PUT /api/v1/book/monitor`
  - `DELETE /api/v1/book/{id}`
  - `GET /api/v1/wanted/missing`
  - `GET /api/v1/qualityprofile`
  - `GET /api/v1/delayprofile`
  - `POST /api/v1/delayprofile`
  - `GET /api/v1/delayprofile/{id}`
  - `PUT /api/v1/delayprofile/{id}`
  - `DELETE /api/v1/delayprofile/{id}`
  - `GET /api/v1/qualitydefinition`
  - `PUT /api/v1/qualitydefinition/{id}`
  - `GET /api/v1/languageprofile`
  - `POST /api/v1/languageprofile`
  - `GET /api/v1/languageprofile/{id}`
  - `PUT /api/v1/languageprofile/{id}`
  - `DELETE /api/v1/languageprofile/{id}`
  - `GET /api/v1/metadataprofile`
  - `POST /api/v1/metadataprofile`
  - `GET /api/v1/metadataprofile/{id}`
  - `PUT /api/v1/metadataprofile/{id}`
  - `DELETE /api/v1/metadataprofile/{id}`
  - `GET /api/v1/customformat`
  - `POST /api/v1/customformat`
  - `GET /api/v1/customformat/{id}`
  - `PUT /api/v1/customformat/{id}`
  - `DELETE /api/v1/customformat/{id}`
  - `GET /api/v1/tag`
  - `POST /api/v1/tag`
  - `GET /api/v1/tag/{id}`
  - `PUT /api/v1/tag/{id}`
  - `DELETE /api/v1/tag/{id}`
  - `GET /api/v1/restriction`
  - `POST /api/v1/restriction`
  - `GET /api/v1/restriction/{id}`
  - `PUT /api/v1/restriction/{id}`
  - `DELETE /api/v1/restriction/{id}`
  - `GET /api/v1/notification`
  - `POST /api/v1/notification`
  - `GET /api/v1/notification/{id}`
  - `PUT /api/v1/notification/{id}`
  - `DELETE /api/v1/notification/{id}`
  - `GET /api/v1/importlist`
  - `POST /api/v1/importlist`
  - `GET /api/v1/importlist/{id}`
  - `PUT /api/v1/importlist/{id}`
  - `DELETE /api/v1/importlist/{id}`
  - `GET /api/v1/remotepathmapping`
  - `POST /api/v1/remotepathmapping`
  - `GET /api/v1/remotepathmapping/{id}`
  - `PUT /api/v1/remotepathmapping/{id}`
  - `DELETE /api/v1/remotepathmapping/{id}`
  - `GET /api/v1/downloadclient`
  - `GET /api/v1/indexer`
  - `GET /api/v1/release`
  - `POST /api/v1/release`
  - `GET /api/v1/manualimport`
  - `POST /api/v1/manualimport`
  - `GET /api/v1/command`
  - `POST /api/v1/command`
  - `GET /api/v1/system/task`
  - `GET /api/v1/system/task/{id}`
- Librarry-native endpoints:
  - `GET /healthz`
  - `GET /api/v1/providers/health`
  - `GET /api/v1/providers/diagnostics`
  - `GET /api/v1/search?query=Project%20Hail%20Mary`
  - `GET /api/v1/integrations/health`
  - `POST /api/v1/integrations/bootstrap`
  - `POST /api/v1/releases/search`
  - `POST /api/v1/grabs`
  - `GET /api/v1/downloads`
  - `POST /api/v1/downloads/actions`
  - `POST /api/v1/downloads/rebalance`
  - `POST /api/v1/downloads/recover-failed`
  - `GET /api/v1/quality-profiles`
  - `POST /api/v1/quality-profiles`
  - `GET /api/v1/authors`
  - `POST /api/v1/authors`
  - `POST /api/v1/authors/monitor`
  - `GET /api/v1/wanted`
  - `POST /api/v1/wanted`
  - `POST /api/v1/wanted/{id}/search`
  - `GET /api/v1/wanted/{id}/releases`
  - `POST /api/v1/wanted/{id}/grab`
  - `POST /api/v1/wanted/monitor`
  - `POST /api/v1/wanted/feed-sync`
  - `POST /api/v1/wanted/upgrades`
  - `GET /api/v1/librarry/history`
  - `GET /api/v1/library/files`
  - `GET /api/v1/library/import-reviews`
  - `POST /api/v1/library/scan`
  - `POST /api/v1/library/import`
  - `POST /api/v1/library/import-completed`
  - `POST /api/v1/library/import-reviews/{id}/resolve`
  - `POST /api/v1/settings/validate`

## Quick Start

Requirements:

- Go 1.23+
- Node.js 22+
- Docker and Docker Compose
- Postgres 16, unless using Compose

Run the full local stack:

```bash
git clone https://github.com/bandoracer/librarry.git
cd librarry/deploy
cp .env.example .env
docker compose up --build
```

Then open:

```text
http://127.0.0.1:5173
```

Run the backend without Docker:

```bash
go test ./...
LIBRARRY_DATABASE_URL=postgres://librarry:librarry@127.0.0.1:5432/librarry?sslmode=disable \
  go run ./backend/cmd/librarry
```

Run the web UI:

```bash
cd web
npm install
npm run dev
```

The Vite dev server proxies API requests to `http://127.0.0.1:8080`.

## Configuration

Start from [deploy/.env.example](deploy/.env.example).

Common settings:

```dotenv
LIBRARRY_DATABASE_URL=postgres://librarry:librarry@postgres:5432/librarry?sslmode=disable
LIBRARRY_HARDCOVER_TOKEN=
LIBRARRY_GOOGLE_BOOKS_API_KEY=
LIBRARRY_PROWLARR_URL=
LIBRARRY_PROWLARR_API_KEY=
LIBRARRY_QBITTORRENT_URL=
LIBRARRY_QBITTORRENT_USERNAME=
LIBRARRY_QBITTORRENT_PASSWORD=
LIBRARRY_SABNZBD_URL=
LIBRARRY_SABNZBD_API_KEY=
LIBRARRY_SABNZBD_USERNAME=
LIBRARRY_SABNZBD_PASSWORD=
LIBRARRY_EBOOK_CATEGORY=books-ebook
LIBRARRY_AUDIOBOOK_CATEGORY=books-audiobook
LIBRARRY_BOOK_TORRENT_ROOT=/data/torrents/books
LIBRARRY_EBOOK_LIBRARY_ROOT=/data/media/books/ebooks
LIBRARRY_AUDIOBOOK_LIBRARY_ROOT=/data/media/books/audiobooks
LIBRARRY_NAMING_AUTHOR_FOLDER={Author}
LIBRARRY_NAMING_BOOK_FOLDER={Title}
LIBRARRY_NAMING_FILE_NAME={Title}{Ext}
LIBRARRY_NAMING_SPACE_REPLACEMENT=
LIBRARRY_MONITOR_ENABLED=true
LIBRARRY_MONITOR_INTERVAL=30m
LIBRARRY_MONITOR_SEARCH_INTERVAL=6h
LIBRARRY_MONITOR_LIMIT=50
LIBRARRY_MONITOR_AUTO_GRAB=false
LIBRARRY_AUTHOR_MONITOR_ENABLED=true
LIBRARRY_AUTHOR_MONITOR_INTERVAL=6h
LIBRARRY_AUTHOR_MONITOR_SYNC_INTERVAL=24h
LIBRARRY_AUTHOR_MONITOR_LIMIT=50
LIBRARRY_FEED_SYNC_ENABLED=true
LIBRARRY_FEED_SYNC_INTERVAL=15m
LIBRARRY_FEED_SYNC_LIMIT=100
LIBRARRY_FEED_SYNC_AUTO_GRAB=false
LIBRARRY_FAILED_DOWNLOAD_ENABLED=true
LIBRARRY_FAILED_DOWNLOAD_INTERVAL=30m
LIBRARRY_FAILED_DOWNLOAD_STALLED_AGE=24h
LIBRARRY_FAILED_DOWNLOAD_LIMIT=50
LIBRARRY_FAILED_DOWNLOAD_AUTO_GRAB=false
LIBRARRY_FAILED_DOWNLOAD_REMOVE=false
LIBRARRY_FAILED_DOWNLOAD_DELETE_FILES=false
LIBRARRY_UPGRADE_SEARCH_ENABLED=true
LIBRARRY_UPGRADE_SEARCH_INTERVAL=12h
LIBRARRY_UPGRADE_SEARCH_LIMIT=50
LIBRARRY_UPGRADE_SEARCH_AUTO_GRAB=false
LIBRARRY_UPGRADE_SEARCH_MIN_DELTA=5
```

Provider notes:

- Open Library works without credentials.
- Hardcover requires `LIBRARRY_HARDCOVER_TOKEN`.
- Google Books requires `LIBRARRY_GOOGLE_BOOKS_API_KEY`.
- Prowlarr requires URL plus API key.
- qBittorrent can use username/password or a trusted LAN setup with auth
  disabled for the calling host.
- SABnzbd requires `LIBRARRY_SABNZBD_URL` and `LIBRARRY_SABNZBD_API_KEY`.
  Username/password are optional and only needed when SABnzbd itself is behind
  basic auth.
- Scheduled wanted monitoring is enabled by default. Set
  `LIBRARRY_MONITOR_AUTO_GRAB=true` only when you want automation to send
  approved releases to the configured download client without a manual click.
- Scheduled author monitoring is enabled by default. It only searches metadata
  and creates or refreshes wanted items; release grabbing still happens through
  wanted monitoring, feed sync, upgrades, recovery, or manual actions.
- Scheduled feed sync is enabled by default. Set
  `LIBRARRY_FEED_SYNC_AUTO_GRAB=true` only when you want feed matches to send
  approved releases to the configured download client without a manual click.
- Failed-download recovery is enabled by default in search-only mode. Set
  `LIBRARRY_FAILED_DOWNLOAD_AUTO_GRAB=true` to queue approved replacements and
  `LIBRARRY_FAILED_DOWNLOAD_REMOVE=true` to remove failed torrents from
  qBittorrent after recovery. `LIBRARRY_FAILED_DOWNLOAD_DELETE_FILES=true`
  also deletes the failed torrent payload when removal is enabled.
- Upgrade search is enabled by default in search-only mode. Set
  `LIBRARRY_UPGRADE_SEARCH_AUTO_GRAB=true` only when you want approved upgrades
  to be sent to the configured download client automatically.
- Naming templates support `{Author}`, `{Title}`, `{Format}`, and `{Ext}`.
  Defaults keep the layout as `Author/Title/Title.ext`.

## Deployment

The default Compose stack starts Postgres, the API, and the built web UI:

```bash
cd deploy
docker compose up --build
```

The TrueNAS custom-app template lives at
[deploy/truenas/install.yaml](deploy/truenas/install.yaml). It intentionally
contains placeholder secrets and local image names. Build or publish your own
`librarry-api` and `librarry-web` images before installing it.

## Development

Useful checks:

```bash
go test ./...
cd web && npm run build
git diff --check
```

The test suite currently covers provider normalization, matching confidence,
settings validation, and API handlers. Fixtures and broader acquisition tests
will grow as the automation path stabilizes.

## Roadmap

- OPF, EPUB, and audio-tag extraction during library scans.
- Author-level missing-book policy and review controls.
- Conflict policies, bulk import review, and per-profile organization rules.
- Better edition selection for narrator, language, format, and ISBN.
- Hardcover and Google Books fixture coverage.
- Conflict-aware queue arbitration and additional download clients.
- Public image publishing and release builds.

## Contributing

Issues and pull requests are welcome while the project is young. The most useful
contributions right now are metadata edge cases, provider fixtures, matching
tests, deployment feedback, and UI workflows for manual review.

Please do not add scraping for Goodreads, Amazon, or Audible to core.

## License

Librarry is licensed under the
[GNU Affero General Public License v3.0](LICENSE).
