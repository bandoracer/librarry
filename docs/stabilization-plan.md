# Librarry stabilization and polish plan

Status: execution in progress across stacked stabilization PRs. See the [implementation ledger](reviews/2026-09-15-implementation.md) for completed changes, evidence, and remaining gates. The full plan is not complete.

Prepared September 15, 2026 against checkout `6c1f32e`. Based on the [product and reliability audit](reviews/2026-09-15-audit.md), current implementation, and standing product decisions. The earlier [parity plan](parity-plan.md) remains a historical implementation record. This document describes the next body of work; it does not certify existing features or deployments.

## Outcome and release strategy

Deliver a book manager that users can install, configure, understand, and operate without silently importing the wrong book, losing chapters, breaking library identity, or repeating acquisitions. Preserve the existing Go/Postgres/React architecture and improve its workflow boundaries.

Plan two releases, with version numbers confirmed against remote tags before execution:

1. **Safety patch, proposed v0.4.2:** stop known unsafe import behavior, preserve scan metadata, honor configuration, enforce requested authentication, fix the settings race and audiobook release search, and expose trustworthy build identity. Ambiguous or multipart payloads require review until the full importer is ready.
2. **Stabilized release, proposed v0.5.0:** complete multipart import/recovery, trustworthy metadata, consistent library semantics, scalable browsing/scanning, polished setup and acquisition flows, qualified integrations, and tested install/upgrade/restore paths. Remain explicit about alpha status until the release gates pass.

The patch is independently useful and must not wait for the redesign. Automatic grabbing remains the default. Cleanup remains enabled by default, but an individual download must meet the new verification rules before deletion. Existing ambiguous imports are retained for review, not automatically grandfathered into cleanup eligibility.

## Product decisions carried into implementation

- English remains the standard search language, with an explicit user override.
- Hardcover is the rich primary provider; Open Library remains usable without credentials; Google Books is exact-match fallback; embedded metadata and OPF are import evidence.
- Manual corrections always win over refresh, merge, scan, and migration.
- Ebook and audiobook editions remain distinct acquisitions and file sets. A work can have both.
- Scheduled auto-grab retains the standing arr-style default. Review is for ambiguity, unsupported payloads, conflicts, and recovery decisions.
- Multi-chapter audiobooks are supported in v0.5.0. Multi-book packs require explicit per-book review and mapping; fully automatic pack splitting is deferred.
- qBittorrent, Transmission, SABnzbd, Prowlarr, and Calibre remain external integrations. New general-purpose download-client administration is outside this plan.
- Preserve existing URLs and Readarr API shapes where possible. Add native endpoints/fields for new functionality and make incompatible changes explicit.
- Migrations are append-only. Do not rewrite migrations already used by installations.
- Never claim support from route counts alone. Record contract-test, real-integration, platform, and deployment verification separately.

## Workflow invariants

These become regression tests and release criteria:

1. A download can only import files proven to belong to that exact download. No shared-folder fallback.
2. Every required file is accounted for before a book is complete. Ignored extras are explicit and never conceal a supported book/chapter file.
3. Cleanup requires a durable successful import record, current destination verification, correct client identity, and actual cleanup eligibility. A paused torrent alone does not prove its seeding goal was met.
4. A rescan enriches file evidence without deleting book associations, overrides, import history, or Calibre identity.
5. Missing/unavailable roots and temporarily unreachable clients are different from confirmed missing books. An outage cannot cause destructive reconciliation or mass re-grabbing.
6. Repeated callbacks, worker overlap, retries, and process restarts converge to one logical acquisition/import outcome.
7. A failed or incomplete operation stays visible and recoverable. No success response after a failed persistence operation.
8. Requested authentication is either enforced or startup fails clearly. No silent fallback to open access.
9. Saved or environment-supplied settings have a visible effective value and source. Unsupported settings cannot appear successfully applied.
10. Library lists, wanted gaps, search badges, details, counts, and compatible APIs agree about a book's state.

## Phase 0 — Establish a reproducible baseline

### S01 — Regression environment and evidence baseline

**Scope:** add a disposable Postgres 16 test stack, temporary media roots, and controlled fixture clients. Record the current checkout, image digests, database migration level, effective nonsecret configuration, library counts, and file inventories before deployment work.

