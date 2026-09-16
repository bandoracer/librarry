# Initial candidate operator review

September 16, 2026. Baseline: `7e5cc142c2f4b466ee4e5195c0fac0caf2a05d38`
(PR #50 plus publication isolation), followed by the two UI fixes in this change.
This is a scoped exploratory review, not completion of the release journey gate.

## Environment and observations

A fresh, uniquely named database on disposable Postgres 16, a local source-built
API and Vite UI, and an actual browser were used. Acquisition workers were
disabled, no download client or indexer was configured, and no release was
grabbed. The only external lookup was public Open Library metadata.

- The empty Library rendered without a crash and led to Add New.
- Searching `Frankenstein Mary Shelley` returned nine Open Library matches.
  The selected edition showed English language and source identity
  `openlibrary:OL46613456M`. Hardcover and Google credentials were absent;
  this does not qualify those providers or bibliographic accuracy.
- The low-confidence match required explicit review. Adding it stored one
  ebook, changed the action to Open book, and displayed the saved identity.
  Its book page correctly reported Missing and no recorded files.
- Search Releases with no Prowlarr configuration displayed only
  `Wanted release search failed: 502`. The API client discarded the server's
  setup reason. The fix now says to configure Prowlarr in Settings → Indexers
  and retry. This was verified against the real local API in the browser.
- Release-search and grab errors now retain other server explanations, including
  uncertain acceptance instructions, rather than reducing them to status codes.
  Eight focused API-client tests cover setup, provider outage, absent error body,
  and uncertain grabs without automatic resubmission.
- At 768×1024, the collapsed sidebar theme button lost its accessible name when
  CSS hid its text. An explicit `aria-label` fixes that; the browser subsequently
  resolves it by its accessible name at tablet width.
- The Indexers form was inspected at desktop, tablet and 390×844 phone width.
  Phone document width and scroll width were both 390px. Mobile navigation opened
  and exposed its named links and close control. Horizontally scrollable Settings
  tabs remain reachable, but their discoverability warrants further polish.

## Remaining release work

The fresh setup still requires operators to move among Settings tabs; there is
no coherent first-run guide. Book details put a long metadata editor/provenance
section ahead of release actions. Those are remaining usability priorities,
distinct from the two reproduced defects fixed here.

No indexer/client credentials, live download, complete browser-driven import,
or live NAS operation were exercised here. The packaged fixture suite separately
covers controlled ebook/chapter imports and failures; its result must be recorded
for the final candidate SHA. It does not replace real-service/platform evidence.

The source build reports development identity; final release qualification must
use paired image digests and embedded source identity. Production-copy upgrade,
rollback, platform runtime qualification and observation remain open in the
[release checklist](../release-checklist.md).
