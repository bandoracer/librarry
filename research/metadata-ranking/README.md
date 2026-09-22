# Metadata ranking research

Frozen read-only research corpus, 2026-09-21 UTC. Its Python experiment performs no
acquisition, server configuration, database writes, or deployment. The subsequent
Go implementation replays these fixtures through its real search pipeline; see
[implementation evidence](../../docs/reviews/2026-09-21-metadata-discovery.md).

- [Expanded 34-query cohort and current 49/50 combined result](expansion/README.md)
- [Design and implementation sequence](../../docs/metadata-ranking-design.md)
- [Measured results](RESULTS.md), [full explanations](results.json)
- [Query intents](cases.json), [manual positive-target labels and sources](labels.json)
- [Provider configuration evidence](provider-status.json)
- [Snapshot checksums and source commit](manifest.json)

The corpus contains direct Open Library search responses, exact work/edition
audits, and live Librarry API responses. Each snapshot records URL, UTC retrieval
time, response status/error and elapsed time. The failed unauthenticated Google
ISBN request is retained as a failure, not a zero-result benchmark. No credentials
are included. Hardcover could not be API-benchmarked without its token.

Reproduce offline with Python 3, no third-party dependencies:

```sh
python3 research/metadata-ranking/evaluate.py
python3 -m unittest discover -s research/metadata-ranking -p 'test_*.py' -v
```

`rank.py` never reads labels. `evaluate.py` calculates positive-target Top-1,
Hit@10 and reciprocal rank capped at 10. It does not claim unjudged items are
irrelevant; there is no reported precision or nDCG. The first 12 queries were
inspected during research; the four cases documented in `blind-intents.md` were
retrieved after weights were frozen. There was no weight tuning after evaluation.
Labels were checked against author/publisher sources and catalog identities;
ambiguous duplicate work records receive explicit treatment in labels/design.

Paths compared:

1. `live`: captured deployed API output with default English/Any settings.
2. `same_pool_rerank`: only surviving live IDs, enriched from direct snapshots;
   cannot recover excluded or unretrieved works. This is a generous rerank ablation.
3. `title_language_fix`: first ten direct title candidates, selected-edition
   language eligibility, legacy lexical score. ISBN and author use their proper
   routes. This is an offline approximation, not a deployed variant.
4. `retrieval_only`: direct general-search provider order with edition-language
   eligibility; author/ISBN use their specific routes.
5. `candidate_v1`: q/title union, bounded quality adjustments, identical eligibility.
6. `without_cover`: same prototype with cover bonus zeroed.

The direct APIs fetched 40 candidates per branch. This broadens the research pool;
production request-size and latency choices remain design gates. No query-specific
book IDs, expected author names or series positions enter the prototype. The
prototype lacks full production author/format/edition gating; read the design
before implementing. Six synthetic contract tests supplement, but do not count
as real-query benchmark successes.

`collect.py` can fill missing snapshots online at no more than one request start
per 1.1 seconds. It retains existing responses, including errors, and has no retry
loop. It targets the maintainer LAN API for live baseline capture. For a genuinely
fresh cohort, copy the script to a new research directory rather than overwrite
this corpus. Do not silently mutate frozen fixtures or labels to improve metrics.

The checked-in data is bibliographic metadata, not book files. Some source audit
records contain catalog descriptions; the experiment does not ingest book text.