Bring the audit reproductions into maintained tests: wrong sibling payload, multipart collapse, settings race, category encoding, swallowed GraphQL errors, authentication boot behavior, and query callback handling. Add a fresh-database regression for issue #1. Keep live provider credentials out of default CI.

Use the existing CC0 EPUB fixture; add generated chapter audio fixtures and explicit multi-book, missing-file, duplicate-file, and ambiguous-metadata examples. Test cleanup can delete only a uniquely created test namespace. Disable external acquisition in baseline integration environments until a test explicitly enables it.

**Done:** one documented command starts the disposable environment and runs relevant tests from a clean checkout. Existing tests still pass; new regressions demonstrate the audited failures before fixes. Missing Docker or credentials are explicit qualification blockers, not skipped tests reported as passing.

### S02 — Continuous integration gates

**Scope:** add Go unit/vet/race jobs, Postgres integration tests, TypeScript/build checks, a small browser smoke suite, and deployment configuration validation. Introduce browser tests early and expand them with each workflow change.

Gate image publication on successful checks. Separate unprivileged PR testing from credentialed release qualification. Use a frontend test runner compatible with the existing Vite/React setup and Playwright for repeatable browser flows. Keep exploratory visual review as a separate check.

**Done:** a deliberately failing regression prevents publication; an empty database renders the app without an uncaught exception. CI artifacts include failing test output and browser traces without secrets.

## Phase 1 — Ship the safety patch

### S03 — Configuration and authentication contract

**Scope:** reconcile Go defaults, all example env files, generic/source Compose, TrueNAS, Unraid, and documented controls. Add a machine-checked list of supported settings and explicit exceptions for host-only variables. Pass completed import/removal/mode and related advertised runtime controls into every deployment variant.

Reject invalid authentication modes and forms/basic startup without working persistence and usable credentials. Preserve deliberately configured local `none` mode. Ensure a persistence failure when saving auth/settings returns a useful error rather than a misleading durable-success response.

Publish a redacted effective-configuration view with default/env/persisted source and restart-required status. Define precedence: explicit environment settings override persisted UI settings, which override defaults; environment-owned fields are identified in the UI. Apply this consistently as settings are migrated.

**Done:** each installer produces the intended runtime values for on/off and nondefault settings. Missing database + requested forms/basic fails closed; explicit none, forms, basic, API-key clients, and feeds pass their intended auth matrix. Saving and restarting preserves supported settings.

### S04 — Safe payload guard and scan-preservation patch

**Scope:** remove shared-download-directory fallback; require an exact payload path or client file inventory. In the patch, send unresolved/multipart/pack payloads to review instead of guessing. Retain originals when source membership or complete import cannot be proven.

Separate scan observations from authoritative metadata at the update boundary. Preserve existing wanted/download links, source paths, manual corrections, and Calibre metadata. Do not blanket-merge untrusted scan JSON into authoritative fields.

Before source cleanup, verify the tracked destination still exists, is outside the source deletion tree, and has adequate successful-import evidence. Legacy ambiguous records require review. Do not derive cleanup authorization solely from the old `imported` string.

**Done:** audited sibling and multipart fixtures cannot be silently imported; a repeated scan leaves an existing imported record's identity intact; cleanup refuses missing destinations, ambiguous old records, and unverified payloads. Include a single-file positive case so safe acquisition continues working.

### S05 — Prowlarr categories and safe configuration swaps

**Scope:** validate and integrate community PR #2, preserving contributor attribution. Use repeated categories for JSON search while preserving Newznab feed semantics. Add ebook/audiobook/any-format contract tests.

Replace acquisition-service wholesale mutation with synchronized immutable client/config snapshots. Each operation observes one configuration generation. Preserve persistence handles and report save failures.

**Done:** all search-format tests pass, live read-only audiobook search returns a normal result/empty-result response, and concurrent health/list/search/settings tests pass under the race detector. No arbitrary release is grabbed for validation.

### S06 — Build identity and patch packaging

**Scope:** embed version, commit, and actual build timestamp; report active auth and applied schema version. Label image digests and expose worker availability without disclosing credentials.

Update the Go and container baseline to a supported version selected during implementation; upgrade vulnerable dependencies in reviewable changes. Scan the actual binaries/images. Triage applicability rather than using severity counts as proof of exploitation. Defer unrelated major frontend framework upgrades.

