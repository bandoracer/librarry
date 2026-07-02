# Readarr Parity Audit

> **v0.4.0 (2026-07-02): the ten model/nomenclature inconsistencies found in
> the post-v0.3.0 audit are closed** — quality ladder + release profiles +
> quality definitions, named metadata profiles, Wanted as a pure gap view
> with Authors under Library, Calibre-managed root folders, Readarr settings
> tab names (Indexers / Download Clients / Quality), System sub-tabs, Readarr
> toolbar verbs and "RSS Sync", Rename Books toggle, add-dialog root/tags
> pickers, and blocklist book links.

> **v0.2.0 (2026-07-01): all six milestones of [parity-plan.md](parity-plan.md)
> were implemented** (delay profiles intentionally skipped per owner decision).
> Closed: blocklist, remove-after-seeding, cutoff-unmet, search-on-add, all
> seven monitor modes, author/book detail pages, poster/overview views, mass
> editor, sort/filters, multiple root folders, remote path mappings, recycle
> bin, series/year naming tokens, rename UI, .cue extras, task scheduler view,
> native notifications (webhook/ntfy/Discord/Telegram) with worker dispatch,
> continuous health checks, disk space, calendar + iCal feed, Hardcover import
> lists + exclusions, author add-filters, native tags, none/basic/forms auth,
> and pg_dump backups. Auto-grab defaults flipped to arr behavior. The tables
> below describe the pre-v0.2.0 state and are retained as the audit record.

Last updated: 2026-07-01. Sources: Servarr wiki markdown (Readarr settings,
library, activity, wanted, calendar, system, quick-start; retired 2025) read
from `github.com/Servarr/Wiki`, compared against Librarry `main` as of this
date.

Purpose: enumerate the gaps a Readarr user would notice, ranked by how visible
they are, and record where Librarry intentionally diverges. Feeds phase-2
planning; see also [ui-backlog.md](ui-backlog.md).

## Tier 1 — surfaces a Readarr user hits immediately

| # | Readarr feature | Readarr behavior | Librarry today | Notes |
|---|---|---|---|---|
| 1 | Calendar | Release calendar with monitored/missing color stripes, iCal feed (tag-filtered, past/future ranges), search-missing-on-view | Absent entirely | Needs reliable release-date metadata; compat `/api/v1/calendar` route exists for API clients but there is no native UI |
| 2 | Library views | Table / Posters / Overview view modes, sort, rich filters, per-author pages (bio, per-book monitor toggles), book detail pages (editions, files, per-book history/search) | Single authors+books split view, text/format filter, 80-row cap; no author or book detail pages; no poster wall | The poster wall and author pages are the arr visual signature |
| 3 | Mass editor | Bulk-edit monitored/quality profile/root folder/tags across authors or books; Search Selected/Filtered | Absent (bulk ops exist only for metadata reviews, import reviews, downloads) | |
| 4 | Cutoff Unmet | Dedicated Wanted tab listing books whose file quality is below profile cutoff, searchable in bulk | Upgrade search exists (worker + toolbar action) but there is no view of which books are upgrade candidates | Backend scoring (`cutoffScore`, `upgradeAllowed`) already exists |
| 5 | Blocklist | Failed/manually-failed releases are permanently skipped; blocklist management UI; "mark failed" from history | No blocklist anywhere; failed-download recovery retries and can replace, but nothing prevents re-grabbing a known-bad release; compat blocklist routes return stubs | Real correctness gap for automation loops |
| 6 | Multiple root folders | Named root folders with per-root defaults (monitor mode, quality/metadata profile, tags), Calibre-managed roots, add-time root selection | Fixed ebook root + audiobook root from settings | Per-root defaults matter for mixed libraries |
| 7 | Monitor modes | All / Future / Missing / Existing / First / Latest / None per author, chosen at add time | All / Future / None on author subscriptions | |
| 8 | Search on add | "Start search for missing books" toggle in the add dialog | Not offered; search happens via monitor runs or manual per-item search | Small change, big familiarity win |
| 9 | Import lists | Goodreads shelves/other users, exclusions, auto-add with monitor/search/root/profile/tag options, 24h sync | Compat schema stub only (`ReadarrImportList`); no UI, no sync | Goodreads itself is out of scope by product rule; Hardcover lists would be the native analog |
| 10 | Metadata profiles | Filters which books get added per author: min popularity, min pages, skip missing-date / no-ISBN / part-books / secondary series, allowed languages, must-not-contain | Absent; author monitoring routes new books to a review queue instead | Librarry's review queue is the intentional alternative, but bulk-filtering knobs would cut review noise |
| 11 | Delay profiles | Usenet/torrent grab delays with tags, bypass-on-best-quality; pending state visible in queue | Absent; compat delayprofile routes are stubs | |

## Tier 2 — settings and depth gaps

