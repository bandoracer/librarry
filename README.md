# Librarry

Librarry is a self-hosted book manager for ebooks and audiobooks. It is designed
as a modern Readarr replacement with a stronger metadata foundation, explicit
provider provenance, and first-class support for manual correction when upstream
book data is incomplete or ambiguous.

> Status: early alpha. Librarry is being built as a Readarr replacement, not a
> qBittorrent, Transmission, or SABnzbd replacement. The primary workflow is
> metadata-first library management: search and normalize book metadata, monitor
> authors, mark books wanted, evaluate releases, import files into ebook and
> audiobook libraries, and preserve manual corrections when provider data is
> wrong.
>
> The current build has real metadata search, provider health, a Library-first
> UI, wanted books, author subscriptions, persisted quality profiles,
> release-decision review, Prowlarr-backed search/feed sync, configured-root
> and ad hoc library scan, manual and completed-download import, Calibre
> Content Server handoff,
> Readarr-compatible author/book/wanted/import/history/config routes, and
> durable manual overrides for title, author, cover, quality profile, and
> monitoring state.
>
> Download clients are deliberately supporting infrastructure. Librarry can send
> approved book releases to configured clients and expose scoped acquisition
> activity when an import or failed download needs operator attention, but the
> goal is Readarr parity: metadata quality, author/book monitoring, import
> confidence, naming, Calibre integration, compatibility APIs, and a clean
> book-library UX.

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
| Manual correction | Supports normal app-level editing workflows. | Native wanted-item editor can correct title, author, cover URL, quality profile, and monitoring state; corrected title/author/cover/profile values write `manual_overrides` rows, provider refreshes preserve those fields, and visible override chips can reset individual fields. | Expand correction to the full author/work/edition graph, embedded file metadata writes, and batch review workflows. |
| Ebook and audiobook handling | Supports ebooks and audiobooks, but Readarr notes that one type of a given book requires one instance; both formats require multiple instances. | Data model and acquisition categories are format-aware for ebooks and audiobooks. | One app should manage both formats without collapsing editions or file targets. |
| Library import | Mature library scan and missing-book detection. | Configured ebook/audiobook roots and ad hoc root paths can be scanned from the Imports UI for ebook/audiobook files; OPF sidecars, embedded EPUB package metadata, MP3 ID3 tags, and M4B/MP4 metadata atoms are extracted during scan/import, files are tracked in Postgres, manual/completed import supports copy, move, hardlink, hardlink-or-copy, keep-both, replace, skip, and fail conflict decisions, completed Librarry-tagged downloads feed the library, unlinked completed downloads are queued for individual or bulk review resolution with explainable file, local-metadata, filename, download-context evidence, and high-confidence wanted-item suggestions, and Readarr-compatible missing pages suppress wanted items already present in tracked files. | Add broader profile-aware automatic import matching. |
| Wanted automation | Mature author/book monitoring, RSS monitoring, automatic grabs, failed-download handling, upgrades, sorting, and renaming. | Wanted queue, author subscriptions with all/future/none missing-book policies, metadata search, persisted quality profiles, Readarr-compatible release restrictions including tag-scoped rules for tagged wanted items, release evaluation, author/book tag persistence with bulk add/remove/replace editor modes, manual/interval wanted monitoring, scheduled author metadata sync, feed-based indexer sync, import-list sync into monitored wanted items, failed-download replacement search/grab, score-based upgrade search/grab, optional paused auto-grab, history, provider health, integration bootstrap, qBittorrent/Transmission/SABnzbd grabs, download reconciliation, file rename previews/apply, import conflict policies, bulk import-review resolution with evidence, and bulk selected-download actions. | Add deeper conflict reporting and broader review controls. |
| Manual release search | Mature manual search with rejection reasons and direct send to download clients. | Prowlarr-backed wanted release search with score/rejection reasons, native persisted Prowlarr/download-client settings, stored release-decision reload, approved/rejected review filters, explicit force-grab for manually selected rejected candidates, paused grab endpoint, manual magnet/torrent-file/torrent-URL/NZB add, stateful Readarr-compatible `/api/v1/release` search/grab adapter that can grab a persisted decision by returned Arr ID, qBittorrent/Transmission controls, and SABnzbd add/list/start/stop/delete support for NZB releases. | Add richer rejection explanations, quality scoring, and broader review queues. |
| Indexers | Native Readarr indexer support plus common Arr ecosystem patterns. | Prowlarr-compatible search client. | Keep Prowlarr as the preferred indexer aggregator instead of duplicating every indexer implementation. |
| Download clients | Supports SABnzbd, NZBGet, qBittorrent, Deluge, rTorrent, Transmission, uTorrent, and others. | Integrates with qBittorrent, Transmission, and SABnzbd as book-acquisition targets. The native Activity page is scoped to Librarry-tagged jobs, failed acquisitions, completed imports, and operator recovery, not general torrent-client management. | Keep clients as replaceable integrations behind small interfaces while Readarr parity work focuses on metadata, monitoring, release decisions, imports, naming, and compatibility APIs. |
| API compatibility | Readarr exposes the standard Arr `/api/v1` API for clients and tooling. | Compatibility shim covers optional Readarr-style API-key auth, common probes and read paths: ping, system status, health, diskspace, Postgres-backed root folders with Calibre settings, resource catalogs, and config records for naming/media-management/host/UI/indexer/download-client settings; calendar, history, parse, queue, blocklist/blacklist, durable author/book list/create/update/delete compatibility, bookfile list/get/update/delete compatibility, rename and retag preview/apply flows, and native-backed command handling for RSS sync, missing search, book search, author search, failed-download recovery, upgrade/cutoff-unmet search, rename/rescan, and database-backed retag state; manual import, missing wanted books, quality profiles, quality definitions, delay profiles, language/metadata profiles, tags, custom formats, restrictions, Webhook notifications with native delivery, import lists, remote path mappings, system tasks, download clients, indexers, release search/grab, plus schema/test/action/bulk routes for common configurable Arr resources. | Expand toward full OpenAPI compatibility, including deeper native config side effects, embedded metadata writes, Calibre content-server operations, and broader native behavior behind compatibility resources. |
| Calibre integration | Supports Calibre library integration and conversion through Calibre Content Server. | Readarr-style Calibre root-folder settings are accepted, validated, persisted, and returned. Imports that land under a Calibre-enabled root are posted to the Calibre Content Server add-book endpoint, store the returned Calibre ID, push basic title, author, and identifier metadata through set-fields, start configured output-format conversions, and refresh conversion status through native API, command calls, or scheduled background polling. Physical deletes for Calibre-backed files call the Content Server delete-books endpoint. Richer edition metadata, embedded metadata writes, path refresh after Calibre renames, and failed-import rollback are not implemented yet. | Implement richer metadata sync, embedded metadata writes, path refresh after Calibre-side changes, and stronger error recovery behind the same root-folder contract. |
| Post-download organization | Mature sorting and renaming. | Completed Librarry-tagged downloads can be imported into format-aware ebook/audiobook roots, mark wanted items imported, use configurable naming templates, choose copy/move/hardlink mode and duplicate-file behavior, queue unlinked files for individual or bulk review, and rename tracked files through native or Readarr-compatible APIs. | Add richer matching evidence and per-profile organization rules. |
| Deployment | Windows, Linux, macOS, NAS, and Docker guidance; no official Docker image according to Readarr docs. | Docker Compose and TrueNAS custom-app templates. | Publish versioned container images and release artifacts. |
| License | GPL-3.0. | AGPL-3.0. | Keep network-service modifications available to users. |