**Done:** local and image builds identify their exact source; status values remain stable across requests; unsupported-runtime/dependency findings are resolved or have a documented, specific nonapplicability assessment. Fresh install and upgrade fixtures pass.

**Patch release gate:** S01–S06 complete, database backup restore tested in isolation, safety regressions pass against the packaged image, and rollback procedure is rehearsed. Update docs and deploy a controlled candidate before tagging the patch. No production changes are part of this planning turn.

## Phase 2 — Make acquisition and import recoverable

### S07 — Explicit book, file, and download relationships

**Scope:** add relational file-to-wanted and file-to-download associations, plus import-operation/file-manifest records. Use existing database download IDs internally and preserve `(client, external_id)` as the external identity. All mutation queries must scope the client correctly.

An import operation records the source manifest, required file set, intended destinations, per-file verification, state, last error, and cleanup status. Support several files for one audiobook. Preserve the existing singular imported-file field as a compatibility projection during transition, not the source of truth.

Append migrations after the current migration head. Backfill existing JSON links only when identifiers resolve unambiguously; record unresolved rows for review. Dual-read/dual-write during the compatibility window, and do not mark old imports verified just because an old status says imported. Report before/after counts and reconciliation exceptions.

**Done:** upgrade of a copy of the current database preserves all associations that can be proven; ambiguous data is visible; two clients sharing an external ID cannot affect each other's records; scans cannot erase relational links.

### S08 — Exact payload manifests and complete audiobook imports

**Scope:** normalize adapter payload inventories with relative paths, sizes, completion and selection state. Resolve remote mappings with path-boundary checks and reject traversal or symlinks escaping the authorized source roots. Reuse existing adapter file APIs where possible.

Import all required chapters and relevant sidecars as a set. Preserve relative disc/chapter order and distinguish separate media formats. Validate that selected content matches the wanted work/edition using identifiers and local metadata; conflicting evidence enters review even for a wanted-tagged download.

Recognize a supported chapter group; route uncertain grouping and multi-book packs to manual mapping. An unknown adapter payload cannot fall back to scanning a shared directory. Expose the complete manifest and exclusions in import review.

**Done:** single EPUB/M4B and multi-disc MP3 fixtures import completely; foreign siblings, incomplete chapters, mixed books, symlink escapes, and conflicting metadata cannot silently satisfy a wanted book.

### S09 — Durable import execution and cleanup

**Scope:** implement persisted transitions such as planned → transferring → verified → committed → cleanup-eligible → cleaned, with retryable failure/review states. Keep all required filesystem operations and database commits recoverable through the operation record.

Stage copies on the destination filesystem, verify byte counts and content hashes, then publish each file atomically. Verify hardlink identity where supported; checksum copies and ambiguous cases. Multi-file visibility is gated by the committed operation; do not pretend a filesystem and Postgres transaction are one atomic operation.

Retain existing destinations until replacement has been verified; use backup/recycle paths for recoverable replacement. Handle cross-filesystem moves as copy/verify/commit before source removal. Propagate short-write, disk-full, permission, sync, and database errors.

Cleanup rechecks the whole required destination set, client identity, source/destination separation, and actual client eligibility. Manual pause is not equivalent to a satisfied seeding goal. If the client cannot provide enough evidence, retain the source and show the reason.

**Done:** fault injection at every transition and process restart produces no lost original, truncated published book, duplicate chapter, or unearned cleanup. Retrying committed operations returns the existing result. Removing source data leaves every required imported file readable and hash-valid.

**Current partial qualification (September 16):** standalone renames now reuse
verified staging, file identity reservations, atomic path/history commit and
post-commit source cleanup. Fault fixtures cover database rollback, cleanup retry,
concurrent owner corrections, destination conflicts and original receipt replay
through repeated renames. Per-file selection retains chapter/companion layouts;
a separate book action moves complete recorded folders while preserving names,
disc paths and verified relative CUE/M3U/OPF references. General chapter/reference
rewriting remains unsupported. Whole-book
retirement, broader disk fault injection and live qualification remain open.
New native-root Calibre handoffs now persist accepted uploads, per-format
conversion acknowledgements and atomic bookkeeping. Unknown sends require
operator decisions. Real disposable Calibre tests cover recovery after metadata
interruption and acknowledgement loss under Digest and Basic. Legacy Calibre
identity repair, remote path refresh and unattended qualification remain open.
This does not mark S09 complete.

