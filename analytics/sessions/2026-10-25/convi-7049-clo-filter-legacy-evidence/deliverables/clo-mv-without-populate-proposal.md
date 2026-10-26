# Proposal: Create the CLO Materialized View Without `POPULATE`

**Date:** 2026-07-29
**Related ticket:** [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via)
**Status:** Proposal for ClickHouse review

## Context

The Performance Insights CLO filter currently reads the broad `moment_annotation_d` table and parses JSON at query time. Preliminary production evidence supports a narrow, typed materialized view for `moment_type = 14`.

The immediate rollout is limited to selected large customers, where a single `POPULATE` scan has the greatest workload and recovery risk. Cresta's existing annotation MVs use `POPULATE`, but ClickHouse discourages that approach on a live source table: inserts arriving during population can be missed, and a large scan is difficult to throttle or resume.

The closest precedent is [`moment_annotation_mv_by_metadata`](https://github.com/cresta/clickhouse-schema/blob/main/conversations/migrations/20230824160348_init_db.up.sql#L598-L614): an inline `ReplicatedReplacingMergeTree` materialized view followed by the Distributed read table `moment_annotation_mv_by_metadata_d`. The CLO design should follow that familiar object shape unless ClickHouse reviewers identify a reason to separate storage from the trigger.

## Proposal

Create an inline materialized view, consistent with `moment_annotation_mv_by_metadata`, but omit `POPULATE`. Then backfill the required 180 days with controlled `INSERT INTO ... SELECT` operations.

1. Create `moment_annotation_mv_by_conversation_outcome` without `POPULATE`, using `ReplicatedReplacingMergeTree` and monthly partitions. New CLO inserts start flowing automatically.
2. Backfill the previous 180 days directly into the MV, one closed time range at a time. Start monthly and use smaller chunks for customers whose measured workload requires it.
3. Create `moment_annotation_mv_by_conversation_outcome_d` as the Distributed read table.
4. Validate coverage, duplicates, resource use, and query latency.
5. Enable `use_conversation_outcome_moment_annotation_materialized_view` only for each selected large customer after its backfill passes validation.

Illustrative shape:

```sql
CREATE MATERIALIZED VIEW moment_annotation_mv_by_conversation_outcome
ON CLUSTER 'conversations'
ENGINE = ReplicatedReplacingMergeTree(
    '/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/moment_annotation_mv_by_conversation_outcome',
    '{replica}',
    update_time
)
PARTITION BY toYYYYMM(conversation_start_time)
ORDER BY (toStartOfHour(conversation_start_time), conversation_id, moment_template_id)
AS
SELECT
    -- Narrow typed CLO columns, identifiers, timestamps, and version column
FROM moment_annotation
WHERE moment_type = 14;

INSERT INTO moment_annotation_mv_by_conversation_outcome
SELECT
    -- Exactly the same target projection as the MV SELECT
FROM moment_annotation
WHERE moment_type = 14
  AND conversation_start_time >= {chunk_start}
  AND conversation_start_time < {chunk_end};
```

The exact DDL, version column, and cluster-level execution method remain to be reviewed.

## Alternative: explicit `TO target_table`

If independent lifecycle management is valuable, create a target table first and define a separate trigger with `CREATE MATERIALIZED VIEW ... TO target_table`. The backfill then writes directly to that target table.

This is an operational alternative, not a backfill requirement. Both forms support manual chunked backfill. The `TO` form makes it easier to drop or replace the trigger without dropping stored data, but it adds another named object and diverges from the current metadata-MV pattern. It does not materially reduce the CPU, I/O, or total rows scanned during backfill.

## Why this approach

- Avoids the missed-insert window associated with `POPULATE`.
- Limits the initial scan to the required 180-day history.
- Makes backfill load observable, throttleable, and resumable.
- Allows validation per partition and per selected large customer before the read-path flag is enabled.
- Stays close to the existing metadata MV structure.
- Keeps retention separate: no TTL is proposed initially.

## Main risk: live/backfill overlap

Because the MV starts ingesting before the historical backfill finishes, a backfill range may overlap rows already delivered by the MV. `ReplacingMergeTree(update_time)` can eventually collapse rows with the same sorting key and version, but duplicates may remain visible before merges and in queries that do not use `FINAL`.

Proposed mitigation:

- Record the MV creation time and use deterministic half-open backfill ranges.
- Backfill old partitions first and the current partition last.
- Compare source and target counts both normally and with deduplication semantics.
- Do not enable the read-path flag until overlap behavior and post-merge counts are accepted.

## Rollout safeguards

- Test in staging and measure duration, rows read, memory, and disk growth.
- Roll out only to the selected large customers; this is not initially a fleet-wide migration.
- Apply in production off-peak, one customer database and one backfill chunk at a time.
- Pause or reduce chunk size if ClickHouse queues or latency degrade.
- Treat each chunk as independently retryable and record completed ranges.
- Keep the feature flag off until the MV and its 180-day coverage are validated.

## Feedback requested from ClickHouse reviewers

1. Is the existing inline metadata-MV shape appropriate for CLO, or is there a concrete lifecycle reason to prefer `TO target_table`?
2. What is the safest way to execute the backfill across shards without duplicating work across replicas?
3. Is eventual `ReplacingMergeTree` deduplication acceptable for the live/backfill overlap, or should we use a stricter cutoff or temporary-table/partition-attach procedure?
4. Should the version column be `update_time`, `create_time`, or another field for CLO's latest-value semantics?
5. Which validation queries and operational limits should gate rollout to each selected large customer?
6. Is monthly chunking appropriate, or should chunk size be based on measured rows/bytes for each customer?

## Recommendation

Proceed with a staging prototype using the familiar inline MV form without `POPULATE`. Use manual, measured chunking to control workload for the selected large customers. Treat `TO target_table` as an alternative if reviewers value independent trigger/storage lifecycle enough to justify diverging from the existing metadata-MV pattern. Do not finalize the production runbook until reviewers confirm shard execution and overlap/deduplication semantics.

## Reference

- [Detailed creation/backfill investigation](clo-mv-clickhouse-creation-investigation.md)
- [Existing inline metadata MV and Distributed table](https://github.com/cresta/clickhouse-schema/blob/main/conversations/migrations/20230824160348_init_db.up.sql#L598-L614)
- [ClickHouse materialized view documentation](https://clickhouse.com/docs/sql-reference/statements/create/view)
- [ClickHouse backfilling guidance](https://clickhouse.com/docs/data-modeling/backfilling)
