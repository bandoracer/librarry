# Operating Librarry

Authentication, background tasks, notifications, backups and diagnostics for the candidate. Health probes report specific evidence; they do not certify an entire installation.

[Documentation index](../README.md) · [Current status](../status.md)

## Contents

- [System Tasks](#system-tasks)
- [Notifications](#notifications)
- [Health & Disk Space](#health--disk-space)
- [Calendar & iCal](#calendar--ical)
- [Authentication](#authentication)
- [Tags](#tags)
- [Backups](#backups)
- [Qualify and inspect shared background workers](#qualify-and-inspect-shared-background-workers)
- [Review notification delivery](#review-notification-delivery)
- [Notification history retention](#notification-history-retention)
- [Support reports and probes](#support-reports-and-probes)
- [Dashboard triage](#dashboard-triage)
- [Compatibility book qualification](#compatibility-book-qualification)

## System Tasks

System → Tasks lists all 13 built-in workers, including disabled workers and
workers missing required dependencies. Status includes recorded runs, last
success, current ownership and instance-specific scheduling/dependency reasons.

- `GET /api/v1/system/tasks` reads shared task status.
- `POST /api/v1/system/tasks/{id}/run` requests a manual run. Busy or disabled
  tasks return 409; unavailable tasks return 503; unknown IDs return 404.

Use [shared background worker diagnostics](#qualify-and-inspect-shared-background-workers)
for run history, review, retention and multi-instance behavior. A displayed
schedule is not evidence that a task has run successfully.

## Notifications

Native notification targets are persisted in `notification_targets` and fan
out grabs, imports, upgrades, download failures, and health issues from both
API-triggered actions and scheduled worker runs (the Readarr-compatible
`/api/v1/notification` webhook resources keep working in parallel).

- `GET /api/v1/notifications` →
  `{"targets":[{id,name,type,settings,triggers:{onGrab,onImport,onUpgrade,onDownloadFailure,onHealthIssue},enabled,createdAt}]}`.
- `POST /api/v1/notifications` creates a target (response `{"target":...}`),
  `PUT /api/v1/notifications/{id}` updates, `DELETE /api/v1/notifications/{id}`
  removes, and `POST /api/v1/notifications/{id}/test` sends a test event and
  returns `{"ok":bool,"error"?}`.

Provider types and their settings keys:

- `webhook`: `url` (required), optional `authorization` header value. Sends an
  arr-ish JSON payload with `eventType`, `title`, `message`, and `fields`.
- `ntfy`: `url` and/or `topic` (server defaults to `https://ntfy.sh`),
  optional `token` (Bearer) and `priority`. Message body plus `X-Title` header.
- `discord`: `webhookUrl` (required). Sends an embeds payload with
  severity-colored embeds and inline fields.
- `telegram`: `botToken` and `chatId` (required). Calls the Bot API
  `sendMessage` method.

Secrets: telegram `botToken` values are redacted to their last 4 characters in
GET responses, and a blank (or redacted) `botToken` on PUT keeps the stored
credential. Webhook/ntfy/discord URLs are operator-entered endpoints and are
returned as stored. Health-issue notifications are opt-in per target
(`triggers.onHealthIssue` defaults to false); all other triggers default on.
Delivery uses a 10 second timeout. Workflow events use the durable outbox;
inspect [notification delivery](#review-notification-delivery) for failures and
uncertain results. Explicit connection tests send immediately.

## Health & Disk Space

- `GET /api/v1/system/health` evaluates continuous health checks and returns
  `{"checks":[{id,severity,name,message}]}` with severity `ok`, `warning`, or
  `error` for every evaluated rule: database persistence (warning when
  missing), indexer configured/reachable (error), download client
  configured/reachable (error), root folders present and accessible (error per
  root), completed-import enabled (warning when disabled), low disk per root
  filesystem (<1 GiB error, <5 GiB warning), and quality profiles present
  (warning). The same evaluator runs on the 5 minute `health-check` task, and
  checks that transition from ok to warning/error dispatch `healthIssue`
  notifications.
- `GET /api/v1/system/diskspace` returns
  `{"disks":[{path,label,freeBytes,totalBytes}]}` for every root folder plus
  the book torrent root, deduplicated by backing filesystem.

## Calendar & iCal

Wanted items persist a confident `releaseDate` (yyyy-mm-dd) when the metadata
result carries a full publication date; year-only books stay off the calendar.
Existing rows were backfilled from edition publish dates in migration 0024.

- `GET /api/v1/librarry/calendar?start=&end=&unmonitored=true|false` returns
  `{"items":[{wantedId,title,authorName,releaseDate,status,monitored,coverUrl}]}`.
  `start`/`end` accept RFC3339 or `yyyy-mm-dd`; the default window is the start
  of the current month minus 7 days through today plus 60 days. Unmonitored
  items are excluded unless `unmonitored=true`.
- `GET /feed/v1/calendar.ics?apikey=&pastDays=&futureDays=` serves all-day
  VEVENTs (UID = wanted id) for external calendar apps. `/feed/` bypasses
  session auth but requires the `apikey` query parameter whenever
  `LIBRARRY_API_KEY` is set (and stays blocked under forms/basic auth without
  an API key, since calendar apps cannot log in).
- The Readarr-compatible `GET /api/v1/calendar` serves the same real
  release-dated items.

## Authentication

Arr-parity in-app auth for the API (`none` default, `basic`, `forms`):

```dotenv
LIBRARRY_AUTH_METHOD=forms
LIBRARRY_AUTH_USERNAME=admin
LIBRARRY_AUTH_PASSWORD=change-me
```

`LIBRARRY_AUTH_USERNAME`/`LIBRARRY_AUTH_PASSWORD` seed or update the single
user row (bcrypt) at startup. API keys (`X-Api-Key`, `apikey`, bearer) keep
working for every method so Readarr-compatible clients never break. With
`none` + `LIBRARRY_API_KEY`, the pre-M6 behavior is unchanged (key required on
`/api/*`).

- `GET /api/v1/auth/status` → `{"method","authenticated","username"?}` (always
  reachable; the web UI gates on it)
- `POST /api/v1/login` `{"username","password","rememberMe"?}` → sets the
  HttpOnly `librarry_session` cookie (30 days with `rememberMe`, browser
  session otherwise); invalid credentials return 401
- `POST /api/v1/logout` clears the session
- `PUT /api/v1/auth/config` `{"method","username"?,"password"?}` switches the
  method at runtime when it is not environment-owned. Explicit environment
  methods or credentials cannot be overwritten in the UI; change the environment
  and restart instead. A blank password keeps the stored one.

Only the API enforces auth: the nginx-served static UI bundle remains publicly
reachable and the UI itself redirects to its sign-in screen based on
`auth/status`. Front Librarry with a reverse proxy (Cosmos/CF Access) if the
static assets themselves must be private.

## Tags

Native tags live in the `tags` table; wanted items and author subscriptions
store comma-separated tag labels. Renaming or deleting a tag rewrites the
label across both columns in one transaction, and labels written through
wanted/author endpoints are auto-registered in the tags table.

- `GET /api/v1/tags` → `{"tags":[{id,label,wantedCount,authorCount}]}` (`id`
  is a stable integer hash; counts aggregate by label)
- `POST /api/v1/tags` `{"label"}`, `PUT /api/v1/tags/{id}` `{"label"}`,
  `DELETE /api/v1/tags/{id}`
- Wanted and author update payloads accept `"tags": ["label", ...]` (legacy
  integer tag ids from compat clients are mapped back to labels)

## Backups

`pg_dump`-based database backups (custom format, restore with `pg_restore`):

```dotenv
LIBRARRY_BACKUP_ENABLED=true
LIBRARRY_BACKUP_INTERVAL=168h
LIBRARRY_BACKUP_RETENTION=4
LIBRARRY_BACKUP_DIR=/config/backups
```

- `POST /api/v1/librarry/backups` runs a backup now and returns
  `{"backup":{name,sizeBytes,createdAt}}`; installs without a database or
  without `pg_dump` answer `501`
- `GET /api/v1/librarry/backups` → `{"backups":[...]}` (also served on the
  compat `GET /api/v1/system/backup`)
- `DELETE /api/v1/librarry/backups/{name}` (names are sanitized to
  `librarry-YYYYMMDD-HHMMSS.dump` basenames)

The scheduled `backup` task creates a dump every interval and prunes to the
newest `LIBRARRY_BACKUP_RETENTION` files. The API image ships
`postgresql16-client`; mount `LIBRARRY_BACKUP_DIR` to keep dumps outside the
container. The Docker Compose examples mount persistent app config at `/config`,
so the default `/config/backups` path survives container recreation. The
database password travels to `pg_dump` via the child process environment and is
never logged.

## Qualify and inspect shared background workers

System → Tasks reads shared database status and the last recorded success.
Every built-in worker remains listed when disabled or missing dependencies.
**Disabled here** reflects this API instance's startup flags; **Unavailable here**
means required configuration/services are absent. Reasons explain what to restore.
Dependency availability does not prove provider reachability: actual failures remain
in run/health evidence. Blocked workers have no local scheduling loop or next-run
time. Manual System Tasks runs return 409 when disabled and 503 when unavailable;
history and review remain accessible. Enable/fix configuration and restart this
API instance to resume scheduling. A peer with different flags may still be running
that worker; shared running state and history remain visible. Stopping automation
across a deployment requires changing every instance.

The Readarr `/api/v1/system/task` routes use the same recorded state. Unknown
native start/finish/duration values are omitted. The Readarr fields retain their
non-null date/time types: unknown dates use `0001-01-01T00:00:00Z` and unknown
duration uses `00:00:00`, with the corresponding `librarryLastStartTimeKnown`,
`librarryLastExecutionKnown`, `librarryNextExecutionKnown` and
`librarryLastDurationKnown` flags false. These are placeholders, not run evidence;
polling never fabricates current-clock executions.
Schedules/dependency reasons describe the responding instance. Database read
failures return 503 instead of a synthetic schedule. Direct domain commands retain
their existing explicit operator scope outside the scheduler.

**History** pages through retained runs, including counts, available operation IDs,
measured duration, completion state and next action. A pass that reports individual
errors is **degraded** and does not advance last success. Warnings are separate. A stopped owner is
shown as interrupted; an old heartbeat does not permit stealing a still-held
worker lock. **Run now** works before the next due time but returns busy while any
API process owns that task. During a database outage, workers refuse new claims
and status reports an outage instead of an empty or healthy task list.

The standalone fixture below runs two API containers with one disposable
Postgres database and a third API without persistence. It blocks a harmless scan query, checks shared running status
and duplicate-trigger refusal, kills the owning API, checks interruption, and
recovers through the peer before restarting the original process. It also checks
disabled-peer history/manual refusal, the separate import-list flag and unavailable
workers without persistence. It never
contacts a live download client or metadata provider.

```sh
DOCKER_CONTEXT=your-test-context python3 scripts/test-worker-packaged.py librarry-api:your-candidate
```

Task ownership uses a session advisory lock. Configure a direct/session-pooled
Postgres connection; transaction-pooling proxies are not supported for workers.
History keeps 100 successful runs per task. Unreviewed failed, degraded and
interrupted runs are preserved until reviewed. **Mark reviewed** acknowledges
the diagnostic only; it does not retry or repair work. **Mark unreviewed** reopens
it. Reviews bind the current state and review timestamp; stale decisions return
409. Use the **Unreviewed failures** filter to find older failures beyond page one.
Reviewed failures become eligible for cleanup after 90 days. Each task completion
removes at most 500 eligible failures, retaining its current run. Hourly
**History Maintenance** also removes up to 500 eligible reviewed failures across
all persisted workers, including workers now disabled. Their current run remains
available for diagnostic readback. Import/acquisition receipts are not pruned. Historical
last success is backfilled from recorded completed runs; older per-item error
counts cannot be reconstructed. Interrupted owners with unknown finish time have
no fabricated duration. Native notifications use the durable outbox described below. These fixtures do
not establish a live multi-instance deployment.

## Review notification delivery

Settings → Connect → Notification delivery lists new queued messages and their
outcomes. **Accepted** records HTTP acceptance, not a read receipt. **Uncertain**
means a request may already have reached the receiver: inspect it before choosing
**Confirm acceptance**, **Review retry**, or **Cancel delivery**. Retry can create
a duplicate, uses the connection's current settings and requires confirmation.
Cancellation stops future attempts but cannot retract a sent request. A stale
review is rejected; close the dialog, refresh and inspect the current entry.
Changed/deleted/disabled connections stop pending messages. New connections do not
receive old events. HTTP 429 uses bounded retries; other ambiguous failures wait
for review. Explicit connection tests remain immediate.

Native and migrated Readarr-compatible webhook connections are covered. History
marks compatibility targets as **Readarr webhook**; their configuration remains
under `/api/v1/notification`. The legacy `onDownload` import flag is honored unless
`onReleaseImport` is explicitly set. Compatibility health messages require
`onHealthIssue: true`. New events retain book/file details from commit time. Use a disposable local receiver
for qualification; do not point test notifications at real people.

```sh
DOCKER_CONTEXT=your-test-context python3 scripts/test-notification-packaged.py librarry-api:your-candidate
```

This fixture creates its own Postgres, API and Python HTTP receiver containers,
verifies pending recovery after restart for both target kinds, kills the API after
the receiver records a request, and verifies uncertainty plus confirmation or
cancellation without a second send. Readarr fixtures also check PUT/Basic settings
and immutable book details after an intervening edit.
It removes the containers/network on exit. The packaged backup fixture also
compares event, delivery, attempt, action and health-state records after restore.

## Notification history retention

**History Maintenance** runs hourly (or through System → Tasks → Run now).
It examines up to 100 eligible notification events per pass. Detailed history can
expire only after the event is at least 90 days old and every delivery has been
accepted or explicitly cancelled for at least 90 days. Pending, retrying, sending,
failed and uncertain deliveries are preserved. A delivery automatically stopped
because its connection changed or disappeared is also preserved until reviewed.
Settings → Connect offers **Confirm cancellation** for those stopped deliveries.
A retry clears the resolution time; accepting/cancelling it again starts a fresh
90-day window. Legacy accepted messages inherit their saved acceptance update
time; legacy cancellations remain unreviewed until explicitly confirmed.

Compaction removes resolved delivery/attempt/action detail and native/Readarr
payload snapshots together in a transaction. One compact event record retains its
UUID, source key, occurrence/compaction timestamps and outcome counts. These
identities are retained indefinitely to prevent an old source event from creating
new deliveries; compact-record count still grows with distinct events. Maintenance
does not delete import/acquisition receipts, domain history, health-transition
state, settings or files, and does not contact receivers. Busy delivery/review
sessions are skipped and retried on a later pass. Maintenance has a 30-second
notification deadline, records actual committed counts, and rolls back any event
whose compaction cannot finish. A later batch resumes the remaining history.

## Support reports and probes

In System, select **Download support report** to save `librarry-support.json`.
The same authenticated read is `GET /api/v1/system/support`. It reports the real
API build, selected effective settings, database connectivity/version, startup
schema, anonymous directory checks and recorded provider/task evidence. It omits
credentials, URLs, private paths, book metadata and free-text logs. Review before
sharing. Partial sections and unknown values are explicit; no external checks or
notifications are triggered by the export.

Use `/healthz` for liveness and `/readyz` for database readiness. A process without
Postgres can answer metadata requests but returns 503 from readiness. A successful
readiness check does not guarantee provider availability, mounts, writable media,
complete imports or a successful backup. The existing setup checklist is separate
from this current-connectivity probe. Worker failures and last success are retained
in System Tasks; downloading support does not acknowledge or clear them.

## Dashboard triage

Needs attention now includes unfinished native imports, pending Calibre handoffs
and unresolved file links as well as review queues. Import counts include work still
running and committed transfers awaiting local cleanup; inspect the saved plan and
lease evidence before retrying. The recovery link opens the unfinished filter.
Refresh attention retries failed count sources. An unavailable source or incomplete
client evidence prevents an all-clear; retained counts may be stale.

Acquisition totals cover all active tracked books, while the action strip previews
recent books only. Imported here means a saved acquisition/import record. Use
Library to inspect current file presence and completeness. Import review counts
cover the complete queue and the review screen pages through it. These are
unreleased stabilization changes.

## Compatibility book qualification

Readarr-compatible book reads now include all active records, with missing/cutoff
pages based on native file evidence. Select books by their emitted numeric ID or
`librarryId`; title matching is no longer accepted. Missing/inactive IDs reject a
whole monitor/editor/delete request, and ambiguous aliases return 409. Refresh
before retrying a conflicting selection. Delete removes tracking, not files.

Run `scripts/test-integration.sh` with `LIBRARRY_TEST_DATABASE_URL` for the
10,001-book traversal, lost/partial files, quality cutoff, numeric sorting,
ambiguous identities, atomic rollback and concurrent-edit fixtures.
`scripts/test-packaged.py` checks complete older identities across restart,
native/compatible state agreement and mutation readback using disposable data.
A client that ignores `librarryStateCounts`, `librarryUnknownBooks` and
`librarryDownloads` cannot interpret an empty missing page as proof of health.
Persistent collision-free numeric IDs, complete non-book compatibility and a
real Readarr migration remain unqualified.