### S10 — Acquisition deduplication and recovery

**Scope:** persist per-book/format acquisition intent and coordinate manual grabs, wanted monitor, feeds, upgrades, and failed-download recovery. Use a short database claim/lease and uniqueness constraints; avoid holding a transaction open during a slow remote call.

When a client request times out after possibly succeeding, reconcile using client identity/tags/infohash before retrying. Record uncertain outcomes rather than blindly issuing another add. Expired leases and process restarts must be recoverable. Apply bounded retries/backoff and blocklisting with visible reasons.

**Done:** concurrent worker/manual requests converge to one logical acquisition, ambiguous client responses do not cause duplicate grabs, and one unreachable client does not incorrectly prove a book absent. Repeat for two clients and verify mutation isolation.

**Additional worker qualification:** registered scheduled/manual System Tasks
passes now share Postgres session ownership and persisted due times. Two packaged
API processes verify peer busy refusal, process-kill interruption and recovery.
Domain acquisition/import journals still protect individual remote effects;
notification outbox delivery and broader worker/live qualification remain open.

### S11 — Scans, missing files, and legacy reconciliation

**Scope:** create persisted, resumable scan jobs with progress and cancellation. Apply paged batches; resume beyond the old 1,000/5,000 limits. Separate local metadata evidence from authoritative identity.

On a successful complete scan, reconcile missing/moved files. A missing root, permission failure, or partial/cancelled scan cannot mark an entire library missing. Reattach moved files only through unambiguous identity evidence; otherwise surface review. Use an explicit file-presence state and preserve import history.

Provide a preview report for existing linkage damage, possible incomplete audiobook imports, and duplicate files. Never bulk-delete or fuzzy-reassign legacy records as an automatic migration side effect.

**Done:** repeated/restarted scans over 10,000 synthetic files converge; a removed file changes presence correctly; an unavailable mount does not; manual corrections survive; legacy repair reports explain each proposed action.

## Phase 3 — Make metadata trustworthy

### S12 — Provider contracts and health

**Scope:** implement Hardcover book/author lookup and author bibliography traversal, rich work/edition/series fields, and list sync using the verified current API. Persist raw provider records and aliases. Add recorded, sanitized fixtures and opt-in live qualification.

Handle HTTP and GraphQL errors, pagination, expired credentials, malformed results, timeouts, rate limits, and partial success. Implement bounded provider concurrency, backoff, and caching. Distinguish configured, reachable, authenticated, degraded, and unavailable where evidence supports those states. Show last checked/success time.

Keep Open Library functional independently. Verify Google Books only runs for the intended exact-match fallback. Provider failures cannot look like authoritative zero-book bibliographies or erase existing metadata.

**Done:** valid/invalid-token and schema fixtures pass; author bibliography/list pagination does not truncate; Open Library-only installs remain usable. Real Hardcover and Google qualification stays pending until working credentials are available.

### S13 — Work/edition matching and monitoring policy

**Scope:** define distinct work, edition, format, language, and author identity rules. Avoid merging adaptations, abridgements, translations, and similarly named authors solely by loose title matching. Preserve provider provenance and field-level overrides when choosing canonical values.

Surface why a result matched; do not present a heuristic score as a calibrated probability. Prefer available exact identifiers, then compatible author/title/edition evidence. Use narrator/abridgement evidence for audiobooks when the provider supplies it; unknown values remain unknown.

Exercise all seven author monitor modes, filters, root/profile inheritance, exclusions, and repeat sync. Distinguish original publication from edition date. An author refresh cannot remove manually selected books or undo exclusions/overrides.

**Done:** a curated regression corpus covers originals/adaptations, same-name authors, multiple languages, ebook/audio editions, no-ISBN records, provider disagreement, future dates, and repeated refreshes. Ambiguous cases enter review and monitoring does not create duplicate wanted records.

## Phase 4 — Unify the domain model and API behavior

### S14 — Library, wanted, and detail contracts

**Scope:** give Library, Missing, Cutoff Unmet, Review, removed items, and detail lookup explicit server-side membership rules. Library includes tracked downloaded and unmonitored books; removal hides the entry; wanted gap views select their actual gap states.