Sources: [Readarr GitHub repository](https://github.com/Readarr/Readarr),
[Readarr website](https://readarr.com/), and
[Servarr Readarr wiki](https://wiki.servarr.com/readarr).

## Current Features

- Go backend with REST APIs for health, metadata search, settings validation,
  provider diagnostics, release search, integration bootstrap, paused grabs, and
  download status.
- React and TypeScript web UI with dedicated views for provider health, metadata
  search, missing-focused wanted queue management, downloads, imports, settings,
  and operations history.
- Metadata search supports both book candidates and direct author identities, so
  an author can be monitored from a provider-backed author record without first
  selecting one of their books.
- Newly monitored authors are immediately eligible for a targeted bibliography
  refresh, which creates or reviews wanted books from metadata before release
  acquisition starts.
- Native integration settings API and UI for persisted Prowlarr, qBittorrent,
  Transmission, and SABnzbd configuration. Saved settings reconfigure the
  running acquisition service and are loaded again on restart.
- Native wanted metadata correction for title, author, cover URL, quality
  profile, and monitoring state, with `manual_overrides` persistence so provider
  refreshes do not overwrite corrected display metadata. Wanted payloads include
  visible override state, and individual override fields can be reset.
- Wanted metadata provenance API and UI show the stored provider records behind
  a selected wanted item, including provider, matched fields, confidence, and
  the active manual overrides that protect corrected fields. Field-level
  evidence also marks canonical values, provider candidates, and conflicts that
  need manual review. A metadata review queue surfaces wanted items with
  conflicts or protected fields before they become hidden library debt, and
  supported provider candidates can be applied as protected corrections without
  copying values by hand.
- Postgres schema for authors, works, editions, series, provider records,
  manual overrides, files, wanted items, releases, and downloads.
- Scoped acquisition activity for configured qBittorrent, Transmission, and
  SABnzbd clients: send approved book releases, inspect Librarry-tagged jobs,
  recover failed acquisitions, and import completed files without trying to
  replace the download client UI.
- Wanted-item persistence and release evaluation with approved/rejected
  decisions.
- Native Wanted queue filters for missing, wanted, grabbed, and all active items,
  with present-file suppression driven by tracked library files.
- Persisted quality profiles for ebooks and audiobooks, including minimum
  scores, upgrade cutoffs, seeder minimums, size limits, preferred terms,
  required terms, and rejected terms.
- Readarr-compatible release restrictions apply to native release evaluation as
  additional required, ignored, and preferred terms; tag-scoped restrictions
  apply to wanted items carrying matching tag IDs.
- Manual and scheduled wanted monitoring with monitor-run summaries and history
  events.
- Author subscriptions with all/future/none missing-book policies and manual or
  scheduled metadata refresh that creates or refreshes wanted items from
  monitored authors; skipped metadata candidates are persisted in an author
  metadata review queue with provider/title/date evidence and can be marked
  wanted or ignored from the monitor review UI. The Wanted page summarizes each
  monitored author's tracked, missing, grabbed, present, and metadata-review
  counts with a direct jump into that author's wanted books.
- Readarr-compatible author/book update and delete endpoints that persist
  monitor/unmonitor state, quality profile changes, and soft removals in
  Postgres.
- Manual and scheduled feed sync for Prowlarr-compatible indexer RSS feeds,
  with release persistence, matching against wanted items, optional paused
  auto-grab, and history events.
- Library scanning for configured ebook/audiobook roots or an ad hoc root path
  from the Imports UI, OPF sidecar, embedded EPUB, MP3 ID3, and M4B/MP4 metadata
  extraction, and manual file import into organized book folders.
- Completed-download import for Librarry-tagged qBittorrent items, with
  imported/error state persisted on download records.
- Calibre Content Server add-book handoff for manual or completed-download
  imports that target a Calibre-enabled Readarr-compatible root folder, plus
  basic set-fields metadata sync, configured output-format conversion starts
  with immediate and scheduled refreshable status snapshots, and delete-books
  handoff when deleting Calibre-backed files physically.
- Pending import review queue for completed downloads that are not linked to a
  wanted item, with import/skip resolution, high-confidence wanted-item
  suggestions, and explainable evidence from the UI.
- Readarr-compatible missing-book endpoints calculate missing state from wanted
  items plus tracked library files, so grabbed-but-unimported books remain
  visible while present files suppress false missing results.
- Readarr-compatible manual import endpoints for pending reviews, folder scans,
  and selected-file imports.
- Readarr-compatible bookfile endpoints for imported library files:
  list/filter/get are mapped from native files, delete removes native file
  records with optional physical file removal, and update persists
  quality/language metadata plus Readarr IDs on the tracked file record.
- Native and Readarr-compatible file rename preview/apply flows using the active
  library naming templates, including `/api/v1/rename` and `RenameFiles`
  command compatibility.
- Readarr-compatible retag preview/apply flows, including `/api/v1/retag` and
  `RetagFiles` command compatibility, that persist desired title, author,
  language, and quality tag state on tracked file records.
- Readarr-compatible calendar, history, and parse endpoints mapped from wanted
  items and Librarry history events.
- Configurable library naming templates for author folder, book folder, file
  name, and optional space replacement.
- Failed-download recovery for qBittorrent error/missing-file states and stale
  no-seed stalled downloads, with replacement search/grab and optional removal
  of failed torrents.
- Readarr-compatible blocklist and legacy blacklist endpoints populated from
  failed active downloads and failed Librarry history events, with single and
  bulk delete clearing download failure state and persisting compatibility
  tombstones for suppressed records.
- Readarr-compatible root folder list, lookup, create, update, and delete
  endpoints backed by Postgres, with environment-configured ebook/audiobook
  roots kept as defaults and Calibre Content Server fields round-tripped.
- Readarr-compatible filesystem, language, localization, log, update, and backup
  support endpoints for common Arr client probes.
- Readarr-compatible resource catalog endpoints for quality definitions,
  language profiles, metadata profiles, metadata consumers, tags, custom
  formats, restrictions, notifications, import lists, import-list exclusions,
  and remote path mappings, with create/update/list and delete persisted in
  Postgres.
- Readarr-compatible `ImportListSync` command for enabled import lists with
  inline book entries, metadata lookup, deterministic fallback records, merged
  tags, and import-list exclusion checks.
- Readarr-compatible Webhook notification delivery for grab, import, upgrade,
  failed-download, and test events using persisted notification resources.
- Readarr-compatible naming, media-management, host, UI, download-client, and
  indexer config endpoints with Postgres-backed compatible PUT/GET persistence,
  plus system-task endpoints derived from Librarry scheduler settings.
- Score-based upgrade search for grabbed/imported wanted items, with profile
  cutoffs, minimum score deltas, optional paused auto-grab, and history events.
- Metadata provider abstraction with initial adapters for Hardcover, Open
  Library, Google Books, and local OPF/embedded metadata; library import uses
  OPF and EPUB package metadata as high-confidence local evidence.
- Prowlarr-compatible release search for book indexers, exposed through both
  native Librarry routes and stateful Readarr-compatible `/api/v1/release`
  search/grab that maps returned Arr release IDs back to persisted Librarry
  release decisions.
- Native wanted release review reloads stored release decisions for the selected
  wanted item, filters approved and rejected candidates, and allows an explicit
  manual `force` grab for a rejected candidate while scheduled automation still
  grabs approved candidates only.
- qBittorrent integration for book categories, paused grabs, all-client and
  Librarry-tagged queue views, polling, simple active-queue rebalancing, single
  and bulk start, stop, delete, recheck,
  import, recovery, priority actions, per-torrent speed limits, manual URL/file
  adds, peer inspection, and tracker editing.
- Transmission integration for torrent add/list/start/stop/delete/recheck,
  torrent-file upload, location changes, labels, label-derived category/tag
  listing, label rename/delete, per-torrent speed limits, detail inspection,
  and file-priority controls.
- SABnzbd integration for NZB/Usenet grabs, queue/history polling, detail and
  queue-file inspection, plus start, stop, delete, rename, category, and
  priority actions, with category list/create/update/delete resource controls.
- Readarr-compatible API shim for common client probes and status views,
  including optional `X-Api-Key`/`apikey`/bearer authentication, `/ping`,
  `/api/v1/system/status`, `/api/v1/health`,
  `/api/v1/system/backup`, `/api/v1/update`, `/api/v1/diskspace`,
  `/api/v1/filesystem`, `/api/v1/language`, `/api/v1/localization`,
  `/api/v1/log`, `/api/v1/rootfolder`, `/api/v1/queue`,
  `/api/v1/blocklist`, `/api/v1/blacklist`,
  `/api/v1/author`, `/api/v1/author/lookup`, `/api/v1/author/editor`,
  `/api/v1/book`, `/api/v1/book/lookup`, `/api/v1/book/editor`, `/api/v1/bookfile`,
  `/api/v1/calendar`, `/api/v1/history`,
  `/api/v1/rename`, `/api/v1/retag`, `/api/v1/parse`, `/api/v1/manualimport`, `/api/v1/wanted/missing`,
  `/api/v1/wanted/cutoff`, `/api/v1/qualityprofile`, `/api/v1/qualitydefinition`,
  `/api/v1/delayprofile`, `/api/v1/languageprofile`, `/api/v1/metadataprofile`,
  `/api/v1/metadata`, `/api/v1/customformat`, `/api/v1/tag`, `/api/v1/restriction`,
  `/api/v1/notification`, `/api/v1/importlist`, `/api/v1/importlistexclusion`,
  `/api/v1/remotepathmapping`, `/api/v1/downloadclient`, `/api/v1/indexer`,
  `/api/v1/release`, `/api/v1/command`, and `/api/v1/system/task`, plus
  read-compatible host, UI, naming, media-management, indexer, and
  download-client config. Download-client, indexer, notification, and import-list
  compatibility resources include schema, test, action, and bulk mutation routes
  expected by common Arr clients.
- Docker Compose and TrueNAS custom-app deployment templates.

## Metadata Strategy

Librarry does not depend on a single book database.

Provider priority:

1. Hardcover is the preferred rich source for modern books, editions, series,
   ebook metadata, and audiobook metadata.
2. Open Library is the open-data backbone for works, authors, editions, ISBNs,
   and covers. Librarry treats Open Library author identity lookup separately
   from author bibliography crawling: author lookup uses `/search/authors.json`,
   while monitored authors with Open Library IDs use `/authors/{id}/works.json`
   before falling back to an author-name book search.
3. Google Books is an API-keyed exact-match fallback, not a primary graph.
4. Local OPF, EPUB package metadata, MP3 ID3 tags, and M4B/MP4 metadata atoms
   are high-confidence import evidence.
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
  - `GET /api/v1/system/backup`
  - `GET /api/v1/update`
  - `GET /api/v1/diskspace`
  - `GET /api/v1/filesystem`
  - `GET /api/v1/language`
  - `GET /api/v1/localization`
  - `GET /api/v1/localization/options`
  - `GET /api/v1/log`
  - `GET /api/v1/log/file`
  - `GET /api/v1/log/file/{filename}`
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
  - `GET /api/v1/rootfolder/{id}`
  - `POST /api/v1/rootfolder`
  - `PUT /api/v1/rootfolder/{id}`
  - `DELETE /api/v1/rootfolder/{id}`
  - `GET /api/v1/queue`
  - `GET /api/v1/queue/details`
  - `GET /api/v1/queue/status`
  - `POST /api/v1/queue/grab/{id}`
  - `POST /api/v1/queue/grab/bulk`
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
  - `PUT /api/v1/author/editor`
  - `DELETE /api/v1/author/editor`
  - `GET /api/v1/book`
  - `POST /api/v1/book`
  - `GET /api/v1/book/lookup`
  - `GET /api/v1/book/{id}`
  - `GET /api/v1/book/{id}/overview`
  - `PUT /api/v1/book/{id}`
  - `PUT /api/v1/book/monitor`
  - `DELETE /api/v1/book/{id}`
  - `PUT /api/v1/book/editor`
  - `DELETE /api/v1/book/editor`
  - `GET /api/v1/bookfile`
  - `GET /api/v1/bookfile/{id}`
  - `PUT /api/v1/bookfile/{id}`
  - `DELETE /api/v1/bookfile/{id}`
  - `DELETE /api/v1/bookfile/bulk`
  - `GET /api/v1/rename`
  - `GET /api/v1/retag`
  - `POST /api/v1/retag`
  - `GET /api/v1/wanted/missing`
  - `GET /api/v1/wanted/missing/{id}`
  - `GET /api/v1/wanted/cutoff`
  - `GET /api/v1/wanted/cutoff/{id}`
  - `GET /api/v1/qualityprofile`
  - `POST /api/v1/qualityprofile`
  - `GET /api/v1/qualityprofile/{id}`
  - `PUT /api/v1/qualityprofile/{id}`
  - `DELETE /api/v1/qualityprofile/{id}`
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
  - `GET /api/v1/metadata`
  - `GET /api/v1/metadata/schema`
  - `POST /api/v1/metadata/test`
  - `POST /api/v1/metadata/testall`
  - `POST /api/v1/metadata/action/{name}`
  - `PUT /api/v1/metadata/bulk`
  - `DELETE /api/v1/metadata/bulk`
  - `POST /api/v1/metadata`
  - `GET /api/v1/metadata/{id}`
  - `PUT /api/v1/metadata/{id}`
  - `DELETE /api/v1/metadata/{id}`
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
  - `GET /api/v1/notification/schema`
  - `POST /api/v1/notification/test`
  - `POST /api/v1/notification/testall`
  - `POST /api/v1/notification/action/{name}`
  - `PUT /api/v1/notification/bulk`
  - `DELETE /api/v1/notification/bulk`
  - `POST /api/v1/notification`
  - `GET /api/v1/notification/{id}`
  - `PUT /api/v1/notification/{id}`
  - `DELETE /api/v1/notification/{id}`
  - `GET /api/v1/importlist`
  - `GET /api/v1/importlist/schema`
  - `POST /api/v1/importlist/test`
  - `POST /api/v1/importlist/testall`
  - `POST /api/v1/importlist/action/{name}`
  - `PUT /api/v1/importlist/bulk`
  - `DELETE /api/v1/importlist/bulk`
  - `POST /api/v1/importlist`
  - `GET /api/v1/importlist/{id}`
  - `PUT /api/v1/importlist/{id}`
  - `DELETE /api/v1/importlist/{id}`
  - `GET /api/v1/importlistexclusion`
  - `POST /api/v1/importlistexclusion`
  - `GET /api/v1/importlistexclusion/{id}`
  - `PUT /api/v1/importlistexclusion/{id}`
  - `DELETE /api/v1/importlistexclusion/{id}`
  - `GET /api/v1/remotepathmapping`
  - `POST /api/v1/remotepathmapping`
  - `GET /api/v1/remotepathmapping/{id}`
  - `PUT /api/v1/remotepathmapping/{id}`
  - `DELETE /api/v1/remotepathmapping/{id}`
  - `GET /api/v1/downloadclient`
  - `GET /api/v1/downloadclient/schema`
  - `POST /api/v1/downloadclient/test`
  - `POST /api/v1/downloadclient/testall`
  - `POST /api/v1/downloadclient/action/{name}`
  - `PUT /api/v1/downloadclient/bulk`
  - `DELETE /api/v1/downloadclient/bulk`
  - `POST /api/v1/downloadclient`
  - `GET /api/v1/downloadclient/{id}`
  - `PUT /api/v1/downloadclient/{id}`
  - `DELETE /api/v1/downloadclient/{id}`
  - `GET /api/v1/indexer`
  - `GET /api/v1/indexer/schema`
  - `POST /api/v1/indexer/test`
  - `POST /api/v1/indexer/testall`
  - `POST /api/v1/indexer/action/{name}`
  - `PUT /api/v1/indexer/bulk`
  - `DELETE /api/v1/indexer/bulk`
  - `POST /api/v1/indexer`
  - `GET /api/v1/indexer/{id}`
  - `PUT /api/v1/indexer/{id}`
  - `DELETE /api/v1/indexer/{id}`
  - `GET /api/v1/release`
  - `POST /api/v1/release`
  - `GET /api/v1/manualimport`
  - `POST /api/v1/manualimport`
  - `GET /api/v1/command`
  - `POST /api/v1/command`
  - `GET /api/v1/command/{id}`
  - `DELETE /api/v1/command/{id}`
  - `GET /api/v1/system/task`
  - `GET /api/v1/system/task/{id}`
- Librarry-native endpoints:
  - `GET /healthz`
  - `GET /api/v1/providers/health`
  - `GET /api/v1/providers/diagnostics`
  - `GET /api/v1/search?query=Project%20Hail%20Mary`
  - `GET /api/v1/integrations/health`
  - `GET /api/v1/integrations/config`
  - `PUT /api/v1/integrations/config`
  - `POST /api/v1/integrations/bootstrap`
  - `POST /api/v1/releases/search`
  - `POST /api/v1/grabs`
  - `GET /api/v1/downloads`
  - `GET /api/v1/downloads/{id}`
  - `GET /api/v1/downloads/resources`
  - `GET /api/v1/downloads/preferences`
  - `PUT /api/v1/downloads/preferences`
  - `POST /api/v1/downloads/categories/actions`
  - `POST /api/v1/downloads/tags/actions`
  - `POST /api/v1/downloads/actions`
  - `POST /api/v1/downloads/{id}/files/actions`
  - `POST /api/v1/downloads/{id}/trackers/actions`
  - `POST /api/v1/downloads/rebalance`
  - `POST /api/v1/downloads/recover-failed`
  - `GET /api/v1/quality-profiles`
  - `POST /api/v1/quality-profiles`
  - `GET /api/v1/authors`
  - `POST /api/v1/authors`
  - `PATCH /api/v1/authors/{id}`
  - `PUT /api/v1/authors/{id}`
  - `DELETE /api/v1/authors/{id}`
  - `POST /api/v1/authors/monitor`
  - `GET /api/v1/authors/metadata/review`
  - `POST /api/v1/authors/metadata/review/{id}/resolve`
  - `GET /api/v1/wanted`
  - `POST /api/v1/wanted`
  - `PUT /api/v1/wanted/{id}`
  - `PATCH /api/v1/wanted/{id}`
  - `DELETE /api/v1/wanted/{id}`
  - `GET /api/v1/wanted/metadata/review`
  - `GET /api/v1/wanted/metadata/{id}`
  - `POST /api/v1/wanted/metadata/{id}/apply`
  - `DELETE /api/v1/wanted/{id}/overrides/{field}`
  - `POST /api/v1/wanted/{id}/search`
  - `GET /api/v1/wanted/releases/{id}`
  - `POST /api/v1/wanted/{id}/grab` (`force: true` explicitly overrides a
    rejected manual-search decision)
  - `POST /api/v1/wanted/monitor`
  - `POST /api/v1/wanted/feed-sync`
  - `POST /api/v1/wanted/upgrades`
  - `GET /api/v1/librarry/history`
  - `GET /api/v1/library/files`
  - `DELETE /api/v1/library/files/{id}`
  - `POST /api/v1/library/files/delete`
  - `POST /api/v1/library/files/rename/preview`
  - `POST /api/v1/library/files/rename`
  - `POST /api/v1/library/calibre/conversions/refresh`
  - `GET /api/v1/library/import-reviews`
  - `POST /api/v1/library/import-reviews/resolve-bulk`
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
LIBRARRY_API_KEY=
LIBRARRY_HARDCOVER_TOKEN=
LIBRARRY_GOOGLE_BOOKS_API_KEY=
LIBRARRY_PROWLARR_URL=
LIBRARRY_PROWLARR_API_KEY=
LIBRARRY_QBITTORRENT_URL=
LIBRARRY_QBITTORRENT_USERNAME=
LIBRARRY_QBITTORRENT_PASSWORD=
LIBRARRY_TRANSMISSION_URL=
LIBRARRY_TRANSMISSION_USERNAME=
LIBRARRY_TRANSMISSION_PASSWORD=
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
LIBRARRY_CALIBRE_REFRESH_ENABLED=true
LIBRARRY_CALIBRE_REFRESH_INTERVAL=15m
LIBRARRY_CALIBRE_REFRESH_LIMIT=200
LIBRARRY_CALIBRE_REFRESH_MAX_ATTEMPTS=1
```

`LIBRARRY_API_KEY` is optional for local development. When it is set, all
`/api/` routes require a Readarr-compatible key through `X-Api-Key`, `apikey`,
`apiKey`, or `Authorization: Bearer ...`; `/healthz` and `/ping` stay open for
service probes. The web UI can store the key per browser from Settings.

Provider notes:

- Open Library works without credentials.
- Hardcover requires `LIBRARRY_HARDCOVER_TOKEN`.
- Google Books requires `LIBRARRY_GOOGLE_BOOKS_API_KEY`.
- Prowlarr requires URL plus API key.
- qBittorrent can use username/password or a trusted LAN setup with auth
  disabled for the calling host.
- Transmission can use username/password or a trusted LAN setup with auth
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
- Calibre conversion refresh is enabled by default when database persistence is
  available. It polls stored conversion jobs and updates completed/failed
  conversion metadata without starting new conversions.
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

- Author-level review controls beyond the current library-aware missing queue.
- Per-profile organization rules.
- Richer Calibre edition metadata sync, embedded metadata writes, path refresh
  after Calibre renames, and import rollback.
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
