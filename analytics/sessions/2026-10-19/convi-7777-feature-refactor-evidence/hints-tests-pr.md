## Summary

Consolidate real ClickHouse reader and full-RPC coverage into the existing test suites on the original production implementation. Replace mocked ClickHouse returns with seeded data and remove redundant regression suites, targets and obsolete SQL snapshots.

This is an independent two-PR stack: regression tests → refactor. No new Assistance Usage API integration, proto/schema changes, or behavior corrections.

## Test plan

- Original reader: 389.661 seconds, passed.
- Refactored reader: 390.892 seconds, passed.

```sh
DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock REUSABLE_CLICKHOUSE_CONTAINER_NAME= GOMAXPROCS=3 go test -p 2 ./insights-server/internal/analyticsimpl -run '^TestRetrieveHintStats$' -count=1 -timeout 20m -json
```

Frozen regression SHA256 (identical at both commits):

- `insights-server/internal/analyticsimpl/retrieve_hint_stats_test.go`: `c2b7032c62b61b0ee987ad41a0c443f8b1771130314dd6759c5987d0d7deb519`

Preserved legacy behavior requiring separate review/correction:

- Mixed sent populations use MAX: Sent/conversation/user denominators are independent branch MAX values, not summed or union-distinct populations.
- Moment-based sent and followed can lack delivered actions: Default sent includes it; both followed paths count it. Action mode excludes it only from sent.
- Knowledge follows accept empty and unlinked action IDs: COUNT DISTINCT includes the empty string and IDs with no matching sent action.
- Metadata exclusion depends on nullable joins: Unmatched IDs are empty strings; both exclusion-only and combined filters return empty. Nullable joins yield intended filtered cohorts.
- Action mode loses behavior-filtered populations: Action table has no behavior column, so the existing false filter removes sent groups.
- Agent attribution can separate sent from followed: Action-mode recipient group has sent1/followed0; detector-only followed group is dropped by the sent-anchored LEFT JOIN.
- Conversation totals retain grouping multiplicity: Each agent group counts the conversation once and the response sums to2.
- Group/policy no-frequency per-user averages remain zero: Group-level per-user averages remain zero because grouped conversion derives its denominator from time buckets, which no-frequency reads omit. Top-level rates retain the original values. Both delivery modes are characterized by full-RPC assertions.

Validation covers local synthetic test containers. CI, staging/production and query performance are not claimed.

## AI knowledge files

No service public API, build command, schema, or domain contract changes.