Keep legacy lifecycle values for compatibility, but derive product presence from verified files/imports and current download evidence. Preserve an explanatory unavailable/stale-evidence signal rather than manufacturing certainty during outages. Define the lifecycle after file deletion and partial audiobook loss.

Fix query function wrappers so TanStack context never becomes a filter argument. Introduce separate query keys/fetchers and direct book/author lookup. Keep imported books visible in Library when repairing wanted filtering. Normalize empty lists and reject invalid filter values.

**Done:** add → grab → import → rescan → rename → unmonitor → remove is consistent across Library, Wanted, details, search badges, Activity, and compatible APIs. Removed rows stay removed and successful imports remain visible.

### S15 — Pagination, counts, and query performance

**Scope:** add bounded server-side paging, filtering, and stable sorting with unique tie-breakers. Return total/filtered counts with defined consistency behavior. Support operations on explicit IDs or a server-defined filtered selection; distinguish selected page from all matching records.

Remove silent whole-collection caps from business decisions. Process background work in fair batches so a continuously busy first page cannot starve older books. Add the indexes justified by actual query plans and eliminate per-row remote calls from listing routes.

**Done:** 10,000-book/file fixtures return accurate counts, access older records directly, and traverse every page. Stable-data paging has no duplicates/gaps. On a documented CI/reference machine, target p95 ≤500 ms for local paged reads of up to 100 records; measure external-provider latency separately. Verify bulk actions affect exactly the chosen scope.

Current S14/S15 progress: native books, author subscriptions, metadata Review,
author candidate reviews and files have complete paged traversal fixtures.
Author candidate decisions now commit atomically with history and preserve
existing book settings. Native file browsing and rename preview traverse every
page. Legacy readers, search badges, compatibility, removed-book browsing and
resumable all-matching jobs remain open; neither milestone is complete. See the implementation ledger for qualification.

### S16 — Readarr compatibility and migration

**Scope:** maintain an endpoint contract matrix with real side effects, supported fields, and unsupported behavior. Reuse native services so compatible routes cannot bypass import, identity, or deletion rules. Do not return success for an unsupported mutation.

Exercise migration preview/apply/retry against a representative Readarr fixture and, when available, a real instance copy. Preserve roots, profiles, authors, books, files, tags, overrides, exclusions, and relevant integration settings with clear conversion warnings. Preview performs no mutation and apply is idempotent. Never modify the source Readarr instance.

**Done:** row-by-row reconciliation is available; a second import creates no duplicates; foreign keys and file associations resolve; unsupported features are explicitly documented. Do not claim a tested real Readarr migration if only fixtures were available.

## Phase 5 — Polish the product around the verified workflows

### S17 — Setup and settings

**Scope:** guide a new operator through authentication, roots/permissions, metadata providers, Prowlarr, download clients, path mappings, and automation choices. Show tested connection state and effective values with a clear next action for failures. Make no-database development mode visibly temporary and prevent misleading persistent-save feedback.

Support entering provider credentials through protected backend settings where practical, following the shared precedence model. Return masked configured-state only; omitted secrets preserve existing values, explicit removal is separate, and updates are tested before success is shown. Keep external env configuration fully supported and documented.

**Done:** a clean installation can be configured from the UI without editing database rows; secrets do not appear in GET payloads/logs; env overrides are obvious; connection errors identify which step failed; restart preserves intended settings. Open Library-only setup has an honest degraded-richness state.

### S18 — Search and add flow

**Scope:** lead with title, author, cover, edition, format, and language. Group genuinely equivalent candidates while keeping alternate editions/adaptations distinguishable. Move provider IDs, scoring inputs, and raw evidence into expandable details.

Use one clear add/review flow with root, format, profile, monitoring policy, and search-on-add semantics. Preserve progress when navigating back. Handle partial provider failures, no results, duplicate tracking, and cancelled/outdated requests without replacing newer results.

**Done:** users can find a known book, distinguish an adaptation, select an audiobook edition, add it once, and understand what happens next. A removed book and an already-downloaded book have intentional, distinct add behavior.

### S19 — Library, Activity, and import review

**Scope:** consistent book/author counters and terminology; clear downloaded/missing/downloading/partial/review explanations; useful book detail with files, edition evidence, history, and scoped actions.

