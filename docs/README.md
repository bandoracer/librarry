# Documentation

Use [current status](status.md) to distinguish candidate source, published images
and the live deployment. The feature guides describe the candidate; historical
`latest` images can behave differently.

## Install

- [Docker Compose, image selection, upgrades and restore](deployment.md)
- [TrueNAS](../deploy/truenas/README.md) · [Unraid](../deploy/unraid/README.md)
- [Provider credentials and matching behavior](provider-setup.md)

## Operate

| Task | Guide |
| --- | --- |
| Configure clients, search releases, recover acquisitions | [Book acquisition](guides/acquisition.md) |
| Configure profiles, authors, feeds and lists | [Metadata and monitoring](guides/monitoring.md) |
| Manage roots, scan, browse and restore tracking | [Library management](guides/library.md) |
| Review payloads, import, rename and recover interrupted work | [Imports and recovery](guides/imports.md) |
| Configure authentication, workers, notifications and backups | [Operations](guides/operations.md) |

## Develop

- [Local development and verification](local-dev.md)
- [Contribution workflow](../CONTRIBUTING.md) · [Security reporting](../SECURITY.md)
- [Architecture overview](architecture.md) · [Frontend structure](frontend.md)
- [Metadata strategy](metadata-strategy.md)

## Implementation reference

These contracts explain storage, transactions, identity and API behavior. They
complement the operator guides rather than acting as installation instructions.

- [API routes](reference/api.md)
- [Metadata](reference/metadata.md)
- [Acquisition](reference/acquisition.md)
- [File integrity and recovery](reference/files.md)
- [Collections and identity](reference/collections.md)
- [Authentication, workers and notifications](reference/operations.md)

## Release and history

- [Release checklist](release-checklist.md)
- [Qualified candidate and immutable image digests](reviews/2026-09-16-release-qualification.md)
- [Changelog](../CHANGELOG.md)
- [Longer-term stabilization plan](stabilization-plan.md)
- [Historical status ledger](history/2026-09-16-status-ledger.md)
- [Historical parity audit](readarr-parity.md), [parity plan](parity-plan.md),
  and [UI audit](ui-backlog.md)

Keep current readiness in `status.md`; keep release evidence in the checklist and
dated reviews. Update the relevant guide when behavior changes instead of
appending feature announcements to the README or status page.
