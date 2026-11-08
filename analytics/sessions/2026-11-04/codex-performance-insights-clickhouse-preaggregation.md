# Performance Insights ClickHouse pre-aggregation investigation

## Request

Evaluate whether ClickHouse materialized views and aggregate target tables can make Performance Insights queries faster for multi-use-case templates. Confirm RCG access, use real volume evidence, and produce a concrete plan without changing production data or product code.

## Access and safety

- Resolved `configv3/prod/rcg/us-east-1/config.yaml` from verified `origin/master` commit `95943b096b70394c3ee7254f2bd13e2a401fc5c8`.
- Connected read-only to the `us-east-1-prod` conversations cluster, database `rcg_us_east_1`.
- Used AWS SSO profile `us-east-1-prod_dev`; its exact config entry was inspected before use and had no restricted marking.
- The config checkout could not fetch normally because remote refs differ only by case on a case-insensitive filesystem. Verified the local `origin/master` against `git ls-remote`, then used a temporary single-branch clone with the approved customer ClickHouse helper.
- Executed `SELECT`, `DESCRIBE`, `SHOW`, and bounded `system.query_log` reads only. Created no table/view and changed no customer data.

## Source reviewed

- `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`
- `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`
- Director Performance Progression component and QA-score request hooks
- Current score/scorecard/conversation DDL in RCG
- ClickHouse official incremental MV, AggregatingMergeTree, cascade, backfill, rollup, and refreshable MV documentation

## Findings

1. Incremental MVs are feasible insert triggers, but do not see final ReplacingMergeTree state.
2. `score`, `scorecard`, and `conversation` are versioned ReplacingMergeTree tables. Current API semantics explicitly select the latest scorecard source version and join scores by ID plus source timestamp.
3. A naive aggregate view would add every inserted version and cannot subtract the old one. Cascading through a replacement table does not fix this because cascades forward inserted blocks.
4. RCG's last 90 days contain 13.32M physical scorecard rows and 58.62M physical score rows across 71 active scoring use cases.
5. Candidate key counts imply about 10.4x row reduction for a narrow daily agent/template/criterion/use-case grain, 2.5x hourly, and 2.7x for daily plus four common filter dimensions.
6. A deterministic 1% scorecard sample found 3.15% with multiple parent versions. Basic grouping dimensions remained stable in that sample, but republished versions still require correction-aware counting.
7. Performance Progression sends multiple QA-stats shapes and can fan out by chapter. It also issues top-agent and optional AI-agent groupings. One universal high-dimensional rollup is unlikely to retain strong compression.
8. One reachable node's seven-day query log had 36,903 finished matching queries: p50 0.39s, p95 2.01s, p99 4.35s, max 26.27s, with average scan 36.42M rows / 1.31 GiB and 410 MiB memory. This excludes canceled/failed queries and other nodes and does not explain away the earlier 47-50s page evidence.
9. RCG already has 6,479 attached tables, so table-per-API proliferation is undesirable.

## Proposed direction

Prefer a correction-aware append-only or signed QA fact followed by a small number of shared daily aggregate tables. Begin with criterion-level and scorecard-level rollups for the common Performance Insights request shapes. Keep dynamic membership at query time and route unsupported metadata/detail shapes to the current raw query.

If ingestion changes cannot lead, prototype a refreshable latest-state snapshot with an explicit freshness target.

## Evidence limits

- Cardinality is not compressed storage size or speedup.
- No physical target table was built, so no target-table query benchmark exists.
- Query-log evidence is from one reachable ClickHouse node and finished queries only.
- The direct API query includes more joins and semantics than the standalone population measurements.
- RCG's approximately 100 configured use cases was not verified; 71 had scorecards in the measured 90-day window.

## Durable output

- [Engineering plan](../../deliverables/performance-insights-clickhouse-preaggregation-plan.md)
- Updated [CONVI-7681](../../work-items/CONVI-7681.md)

## Next action

Instrument normalized request shapes and correlate the known slow page requests to query IDs. In parallel, trace the projection writer to decide whether it can emit an authoritative finalized/signed scorecard version. Only then build the first non-production rollup and measure actual parity, storage, query latency, and ingestion overhead.