Activity shows acquisition → download → import → cleanup progress and the concrete next action. Review displays full source/destination manifests, conflicting evidence, required chapter counts, matching choices, and previewable manual mapping. Bulk outcomes show partial failures instead of a blanket success toast.

**Done:** a failed import is discoverable from the dashboard and recoverable without SQL/log spelunking; all required chapters and their order are visible; wrong matches can be corrected without erasing provenance. Navigation and old deep links continue to work.

### S20 — Responsive behavior, accessibility, and resilience

**Scope:** compact mobile filters/actions, readable book rows/cards, keyboard-accessible dialogs, focus restoration, labelled icon actions, adequate contrast, and consistent loading/empty/error states. Add route-level error boundaries with retry/report details and graceful chunk-load recovery.

Review desktop at 1280/1440, tablet at 768, and mobile at 375/390 px. Keep the established visual language; consolidate inconsistent primitives and copy. Empty, large, degraded, and partially configured states receive the same attention as populated screenshots.

**Done:** core workflows work with keyboard and mobile navigation; no inaccessible primary action or accidental page-wide horizontal overflow; browser tests fail on unexpected console/page errors. Exploratory review exercises clicks, scrolling, slow responses, and reconnects rather than relying only on screenshots.

## Phase 6 — Prove integrations and operations

### S21 — Complete integration matrix

Qualify supported versions explicitly when execution starts. Every adapter receives contract fixtures; real-service tests carry separate evidence.

| Integration | Required proof |
| --- | --- |
| Prowlarr | Ebook/audio/any search, feeds, repeated categories, timeout and invalid credentials, magnet redirect and torrent payload handoff |
| qBittorrent | Tagged legal fixture, file inventory, start/stop, completion, import, goal-aware cleanup, v4/v5 state differences, wrong/missing path |
| Transmission | Equivalent lifecycle, client identity, file inventory, stopped versus completed/seeding-goal behavior |
| SABnzbd | Authorized test payload or controlled fixture service, extraction completion, output paths, history/retry semantics, import and scoped cleanup |
| Hardcover | Book/author/edition data, full bibliography/list pagination, exclusions, credential expiry, throttling |
| Open Library / Google Books | Credential-free backbone behavior, exact fallback constraints, partial failures, identifiers and language normalization |
| Calibre | Per-root add/update/convert/status behavior, duplicate handoff, rename/path refresh, failure recovery and file availability verification |
| Notifications | Relevant event payloads, retry/deduplication, failure visibility, secret redaction, controlled local/test destination only |
| Calendar | Full-date versus year-only handling, timezone boundaries, authenticated feed, actual calendar-client consumption |

Do not grab arbitrary indexer results. Test downloads remain tagged and isolated. Real notifications to other people are not part of integration qualification; use a local capture server or an explicitly designated test recipient.

**Done:** record version, fixture, actions, API readback, file counts/hashes, and cleanup result per integration. A credential/service blocker is named and visible in the release support matrix. Core promised paths block the release if unverified; optional paths must be explicitly labelled experimental instead of implied tested.

### S22 — Backup, upgrade, restore, and security qualification

**Scope:** restore a real-format database dump into a fresh isolated database and verify application login, settings, book/file links, task state, and pending import recovery. Document that a database backup is not a media backup; capture the configuration needed to recover and handle its secrets appropriately.

Test upgrades from the audited schema and selected supported tagged versions. Use migration serialization so concurrent starts cannot race schema changes. Demonstrate behavior on migration failure and rollback from a failed release.

Exercise auth/session expiry/logout/password-change revocation, login throttling, CSRF/origin protections for browser mutations, API keys, protected downloads/feeds, and trusted-proxy handling for secure cookies. Review path and input limits around imports/uploads. Limit security work to actual application boundaries and dependency findings; record concrete results.

**Done:** restored instance matches expected records and associations; media verification is separately reported; rollback preserves files and configuration; forms/basic/API-key access contracts pass; no secret-bearing support output. No restore writes to the live database during qualification.

### S23 — Operational truth and bounded maintenance

**Scope:** reliable task history with operation IDs, counts, durations, failures, last success, and next action. Persist enough history for restart diagnosis without unbounded per-minute growth. Add retention for routine history while preserving unresolved failures and important import/acquisition evidence.

