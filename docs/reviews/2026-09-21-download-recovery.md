# Acquisition recovery — September 21, 2026

Completed transfers awaiting import review were reported as Downloading/Queued.
Recovery also rejected its own search after modifying the book revision. The
release supplies precise activity and review states across search, library,
download queue and acquisition actions. Pending review reasons survive worker
retries; conventional author spelling and known-series title decoration no longer
produce false identity conflicts. Conflicting ISBNs, volumes and formats remain
reviewable. Idle metadata retrieval uses the existing 24-hour timeout.

## Local verification

Full Go suite against disposable PostgreSQL, Go vet, 36 frontend tests, production
web build, deployment contracts and 48 desktop/mobile browser checks passed.
Regressions cover persisted review reasons and client identity, file presence,
client outages, metadata timeouts and recovery after a wanted revision changes.
Browser checks use intercepted acquisition responses.

## Live recovery

- The Lightning Thief and The Sea of Monsters were previewed and explicitly
  imported after embedded metadata and selected file mappings were checked.
- Steve Jobs was likewise verified and imported; its author used surname-first
  spelling. Extraneous text files remained excluded.
- A Brief History of Time's existing EPUB was verified and adopted in place for
  the current Hardcover entry. The old entry remains removed. The redundant
  torrent was removed without deleting files. Monitoring was disabled on the
  recovered entry to avoid further automatic acquisition of an unscored copy.
- The original stalled Labyrinth release was blocklisted. An approved alternate
  completed and imported automatically after obtaining peers; its payload is PDF.
  The failed original was removed without deleting files.

All five entries now have a present library file. The library contains seven
file records. Operational imports were the user-requested recovery work; automated
qualification did not start external downloads. No Kindle delivery was requested.

## Release verification

- Source: `bb3f5cbe7035e6d1038d708d608f633225c66036`.
- [PR #57](https://github.com/bandoracer/librarry/pull/57), merged at
  `4af64602adfc9279a9bbab22e2470fafc6699bbf`.
- [Candidate qualification](https://github.com/bandoracer/librarry/actions/runs/35695526937): all six jobs passed, including Go vet/PostgreSQL race tests,
  36 frontend tests, the production build, 177 browser passes with one existing
  skip, deployment contracts, Calibre, packaged imports/restore/workers/notifications,
  and the configured fixable HIGH/CRITICAL vulnerability gate.
- [Paired latest promotion](https://github.com/bandoracer/librarry/actions/runs/35697089017): passed; AMD64/ARM64 indexes verified.
- NAS readiness and health pass. API/web revision labels match the qualified source.
  Live search identity responses confirm all five repaired books have present files,
  and browser readback shows the three Percy Jackson rows as **In library**.
  The Labyrinth PDF remains eligible for a higher-quality upgrade; it is complete,
  not actively downloading. No affected entry has a pending import review.

| Component | Published index | Running AMD64 image |
| --- | --- | --- |

| api | `ghcr.io/bandoracer/librarry-api@sha256:5ee4866f4c854ffb56021ac5135d6e621a80192cd44955d0cb3cee8963ca5fb7` | `sha256:de249bf4e183e224ece5a6c7c24f71520d337e579023bf77dc50eb86f5eb0948` |
| web | `ghcr.io/bandoracer/librarry-web@sha256:4e04d70373bf79cb52d359a08fb2ecad026724c3daac5817a92e10c73b098cca` | `sha256:a404a70ad6e71819cf7c7733d0d2ce48894b3e1f6ca7530fd96e6cfdf2c59343` |

## Preservation

A validated database dump, private configuration and media hashes were captured
before deployment. Only API/web image references changed in the saved NAS config.
All seven file records, seven file/book links, manual overrides, 59 migration
records and seven media hashes matched after deployment. No schema change was
required. The previous image pair and private checkpoint remain available for
rollback.
