# Authentication, worker and notification contracts

Implementation contracts for contributors. Operator instructions live in the [guides](../README.md#operate). These describe candidate source behavior, not complete compatibility or live qualification.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [Stabilization boundaries](#stabilization-boundaries)
- [Authentication configuration transactions](#authentication-configuration-transactions)
- [Shared scheduled-worker ownership and history](#shared-scheduled-worker-ownership-and-history)
- [Transactional native notification delivery](#transactional-native-notification-delivery)
- [Readarr-compatible webhook delivery](#readarr-compatible-webhook-delivery)
- [Resolved notification history compaction](#resolved-notification-history-compaction)
- [Worker availability and compatibility status](#worker-availability-and-compatibility-status)
- [Redacted operational support](#redacted-operational-support)

## Stabilization boundaries

Acquisition operations hold one immutable client/configuration generation. Settings
updates atomically publish a new generation; in-flight operations keep their
original clients. Download mutations bind external IDs to a client; legacy callers
without a client may mutate only an unambiguous external ID.

Library scans call an observation-specific store method. The scan updates physical
file evidence and keeps fresh local metadata under `scanEvidence`; it preserves
existing names, wanted/download links, source paths, and Calibre metadata.

Verified completed imports retain `verifiedDownload` evidence (client, external
ID, SHA-256) in file metadata as a compatibility projection of the durable
[operation ledger](files.md#durable-completed-import-operations-unreleased). Cleanup compares fresh client inventory and all required
filesystem hashes before requesting deletion.

`GET /api/v1/wanted?view=library` includes tracked imported/unmonitored books and
excludes removed/ignored entries. `cutoff-unmet` retains its separate membership.
Unknown views return 400. Direct book/file lookup bypasses collection caps;
[native collection endpoints](collections.md) provide full paged browsing. System status reports build version/commit/time, active authentication,
and the applied migration filename/number. Local builds without an injected build
timestamp report `unknown` rather than the request time.

## Authentication configuration transactions

The auth store commits the single-user credentials, session revocation and
`auth-config` resource together. In-memory enforcement changes only after commit.
A database advisory lock serializes configuration and session creation; a session
must still match the credential hash verified at login. Credential updates revoke
existing sessions, and session validation verifies the stored user identity.
Environment-owned method/credentials are exposed as lock flags in auth status;
the web settings form disables those controls and the API rejects overwrite attempts.

## Shared scheduled-worker ownership and history

With database persistence, the scheduler claims a task-specific Postgres session
advisory lock before invoking a registered worker. Manual System Tasks requests
claim synchronously, so another API process returns 409 while the owner runs.
Scheduled claims additionally check `worker_tasks.next_run_at`; starting another
API or restarting one cannot immediately repeat a recently completed scheduled
pass. Manual triggers can override the due time but not active ownership.

Migration 0047 stores the current run/due time in `worker_tasks` and diagnostic
runs in `worker_task_runs`. An active lock, not heartbeat age, establishes running
status. The owner refreshes its heartbeat every ten seconds and cancels its worker
context when coordination fails. A lost session is shown as interrupted, and a
successor marks the abandoned run interrupted before claiming. Completion is
bound to the same run ID and connection. Jobs must honor cancellation; remote
requests already accepted cannot be rolled back by scheduler coordination. The
acquisition/import journals remain responsible for individual side-effect safety.
This is shared scheduler ownership, not an exactly-once external delivery claim.
Use a direct or session-pooled Postgres connection, not transaction pooling.

Manual registry runs join application shutdown alongside scheduled runs. Panics
are recorded as unverified failures without exposing panic payloads. Migration
0050 adds structured counts, bounded operation UUIDs, next action, review timestamp,
and a separate last-success identity/time. Reports with errors become `degraded`
even if the worker returns no top-level error. Last success advances only on a
clean completion. Import cleanup, backup pruning and notification delivery outcomes
contribute to the report. Historic completed states remain historical evidence,
not reconstructed per-item qualification.

Retention keeps 100 successes, protecting current/last-success identities even
with skewed timestamps. Unreviewed failures are never routine deletion candidates.
Up to 500 failures reviewed more than 90 days ago are pruned during each completion;
the current run and domain recovery journals remain. Hourly History Maintenance
also prunes up to 500 eligible reviewed failures across all persisted workers,
including disabled workers, while preserving their current run. Lock-based interruption requires the current owner identity as well
as its PostgreSQL session lock; a recycled PID cannot make an old row active.
An abandoned run has no invented completion timestamp/duration.
`GET /api/v1/system/tasks` reads shared state and returns an unavailable response
when that state cannot be read. `GET /api/v1/system/tasks/{id}/runs` exposes the
retained runs for a registered task with `view=all|unreviewed`, `limit` (1–100)
and `offset`. Count and page use one materialized effective-state snapshot.
`POST /api/v1/system/tasks/{id}/runs/{runId}/review` accepts `reviewed`,
`expectedState` and `expectedReviewedAt`; only inactive failures can be reviewed,
and stale state/review timestamps return 409. System Tasks offers paginated
history and review in an accessible dialog. Direct business
API/compatibility operations still use their domain-level coordination rather
than becoming scheduler jobs.

## Transactional native notification delivery

Migration 0048 adds immutable `notification_events`, per-target
`notification_deliveries`, attempt/action history, and persisted health states.
An insert trigger captures `release_grabbed` and `book_imported` history. A
separate trigger captures the transition into `downloads.failed_at`, avoiding
reliance on the recovery worker's later best-effort history write. Both execute
inside the domain transaction. Health observations serialize by check ID and
atomically capture only ok/unknown-to-unhealthy transitions. No historical
backfill occurs. Domain callback sends have been removed for native targets.

Fan-out records only targets enabled for that event at capture time, their IDs,
names/types and exact `updated_at` revisions. Event payloads allow-list basic
book/release labels and identifiers; raw release URLs, target settings and
credentials are not copied. The sender fetches current settings under a short
shared row lock, refuses changed/deleted/disabled targets and commits `sending`
plus an attempt token before HTTP. Settings may change after a send starts; those
changes cannot retract an in-flight request. No DB transaction spans remote I/O.

A per-delivery advisory session lock serializes send and operator resolution.
Completion writes through the original connection/token. Acquiring an abandoned
`sending` entry marks it uncertain rather than issuing another request. Network
errors, 408/5xx and a lost result save are uncertain. 2xx is acceptance; other
3xx/4xx fail. Redirects are not followed. 429 alone gets automatic backoff
(minimum exponential minutes, respecting Retry-After up to 24 hours) with at most
five total sends. Longer requested waits require operator review. Explicit retry
uses a current target revision and a current delivery revision, and records an
audit action. `X-Librarry-Delivery-ID` and `X-Librarry-Event-ID` remain stable across
retry; webhook event timestamps describe the original committed event.

The shared `notification-delivery` task advances at most 25 entries per pass at a
15-second interval. `GET /api/v1/notification-deliveries?limit=25&offset=0` returns
one repeatable-read page/count without settings; `POST
/api/v1/notification-deliveries/{id}/resolve` accepts retry/accepted/cancel plus
confirmation and expected revisions. The latest send state is shown in Settings
→ Connect; attempt and resolution records remain in Postgres and backups. Explicit
connection tests do not enter the queue. Migration 0049 extends this mechanism to Readarr-compatible webhooks, as described
below.

## Readarr-compatible webhook delivery

Migration 0049 namespaces delivery targets as `native` or `compat`; identical UUIDs
in the two resource tables cannot collide. New notification events capture enabled,
matching `compat_resources` webhook targets. The migration never adds recipients
to older events. `onReleaseImport` takes precedence over the legacy `onDownload`
alias, including settings readback; health notifications are opt-in through
`onHealthIssue`. Unsupported implementations are not treated as webhooks.

Before an event is inserted, a trigger saves an allow-listed `compat_context`
snapshot of the relevant wanted book, exact client/download, selected release and
ordered imported file set. UUID lookups retain index use and tolerate missing or
non-UUID legacy identifiers. Unknown import release identity never borrows another
release from the download. Provider download/info URLs, arbitrary file metadata
and target settings are excluded. Upgrade acquisition receipts now persist their
original current/cutoff scores so post-crash history repair retains those values.

The API installs the payload adapter before starting workers. It reconstructs the
existing book/author/download/release/import/bookFile shapes from the saved
snapshot, includes every imported file in `bookFiles`, and keeps the original
event ID/time. Source describes the persisted trigger, not the API request that
happened to finish recovery. Compatibility settings are loaded from the current
resource under revision/enable/trigger checks; its existing field aliases, method,
Authorization and Basic authentication remain supported. Bodies cannot be replayed
implicitly by Go's transport, including custom GET/PUT methods. Redirects and
secret-bearing network errors are handled the same way as native delivery.

API callback sends are removed. Manual API actions, scheduled work and domain
recovery now reach the same commit-time capture. Compatibility test/test-all remain
explicit synchronous requests. Delivery history identifies Readarr webhooks;
configuration remains under `/api/v1/notification`. Attempt and resolution state
uses the same session ownership, review controls and backup guarantees as native
connections. This qualifies generated fixtures, not live third-party consumers.

## Resolved notification history compaction

Migration 0051 introduces `notification_deliveries.resolved_at` and event
`archived_at`/`retention_summary`. HTTP acceptance and confirmed acceptance/cancel
start the resolution window; retry clears it. Automated connection-change
cancellations are unresolved. Legacy accepted rows inherit `updated_at`; legacy
cancellations are conservatively left unresolved.

Hourly `history-maintenance` selects at most 100 events older than 90 days whose
entire recipient set has been resolved for 90 days. Empty-recipient events use
creation time. Each event has its own transaction: lock the event with SKIP LOCKED,
try the existing delivery advisory keys with transaction locks, recheck resolution,
delete delivery/attempt/action detail, and replace both payload snapshots with a
bounded count summary. Holding the event row prevents new foreign-key references;
nonblocking delivery locks protect concurrent send/review/retry. A failure rolls
back that event; prior committed counts remain in the task report. The notification
pass has a 30-second deadline.

The event UUID, unique source key and timestamps survive forever. Compaction never
removes this replay barrier, recreates recipients, or changes health episode state.
Re-enqueuing an archived source key still does nothing. Detailed history is bounded
by the resolved retention window; unresolved records and compact event identities
are intentionally retained. Import/acquisition journals and domain history are
outside this policy. Restore must include compact records as well as active outbox
rows; deleting them manually can permit replay of the same source event.

## Worker availability and compatibility status

All 13 built-in workers register their definitions even when disabled or missing
configured dependencies. `Task.DisabledReason` and `UnavailableReason` separately
describe this instance's startup policy and prerequisites. Blank reasons preserve
the previous enabled/available default for registry callers. Blocked definitions
may omit a body; active definitions still require one. Startup skips their loops,
and manual/internal claim paths refuse them. Native manual requests return 409 for
disabled tasks and 503 for unavailable dependencies. History/review continue to use
the registered identity and shared database records.

`TaskStatus.enabled`/`available` and their reasons describe the responding instance.
Shared running/outcome/history/last-success evidence remains visible even when a
peer runs a locally disabled task. `nextRunAt` is omitted for local blocked tasks;
`lastFinishedAt` is only present when completion is recorded. A missing database
configuration produces an unavailable inventory; loss of configured persistence
returns 503 instead of invented status. Dependency availability means configured
prerequisites, not successful provider checks.

Readarr task names/IDs remain stable. The compatibility routes now map the registry
statuses and saved start/finish/duration/due times instead of deriving fake runs
from `now`. Native unknown times are omitted. Readarr's
[TaskResource](https://github.com/Readarr/Readarr/blob/develop/src/Readarr.Api.V1/System/Tasks/TaskResource.cs)
uses non-null DateTime/TimeSpan fields, so compatibility dates use the year-1 zero
value and duration uses zero when unknown. Four `librarry*Known` booleans explicitly
separate these placeholders from recorded start/finish/due/duration evidence.
Database failures propagate as 503.
ImportListSync uses its own actual interval and enable flag rather than feed-sync
settings. `LIBRARRY_IMPORT_LIST_SYNC_ENABLED` defaults true and is forwarded by
all deployment variants. Explicit native/compatibility sync commands remain
separate from scheduled execution. No schema migration is needed for this change.

## Redacted operational support

`GET /api/v1/system/support` uses the ordinary API/session authentication policy
and returns a non-cacheable attachment (`formatVersion: 1`). System downloads it
on demand. The API explicitly selects safe fields rather than serializing config,
logs, provider diagnostics, task errors or notification payloads. It includes build
identity, an explicitly unknown image digest, a current bounded Postgres ping and
numeric server version when readable, schema observed at process startup, selected
automation flags, effective library policy, anonymous roots and registered worker
status. Unknown enum values are reported as unknown. All list fields remain arrays.
Sections that cannot be read are marked unavailable; failures do not erase the
other evidence or expose underlying connection errors.

Metadata provider observations are process-local and retain their actual request
and success times. Snapshot generation does not perform provider/download-client
IO or manufacture fresh observations. Client endpoints are reduced to configured
booleans; recorded health/version evidence is included from the same configuration
generation, while unrecorded reachability and remote versions stay unknown. Worker policy
and next run belong to this instance; saved runs can come from peers. Only built-in
provider/task identities are exported. Free-text names, errors, outcomes, paths,
URLs, usernames, credentials and book metadata are excluded. This report is not a
complete configuration backup or historical incident log.

`GET /readyz` returns only `status` and `checkedAt`: 200 when the current database
ping succeeds, otherwise 503. It is public like `/healthz`, which checks liveness
and stays 200 during a database outage. Both are proxied by nginx. Readiness does
not depend on remote providers or root presence and does not certify schema
compatibility after startup, media integrity or workflow completion. Root checks
in the protected support report have a 500ms deadline and a process-wide cap of
four in-flight filesystem calls so stalled NAS calls cannot accumulate unbounded
goroutines. A directory being present does not prove it is the expected mount or
that it is writable. Missing directories, errors and timeouts remain distinct.
