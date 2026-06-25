# Librarry

Librarry is a self-hosted book manager for ebooks and audiobooks. It is designed
as a modern Readarr replacement with a stronger metadata foundation, explicit
provider provenance, and first-class support for manual correction when upstream
book data is incomplete or ambiguous.

> Status: early alpha. The current build has real metadata search, provider
> health, Prowlarr release search, qBittorrent category bootstrap, and paused
> qBittorrent grabs with download polling and torrent actions. Full library
> import, wanted-item automation, and post-grab file organization are still in
> progress.

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
| Library import | Mature library scan and missing-book detection. | Database schema exists; full import workflow is not implemented yet. | Import OPF, EPUB, audio tags, and existing folders as evidence for matching and review. |
| Wanted automation | Mature author/book monitoring, RSS monitoring, automatic grabs, failed-download handling, upgrades, sorting, and renaming. | Metadata search, release search, provider health, integration bootstrap, paused qBittorrent grabs, and download reconciliation. | Wanted-item workflow from metadata result to release selection to queued download to organized file. |
| Manual release search | Mature manual search with rejection reasons and direct send to download clients. | Prowlarr-backed release search, paused grab endpoint, and qBittorrent controls. | Add rejection explanations, quality scoring, and manual review queues. |
| Indexers | Native Readarr indexer support plus common Arr ecosystem patterns. | Prowlarr-compatible search client. | Keep Prowlarr as the preferred indexer aggregator instead of duplicating every indexer implementation. |
| Download clients | Supports SABnzbd, NZBGet, qBittorrent, Deluge, rTorrent, Transmission, uTorrent, and others. | qBittorrent add/list/start/stop/delete/recheck/priority controls are implemented; SABnzbd interface is stubbed. | Add clients only behind small interfaces once metadata and matching are stable. |
| Calibre integration | Supports Calibre library integration and conversion through Calibre Content Server. | Not implemented. | Possible future integration, but not before import, matching, and organization are reliable. |
| Post-download organization | Mature sorting and renaming. | Not implemented beyond acquisition interfaces. | Format-aware organization for ebooks and audiobooks with manual import review. |
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
  search, download management, and integration settings.
- Postgres schema for authors, works, editions, series, provider records,
  manual overrides, files, wanted items, releases, and downloads.
- Postgres-backed download reconciliation from qBittorrent state.
- Metadata provider abstraction with initial adapters for Hardcover, Open
  Library, Google Books, and local OPF/embedded metadata.
- Prowlarr-compatible release search for book indexers.
- qBittorrent integration for book categories, paused grabs, polling, start,
  stop, delete, recheck, and priority actions.
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
LIBRARRY_EBOOK_CATEGORY=books-ebook
LIBRARRY_AUDIOBOOK_CATEGORY=books-audiobook
LIBRARRY_BOOK_TORRENT_ROOT=/data/torrents/books
```

Provider notes:

- Open Library works without credentials.
- Hardcover requires `LIBRARRY_HARDCOVER_TOKEN`.
- Google Books requires `LIBRARRY_GOOGLE_BOOKS_API_KEY`.
- Prowlarr requires URL plus API key.
- qBittorrent can use username/password or a trusted LAN setup with auth
  disabled for the calling host.

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

- Library import and file fingerprinting for ebooks and audiobooks.
- Wanted-item workflow from metadata result to release search to queued grab.
- Post-download organization and manual import review.
- Better edition selection for narrator, language, format, and ISBN.
- Hardcover and Google Books fixture coverage.
- Optional download clients beyond qBittorrent.
- Public image publishing and release builds.

## Contributing

Issues and pull requests are welcome while the project is young. The most useful
contributions right now are metadata edge cases, provider fixtures, matching
tests, deployment feedback, and UI workflows for manual review.

Please do not add scraping for Goodreads, Amazon, or Audible to core.

## License

Librarry is licensed under the
[GNU Affero General Public License v3.0](LICENSE).
