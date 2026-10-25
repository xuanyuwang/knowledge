## Summary

Extract reusable feature row selection while keeping the legacy RPC aggregation, grouping, arguments and response conversion. The parent PR adds regression tests against the original implementation; this child retains those test files byte-for-byte.

This is an independent two-PR stack: regression tests → refactor. No new Assistance Usage API integration, proto/schema changes, or behavior corrections.

## Test plan

- Original reader: 471.971 seconds, passed.
- Refactored reader: 473.421 seconds, passed.

```sh
DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock REUSABLE_CLICKHOUSE_CONTAINER_NAME= GOMAXPROCS=3 go test -p 2 ./insights-server/internal/analyticsimpl -run '^(TestRetrieveSummarizationStats|TestRetrieveKnowledgeAssistStatsClickhouse)$' -count=1 -timeout 20m -json > /tmp/convi7777-summary-genai-merged-before.jsonl 2>&1
```

Frozen regression SHA256 (identical at both commits):

- `insights-server/internal/analyticsimpl/retrieve_summarization_stats_test.go`: `618bab4893f0148ba6c6d1e6236b6aa8600f58dccd86832d6b75dd6ea4dc6914`
- `insights-server/internal/analyticsimpl/retrieve_knowledge_assist_stats_test.go`: `967b7842bf1c80b54c935cbdde78932e7ce33267afad83b6138510a4de1619db`

Preserved legacy behavior requiring separate review/correction:

- Metadata exclusion drops rows with no excluded metadata under default joins: Both Summary and GenAI return an empty response on the original code: LEFT JOIN fills the unmatched string key with its default and the existing IS NULL check does not pass.
- GenAI denominator differs across response levels: Bucket averages .5/1; result average1; response average2.
- GenAI annotation-only groups are dropped: No result group, despite feature annotations existing.
- Summary eligibility differs from GenAI: Summary counts generated=used=1; GenAI separately rejects source7/dev evidence.

Expanded real-RPC coverage caught a GenAI metadata include/exclude extraction regression: joined wildcard output hid the agent/time columns required by the outer aggregate. The refactor uses explicit canonical projections; the original tests and expected responses remain unchanged. Failed and passing evidence are recorded in the construction report.

Validation covers local synthetic test containers. CI, staging/production and query performance are not claimed.

## AI knowledge files

No service public API, build command, schema, or domain contract changes.
