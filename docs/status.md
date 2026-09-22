# Current status

Last verified: September 21, 2026. This is the authoritative readiness summary.
Feature guides describe candidate behavior; historical records are evidence of
past checkpoints, not current deployment claims.

Librarry is early alpha. The stabilization candidate is qualified for a
**controlled rollout**, now deployed on the maintainer NAS. It is not yet
qualified as an unattended replacement for
an existing Readarr installation.

## Metadata discovery rollout

The September 21 retrieval/edition fixes, evidence UI, edition chooser and bounded
series search are deployed on the maintainer NAS and promoted to `latest`. The application search pipeline places an intended result first for 49/50 diagnostic
queries and within ten for all 50. Bounded author rescue, returned-edition selection,
live read-only checks and desktop/mobile checks are recorded in the
[expanded iteration](reviews/2026-09-21-metadata-discovery-iteration.md).
An explicit Hardcover Series mode and translated-title author rescue are included; see [series and edge cases](reviews/2026-09-21-series-and-edge-cases.md).
The initial series query used a forbidden substring operator. The corrected
Series search followed by ID-based membership lookup passes a live Percy Jackson
probe, including ordered ebook/audio editions. Hardcover and Google Books now both
pass live authentication checks. Production Hardcover title search returns books
with covers; see [Hardcover setup](reviews/2026-09-21-hardcover-credentials.md) and
[Google credential setup](reviews/2026-09-21-google-books-credentials.md). Unconstrained
ambiguous titles, typo rescue, complete enumeration, independently judged coverage
and broad credentialed catalog qualification remain outstanding. Live title, author,
ebook-ISBN and Percy Jackson series probes passed, as did combined-provider search.
See the [release and preservation record](reviews/2026-09-21-metadata-rollout.md).

## Search-to-download simplification (source only)

The next UI change removes legacy confidence badges and routine add confirmations.
A selected book has a **Download ebook/audiobook** action that saves, searches,
and starts the best approved release; **Add Book** saves without an immediate
search. Options and provider evidence are expandable. Explicit edition conflicts,
missing authors, saved-identity checks and release approval remain enforced.
Verified locally with the full Go suite against disposable PostgreSQL, 32 frontend
tests, the production web build, deployment configuration checks and 38 targeted
desktop/mobile browser checks. Browser tests use intercepted acquisition responses;
no live downloads were submitted. This change is not yet deployed; the live
metadata rollout above remains current.

## Discovery cleanup (source only)

The next ranking pass addresses the live DDIA empty-record and typo/noise cases,
adds expandable related/incomplete sections, and preserves healthy Hardcover
records during partial edition validation failures. It also groups verified work
aliases while retaining selected edition identity. The 24-query credentialed live probe, 50-query captured benchmark, metadata race
tests, full Go suite and 42 desktop/mobile checks passed. Source-only evidence and
the remaining gaps are in the [cleanup review](reviews/2026-09-21-discovery-cleanup.md).

## Adaptation evidence (source only)

Provider categories, subjects/genres and edition abridgment evidence now support
concise graphic/abridgment labels and secondary placement when a coherent original
exists. Explicit comic searches, exact ISBNs and standalone graphic works retain
primary placement. See the [adaptation review](reviews/2026-09-21-adaptation-ranking.md).
This pass is awaiting packaging and deployment with the two changes above.

## Kindle delivery

Manual native EPUB/PDF delivery is deployed on the maintainer NAS. Settings,
a test document, and durable per-book history are available; see the
[Kindle guide](guides/kindle.md). The exact published API image passed isolated
Kindle checks on native AMD64 and ARM64. The configured Resend server accepted
one setup test on September 17 at 08:29 UTC. The owner subsequently corrected
the destination; that earlier acceptance does not verify the corrected address.
No test has been sent to the corrected destination, and Kindle receipt remains
unconfirmed. Uncertain attempts are never automatically retried.
[Implementation checks](reviews/2026-09-17-kindle-implementation.md) and
[rollout evidence](reviews/2026-09-17-kindle-rollout.md) record the boundaries.

## Source, images and deployment

