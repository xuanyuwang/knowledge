# Direct ClickHouse measurement session

**Date:** 2026-07-28
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Target:** production conversations ClickHouse, database `nclh_us_east_1`

## Requested measurement

- Use the supplied six-month Performance Insights request containing a boolean-true CLO filter.
- Query ClickHouse directly.
- Set the client/server execution timeout to 120 seconds to match the frontend threshold.
- Start with concurrency 1 and use the slowest `qaScoreStats:retrieve` request/query shape as the page-performance proxy.

## Credential safety

- Reviewed `xuanyu-scripts/connect-clickhouse.zsh` after checking it for BREAK GLASS markers.
- The shared AWS credentials file contains one explicitly labeled BREAK GLASS profile, `full-access-uDzyYi7`; it was excluded and not read or used.
- The target config profile `us-east-1-prod_dev` is a separate generated AWS profile and is not labeled BREAK GLASS.
- The intended ClickHouse credential is `clickhouse/us-east-1-prod/users/admin`; the command was designed not to print or persist it.

## Execution status

The first read-only connectivity command was rejected before execution because the escalation approval service returned an internal unsupported-model error. After explicit user approval, the retry succeeded.

## Measurements completed

- Connected read-only to ClickHouse 26.3.12.3 and database `nclh_us_east_1`.
- Recovered the exact supplied CLO query from `system.query_log`; it depends on an external `agent_filter`, so it was not blindly replayed.
- Aggregated recent page-load CLO requests and found a maximum of 85.83 s.
- Located a no-CLO query with the same date, scorecard, use case, daily grouping, voicemail conversation filter, and result cardinality; it completed in 66.67 s.
- Ran distributed and local `EXPLAIN indexes = 1`.
- Ran one tagged self-contained annotation CTE measurement and two paired count microqueries, all with 120-second limits.
- Collected coordinator and shard ProfileEvents from `system.query_log`.

All ClickHouse queries completed without exception. No writes, cache clearing, forced merges, `FINAL`, or concurrency load were used.

Detailed results: `deliverables/clo-filter-performance-results-2026-07-28.md`.
