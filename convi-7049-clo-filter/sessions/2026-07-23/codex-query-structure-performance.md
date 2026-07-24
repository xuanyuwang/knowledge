# CONVI-7049 moment query structure and performance review

Date: 2026-07-23

## Context

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Branch/worktree context: `main`
- Change reviewed: `cresta/go-servers#29175`, merge commit `573b275f24`
- Assumption: “new moments” means the newly supported `CONVERSATION_OUTCOME` (CLO) moment filter, and “old moments” means the existing `CONVERSATION_METADATA` filter.

## Sources reviewed

- Generated SQL:
  - `insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveQAScoreStats_FilterByConversationOutcomeMomentGroup_request.sql`
  - `insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_request.sql`
- Query construction:
  - `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`
  - `insights-server/internal/analyticsimpl/common_clickhouse.go`
- ClickHouse schema:
  - `/Users/xuanyu.wang/repos/clickhouse-schema/conversations/migrations/20230824160348_init_db.up.sql`
- Historical evidence:
  - `go-servers` commit `02b29f87de` introduced the metadata materialized-view path.
  - `go-servers` commit `53fb9620b7` removed the deprecated raw-table metadata path after the metadata-view flag was enabled in every production cluster.
  - `go-servers` commit `fd351e3029` moved additional metadata filtering paths from `moment_annotation_d` to `moment_annotation_mv_by_metadata_d`.

## Query comparison

The outer QA score query is unchanged. Both filter types:

1. Build a `t1` CTE that finds the latest annotation timestamp per conversation.
2. Build a `t2` CTE that applies the selected value predicate.
3. Join `t1` to `t2` by conversation ID and latest timestamp.
4. Join the resulting conversation IDs into `scorecard_score_per_conversation`.

The important differences are inside the moment CTEs.

### Existing metadata moments

- Source: `moment_annotation_mv_by_metadata_d`
- Latest column: `update_time`
- The view is prefiltered to `moment_type = 19`.
- Values are typed columns:
  - `metadata_string_value`
  - `metadata_number_value`
  - `metadata_bool_value`
- The view has 7 projected columns and monthly partitioning.

### New CLO outcome moments

- Source: `moment_annotation_d`
- Latest column: `create_time`
- Adds `moment_type = 14`.
- Values are parsed at query time from the String payload column with:
  - `JSONExtractString`
  - `JSONExtractFloat`
  - `JSONExtractBool`
  - `JSONHas` for exact false/zero correctness
- The base table contains all moment types and roughly 50 columns. It is not declared with monthly partitioning in the canonical DDL.

## Performance assessment

There is a material structural performance regression risk for CLO queries:

- The new path scans the broad raw annotation table instead of the metadata-only materialized view.
- JSON parsing is performed per candidate row and repeated in both numeric-bin bounds.
- Each selected moment group still produces two table scans plus a self-join.
- The raw table primary key is `(hour, agent_user_id, policy_id, ...)`, which is not aligned with the query’s `moment_template_id` and outcome-value predicates.

There is a stronger mixed-filter risk:

```go
fetchFromMetadataView := !qaAttributeHasConversationOutcomeMomentGroup(req.FilterByAttribute)
```

This is a request-wide switch. If any CLO group exists, every metadata include/exclude group in the same request is also moved from `moment_annotation_mv_by_metadata_d` back to `moment_annotation_d`. This revives the raw metadata query path that commit `53fb9620b7` had explicitly removed after the optimized view was enabled in all production clusters.

An outcome group with no `OutcomeValueAttributes` can also flip the request-wide switch even though no outcome condition is emitted.

The generated SQL fixture for metadata filters is byte-for-byte unchanged before and after PR #29175 when no CLO group is present. Therefore:

- Metadata-only query performance should be unchanged.
- CLO-only queries use a more expensive storage and predicate path.
- Mixed CLO + metadata queries can regress both the new and old filter portions.

This is a structural conclusion, not a measured production latency result. No ClickHouse runtime or query-log access was used in this review.

## Recommended validation

Compare the following three requests for the same customer, time range, scorecard, grouping, and result cardinality:

1. One metadata filter.
2. One CLO filter.
3. The same metadata filter plus the CLO filter.

Capture `EXPLAIN indexes = 1` and `system.query_log` metrics:

- `read_rows`
- `read_bytes`
- `query_duration_ms`
- `memory_usage`
- selected partitions and granules

Use both a short range and a representative long Performance Insights range. The mixed case is the most important regression test.

## Recommended query design

Short term:

- Choose the annotation source per moment group, not once per request.
- Keep metadata CTEs on `moment_annotation_mv_by_metadata_d`.
- Use `moment_annotation_d` only for CLO CTEs.
- Route based on a parsed, valid group with value attributes rather than moment type alone.

Long term:

- Add a conversation-outcome materialized view with monthly partitioning and typed boolean/number/string columns.
- Align its ordering with the actual filters, including time and `moment_template_id`.
- Avoid repeated JSON extraction in query-time predicates.

