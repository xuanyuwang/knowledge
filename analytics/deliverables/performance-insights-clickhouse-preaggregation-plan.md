# Performance Insights ClickHouse pre-aggregation plan

**Date:** 2026-11-04
**Status:** Investigation complete enough to choose the next experiment; production design is not yet approved
**Primary API:** `RetrieveQAScoreStats`

## Recommendation

Do not attach a simple daily aggregation materialized view directly to `score_d` or `scorecard_d`.

The idea is feasible only after the input is made correction-aware. The current source tables use `ReplacingMergeTree` and contain multiple source versions. An incremental ClickHouse materialized view fires for every inserted block; it does not receive the final replacement state and cannot automatically subtract an older scorecard version. A direct `agent x day x criterion` rollup can therefore double-count revised scorecards.

The best first proof is a two-layer design:

1. produce a correction-aware, append-only QA fact or signed delta for each finalized scorecard version;
2. aggregate that fact into a small number of shared `AggregatingMergeTree` tables, starting with the common Performance Insights shapes;
3. retain the current raw query for unsupported filters, metadata-moment paths, details, and parity fallback.

If changing ingestion is too large for the first release, benchmark a refreshable snapshot as the interim option. It can recompute latest-version-correct aggregates on a schedule, trading freshness and rebuild cost for simpler correctness.

## What ClickHouse supports

Official ClickHouse documentation confirms the main mechanics:

