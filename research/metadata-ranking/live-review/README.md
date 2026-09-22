# Credentialed discovery review

`baseline.json` contains the 16 live NAS search responses that motivated this
iteration. Work descriptions were omitted; identity and edition evidence remain.
Queries used English and the UI's Any format default except the explicit audiobook
case. This is normalized application output, not raw upstream provider responses.

The metadata regression suite replays these candidates through the service sorter
and asserts preservation of the originally correct first results, promotion of the
complete DDIA work, separation of King Lear and companion material, and retention
of edition/series identities. It also continues the existing 50-query captured
provider benchmark.

`final-live.json` records a fresh read-only run using the candidate Go service and
production provider transport. It includes eight additional queries whose intended
books were declared before probing in `holdout-intents.json`. Those are diagnostic
holdouts, not independently judged relevance labels. `Educated` exposed an interim
ranking regression; after its fix it is a regression case, not an untouched holdout.

No library items or downloads were created. Credentials are not included. Search
results and provider availability can change. See the [review](../../../docs/reviews/2026-09-21-discovery-cleanup.md)
for qualification boundaries and remaining issues.
