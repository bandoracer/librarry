# Imports and recovery

Review completed payloads, import files, rename verified sets and recover interrupted native or Calibre operations. Preview destinations before applying reviewed changes.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Calibre Conversion Refresh](#calibre-conversion-refresh)
- [Completed Download Handling](#completed-download-handling)
- [Inspecting interrupted imports](#inspecting-interrupted-imports)
- [Replace reviewed completed-download destinations](#replace-reviewed-completed-download-destinations)
- [Recover a standalone file rename](#recover-a-standalone-file-rename)
- [Rename a complete recorded book folder](#rename-a-complete-recorded-book-folder)
- [Qualify the Calibre HTTP client against a real server](#qualify-the-calibre-http-client-against-a-real-server)
- [Recover a Calibre handoff](#recover-a-calibre-handoff)
- [Browsing import reviews](#browsing-import-reviews)
- [Choosing books for imports](#choosing-books-for-imports)

## Calibre Conversion Refresh

Calibre conversion status polling can run on an interval when database
persistence and Calibre root-folder settings are available:

```dotenv
LIBRARRY_CALIBRE_REFRESH_ENABLED=true
LIBRARRY_CALIBRE_REFRESH_INTERVAL=15m
LIBRARRY_CALIBRE_REFRESH_LIMIT=200
LIBRARRY_CALIBRE_REFRESH_MAX_ATTEMPTS=1
```

Manual refreshes are available through
`POST /api/v1/library/calibre/conversions/refresh` or the Readarr-compatible
`RefreshCalibreConversions` command. Scheduled refreshes poll stored conversion
jobs and update completed or failed conversion metadata; they do not start new
conversion jobs.

## Completed Download Handling

Completed librarry-tagged downloads import automatically, matching arr
completed-download handling. Downloads linked to a wanted item import
directly; unlinked files are auto-matched against wanted metadata and fall
back to the import review queue. The worker requires database persistence and
runs every minute by default:

```dotenv
LIBRARRY_COMPLETED_IMPORT_ENABLED=true
LIBRARRY_COMPLETED_IMPORT_INTERVAL=1m
LIBRARRY_COMPLETED_IMPORT_LIMIT=50
LIBRARRY_COMPLETED_IMPORT_MODE=hardlinkOrCopy
LIBRARRY_COMPLETED_REMOVE_ENABLED=true
```

`hardlinkOrCopy` keeps torrents seeding without duplicating disk space when
the torrent root and library roots share a filesystem, and falls back to copy
when they do not. Set the mode to `copy` or `move` to override. The manual
"Import Completed" action in the Activity queue remains available for
immediate imports.

`LIBRARRY_COMPLETED_REMOVE_ENABLED=true` (arr parity, default on) deletes an
imported download — with its data — once the client reports seeding has
finished (qBittorrent `stoppedUP`/`pausedUP`, Transmission stopped and done).
Imports use hardlink-or-copy, so the library copy survives. Set it to `false`
to leave finished torrents in the client.

## Inspecting interrupted imports

On the stabilization branch, Library Import → Import recovery shows saved native
completed-download plans, their source/destination files, attempts, failure reason,
and cleanup state. Retry import resumes that exact plan. Changing naming or mode
settings does not rewrite an in-flight plan. Changed bytes, missing sidecars, or a
removed book stop the retry and retain the original download.

The recovery API is `GET /api/v1/library/import-recovery`; retry is
`POST /api/v1/library/import-operations/{id}/retry`. Legacy link issues are visible
but require an explicit metadata correction; they never gain verification merely
from migration. Each collection has independent Previous/Next controls and exact
matching totals. Use “Show only unfinished imports and Calibre handoffs” to omit
completed work; pending manual/replacement cleanup remains included. Legacy issues
always show unresolved links. Return to first pages to see newly created records.

The endpoint accepts `limit=1..100` (default 100), `unfinishedOnly=true|false`, and
independent `operationsCursor`, `calibreCursor`, `issuesCursor` values from each
collection's `nextCursor`. Cursors are opaque and bound to collection/filter;
reset them when changing the filter. Records sort by creation time and identity,
so heartbeats and retries do not move them between pages. One response uses a
consistent database snapshot; consecutive pages reflect live changes, not a frozen
export. Resolved/deleted rows may disappear and newer records appear on page one.

Native recovery shows observed time, last recorded activity, verified manifest-file
count and lease purpose/state. A held lease is ownership evidence, not byte
progress. An expired lease does not prove the old process has stopped. Large files
can take time between journal updates. The UI disables transfer retry while the
snapshot has a held lease; the backend always rechecks ownership and saved bytes.
After commit, a lease for pending local cleanup is labeled as a cleanup lease;
retry stays disabled while it is held. Finished work has no active lease purpose. Reading recovery does not probe files or clients.

Completed imports support copy, hardlink and hardlinkOrCopy. For destination
conflicts keep both is the default; reviewed replacement requires a current preview.
Identifiable chapter sets import together with their sidecars. Uncertain sets and
multi-book packs appear above the import table as a complete file list. Assign
books per file (or apply one book to all), explicitly retain unwanted files,
confirm the assignments, then preview destinations and import. Editing assignments
invalidates the preview. A server-side content/path change also requires a new
preview. Required retained files block automatic source deletion.

Skip/Reject stop automatic import of that client/download without deleting files.
Use Resolved → Reopen review to reconsider. Reopening returns to manual review;
the next worker run does not silently import it. Native manual imports share the
durable execution engine; remote Calibre handoff recovery remains separate work.


Native completed-import recovery records temporary copies before writing them.
Retry reclaims an interrupted operation's recorded stage before resuming its
manifest. The recovery panel shows these temporary paths when present. Files
without journal ownership are retained, including older `.librarry-copy-*` and
`.librarry-import-*` leftovers; no directory-wide cleanup is performed.


Manual file import also creates an operation, with an optional wanted book.
Copy/hardlink/move retries reuse that operation rather than creating another
renamed copy. Move removes sources only after the manifest and records commit.
Configured same-basename extras are required members, so sidecar failures remain
visible. CUE/playlist sidecars preserve the media basename to keep references valid.

For manual Replace, the original remains at a recorded recovery path until the
new file is verified and committed. If a configured recycle bin is unavailable,
cleanup reports an error and retains that previous file. Repair the bin and use
Imports → Import recovery → Retry cleanup. A committed manual copy with completed
cleanup still retains its source; the UI distinguishes this from a completed move.

## Replace reviewed completed-download destinations

In a pending file-set review, choose **Replace reviewed files** under **Existing
destinations**, confirm the book assignments, then preview again. The preview
identifies existing files to replace and is bound to their current content.
Changing the choice invalidates the preview. Keep both is the default.

The old file bytes are retained at recorded recovery paths until the complete new
set commits. **Import recovery** separately reports **Replacement backups** and
source cleanup. A recycle-folder failure keeps backups and exposes **Retry
cleanup**. Cleaning a replacement backup does not remove the download's source.

A different book's assigned file or Calibre-managed file cannot be replaced by
this flow. An existing book directory with extra old chapters/unknown files outside
the new manifest requires review or Keep both; full old-layout retirement is not
yet implemented. Matching chapter/sidecar replacements preserve tracked IDs and
manual names/notes. Historical manifests continue describing their original bytes.

## Recover a standalone file rename

Rename Files shows the reviewed source/destination pair and binds Apply to that
preview. Refresh preview after a change warning. A failure after planning exposes
its saved operation in Imports → Import recovery as **Saved file rename**. Use
**Retry rename** for unfinished publication or **Retry cleanup** once the new path
is committed. Retrying uses the captured destination even if naming settings have
changed. The original remains until the verified copy and the existing file row
commit; a pending old source is excluded from scan discovery.

The file ID, owner names/notes, book/download links and original import provenance
are preserved. An old import retry can return the verified renamed location.
Missing files, changed bytes, a conflicting destination or an ownership change
remain errors requiring review; a retry does not overwrite unrelated bytes.

Chapter sets and companion files show **Retained** with a reason directing you
to the book's **Rename book folder** action. Calibre-managed files remain under
Calibre's control. Per-file selection applies only to the displayed page; there
is no unattended collection-wide rename job.

## Rename a complete recorded book folder

Open a book and choose **Rename book folder**. Preview checks its latest complete
committed import against every linked file, current bytes, companion references
and naming settings. It shows the source and destination folders and pages through
the entire captured set. Apply moves **every file in this plan**, including rows
on other preview pages. The target folder follows the book's current metadata and
selected library root; basenames and relative disc paths stay unchanged.

CUE, M3U/M3U8 and local OPF references must resolve to recorded members within the
book folder. Unsupported encodings, absolute/escaping or missing file references,
unrecorded files, shared ownership and incomplete/legacy sets retain the folder
for review. This action does not rewrite chapters or playlists to new names.
Calibre owns its own layout. A verified scan move within the recorded book folder
can be retained, provided the current companion references are still valid.

Apply requires the current preview revision. If interrupted, refresh and choose
**Resume complete book rename**, or find **Saved book folder rename** in Imports.
Recovery keeps the captured destination even after naming settings change. All
media paths become visible in one database commit; originals remain until then.
Partial source cleanup can resume after restart. Existing file IDs, manual
metadata, book/download associations and monitoring remain unchanged.

## Qualify the Calibre HTTP client against a real server

The client supports Calibre's default Digest authentication on HTTP and explicit
Basic authentication. Configure the root's username/password normally; the client
first reads the server's authentication challenge from library-info. Redirects
are rejected, so configure the final server URL and prefix. Credentials belong
in the root fields, not in the host URL or query string.

The following fixture creates a new authenticated Calibre library in a disposable
container, uploads the repository's legal EPUB, updates metadata, converts to TXT,
deletes the created book and verifies that the library is empty. It does not use
homelab paths, accounts or media. `DOCKER_CONTEXT` is optional and scopes this
script without changing the global Docker context.

```sh
docker build -f scripts/fixtures/calibre/Dockerfile -t librarry-calibre:fixture .
python3 scripts/test-calibre.py
LIBRARRY_CALIBRE_AUTH_MODE=basic python3 scripts/test-calibre.py
```

The fixture prints its actual Calibre version. September local qualification used
Debian's Calibre 8.5 on ARM64. Ordinary Go test runs skip the disposable-server
case; CI builds the fixture and tests both modes. The contract verifies protocol
identities and actual remote effects. Set `LIBRARRY_TEST_DATABASE_URL` to the
disposable Postgres instance to also run real handoff recovery through fresh
service instances after metadata failure and lost upload acknowledgement. CI
runs both client and handoff tests. Calibre-managed sources do not earn download
cleanup eligibility.

## Recover a Calibre handoff

Open **Imports → Calibre handoffs**. An accepted book ID stays attached to the
saved original root across retry and restart. **Retry Calibre handoff** resumes
metadata syncing, known conversion jobs and local bookkeeping. Running conversions
also resume through the configured conversion background task. Disabling that
task requires timely manual checks; expired server jobs need review.

For an uncertain upload, inspect the original Calibre library first. Confirm that
the earlier request has stopped, then enter the existing Calibre book ID and choose
**Attach existing book**. Librarry verifies the ID and source format; you must
verify that it is the right book. If you verified that the upload is absent,
**Confirmed absent: allow another upload** permits a new send. A similar decision
is required for uncertain/failed conversions: verify an existing output format or
allow another conversion. After saving a decision, retry the handoff.

Recovery never silently changes servers or deletes the retained source. Restore
the original root configuration if the target has changed; password rotation is
allowed. Changes to the wanted destination or file associations require resolving
those owner settings before retry. These controls apply to new journal-backed
handoffs; old IDs and external Calibre file moves are not automatically repaired.

## Browsing import reviews

Imports → Import reviews uses pages of 50 reviews with complete matching/total
counts. Search title, author, source path or reason; filter by Pending/Resolved/All,
format, and file matches versus download payloads. Resolved includes imported,
skipped and rejected records; skipped/rejected payload reviews retain their
existing Reopen action. Pending payloads use the same page controls as file reviews.

Select page selects pending file rows only. Page or filter changes clear selection
and temporary match choices. Bulk actions send the selected IDs, never the entire
matching queue. Payloads still require individual file assignments and a current
preview. Changing pages discards unsubmitted payload edits; return and preview again.
Newly created reviews appear on the first page; retries do not change creation order.
Counts are a database snapshot per response, not a frozen export across pages.
Failed reads show an error and retry; First review page remains available to recover
from an obsolete cursor.

## Choosing books for imports

The manual-import and payload-file book selectors show 50 choices per page. Open
**Search and browse books** under a selector to search title, author or saved book
ID and reach older pages. Choices use saved local identities and owner-edited
labels; there is no provider or download-client request. Imported and unmonitored
active books remain selectable for explicit replacement/repair, while removed and
ignored books are omitted.

The current selection stays pinned even when it is outside a search or page.
Search/paging does not change the intended book. Changing the manual format clears
its old selection. If a selected identity is no longer active or no longer matches
the file format, it is shown as unavailable, not silently replaced by another book.
Database errors offer Retry books and retain the selected ID. Payload assignments,
retained-download choices and current-preview requirements remain unchanged.
