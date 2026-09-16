# Stabilization candidate qualification

Application source: `6e209ffd8d3724879e52187e27790029e66af986`.
Candidate PR: [#52](https://github.com/bandoracer/librarry/pull/52).
This continues the earlier focused review and browser walkthrough. Documentation
commits after this source do not change the qualified application or images.

The candidate passed the documented review and qualification scope and is ready
for a controlled rollout. Live rollout and the 72-hour/20-controlled-case
observation gate remain open. No stable release or unattended-use approval is
claimed.

## Broader risk review

The remaining stack received a risk-based review of implementation and its
regression tests. This is not an exhaustive line-by-line certification.

- Metadata matching, merge and caching: exact Google fallback, ISBN validation,
  language/format boundaries, provider IDs, immutable cached results and cache
  invalidation. Provider request budgets serialize shared hosts, bound retry
  delays and propagate cancellation.
- Author/list automation: complete provider pagination, fail-closed limits,
  partial-date handling, saved exclusions, manual overrides, stale review
  rejection and destination/monitoring preservation for already tracked books.
- Acquisition and upgrades: evidence-based eligibility, explicit selections,
  current settings revalidation, scheduling fairness and persisted uncertain
  acceptance. Empty or invalid upgrade selections do not broaden to all books.
- Library identity: file observation and move reconciliation, checksum/size and
  path guards, stable associations, retained removed-book records and deliberate
  monitoring choice on restoration.
- Calibre handoff: durable plans bind the source and target; upload receipts are
  recorded before conversion. Real disposable Calibre coverage remains separate
  from native filesystem import coverage.
- Workers and notifications: shared ownership locks, cancellation after lost
  ownership, interrupted history, retention of unreviewed failures, outbox claim
  locks, no automatic uncertain-delivery replay, target revisions and permanent
  source-event barriers.
- Operator surfaces: bounded paging, stale selection protection, saved recovery
  plans, diagnostic allowlists and retained failure explanations.

No additional backend blocker was found in this review. The browser walkthrough
found three UI defects, fixed in this candidate: removal text falsely implied
metadata loss; removing an author's last book navigated to a now-missing author;
manual magnet/upload errors hid uncertain-acceptance instructions behind HTTP 502.
The last fix adds JSON and multipart regression cases without automatic retries.

## Controlled browser journey

The application used a new disposable Postgres database, isolated media roots,
disabled acquisition workers and a simulated qBittorrent server. No indexer
release was downloaded. Open Library search used the real public service.

- Configured Forms authentication; incorrect credentials failed visibly, valid
  login worked, and the session/settings survived an API restart. Roots were
  preconfigured for isolation; this does not claim a new-install root wizard.
- Searched for Frankenstein in English, reviewed the Open Library provenance and
  low-confidence warning, and added a selected edition.
- Imported the repository's legal EPUB fixture with an explicit book assignment.
  Its content is fixture evidence, not a claim that it is the selected edition.
  A database trigger deliberately rejected file persistence. The failed import
  retained its source, recorded no file, and remained visible after restart.
  Retrying the saved operation through the UI committed one file and association;
  the source was removed only after verified commit, with matching bytes.
- Saved a title override, renamed the book folder, removed tracking and restored
  it. The file hash, association and overrides survived; restore left monitoring
  off. Repeating removal verified the corrected Removed Books destination.
- Submitted one controlled magnet to a fixture that accepted it but returned
  502. Activity showed an uncertain acquisition; Check Client reconciled it.
  The fixture's add count remained one, proving no second submission.
- Attempted a chapter import whose declared third chapter was absent. The app
  queued review and published no chapter files. After restoring the missing
  fixture bytes, the UI preview and reviewed import completed. All three chapter
  hashes match their sources, and the cover sidecar was retained. Chapter bytes
  are synthetic fixtures; this does not qualify audio decoding or playback.
- A second payload mixed those chapters with a foreign EPUB. Automatic import
  held it for review with explicit format/title/author conflicts. Selecting
  "Keep in downloads" excluded that EPUB from the destination preview. Rejecting
  the payload preserved its originals and left the tracked file count unchanged.
- Inspected Library at 1440×1000, 768×1024 and 390×844. The phone document stayed
  within its viewport; wide tables scroll inside their containers. Screenshots
  and browser snapshots are retained under ignored `output/playwright/`.
  Phone navigation moved focus into the drawer, Tab reached its links, and Escape
  closed it and restored focus to the opener. The long import review also stayed
  within the 390-pixel document width.

## Qualification boundary

Production rollout, reverse-proxy repair, live provider credentials, the planned
72-hour/20-controlled-case observation period, stable promotion and real Readarr
migration/consumer qualification remain separate gates. None is implied by a
successful fixture or production-copy rehearsal.

## Published artifact identity

[Explicit candidate run 35157133068](https://github.com/bandoracer/librarry/actions/runs/35157133068)
passed all six jobs: publication policy, Go/Postgres/frontend/browser/deployment
verification, disposable Calibre, packaged qualification and configured scan,
and both multiarchitecture image builds. The frontend suite passed 25 tests;
the browser suite contains 143 tests. The paired image identity artifacts record
the source commit, unique candidate tag, digest, run and attempt.

Tag: `candidate-6e209ffd8d3724879e52187e27790029e66af986-35157133068-1`.

| Image | Platform | Digest |
| --- | --- | --- |
| `ghcr.io/bandoracer/librarry-api` | Index | `sha256:5b42f0e33c465026e4105a740baba72202df606389e614d150ac1e8da2334cd2` |
| API | linux/amd64 | `sha256:46d511edfd43c33e72756a409418f9467e927b313213ca70b173085874e65820` |
| API | linux/arm64 | `sha256:5f40c473aa905e9101da27eebaeb6fc33486229d885b68b73fa6c9478bcea993` |
| `ghcr.io/bandoracer/librarry-web` | Index | `sha256:a8c9ea2acae6fa155c73715aa4be2e7b3ce8cb42dc7458795f16be9ce2cdf4db` |
| Web | linux/amd64 | `sha256:74b543307d16f93dc9f91dd1bb332265081c09090a1a82a17bc649c7c647c5d2` |
| Web | linux/arm64 | `sha256:dff138466570849469b8fc8681024d345dff10cf0addb9cb231503db8d5572b5` |

Both immutable index references were pulled and tested on native Linux ARM64
(local Colima VM) and native Linux AMD64 (isolated containers on the NAS).
The API reported the exact source commit and migration 58 on both platforms;
the API and web OCI source labels also matched the recorded commit.

On each architecture, all three packaged suites passed: application/import/
restore/authentication, two-process worker failover and notification uncertainty/
replay/retention. The test-only NAS launchers used sudo for Docker, and disposable
worker/notification cleanup removed their anonymous volumes; their assertions
and the application images were unchanged. No live integration credentials or
live media mounts were supplied to these fixtures.

The exact pulled API and web image bytes were exported and scanned on each
architecture with pinned Trivy 0.74.0. All four reports had zero fixable HIGH or
CRITICAL findings under the configured `--ignore-unfixed` policy. This statement
does not assert that lower-severity or unfixed vulnerabilities are absent.

Both `latest` digests matched before and after publication:

- API: `sha256:8c7d8ae7455207f59aac854b6d0e2c758f3737ea5a02bf0b84b587027be5460e`.
- Web: `sha256:85982d333defaae0e55ce45e7ffb2257e67c8a020d68ae80ddbb001e5aba2be6`.

## Production-copy upgrade and rollback

The live database was at `0029_calibre_roots.sql`, despite the old API's migration
status placeholder. A read-only custom-format database dump, all three tracked
media files and the original API/web images were copied for the rehearsal.
The sandbox used PostgreSQL 16 and an internal Docker network with no external
egress. Sessions/users were removed from the copy, import lists and notification
targets disabled, and live integration credentials excluded from its runtime.
The live containers, database and media were not changed.

The original-image baseline was healthy before upgrading. The published candidate
then migrated the copy to `0058_book_evidence_link_lookup.sql`. Every pre-existing
column value matched in these tables:

| Table | Rows preserved |
| --- | ---: |
| wanted_items | 6 |
| files | 3 |
| manual_overrides | 1 |
| provider_records | 23 |
| works | 6 |
| editions | 6 |
| downloads | 130 |
| root_folders | 2 |
| compat_resources | 1 |

The original database had no manual override; the one above was deliberately
added only to the sanitized copy before upgrade as a preservation probe. All
three backfilled relational file/book pairs matched the original metadata IDs.
All three copied media sizes and SHA-256 hashes remained identical.

Rollback restored the pre-upgrade snapshot and media copies, then booted the
original API/web bytes. A deliberate post-upgrade title mutation disappeared,
all nine table baselines matched again, all three hashes matched, the schema
returned to 0029, and the original web/API served version 0.4.0. Rollback used a
snapshot restore, not an in-place reverse migration.

Original running image configuration digests, independently verified against
the exported image archives:

- API: `sha256:7538da423044e6cb80e3948ff56265b8d54d145e3ea3874f7af21fe54d8d0aea`.
- Web: `sha256:4fad90f43378a089e5ef4ff28f6285e94d9cfb98eb699ceac9480e1c4fc44b82`.

The candidate ran natively on ARM64 for this rehearsal; the original AMD64 images
ran under local emulation. Separate native NAS fixture qualification covers the
candidate's AMD64 runtime. Private snapshots and local evidence are retained in
the restricted, ignored `output/release-review/production-copy/` directory.

## Closeout

Temporary browser/API/client/database services, upgrade containers and native
NAS qualification containers were removed. Private snapshot artifacts remain
restricted and ignored by Git. The original working checkout remains unchanged.
The live API/web container IDs, image IDs and start times match the pre-test
readback; live Postgres still runs with a start time predating qualification.
LAN `/healthz` returned 200; the Cosmos hostname still returned 502. Main was not
merged, the live application was not deployed, and no stable alias was promoted.
