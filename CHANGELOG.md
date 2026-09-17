# Changelog

User-visible changes are recorded here. Read [current status](docs/status.md) for
what is published, deployed and qualified. A candidate tag is not a stable release.

## Unreleased — stabilization candidate

Qualified application source: `6e209ffd8d3724879e52187e27790029e66af986`.
Candidate publication and platform digests are in the
[September qualification report](docs/reviews/2026-09-16-release-qualification.md).

### Reliability

- Durable import, replacement and rename plans preserve original media until
  verified commit and expose interrupted work for recovery.
- Completed imports inspect exact payload inventories and complete chapter sets;
  ambiguous or mixed-book payloads require mapping and destination review.
- Acquisition receipts let operators reconcile uncertain client acceptance
  without blindly resubmitting downloads.
- Resumable scans preserve file identity and distinguish unavailable evidence
  from confirmed missing media.
- Requested authentication fails closed on invalid settings or persistence
  failure. Credential changes and session revocation commit together.
- Workers coordinate across API instances; notifications preserve uncertain
  deliveries and replay barriers through restarts and retention.

### Operator workflows

- Paginated native and compatibility book collections, author/file/review views,
  explicit selections and complete counts replace truncated collection reads.
- Removed-book restoration preserves settings and file links, with monitoring
  off by default. Removal text and navigation now point to that recovery flow.
- Release and manual-download failures retain actionable server explanations.
- Provider/client checks, task history and redacted support reports separate
  configuration from observed health.
- The cold-statistics missing-books query uses bounded linked-file lookups.

### Packaging and documentation

- Append-only migrations 0030–0058; older migrations remain unchanged.
- Publication is explicit and candidate-only. Main/PR/tag builds cannot advance
  `latest` or release aliases.
- Qualification covers native AMD64/ARM64 images and a production-copy
  upgrade/rollback. Live rollout and observation remain open.
- Documentation now separates installation, operator guides, implementation
  reference, current readiness and historical milestones.

## Historical alpha versions

Earlier alpha tags predate this stabilization work. Consult
[Git tags](https://github.com/bandoracer/librarry/tags) and the
[historical ledger](docs/history/2026-09-16-status-ledger.md) for their record.
Those milestone descriptions are not qualification of the current candidate or
evidence that an installation has upgraded.
