# Stabilization pause point — September 16, 2026

The owner requested a pause after the current compatibility change on
`codex/compat-book-collections`, stacked on PR #49 (`codex/search-book-identity`). No next
implementation phase should start without resuming the discussion. The full
S01–S25 plan is unfinished; this is a review checkpoint, not a release approval.

## Overall assessment

The project now has much stronger boundaries around its highest-risk operations:
which files belong to a download, whether a chapter set is complete, when bytes
may be removed, how interrupted work recovers, and what the UI may claim from
available evidence. Those changes have maintained database, failure, concurrency,
browser and packaged-app fixtures. The work remains a stack of draft changes;
passing fixtures do not make the existing homelab or every advertised integration
qualified.

The next decision should be release scope and review strategy. Continuing to grow
the stack without consolidating its review and qualification would make delivery
harder. Choose a candidate boundary, review the combined behavior and migrations,
then qualify that candidate. The original safety-patch intent should be revisited
explicitly rather than silently treating all stabilization work as one release.

## What has been implemented

| Area | Concrete progress | Practical limit |
| --- | --- | --- |
| Baseline and safety (S01–S06) | Disposable Postgres/client/media fixtures, CI gates, auth startup/save failure handling, deployment setting checks, repeated Prowlarr categories, synchronized configuration snapshots, patched runtime/dependencies and actual build/schema identity | Broader effective configuration/source display, fresh candidate security scans, live search, rollout and rollback proof remain |
| File and acquisition integrity (S07–S11) | Relational associations, exact payload manifests, chapter/sidecar imports, durable native/manual import and replacement plans, staged copies, verified cleanup, uncertain-grab reconciliation, scan persistence, guarded missing/moved evidence, repair previews and durable renames | Changed chapter-set/whole-book retirement, broader disk faults, legacy Calibre repair and real NAS/client lifecycles remain |
| Metadata and authors (S12–S13) | Error-aware provider health, bounded requests/cache/backoff, exact Google fallback, bibliography/list traversal, edition evidence, policy/exclusion handling, destination inheritance and protection of existing tracking | Rich provider credentials/live coverage, wider series/edition behavior, durable raw provenance coverage and a broader matching/policy corpus remain |
| Collections and identity (S14–S16) | Complete native book/author/file/review/recovery pages; truthful counts and presence; exact upgrade selection; explicit inactive-book restore; full saved search identity checks; complete compatible book arrays and bounded missing/cutoff pages; atomic selected book edits | Other legacy readers and compatible resources, numeric ID mapping, resumable all-matching jobs, real Readarr migration and consumer qualification remain |
| Product workflows (S17–S20) | Import/recovery review, pinned book selectors, metadata/author decisions, dashboard attention, error recovery, keyboard/mobile controls and growing browser coverage | A coherent new-install setup, effective configuration throughout Settings, broader search/add/activity polish, tablet and full accessibility review remain |
| Operations (S21–S23) | Real disposable Calibre contracts, durable native/compatible notification delivery, shared worker ownership, failure-preserving history/retention, redacted support export, distinct readiness and timestamped integration health | Complete real-service/version matrix, calendar consumption, full security/upgrade/rollback qualification, mount identity, stall classification and live freshness remain |
| Delivery (S24–S25) | Repeated local API/web builds, packaged import/auth/restart tests and isolated dump/restore; deployment contract checks and extensive documentation | Native TrueNAS/Unraid and both-architecture runtime proof, production-copy upgrade/restore, immutable release images/tags, controlled rollout, Cosmos repair and the 72-hour/20-case soak remain |

These rows summarize progress rather than declaring whole milestones complete.
The [implementation ledger](2026-09-15-implementation.md) contains per-change
fixtures, observations and limitations. The [full plan](../stabilization-plan.md)
remains the acceptance contract.

## Final change at this checkpoint

Compatibility readers previously saw only 200 books, and missing detection relied
on a limited file list and old lifecycle strings. Identity matching accepted
titles and alias hashes; bulk mutations could skip invalid IDs and partly apply.

The current change reads all active books, uses native evidence for missing and
cutoff states, provides bounded SQL pages with accurate counts/sorts, and validates
all selected targets before transactional edits. Removed/ignored or stale targets
cannot be silently reactivated through these routes. Explicit invalid release
identities cannot fall through to an unscoped acquisition. Deletes retire
tracking and retain media.

The historical numeric hash can still collide. Lookup rejects ambiguity and the
native UUID remains available, but arrays do not yet have a persistent unique
numeric mapping. Single-ID resolution currently reads the full compatibility
collection. Other payload fields/resources and multi-file manual import batches
are outside this change's atomicity guarantee.

## Qualification at this checkpoint

The focused fixtures reach 10,001 active books and traverse 101 bounded missing
pages without gaps or duplicates. Full ordinary and race-enabled Go/Postgres tests pass, as do
15 web unit tests, 143 desktop/mobile browser tests (one expected skip), production
build, Go vet and deployment contracts. Local schema-57 API/web images pass
packaged imports, auth, restart recovery, compatibility mutations and an isolated
465,707-byte database restore. These are local fixture/container results.

The immediately preceding PR #49 has all five GitHub checks green. The new PR’s
CI status must be read separately; the local result is not a claim about its CI.

## Deployment and release truth

No September production deployment, release tag, public image publication, live
database restore or unattended soak has been performed. The earlier audit saw
legacy version 0.4.0 on the LAN and a 502 from Cosmos; those are historical
observations, not a new live check at this pause point. Fixture restores do not
prove recovery of live media/configuration. Hardcover/Google credentials, an
appropriate Readarr source copy and authenticated NAS access remain necessary
for their respective qualification work.

## Suggested evaluation order

1. Review the combined safety/import/schema/auth changes and decide the smallest
   candidate worth shipping. Review draft-stack ordering and merge strategy.
2. Walk through that candidate as an operator: fresh setup, search/add, acquisition,
   chapter import, deliberate failure, recovery, removal/restore and upgrade.
3. Close its concrete blockers with real-service and platform evidence; keep
   unsupported integrations visibly experimental rather than claiming parity.
4. Rehearse backup/upgrade/rollback on isolated copies before any production
   change, then use a controlled rollout and timed observation.

Do not infer a completion percentage from PR count or passing test count.