| # | Area | Readarr | Librarry | Notes |
|---|---|---|---|---|
| 12 | File naming | Rich token system (subtitle/series/position/year/quality/media-info/release-group), space+case dropdowns, rename toggle, illegal-char handling, naming previews for both folder and file | `{Author}/{Title}/{Format}/{Ext}` + space replacement, live preview | |
| 13 | Rename/organize | Preview + execute rename of existing library files from UI (and compat `/api/v1/rename`) | Backend endpoints exist (`POST /api/v1/library/files/rename[/preview]`) but NO UI calls them — phantom backend surface | Ledger item: expose in Library or Media Management |
| 14 | Recycle bin | Deleted files go to a recycle folder with scheduled cleanup | Absent; deletes are deletes | |
| 15 | Propers/repacks | Prefer-and-upgrade policy setting | Absent (no proper/repack concept in scoring) | Low priority for books |
| 16 | Import extras | Import extra files (`.cue` etc.) alongside book files | Absent | Matters for audiobook chapters |
| 17 | File mgmt knobs | Watch root folders, rescan-after-refresh policy, fingerprinting, change-file-date, skip-free-space, min-free-space, chmod/chown | Absent | |
| 18 | Remote path mappings | Download-client/Calibre path translation table | Absent | Needed when client and Librarry see different mounts; today paths must match |
| 19 | Indexer management | Multiple indexers with per-indexer RSS/auto/interactive toggles, categories, priority, seed ratio/time goals, min age/retention/max size, RSS interval setting (10–120 min) | Single Prowlarr connection (by design); RSS/feed interval is env-only; no per-indexer controls or seed goals | Prowlarr-only is intentional; per-indexer seed goals and a UI for sync interval are real gaps |
| 20 | Download clients | Multiple clients with priorities, per-client categories, post-import category, initial state, remove-after-seed-goal, CDH/failed-handling toggles in UI | One client per type; auto-import now on (worker); no remove-completed-after-seeding, no priorities, no post-import category; automation toggles are env-only | Remove-after-seeding closes the torrent lifecycle |
| 21 | Notifications | Many providers (Discord, Telegram, email, …), rich trigger set incl. rename/delete/retag/health, test button, UI management | Webhook only via compat API; no UI; background workers do not dispatch webhooks at all | |
| 22 | Tags | First-class tags linking authors/books ↔ indexers, release profiles, import lists, delay profiles | Wanted items store tags (ints) but no tag UI or linking semantics | |
| 23 | Auth & security | Forms/basic auth in-app, API key regen UI, cert validation, proxy settings | `LIBRARRY_API_KEY` env + browser-stored key; no login UI | Already flagged in status.md for WAN exposure |
| 24 | Backups | Scheduled zip backups + restore UI | Compat stub returns `[]`; no backups | Postgres dump guidance exists in deploy docs instead |
| 25 | System depth | Tasks page (schedules, next/last run, manual trigger), Events, Log files viewer, disk space, update channel, broad health-check battery | Setup checklist + provider/integration health + compat report; worker schedules invisible in UI | A Tasks view showing worker last/next runs would close most of this |
| 26 | Quality definitions | Editable per-quality size limit table (MB/min for audio) | Score-based profiles with max size + terms | Different model (deliberate), but there is no per-format size definition |
| 27 | Interactive pickers | Filesystem browser for paths, root folder picker | Text inputs for all paths | |

## Intentional divergences (do not "fix" without a decision)

- **Metadata sources.** Readarr died with its Goodreads-derived metadata.
  Librarry's Hardcover + Open Library + Google Books with provenance and
  manual-override-wins is the founding design (AGENTS.md); no Goodreads/
  Amazon/Audible scraping in core.
- **Prowlarr-only indexers.** Librarry delegates indexer fan-out to Prowlarr
  rather than reimplementing Newznab/Torznab management.
- **Review-first automation.** Auto-grab defaults off everywhere
  (`LIBRARRY_*_AUTO_GRAB=false`) per product rule; Readarr auto-grabs RSS
  matches by default. Flip only as an explicit product decision.
- **Score-based quality model.** One profile type (min/cutoff/preferred score,
  terms, seeders, size) instead of Readarr's quality-ladder + release
  profiles + delay profiles trio.
- **Download-client depth.** Librarry exceeds Readarr here (tracker/peer/file
  management, categories/tags, client preferences) — deliberately scoped to
  book acquisition.

## Where Librarry is ahead of Readarr

Metadata provenance and per-field correction UI with provider records;
explainable release decisions (visible rejection reasons); acquisition queue
with per-book next actions; import review queue with candidate matching;
Readarr-compatible API layer for ecosystem tools; modern responsive UI;
auto-import with hardlink-or-copy (parity as of 2026-07-01).

## Suggested phase-2 ordering (parity-driven)

Superseded by the full execution plan in [parity-plan.md](parity-plan.md)
(six milestones: automation correctness → familiarity wins → library
experience → roots & files → operability → long tail).
