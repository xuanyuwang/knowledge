## Summary

Consolidate real ClickHouse reader and full-RPC coverage into the existing test suites on the original production implementation. Replace mocked ClickHouse returns with seeded data and remove redundant regression suites, targets and obsolete SQL snapshots.

This is an independent two-PR stack: regression tests → refactor. No new Assistance Usage API integration, proto/schema changes, or behavior corrections.

## Test plan

- Original reader: 201.059 seconds, passed.
- Refactored reader: 204.089 seconds, passed.

```sh
DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock REUSABLE_CLICKHOUSE_CONTAINER_NAME= GOMAXPROCS=3 go test -p 2 ./insights-server/internal/analyticsimpl -run '^TestRetrieveSuggestionStats$' -count=1 -json
```

Frozen regression SHA256 (identical at both commits):

- `insights-server/internal/analyticsimpl/retrieve_suggestion_stats_test.go`: `ac15d03c68ea728860b010043c8d402a24f53bdf8c9951732d2944808ccfaa8a`

Preserved legacy behavior requiring separate review/correction:

- Default LEFT JOIN exclusion drops nonmatching rows: The joined String key defaults to empty string; IS NULL is false, so the row is dropped. The same fixture qualifies with join_use_nulls=1.
- Feature eligibility differs from conversation population: Legacy feature statistics retain it. Tests pin this behavior to avoid changing scope during extraction.

Validation covers local synthetic test containers. CI, staging/production and query performance are not claimed.

## AI knowledge files

No service public API, build command, schema, or domain contract changes.