Provide redacted support diagnostics with build/digest, schema, effective configuration, provider/client versions where discoverable, roots, and task status. Separate liveness from readiness and degraded external integrations. Avoid aggressive health probes that consume provider quotas.

**Done:** a provider outage, stuck import, missing mount, disabled worker, and failed backup each appear accurately with freshness information; recovery clears the condition; support data identifies the real build and omits secrets.

## Phase 7 — Release and live verification

### S24 — Packaging and documentation

**Scope:** fresh installs and upgrades through generic Compose, source build, TrueNAS, and Unraid; verify UID/GID permissions, mount paths, config persistence, health checks, reverse-proxy routes, and backup tools. Qualify amd64 and arm64 runtime behavior, not just manifest existence.

Correct README, status, architecture, deployment, local development, provider setup, metadata policy, frontend docs, and stale UI/parity claims. Clearly distinguish implemented, fixture-tested, integration-tested, and live-deployed features. Add contributor setup, reproducible bug-report guidance, and a security-reporting route.

Build and publish immutable candidate images for the same source commit, then promote their tested digests to version tags. Keep API and web versions paired. Add changelog and upgrade warnings; include the community contributor's credit. Public communication and GitHub issue replies are a separate execution action, not performed by this plan.

**Done:** a new operator can follow the docs literally; container settings match runtime; anonymous image pulls work; tagged builds map to a commit and validation record. Platform validation gaps are clearly labelled rather than certified from Compose rendering alone.

### S25 — Controlled homelab rollout and soak

**Preflight:** verify current live image digests and database state anew. Capture database backup and recoverable media/config snapshot or equivalent backup for touched paths. Prepare the exact previous images/configuration and a tested rollback procedure.

**Candidate rollout:** run the candidate against isolated copies and fixture media first. Shadow evaluation may read real inputs but must not grab, import, or delete. Prevent two active instances from acting on the same production download/library roots. Deploy a matched API/web pair only after candidate gates pass.

**Live proof:** use `/mnt/HDD_pool/vault/media-stack:/data`, existing ebook/audio roots, and `/config` according to the verified homelab configuration. Validate health, authentication, one controlled ebook and multipart-audiobook loop, rescan, task outcomes, and file hashes. Verify LAN and Cosmos independently and diagnose the recorded HTTPS 502 before declaring the public route healthy.

**Soak:** require at least 72 hours plus 20 controlled lifecycle cases across the staging/live qualification environments, including retries, restarts, and review recovery. Require real scheduled monitor/feed/import runs. Long-interval tasks such as weekly backups are tested with shortened schedules only in isolation; distinguish that evidence from a manual live run or a real weekly run.

**Release stop conditions:** wrong identity, missing required files, unexpected source deletion, repeat grabs, auth bypass, migration damage, unbounded retries, or inconsistent durable state. Stop affected automation, preserve evidence, and follow the tested recovery plan.

**Done:** the acceptance ledger records exact SHAs/digests, migration versions, dimensions, fixtures, hashes, scheduler outcomes, exceptions, and rollback proof. No open P1 or unresolved safety condition. Close issue #1 only after the clean-install regression is reproduced fixed. Promote the release only after these gates.

## Dependencies and implementation order

Each S-number is a reviewable work package. Split schema changes, service behavior, UI, and qualification into smaller PRs as needed; do not accumulate an entire phase in one unreviewable diff. Include behavior tests and relevant docs with the change that introduces them.

| Order | Packages | Dependencies / release role | Relative effort |
| --- | --- | --- | --- |
| 1 | S01–S02 | Baseline and gates for all work | Medium |
| 2 | S03–S06 | Safety patch; depends on baseline tests; isolated restore prerequisite from S22 | Medium–large |
| 3 | S07–S11 | Import foundation and recovery; each builds on the preceding data contracts | Largest workstream |
| 4 | S12–S13 | Metadata correctness; can progress after S01/S03, before final UI decisions | Large |
| 5 | S14–S16 | State/API contracts; S14 uses S07/S11, S15 uses S14, migration uses finalized mappings | Large |
| 6 | S17–S20 | Product polish; depends on configuration, metadata, and API contracts | Large |
| 7 | S21–S23 | Qualification added throughout; final matrix after relevant behavior lands | Large; external-service dependent |
| 8 | S24–S25 | Packaged release, live verification, soak | Medium plus observation time |