- [Incremental materialized views](https://clickhouse.com/docs/concepts/features/materialized-views/incremental-materialized-view.md) act as insert triggers over newly inserted blocks and shift work from reads to writes.
- [AggregatingMergeTree](https://clickhouse.com/docs/reference/engines/table-engines/mergetree-family/aggregatingmergetree) stores aggregate states. Inserts use `-State`; reads use the matching `-Merge` function.
- Background merges are asynchronous. Queries must merge aggregate states; `FINAL` is generally not the intended read pattern.
- The target sorting key should align with the grouping key.
- [Cascading materialized views](https://clickhouse.com/docs/concepts/features/materialized-views/cascading-materialized-views.md) forward the inserted block, not the final merged state of an upstream Replacing/Summing/Aggregating table.
- JOINs are allowed, but only inserts into the left-most source trigger the view. Updates to a joined lookup do not repair historical target rows, and large joins can slow ingestion.
- [Backfilling guidance](https://clickhouse.com/docs/guides/clickhouse/data-modelling/backfilling.md) advises against `POPULATE` for a live, sizable source. Use a cutoff, duplicate target/view, separately backfill, validate, and move partitions.
- [Refreshable materialized views](https://clickhouse.com/docs/materialized-view/refreshable-materialized-view) periodically rerun the full query and are suitable when incremental insert-trigger semantics cannot express the required correction.

So the storage pattern is supported, but the current source semantics prevent the simplest version of it from being correct.

## Current query and page shape

`RetrieveQAScoreStats` does more than average score rows. Current backend code:

- chooses the latest scorecard source version;
- joins score rows on both `scorecard_id` and `scorecard_last_update_time`;
- applies conversation population, version, ACL, time-basis, score-resource, status, voicemail, duration, user, criterion, template, use-case, and metadata-moment logic;
- supports groupings including time range, criterion, agent, agent tier, QA analyst, appeal actors, and submitter.

The Performance Progression component can issue several `RetrieveQAScoreStats` calls for one page:

- criterion x time-range cells;
- whole-template time-range totals;
- agent x criterion x time-range data for CSV;
- one time-range request per chapter;
- criterion x agent-tier requests for Top Agents;
- criterion x agent requests for the optional AI Agent row.

This means one page can amplify an expensive query shape. It also means a single target key containing every possible filter and grouping dimension would likely preserve too many rows to deliver strong compression.

## RCG production measurements

Access was confirmed read-only against `rcg_us_east_1` on the `us-east-1-prod` conversations cluster. No tables, views, or customer data were changed.

### Existing storage

| Table | Approximate rows | Compressed size | Engine |
|---|---:|---:|---|
| `score` | 119.25M | 6.27 GiB | `ReplicatedReplacingMergeTree(..., update_time)` |
| `scorecard` | 31.97M | 2.19 GiB | `ReplicatedReplacingMergeTree(..., update_time)` |
| `conversation` | 3.80M | 472 MiB | `ReplicatedReplacingMergeTree(..., update_time)` |

The connected cluster reports 6,479 attached tables, already above its 5,000-table warning threshold. Creating a separate table and view for every API/customer would increase operational cost; prefer a few semantic tables shared by compatible query shapes.

### Last 90 days

| Measurement | Result |
|---|---:|
| Physical scorecard rows | 13,318,471 |
| Approximate unique scorecards | 12,855,818 |
| Active scoring use cases | 71 |
| Approximate agents | 3,879 |
| Approximate templates | 407 |
| Physical score rows | 58,621,918 |
| Approximate unique score IDs | 54,075,463 |
| Approximate criteria | 4,406 |

Use-case volume is highly skewed. The median use case had about 13,250 scorecard rows, p90 about 409,113, and the largest about 4.44M. RCG may have roughly 100 configured use cases, but only 71 had scorecards in this measured window.

### Candidate rollup cardinality

These figures are cardinality estimates, not stored-size or latency benchmarks:

| Candidate grain | Distinct keys | Row reduction from 58.62M physical score rows |
|---|---:|---:|
| day x use case x agent x template x criterion | 5,642,545 | about 10.4x |
| hour x use case x agent x template x criterion | 23,479,093 | about 2.5x |
| daily grain plus submitter, manual/auto, voicemail, duration bin | 21,481,482 | about 2.7x |

The 10.4x form is attractive but cannot answer all current filters. Adding only four common dimensions reduces the row benefit to about 2.7x, before aggregate-state storage overhead.

### Version behavior

Across the full 90-day population, approximate distinct `(scorecard_id, scorecard_last_update_time)` versions were 13,362,313 for 12,856,052 scorecard IDs, or 1.039 versions per ID.

A deterministic 1% scorecard-ID sample contained 128,675 IDs:

- 4,051 (3.15%) had more than one parent source version;
- maximum observed versions per ID was 3;
- agent, use case, template, scorecard day, manual mode, voicemail, and duration bin did not change within the sampled IDs.

A deterministic 1% score-ID sample contained 541,348 score IDs:

- 29,232 (5.40%) appeared under more than one parent version;
- only 38 score IDs changed their sampled numeric/N/A/scorecard-score tuple;
- maximum parent versions per score ID was 4.

Stable dimensions make a correction-aware rollup more promising, but they do not make a direct insert-trigger rollup correct: republished versions still add rows again.

### Recent query-log evidence

On the connected node, 36,903 finished initial queries in the last seven days matched the current `filtered_scorecard` QA-stats shape:

| Metric | Result |
|---|---:|
| p50 | 0.39s |
| p95 | 2.01s |
| p99 | 4.35s |
| maximum | 26.27s |
| average rows read | 36.42M |
| average bytes read | 1.31 GiB |
| average memory | 410 MiB |

This is evidence of high scan cost, not a complete latency baseline. It covers one reachable node and finished queries only, and excludes canceled/failed requests. It does not invalidate the earlier browser evidence of four parallel 47-50s calls or the earlier observability tail; those requests may have occurred on another node, waited outside the recorded query, or ended without `QueryFinish`.

## Proposed data design

### Layer 1: correction-aware QA fact

Preferred contract: emit one append-only fact or signed delta when a scorecard version becomes the authoritative analytics version.

Minimum identity and dimensions:

```text
scorecard_id
scorecard_source_version
score_id / criterion_id
scorecard_time, submit_time, conversation_start_time
usecase_id, template_id, template_revision
agent_user_id, submitter_user_id
manual/auto, voicemail, duration_bin
numeric value, percentage value, N/A, weight, max value, autofail state
sign or replacement identity
```

The producer must define how a later version retracts or supersedes the earlier version. Do not derive this only from asynchronous ReplacingMergeTree merges.

### Layer 2: shared rollups

Start with two target shapes rather than one table per API:

1. **QA criterion daily rollup** for criterion/time, agent/criterion, and agent-tier-compatible queries. Candidate key: `(profile, day, usecase, template, criterion, agent, score_resource, manual_mode)` plus only dimensions proven necessary by request telemetry.
2. **QA scorecard daily rollup** for whole-template totals and autofail/scorecard-resource semantics, where criterion-level states are insufficient or would give the wrong weighting.

Store mergeable numerators, denominators, counts, weight sums, N/A counts, exception/overwrite counts, and distinct-count states only where their exact semantics have been proved. Do not store a finalized average.

Keep dynamic team/group membership out of the rollup key. Resolve the authorized/user population at query time and filter/join on `agent_user_id`, otherwise membership changes would require historical rewrites. Metadata moments should initially use the raw path unless a stable low-cardinality projection is designed separately.

### API routing

Route only a request that the rollup can answer exactly. The backend should make the decision explicit and observable:

```text
supported grain + supported filters + supported grouping
  -> rollup query
otherwise
  -> current raw query
```

Log the route, request-shape fingerprint, rows/bytes read, latency, and fallback reason. This prevents a fast path from silently changing ACL, latest-version, N/A, weighting, or time-basis behavior.

## Alternatives

| Option | Correctness | Expected reduction | Freshness | Main cost |
|---|---|---|---|---|
| Direct incremental MV on current tables | Not acceptable | High | Near-real-time | Double-counts versions/corrections |
| Append-only finalized/signed QA fact plus rollups | Best long-term | High | Near-real-time | Ingestion contract and correction protocol |
| Latest entity-state table, aggregate at query time | Correct if replacement is explicit | Moderate | Near-real-time | Less row reduction; still aggregates many entities |
| Refreshable latest-state snapshot | Correct per refresh query | High | Scheduled | Full recomputation and freshness delay |
| Hybrid immutable/final rollup plus raw fallback | Good if finality is rigorous | High for common path | Mixed | More routing and parity testing |

## Execution plan

### 1. Capture request shapes

For at least seven representative days, record normalized `RetrieveQAScoreStats` request shapes and correlate them with ClickHouse query IDs. Measure calls per page load, selected use-case count, time range, groupings, filters, chapters, latency, rows/bytes, memory, failures, and queue/wait time. Separate cold/warm and finished/canceled queries.

Deliverable: the smallest set of shapes covering at least 80-90% of Performance Insights traffic and the exact slow shapes from CONVI-7681.

### 2. Prove correction semantics

Trace scorecard projection writers and confirm whether a finalized version event or signed replacement can be emitted. Test create, edit, resubmit, publish, manual override, AutoQM overwrite, N/A, delete/void, late arrival, and out-of-order delivery.

Exit condition: replaying those events gives the same latest score and counts as the current ID-plus-source-version query.

### 3. Build a non-production prototype

In an approved test cluster or disposable database:

- create the correction-aware fact and one daily criterion rollup;
- backfill a fixed RCG-like 90-day slice using a cutoff, never `POPULATE` on live ingestion;
- capture actual compressed bytes, parts, merge time, insert amplification, and backfill duration;
- benchmark single-use-case and 3/10/50-use-case requests, common date ranges, agent/criterion/time groupings, and concurrent page loads.

Do not use the 10.4x row estimate as a storage or speed claim. Measure it.

### 4. Parity test the API

Run old and candidate queries over the same frozen requests and compare complete responses. Include ACL-restricted users, dynamic groups, time-basis choices, metadata inclusion/exclusion, mixed manual/AutoQM, revisions, autofail, N/A, weighting, and chapter/whole-template totals.

Required result: exact parity or a documented intentional product change. Any mismatch keeps that shape on the raw path.

### 5. Load and failure testing

Measure p50/p95/p99, rows/bytes, ClickHouse and Go memory, CPU, insert latency, parts/merge backlog, and concurrent page fan-out. Test delayed blocks, duplicate delivery, out-of-order corrections, target lag, rebuild, and rollback.

Initial performance target for discussion: p95 under 5s for the known common page requests and no regression to ingestion SLOs. Set the final target from measured current traffic and product expectations.

### 6. Controlled rollout

Backfill with a cutoff into duplicate tables, validate counts/checksums, then move partitions or switch routing. Start with shadow reads, then a small profile allowlist, with automatic fallback. Monitor latency, parity mismatches, lag, merges, parts, and insert cost. Keep the raw route until the backfill and correction protocol have survived production updates.

## Decisions still needed

1. Can ingestion emit an authoritative finalized or signed scorecard-version event?
2. What freshness is acceptable if a refreshable snapshot is the interim choice?
3. Which request shapes must be on the first fast path, based on telemetry rather than API capability alone?
4. Are metadata-moment filters allowed to remain on the raw path initially?
5. What p95/p99 and ingest-overhead thresholds define success?

## Bottom line

RCG has enough volume and enough repeated aggregation to justify a prototype. A simple daily grain could reduce row cardinality by about 10x for a restricted query shape, while a more filter-complete grain measured only about 2.7x. The production query log confirms large scans even when latency is currently low on the observed node.

The materialized-view idea is viable, but the first engineering task is not creating the view: it is establishing a correction-aware input contract and choosing the few request shapes that preserve both correctness and useful compression.
