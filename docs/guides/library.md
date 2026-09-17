# Library management

Manage roots, scan files, interpret recorded book presence, browse the library and restore removed tracking.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Library Roots](#library-roots)
- [Multiple root folders](#multiple-root-folders)
- [Calibre-managed root folders](#calibre-managed-root-folders)
- [Remote path mappings](#remote-path-mappings)
- [Recycle bin](#recycle-bin)
- [Import extras](#import-extras)
- [Resumable library scans](#resumable-library-scans)
- [Preview legacy library repairs](#preview-legacy-library-repairs)
- [Reattach moved native files](#reattach-moved-native-files)
- [Book status evidence](#book-status-evidence)
- [Full book collection browsing](#full-book-collection-browsing)
- [Browsing library files](#browsing-library-files)
- [Removed and ignored books](#removed-and-ignored-books)

## Library Roots

Library scans and manual imports use these roots by default:

```dotenv
LIBRARRY_EBOOK_LIBRARY_ROOT=/data/media/books/ebooks
LIBRARRY_AUDIOBOOK_LIBRARY_ROOT=/data/media/books/audiobooks
LIBRARRY_NAMING_AUTHOR_FOLDER={Author}
LIBRARRY_NAMING_BOOK_FOLDER={Title}
LIBRARRY_NAMING_FILE_NAME={Title}{Ext}
LIBRARRY_NAMING_SPACE_REPLACEMENT=
LIBRARRY_RENAME_BOOKS=true
LIBRARRY_STANDARD_SEARCH_LANGUAGE=English
LIBRARRY_RECYCLE_BIN=
LIBRARRY_RECYCLE_BIN_RETENTION=168h
LIBRARRY_IMPORT_EXTRA_FILES=.cue
```

Naming templates accept `{Author}`, `{Title}`, `{Series}`, `{SeriesPosition}`,
`{Year}` (first published), `{Format}`, and `{Ext}`. Tokens with no value
collapse cleanly — separators and brackets that only decorated an empty token
are removed, so `{Series} #{SeriesPosition} - {Title}` renders as just the
title for standalone books. Series and year come from the wanted item's
provider metadata (manual overrides win) or embedded file metadata during
scans and renames.

`LIBRARRY_RENAME_BOOKS` controls whether imports apply the naming templates.
Librarry defaults to `true` (note Readarr ships with renaming off). With
renaming off, imports still land in the author folder but keep the original
source filename — the book folder and file-name templates are skipped. The
toggle is exposed as `renameBooks` on `GET`/`PUT /api/v1/library/config`
(omitting the key on `PUT` leaves the stored value alone), and explicit
rename operations through `POST /api/v1/library/files/rename` keep using the
templates regardless.

## Multiple root folders

The two env roots above are the seed values for the native multi-root model.
`GET /api/v1/library/root-folders` lists root folders (auto-seeding "Ebooks"
and "Audiobooks" rows from the configured legacy roots the first time it runs
against an empty table) and returns `{"rootFolders": [...]}` with
`accessible` and `freeSpaceBytes` computed per path. `POST
/api/v1/library/root-folders` creates a root (`name`, `path`, `mediaFormat` of
`ebook`/`audiobook`, optional `defaultQualityProfile`,
`defaultMissingBookPolicy`, `defaultTags`, `isDefault`); `PUT
/api/v1/library/root-folders/{id}` updates one, and `DELETE
/api/v1/library/root-folders/{id}` removes one. Deleting the last root of a
format while tracked files still live under it is refused with `409` and a
reason. Marking a root `isDefault` clears the flag from other roots of the
same format.

Scans walk every root of the requested format. Imports pick the wanted item's
pinned root when set (`rootFolderId` on `PUT /api/v1/wanted/{id}`, or at add
time via `rootFolderId` on `POST /api/v1/wanted` — validated to exist and to
match the wanted format, `400` otherwise), then the format's default root,
then the legacy config roots. `GET`/`PUT /api/v1/library/config` keep
working: the legacy two-root fields map onto the per-format default root
folders.

## Calibre-managed root folders

Native root folders carry an optional `calibre` object on `GET`/`POST`/`PUT`:
`{enabled,host,port,urlBase,username,password,library,convertFormats,
outputProfile,useSsl}` (migration 0029). The Calibre content server requires
authentication, so `enabled:true` needs `host`, `port`, `username`, and
`password` (`400` otherwise). The password is redacted to `""` in responses;
sending a blank password on `PUT` keeps the stored credential
(notification-target pattern).

Imports whose destination resolves to a Calibre-managed root skip the
move/hardlink+naming path entirely: the source file is posted to the root's
Calibre Content Server (add-book, metadata set-fields, and conversion jobs
for `convertFormats`). Calibre reports only the new book id — never a library
path — so the tracked file keeps its source path with import status
`calibre`. Rename endpoints skip files under Calibre-managed roots; the
preview row carries `reason: "managed by Calibre"`. The older
Readarr-compatible `isCalibreLibrary` root-folder metadata flow below keeps
working unchanged.

## Remote path mappings

When a download client reports paths Librarry cannot reach (split hosts,
different Docker mounts), add a remote path mapping: `GET
/api/v1/library/remote-path-mappings` returns `{"mappings": [...]}`, with
`POST`, `PUT /{id}`, and `DELETE /{id}` for CRUD. Each mapping has `host`
(download client name, empty matches every client), `remotePrefix`, and
`localPrefix`. The longest matching remote prefix wins and is applied as a
dumb prefix rewrite before completed-download import reads the client's save
path.

## Recycle bin

Set `LIBRARRY_RECYCLE_BIN` to a folder to keep deleted or replaced library
files instead of removing them: files move into
`<bin>/<yyyy-mm-dd>/<original-name>` (falling back to copy+delete across
filesystems, and to a plain delete when the bin is unusable). Day folders
older than `LIBRARRY_RECYCLE_BIN_RETENTION` (default `168h`) are purged during
the completed-download-import worker tick. The active bin path is surfaced as
`recycleBin` in `GET /api/v1/library/config`.

## Import extras

`LIBRARRY_IMPORT_EXTRA_FILES` (comma-separated extensions, default `.cue`)
copies sibling files that share the imported source's basename into the
destination folder, renamed to match the organized file (audiobook cue sheets
survive imports). Extras are best-effort: they are not tracked and failures
only log at debug.

`POST /api/v1/library/scan` indexes existing files. `POST /api/v1/library/import`
imports a single source file into the organized format root using `importMode`
values of `copy`, `move`, `hardlink`, or `hardlinkOrCopy`.
`conflictAction` controls duplicate destinations with `rename`/keep-both,
`replace`/overwrite, `skip`, or `fail`; `overwrite=true` is treated as
`replace`.
`POST /api/v1/library/import-completed` imports completed Librarry-tagged
downloads into the same organized roots when they are linked to a
wanted item. Unlinked completed downloads are queued in
`GET /api/v1/library/import-reviews` and resolved through
`POST /api/v1/library/import-reviews/{id}/resolve` or
`POST /api/v1/library/import-reviews/resolve-bulk` with the same import mode and
conflict policy fields. Review rows include confidence and evidence metadata
from filename parsing, OPF/embedded/audio tags, source file facts, and download
context so operators can see why manual review is required. OPF sidecars and embedded
EPUB package metadata plus MP3 ID3 and M4B/MP4 audio tags are extracted during
scan/import and used for title, author, identifiers, language, publisher,
series, album, year, and track evidence before falling back to filename parsing.
Readarr-compatible `/api/v1/retag` previews and applies title, author, language,
and quality tag state on tracked file records for compatibility clients; it does
not rewrite embedded EPUB, MP3, or M4B metadata yet.
`LIBRARRY_STANDARD_SEARCH_LANGUAGE` defaults provider metadata search, manual
release search, and wanted-item release search to English. Operators can change
the persisted preference from Settings; explicit wanted-item language overrides
still win for release scoring.

If the destination path is inside a Readarr-compatible root folder with
`isCalibreLibrary=true`, Librarry also posts the imported file to the configured
Calibre Content Server add-book endpoint and stores the returned Calibre ID on
the file metadata. It then pushes basic title, author, and identifier metadata
through the Content Server set-fields endpoint. If `outputFormat` is configured
on the root folder, Librarry starts Calibre conversion jobs for missing target
formats and captures an immediate status snapshot. Stored conversion jobs can be
refreshed with `POST /api/v1/library/calibre/conversions/refresh` or the
Readarr-compatible `RefreshCalibreConversions` command, and the API process can
poll those jobs on an interval with `LIBRARRY_CALIBRE_REFRESH_ENABLED=true`.
Physical deletion of a Calibre-backed file calls the Content Server
delete-books endpoint before removing the local file record. Richer edition
metadata, embedded metadata writes, path refresh after Calibre renames, and
general Calibre-server rollback are not implemented. Native file rollback and
[Calibre handoff recovery](imports.md#recover-a-calibre-handoff) have separate
contracts; a recorded remote acceptance is not automatically undone.

## Resumable library scans

The UI starts a job promptly with POST `/api/v1/library/scans` (202 Accepted),
then polls saved progress while the scheduler performs all file work.

POST `/api/v1/library/scan` still accepts `root`, `format`, and `limit`. Its response
now includes `jobId`, `state`, `phase`, `hasMore`, and `missing`. `limit` controls
only the first batch (default 1,000, maximum 5,000); it no longer truncates the
entire scan. A persisted scheduler task advances user-created jobs in 500-entry
batches every five seconds. It does not initiate new scans on its own. Closing the
browser does not stop a saved job; use **Cancel scan** in Imports.

GET `/api/v1/library/scans` returns up to 100 jobs, with unfinished/failed jobs
first. POST `/api/v1/library/scans/{id}` accepts `action: cancel` or `action: retry`.
Retry resumes the saved path queue; cancelled jobs require a new scan. Files added
or deleted while walking are reconciled conservatively, and interruptions retain
progress. Missing observations are staged until successful completion and checked
again before publication. No failed/cancelled scan publishes partial missing-file
changes. Finished jobs retain summary/root evidence and discard their path queues.

Root directory device/inode identity is checked across batches and successful
scans. If a root has changed, first verify the intended library is actually mounted.
The custom-root form exposes a replacement-folder acknowledgement after that
error; the equivalent API field is `acceptRootChange: true`. A root changing during
a running job cannot be accepted in that job. Start a new scan after inspection.
Nested device mismatches block missing-file conclusions. Root identity is currently
implemented on Unix (qualified on macOS/Linux); unsupported platforms fail clearly.

File `presenceState` is `unknown`, `present`, or `missing`, separate from import
history. Older records begin unknown; a scan must first observe a file before a
later scan can mark it missing. Native verified imports mark their destinations
present. Imports displays missing local files explicitly. Native book evidence,
moved-file reconciliation and repair previews are described below; compatible-API
parity remains outstanding. Scans never fuzzy-reassign a file automatically.

## Preview legacy library repairs

In Imports, choose **Preview library repairs**, then **Continue report** until
all pages are checked. Clean pages still have to be continued. A failed page can
be retried without discarding earlier findings; **Start a fresh report** reruns
from the beginning. The report shows why a record needs review and recommends an
action. It makes no changes.

A duplicate recorded hash is not permission to delete a file. Verify fresh bytes,
intentional copies/hardlinks and book assignments first. A legacy audiobook marked
imported can still lack evidence that every chapter arrived; single-file books
can be valid. Compare the original client inventory to actual destinations.
Possible move candidates require fresh path/content verification before the old
file identity is reattached. Deliberate moves and replacements can explain a
historical import-manifest discrepancy; keep that original history intact.

The report inspects saved database evidence rather than current media or client
inventories. Related-record samples show up to 20 paths with full candidate
counts. Each page is a current observation; start a fresh report after other
library changes. The preview does not apply repairs. Completed scans can perform the narrowly
verified move reattachment described below.

## Reattach moved native files

Scan all involved roots after moving files outside Librarry. A successful complete
scan can reattach the original file ID when it confirms the old path is absent,
finds exactly one matching new location and the new scan-created record has no
manual changes, book/download associations or import references. Copies with
identical recorded content make the match ambiguous and remain in repair preview.
Both paths must belong to this scan's roots. Calibre-managed records and roots
stay under Calibre review.

**Library scans** shows the reattached count. **View reattached files** shows the
old/new paths and retained IDs, with **Load more reattached files** for longer
history. Reconciliation changes database paths, not media on disk. The original
manual names, associations and import history remain intact; an old cleanup
receipt cannot authorize deletion merely because the file was reattached.

A destination changed after it was hashed requires a fresh scan. Failed database
completion can be resumed; cancellation leaves original identities unchanged.
Records created before discovery evidence was introduced are retained for manual
review rather than guessed into an existing book. Book presence uses the recorded
evidence described next. Live rollout observation remains separate qualification.

## Book status evidence

Native wanted/library/detail responses include `stateEvidence`. `files.state` is
present, missing, incomplete, unknown or unavailable. Download evidence is fresh,
partial, unavailable or notConfigured. An ebook needs recorded present media; an
audiobook needs a complete matching committed media manifest. A partial chapter
loss is Incomplete; legacy audiobook files without a manifest are Unknown, even
if their old lifecycle says imported. Run a scan to update observations; do not
change lifecycle rows to manufacture completeness. A rename with retained IDs and
matching content preserves presence. Restoring a missing chapter and successfully
scanning restores completeness if its manifest identity still matches.

Book details explain unavailable evidence. A client outage never establishes that
a book is missing or resurrects stale download rows; known positive files retain
their presence with a warning. A completed, already-imported source still seeding
in the client does not hide later library damage. These are recorded import/scan
observations, not a live mount health check. Scanning an unavailable root retains
prior presence. Sidecar damage continues to block cleanup verification separately.

Wanted includes Incomplete/Unknown filters over the full paged collection.
Readarr-compatible book reads share recorded presence evidence. Author file-based
monitoring policies retain their separate association-based behavior.
The native status path may spend up to five seconds on live client requests;
the page shows recorded evidence rather than a fresh scan of every file.

## Full book collection browsing

Library and Wanted's Missing, Incomplete, Unknown and Cutoff Unmet tabs use native
server pages of up to 100 books. Filters/sorts apply to the full tracked collection;
counts no longer mean just the first loaded 200 books. Previous/Next navigate
pages; Refresh page refreshes the current page. Changes while browsing can move
books, so return to the first page to restart a traversal after edits.

Selections clear on page/filter/tab changes. Select shown, mass edits, selected
search and selected upgrade actions affect only the current selected page.
Scheduled batch buttons remain separate 50-book actions. Metadata Review now
uses its own paged collection. Native file browsing is paged separately, as
described next; broader compatibility resources and collection-wide jobs retain
separate scaling work.

## Browsing library files

Imports and each book detail show Tracked files with up to 100 rows per page.
Search covers paths, titles and authors across the full collection. Format,
recorded presence and path/title/recent-update sort filters reset to page one.
Previous files, Next files and Refresh files navigate the collection. Counters
cover the full collection, or all files linked to the current book; they do not
shrink to the loaded page. An unavailable collection offers Retry files and
retains no misleading empty-list claim or stale Imports totals.

Presence means the last recorded scan/import observation, not a fresh disk
check. Unknown is explicit. Import status is separate: an imported file can be
missing locally. Book completeness still uses the full native import/file evidence,
not whichever chapter page is displayed. Open book follows recorded relational
associations; stale JSON hints do not define membership. Unassigned files remain
browsable. Destinations belonging to unfinished imports stay hidden until commit.

Rename Files remains available when the book list is empty. Its preview uses the
same file collection, with search, format and previous/next controls. Selection
and Apply cover only shown files. Changed files are initially selected on each
page; review the paths and uncheck any to exclude before applying. Collection-wide
resumable rename/bulk jobs remain separate work. Concurrent changes can affect
subsequent pages; Refresh preview reloads the current page.

## Removed and ignored books

Open Library → Removed to find inactive records, including books older than the
former list limits. Search title, author or saved book ID, filter format/status,
and use the page controls. Counts cover every inactive record; pages are ordered
by creation time, so edits do not move records. Last updated is not a removal date.
Use the book link to inspect existing files and metadata evidence.

Restore opens a review of the saved record. Monitoring is unchecked by default;
enabling it permits scheduled acquisition under the existing automation settings.
Restore itself neither searches nor grabs. Root/profile/tags, metadata overrides,
author policy, file links and prior history stay unchanged. It does not recover
physically deleted files. A concurrent edit, another restore or a new removal
rejects the old review. Use Reload book, inspect the new settings and submit again.
A successful restore writes one history event in the same transaction. If the
response is interrupted, refresh the list/details before retrying.

Direct links to inactive books show their status and the same explicit Restore
flow. Their regular monitoring/edit/release controls return after restoration;
file and provenance inspection remain available.