| State | Verified position |
| --- | --- |
| Application candidate | `ca6c8432689b69545448d3cb30879d9bc7b08e0a`, merged through [PR #54](https://github.com/bandoracer/librarry/pull/54) at main commit `f23c674e99d4923a14705e882631f28c161fb2fa`. Later documentation-only commits do not change the runtime. |
| Published candidate | Paired AMD64/ARM64 API/web indexes from [run 35675737504](https://github.com/bandoracer/librarry/actions/runs/35675737504); immutable references are recorded in the [rollout](reviews/2026-09-21-metadata-rollout.md). |
| Latest aliases | Both point to this qualified pair, verified by [promotion 35677049125](https://github.com/bandoracer/librarry/actions/runs/35677049125). |
| Live homelab | Metadata candidate API/web digests, native AMD64, source `ca6c843`, database migration 0059. All three media hashes, file records, file/book links, manual overrides and migration records are unchanged. |
| Endpoints at last check | LAN `/healthz` and `/readyz`: 200. Hardcover and Google Books: ready and authenticated. Live browser: covers, edition switching, and numbered series results verified. |

Do not infer deployment from a merged commit or successful image build. Installer
defaults now select the promoted `latest` pair. Use both recorded immutable image
references when a deployment must remain pinned.

## Stabilization qualification (preceding Kindle rollout)

- Broader risk review of metadata, acquisition, imports, identity, workers,
  notifications, authentication and operator controls. Three UI defects found
  during the continued journey were fixed; no additional backend blocker was
  found within the documented review scope.
- All six candidate CI jobs passed: Go vet/Postgres race tests, 25 frontend tests,
  143 browser tests, build/deployment checks, disposable real Calibre, packaged
  recovery/restore, scans and both multiarchitecture builds.
- A fresh browser journey covered Forms login/restart, real English Open Library
  search/add, uncertain acquisition reconciliation without resubmission, EPUB and
  chapter imports, missing/foreign-file review, failure/restart recovery,
  rename/remove/restore and desktop/tablet/phone layouts.
- The exact published images passed the application, worker and notification
  suites on native ARM64 and native AMD64. Four exact-image scans found zero
  fixable HIGH/CRITICAL findings under the configured `--ignore-unfixed` policy.
- A sanitized production copy migrated 0029→0058, preserving all existing fields
  across nine tables, three exact file/book associations and three media hashes.
  Restoring the pre-upgrade snapshot and original image bytes passed rollback
  and reverted a deliberate copy-only mutation.

The [qualification report](reviews/2026-09-16-release-qualification.md) records
methods, digests and boundaries. Chapter bytes were synthetic, roots were
preconfigured, and client failure cases used controlled fixtures. These checks
do not establish audio playback, unattended operation or a real Readarr migration.

## Known gaps

1. The planned 72-hour/20-controlled-case observation period remains incomplete.
   It started September 17 at 00:21 UTC (September 16 locally). The owner
   explicitly authorized immediate `latest` promotion to replace the broken old
   build; observation is no longer a publication prerequisite. This decision
   does not turn unfinished observation into passed evidence.
2. Live Hardcover credentials and broader rich-provider qualification. Google
   Books credentials are configured and passed a live health lookup on September
   21; broader fallback/ranking qualification remains separate. Provider adapters
   have fixture coverage; Open Library has real-search evidence.
3. A real Readarr migration and consumer qualification. Compatibility remains
   partial, including persistent collision-free numeric identity mapping and
   non-book resource contracts.
4. Broader product work in the [stabilization backlog](stabilization-plan.md),
   including richer Calibre behavior, settings-source consistency and durable
   collection-wide operations. Implemented endpoints do not prove full parity.

Use the [release checklist](release-checklist.md) for gate evidence and the next
rollout steps. The owner requested the Kindle addition after stabilization;
other expansion remains subject to the documented release priorities.

## Current live deployment notes

These are maintainer homelab values, not install defaults:

- TrueNAS app: `librarry`; portal: `http://192.168.1.221:30200/`.
- Cosmos hostname: `https://librarry.borchetta.xyz/` (Cosmos sign-in required).
- Live images: immutable GHCR pair from the Kindle rollout report.
- A private pre-Kindle database/config/media checkpoint and the preceding
  stabilization image pair are retained for immediate rollback.
- Original `librarry-api:local` and `librarry-web:local` images and a fresh
  pre-upgrade database/config/media checkpoint are retained for rollback.
- Postgres: `/mnt/HDD_pool/vault/app-config/librarry/postgres`.
- Config mount: `/mnt/HDD_pool/vault/app-config/librarry/config:/config`.
- Media mount: `/mnt/HDD_pool/vault/media-stack:/data`.
- Roots: `/data/media/books/ebooks`, `/data/media/books/audiobooks`;
  downloads: `/data/torrents/books`; standard search language: English.

Prowlarr, qBittorrent and Open Library passed read-only checks after rollout.
All three existing books have present-file evidence after a successful scan;
media hashes and associations are intact. Automation settings were restored and
initial worker passes reported no errors or new grabs. Browser Library and
System checks passed. Do not publish live
credentials, database dumps or private library inventories in issues or commits.

## Historical evidence

- [Kindle rollout and SMTP acceptance](reviews/2026-09-17-kindle-rollout.md)
- [Live rollout and rollback checkpoint](reviews/2026-09-16-live-rollout.md)
- [September qualification](reviews/2026-09-16-release-qualification.md)
- [Initial audit](reviews/2026-09-15-audit.md) and [implementation ledger](reviews/2026-09-15-implementation.md)
- [Preserved status and milestone ledger](history/2026-09-16-status-ledger.md)
- [Historical Readarr parity audit](readarr-parity.md) and [UI audit](ui-backlog.md)

Historical claims can predate later fixes or stricter qualification. This page
and the release checklist take precedence for current readiness.
