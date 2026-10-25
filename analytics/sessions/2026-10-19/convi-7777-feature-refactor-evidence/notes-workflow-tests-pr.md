## Summary

Consolidate real ClickHouse reader and full-RPC coverage into the existing test suites on the original production implementation. Replace mocked ClickHouse returns with seeded data and remove redundant regression suites, targets and obsolete SQL snapshots.

This is an independent two-PR stack: regression tests → refactor. No new Assistance Usage API integration, proto/schema changes, or behavior corrections.

## Test plan

- Original reader: 150.708 seconds, passed.
- Refactored reader: 148.66 seconds, passed.

```sh
DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock REUSABLE_CLICKHOUSE_CONTAINER_NAME= GOMAXPROCS=3 go test -p 2 ./insights-server/internal/analyticsimpl -run '^TestRetrieve(NoteTaking|GuidedWorkflow)Stats$' -count=1 -timeout 20m -json
```

Frozen regression SHA256 (identical at both commits):

- `insights-server/internal/analyticsimpl/common_notes_workflow_usage_test.go`: `c34ae44465de43c2ecde84f0a24bf6dd4d155620949549bcc9e48bc943fb9620`
- `insights-server/internal/analyticsimpl/retrieve_note_taking_stats_test.go`: `3b59601489966cee0a796851ef6e67533354e9d98b0405d24a1de3dfdd242d85`
- `insights-server/internal/analyticsimpl/retrieve_guided_workflow_stats_test.go`: `b9956ca12ccabda8ff5bbbd41b5f12ad8aedeafc45cec4de471e695c73aabb2a`

Preserved legacy behavior requiring separate review/correction:

- Metadata exclusions return zero aggregate in this test environment: Zero usage/conversation counts despite a nonexcluded matching conversation. SQL checks joined exclusion ID IS NULL; default nonnullable joins are a likely explanation, not separately verified deployment configuration.
- Empty or missing Guided Workflow session IDs count as use: One distinct empty session key contributes one use.
- Ungrouped year-1 timestamp: Row TimeRange.StartTimestamp is 0001-01-01T00:00:00Z.

Validation covers local synthetic test containers. CI, staging/production and query performance are not claimed.

## AI knowledge files

No service public API, build command, schema, or domain contract changes.
