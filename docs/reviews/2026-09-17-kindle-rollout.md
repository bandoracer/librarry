# Kindle rollout — September 17, 2026

## Source and publication

[PR #53](https://github.com/bandoracer/librarry/pull/53) merged source
`3d89a6358da7ad5064718c2e3f0c65d48021922b` into main at
`173b0de7b9bd3616d0c4ae935d5e43f921700a98`.
All six PR checks passed. Explicit candidate publication
[run 35197792820](https://github.com/bandoracer/librarry/actions/runs/35197792820)
also passed all six jobs: publication policy, disposable Calibre, application
checks, packaged qualification, and both image builds. Application checks include
Go vet/Postgres race tests, frontend unit/build checks and browser smoke tests.
Packaged qualification includes imports, isolated restore, worker and notification
fixtures, and Trivy scans with the existing fixable HIGH/CRITICAL policy.
Those scans cover CI's native candidate builds; they are not an assertion of
independent scanning of every final architecture manifest.

Published tag:
`candidate-3d89a6358da7ad5064718c2e3f0c65d48021922b-35197792820-1`.

| Component | Immutable index |
| --- | --- |
| api | `ghcr.io/bandoracer/librarry-api@sha256:e06622ef5fe1a9de41a6e73224e0904be1dff5b20ee366b306b5fa89142a915e` |
| web | `ghcr.io/bandoracer/librarry-web@sha256:9f81641a8c8cb663d748c7b57b5b6d1b54e3c429d04020118c633a2ad8e1a3bf` |

| Component | Architecture | Manifest |
| --- | --- | --- |
| api | amd64 | `sha256:0c3282872bdf382d414a056acc44ddb9c3db9c53f4059ba04ebdcd02f12351ba` |
| api | arm64 | `sha256:24798d57d779909cb09f64942c289a4dd7c416f62dac65868767974441f18ee6` |
| web | amd64 | `sha256:50e2f6f2be037e5546d97c5ba5a6615b02e3e4cc26e1a0a220472e96cbf7db9a` |
| web | arm64 | `sha256:dc9fc47338d0dc438ef7e1f80d4a477d2bbd9e5f96a91a8e6f28640db39a6d96` |

The NAS uses these index references and their native AMD64 manifests. `latest`
remains the preceding stabilization pair; this rollout did not change that alias.

## Exact-image Kindle checks

The published API index was run independently on native ARM64 (Mac/Colima) and
AMD64 (NAS) with disposable Postgres and a TLS SMTP fixture. Both passed:

- Source identity and migration 0059; settings credential redaction.
- Test-document acceptance and replay without a second SMTP submission.
- Native PDF attachment SHA-256 preservation.
- Disconnect after DATA produces an uncertain result.
- API restart preserves the uncertain attempt; replay does not resubmit it.
- Durable history and rejection of a symlink escaping the ebook root.

Each isolated fixture received exactly three messages and was removed afterward.
No production credentials or books were used. The fixture initially polled an
obsolete dynamic Docker port after restart; refreshing the port mapping corrected
the harness, and both complete runs then passed. This required no product change.

## Production rollout and readback

A private checkpoint under
`/mnt/HDD_pool/vault/app-config/librarry/releases/2026-09-17-kindle/`
contains the previous Compose configuration, validated custom-format database
dump, file/link/manual-override snapshots and media hashes. API/web writers were
stopped for the dump and restarted. Readiness uses the actual LAN binding rather
than localhost. Credential-bearing files remain private on the NAS.

TrueNAS was updated with the paired indexes and environment-based SMTP defaults.
Resend uses implicit TLS on port 465 and `books@mail.borchetta.xyz` as sender.
The owner supplied the destination and confirmed sender approval. Secrets and
the private destination are excluded from this report.

Live readback confirmed application source `3d89a63`, schema 0059, readiness,
unchanged file identity and file/book links, unchanged manual overrides, and
identical hashes for all three existing library files. The live browser showed
three downloaded books and the new Settings → Kindle page with delivery enabled
and the accepted test in its history.

At **2026-09-17 08:29:54 UTC**, one small Librarry setup test was explicitly
submitted through the live API. Resend acknowledged SMTP acceptance. No existing
library book was sent. Physical Kindle arrival and Amazon processing are not
confirmed by that acknowledgement. The owner subsequently corrected the Kindle
destination. Saved application settings, NAS deployment defaults and local
environment files were corrected; no additional test was sent. The original
attempt remains in history with its actual recipient. Delivery to the corrected
address therefore remains unverified.

## Rollback and limits

The saved Compose configuration restores the preceding exact image pair and
settings. Migration 0059 adds independent tables, so image rollback can retain
the database and delivery history rather than overwrite it with an older dump.
The full database checkpoint is retained as a separate recovery option. A
production rollback was not exercised during this rollout.

EPUB/PDF only, maximum source size 25 MiB. AZW3 requires separate conversion.
There is no automatic send-on-import or delivery retry. Existing stabilization
observation, Readarr migration and rich metadata qualification gaps remain open.
