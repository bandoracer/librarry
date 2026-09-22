# Discovery and download rollout — September 21, 2026

The simplified download flow, discovery cleanup and adaptation evidence are merged,
deployed on the maintainer NAS, and promoted to `latest`.

## Source and images

- Source: `8591bd98cb121fec90e71b6772ab805605fb6272`.
- [PR #55](https://github.com/bandoracer/librarry/pull/55), merged at
  `c863182f1a7e231cdfc9a95655f4ed0d48acc2b8`.
- [Candidate qualification and publication](https://github.com/bandoracer/librarry/actions/runs/35687364315): all six jobs passed.
- [Paired latest promotion](https://github.com/bandoracer/librarry/actions/runs/35688429097): succeeded, verifying both aliases and AMD64/ARM64 index membership.

| Component | Published index | Running AMD64 image |
| --- | --- | --- |
| API | `ghcr.io/bandoracer/librarry-api@sha256:acb894981d3de7aec6330e50272da5756e76f0525033f1c7abbd2bfe5655d633` | `sha256:610ba1017973414f41bdb1825ad22367856b4d31d7fa440c000ef514abc145d9` |
| Web | `ghcr.io/bandoracer/librarry-web@sha256:cb4c4552fc0c394485dbee25a574c383156bbcd7180f26c19758c567dcf06375` | `sha256:58553d6ff88e2f9a7b13806b4a410f30d5f09ffdb0a76d0077615dce3f93eaaa` |

The NAS compose configuration is pinned to these immutable references. Runtime
image labels and `/api/v1/system/status` both identify the source above. Migration
remains `0059_kindle_delivery.sql`; this release adds no migration. Later
documentation commits do not change the runtime.

## Qualification

- Go vet and full PostgreSQL race suite, 33 frontend tests, production build,
  171 browser tests with one skip, and deployment contracts passed in CI.
- Disposable Calibre checks passed. Packaged import/recovery/restore, workers and
  notification checks passed; both image scans passed the configured fixable
  HIGH/CRITICAL gate. Both architectures were built and published.
- Local metadata race tests and captured ranking cohorts passed. All 28 diagnostic
  live-provider searches put the intended work/series first; see the
  [adaptation review](2026-09-21-adaptation-ranking.md).
- Wider 50-result service probes separated the Hobbit comic and abridgment, and
  the identified Stand comic volumes. The public book-search API remains bounded
  to ten results, while Series returns up to fifty. Alternatives below that book
  page are not proof of missing normalization.
- Nine post-deployment API searches passed: A Brief History of Time, DDIA,
  project hail marry, The Hobbit, The Stand, Sapiens, The Stand comics, Watchmen,
  and Percy Jackson and the Olympians. No provider errors occurred in these
  readbacks. Labels distinguish the Sapiens work's misassigned graphic ebook and
  Stand comic from original editions; Watchmen stays primary.
- Live browser readback shows seven primary Percy Jackson novels, collapsed
  related/incomplete sections, and a working detail/edition chooser with Download,
  Add Book, collapsed Options and collapsed source details. The layout was
  visually inspected. No live acquisition was submitted.

## Preservation and recovery

A private pre-deployment configuration/database checkpoint and media hashes were
recorded, and the database dump was validated. Only the API/web image references
changed in the saved TrueNAS configuration. After rollout, all three file records,
three file/wanted links, manual overrides, all 59 migration records and all three
media hashes matched the checkpoint. `/healthz` and `/readyz` returned 200.

The preceding metadata image pair and checkpoint are retained for rollback.
No credentials, database contents or private library inventory are published here.

## Remaining limits

Unverified duplicates remain separate. Catalog records lacking useful content
evidence, including a miscategorized Sapiens graphic record, can remain
unclassified. This is a diagnostic search cohort, not catalog-wide accuracy or
unattended acquisition qualification. The broader observation/migration gaps in
[status](../status.md) remain open. There were no open repository issues to close.
