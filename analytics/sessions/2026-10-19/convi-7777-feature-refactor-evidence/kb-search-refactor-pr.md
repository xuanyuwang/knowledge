## Summary

Extract reusable feature row selection while keeping the legacy RPC aggregation, grouping, arguments and response conversion. The parent PR adds regression tests against the original implementation; this child retains those test files byte-for-byte.

This is an independent two-PR stack: regression tests → refactor. No new Assistance Usage API integration, proto/schema changes, or behavior corrections.

## Test plan

- Original reader: 186.292 seconds, passed.
- Refactored reader: 186.539 seconds, passed.

```sh
DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock REUSABLE_CLICKHOUSE_CONTAINER_NAME= GOMAXPROCS=3 go test -p 2 ./insights-server/internal/analyticsimpl -run '^TestRetrieveKnowledgeBaseStats$' -count=1 -json
```

Frozen regression SHA256 (identical at both commits):

- `insights-server/internal/analyticsimpl/retrieve_knowledge_base_stats_test.go`: `17cb995d15b0954e43d2f0d9a2158e53644b7fba314dc0a0c4f647be90dd6ce2`

Preserved legacy behavior requiring separate review/correction:

- Default LEFT JOIN exclusion drops nonmatching rows: The joined String key defaults to empty string; IS NULL is false, so the row is dropped. The same fixture qualifies with join_use_nulls=1.
- Feature eligibility differs from conversation population: Legacy feature statistics retain it. Tests pin this behavior to avoid changing scope during extraction.

Validation covers local synthetic test containers. CI, staging/production and query performance are not claimed.

## AI knowledge files

No service public API, build command, schema, or domain contract changes.