The critical path is baseline → safety patch → file/import relationships → recoverable multipart import → unified state → end-to-end qualification → release. Provider qualification can advance independently of file execution once its configuration contract is established. This dependency map does not create additional tasks or agents.

## Test and release acceptance ledger

Record evidence as work completes; initial state for every item below is **not yet qualified under this plan**.

| Area | Required cases | Pass condition |
| --- | --- | --- |
| Install/auth | Fresh DB; no DB; unavailable DB; invalid credentials/mode; restart; upgrade | Honest startup and readiness; requested auth never bypassed |
| Import ownership | Correct path; renamed payload; missing payload; sibling book; traversal/symlink | Exact ownership or explicit review |
| Media completeness | EPUB, M4B, chapter MP3, multi-disc, sidecars, partial content, book pack | All required media verified; ambiguous mapping reviewed |
| Durability | Disk full; read-only root; DB failure; remote timeout; kill/restart at each transition | Originals recoverable; no false completion or duplicate outcome |
| Cleanup | Seeding goal; manual pause; missing destination; same source/destination tree; legacy record | Only proven, scoped cleanup; complete library survives |
| Scan/state | Repeated scan; missing/moved file; unmounted root; cancel/resume; overrides | Identity and correct state preserved; no mass false-missing |
| Automation | Manual/monitor/feed overlap; upgrade; blocklist; uncertain add; two clients | One logical acquisition; bounded recovery and clear history |
| Scale | 201/501/5,001 boundary fixtures; 10,000-record collection; paging/bulk/scan | Complete reachable data, accurate counts, fair processing |
| UI | Empty/full/error; setup/search/add/review; details; keyboard; mobile; reconnect | No blank screens, hidden primary actions, or misleading success |
| Recovery | Backup create/restore; old-schema upgrade; candidate rollback | Reconciled records, usable auth/config, media consistency |
| Packaging | Four deployment variants, two architectures, LAN/proxy routes | Rendered config plus actual execution evidence for advertised support |

## Release gates and blockers

- The **safety patch** requires maintained regressions, safe ambiguity handling, working deployment switches, enforced auth, passing races, search fix, trustworthy identity, isolated restore proof, and controlled image verification. It does not claim multipart auto-import support.
- The **stabilized release** additionally requires complete imports/recovery, scalable state/API behavior, core provider qualification, polished core flows, fresh-install and upgrade/restore proof, qualified packaging, and successful soak.
- Valid Hardcover/Google credentials, real optional client services, a Readarr source copy, native NAS targets, and a functioning container runtime may be needed for specific gates. Request only missing inputs when the relevant execution step is reached; continue independent work. Do not hunt credentials or substitute fixture results for live qualification.
- Keep an explicit exception list for optional integrations/platforms. Core safety, authentication, data recovery, and advertised primary workflows are not waivable by relabelling them tested.
- No fixed calendar estimate is asserted. The schema/import recovery work and external qualification dominate uncertainty; reassess remaining effort after the safety patch and first complete multipart lifecycle.

## Migration and rollback rules

1. Back up and rehearse restore before applying new schema to the live database.
2. Prefer additive schema and a compatibility window; preserve old fields until consumers and rollback versions are understood.
3. Backfill in bounded, resumable batches with counts and unresolved-row reports. Identity ambiguity never authorizes a guessed repair.
4. Roll back images only when the previous version is demonstrated compatible with the new schema/data. Otherwise restore the paired database/config snapshot and reconcile media changes using the operation ledger.
5. A database restore alone cannot undo file moves or deletes. Retain originals/recycle evidence and require filesystem backup/snapshot protection for rollout paths that mutate existing media.
6. Upgrade should not replay a backlog of unknown historical imports into deletion. Legacy records retain an explicit verification/review state.

## Maintenance after release

Keep regression fixtures as the durable record of fixed defects. Run dependency and packaged-image checks regularly, triage community issues with reproducible reports, and keep support claims tied to evidence. Update the status document after actual deploys and release notes after shipped changes. Break up oversized modules along the service boundaries touched by this work; avoid unrelated rewrites.

Success means a new user can install Librarry, configure a provider and client, acquire a book, see every required file in the library, recover from a failure, and trust the reported state after a restart or upgrade.
