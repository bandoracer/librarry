# Changelog

User-visible changes are recorded here. Read [current status](docs/status.md) for
what is published, deployed and qualified. A candidate tag is not a stable release.

## Unreleased

- Use Hardcover's supported series-search endpoint and explicit IDs instead of
  forbidden substring filters; prioritize usable editions over sparse work records.

- Add New has an explicit Hardcover Series mode with source-reported positions,
  bounded results and clear missing-credential errors.
- Explicit author rescue now includes translated edition titles; searches such as
  `The Employees by Olga Ravn` recover the original work and English edition.

- Title/author discovery recognizes returned author identities and uses one bounded
  structured rescue for explicit author queries when needed. Add New groups
  returned editions of a verified work and preserves the selected edition on add.

- Book discovery now preserves provider relevance, searches Open Library across
  title/author/subtitle context, and selects a coherent matching edition. English
  editions are no longer excluded by the order of a work's language list.
- Exact ISBN matches use checksum-validated edition identifiers. Search keeps
  conflicting exact editions visible with review reasons; unknown media formats
  stay unknown. Text similarity alone no longer merges search identities.
- Add New shows match evidence instead of percentage scores, prefers edition
  covers, handles failed images, and shows provider errors alongside usable results.
  Unknown edition formats require the existing add-review step.

## September 16, 2026 — stabilization release

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
- Build publication remains explicit and candidate-only. A separate manual
  workflow promotes qualified digest pairs to `latest` without rebuilding;
  automatic main/PR/tag builds cannot advance release aliases.
- Qualification covers native AMD64/ARM64 images and a production-copy
  upgrade/rollback. The qualified pair is deployed on the maintainer NAS;
  both `latest` tags now point to these exact images by owner request. The
  longer observation period remains incomplete.
- Documentation now separates installation, operator guides, implementation
  reference, current readiness and historical milestones.

## Historical alpha versions

Earlier alpha tags predate this stabilization work. Consult
[Git tags](https://github.com/bandoracer/librarry/tags) and the
[historical ledger](docs/history/2026-09-16-status-ledger.md) for their record.
Those milestone descriptions are not qualification of the current candidate or
evidence that an installation has upgraded.
