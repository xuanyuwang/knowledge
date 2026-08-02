# Proposal: CLO Materialized View Using `TO target_table`

**Date:** 2026-07-30
**Related tickets:** [CONVI-7383](https://linear.app/cresta/issue/CONVI-7383/improve-clo-conversation-outcome-filter-query-performance-via), [CONVI-7049](https://linear.app/cresta/issue/CONVI-7049/support-clo-filter-in-performance-insights)
**Status:** Recommended approach after ClickHouse review
**Precedent:** [INSI-4097](https://github.com/cresta/clickhouse-schema/blob/fdc0b755e2297698fd9a544ff8456567bd6e8c39/conversations/scripts/run_queries_in_existing_database/history_queries/queries_fix_metadata_moment_value_count_replicated.sql)

## Summary

Create a CLO materialized view using the **`TO target_table`** pattern already used in production for `metadata_moment_value_count_mv` (INSI-4097):

1. Create a **replicated storage table** that holds the projected CLO rows.
2. Attach a **trigger MV** with `TO storage_table` and **no `POPULATE`** so live inserts start immediately.
3. **Backfill history** with explicit `INSERT INTO storage_table SELECT ...` (180 days, chunked for large customers).
4. Create the **Distributed read table** pointing at the storage table, not the trigger.
5. Enable the Insights flag only for each selected large customer after validation.

This separates storage from the trigger. We can drop and recreate the trigger without losing backfilled data, which is the main reason to prefer this over the older inline `ENGINE = ... POPULATE` pattern used by `moment_annotation_mv_by_metadata`.

## Problem

Performance Insights CLO filters read `moment_annotation_d`, which contains all moment types. For large customers, parsing JSON from the broad table is a measured bottleneck. Beta shipped without a CLO-specific MV.

Initial rollout is limited to **selected large customers**, not a fleet-wide migration.

## Why `TO target_table` (INSI-4097 precedent)

In [INSI-4097](https://github.com/cresta/clickhouse-schema/blob/fdc0b755e2297698fd9a544ff8456567bd6e8c39/conversations/scripts/run_queries_in_existing_database/history_queries/queries_fix_metadata_moment_value_count_replicated.sql), Cresta replaced a broken inline MV with:

| Object | Role |
|---|---|
| `metadata_moment_value_count` | Replicated storage table (`ReplicatedAggregatingMergeTree`) |
| `metadata_moment_value_count_mv` | Trigger only: `CREATE MATERIALIZED VIEW ... TO metadata_moment_value_count` |
| `metadata_moment_value_count_mv_d` | Distributed read table over the **storage** table |

The runbook explicitly drops and recreates only the trigger MV while keeping the target table and its data. That lifecycle benefit is the reason to follow this pattern for CLO.

Contrast with the older inline metadata annotation MV:

```sql
-- Existing inline pattern (moment_annotation_mv_by_metadata)
CREATE MATERIALIZED VIEW moment_annotation_mv_by_metadata
ENGINE = ReplicatedReplacingMergeTree(...)
POPULATE AS
SELECT ...
FROM moment_annotation
WHERE moment_type = 19;
```

That pattern couples storage and trigger. Dropping the MV drops the data unless `POPULATE` is used again.

## Proposed object model for CLO

| Object | Engine / role |
|---|---|
| `moment_annotation_by_conversation_outcome` | `ReplicatedReplacingMergeTree(..., update_time)` — durable storage |
| `moment_annotation_mv_by_conversation_outcome` | Trigger MV: `TO moment_annotation_by_conversation_outcome`, no `POPULATE` |
| `moment_annotation_mv_by_conversation_outcome_d` | `Distributed` over the **storage** table |

Naming follows INSI-4097: storage without `_mv`, trigger keeps `_mv`, distributed keeps `_mv_d` but reads storage.

The go-servers test schema and distributed local table name will need to point at `moment_annotation_by_conversation_outcome` instead of treating the inline MV as storage. The read-path table name stays `moment_annotation_mv_by_conversation_outcome_d`.

Filter: `moment_type = 14` (Cresta Modeled Outcome / conversation outcome moments).

## Rollout sequence

Applies per customer database, off-peak, starting with staging then selected large prod customers.

```text
1. DROP Distributed _d (if recreating)
2. DROP old inline MV SYNC (if present)
3. CREATE storage table moment_annotation_by_conversation_outcome
4. CREATE trigger MV ... TO moment_annotation_by_conversation_outcome
5. CREATE Distributed _d AS storage table
6. Backfill 180 days into storage table (chunked)
7. Validate counts, overlap, query latency
8. Enable use_conversation_outcome_moment_annotation_materialized_view for that customer
```

## Illustrative DDL

Column projection matches the draft in `go-servers` test schema. Exact typed-column semantics remain open (see [open questions](#open-questions)).

```sql
-- 1. Drop distributed first.
DROP TABLE IF EXISTS {DATABASE}.`moment_annotation_mv_by_conversation_outcome_d`
  ON CLUSTER 'conversations' SYNC;

-- 2. Drop any prior inline MV.
DROP TABLE IF EXISTS {DATABASE}.`moment_annotation_mv_by_conversation_outcome`
  ON CLUSTER 'conversations' SYNC;

-- 3. Replicated storage table.
CREATE TABLE IF NOT EXISTS {DATABASE}.`moment_annotation_by_conversation_outcome`
  ON CLUSTER 'conversations'
(
    `conversation_id` String,
    `conversation_start_time` DateTime64(6),
    `moment_template_id` String,
    `outcome_string_value` String,
    `outcome_number_value` Float64,
    `outcome_bool_value` Bool,
    `outcome_value_type` String,
    `update_time` DateTime64(6)
)
ENGINE = ReplicatedReplacingMergeTree(
  '/clickhouse/tables/{cluster}-{shard}/{database}/v0.24/moment_annotation_by_conversation_outcome',
  '{replica}',
  update_time
)
PARTITION BY toYYYYMM(conversation_start_time)
ORDER BY (toStartOfHour(conversation_start_time), conversation_id, moment_template_id);

-- 4. Trigger MV (forward-fills from now on; no POPULATE).
CREATE MATERIALIZED VIEW IF NOT EXISTS {DATABASE}.`moment_annotation_mv_by_conversation_outcome`
  ON CLUSTER 'conversations'
  TO {DATABASE}.`moment_annotation_by_conversation_outcome`
AS
SELECT
    conversation_id,
    conversation_start_time,
    moment_template_id,
    JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') AS outcome_string_value,
    JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') AS outcome_number_value,
    JSONExtractBool(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') AS outcome_bool_value,
    multiIf(
        JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value'), 'string',
        JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value'), 'number',
        JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value'), 'boolean',
        ''
    ) AS outcome_value_type,
    update_time
FROM {DATABASE}.`moment_annotation`
WHERE moment_type = 14;

-- 5. Distributed read table over storage (not the trigger).
CREATE TABLE IF NOT EXISTS {DATABASE}.`moment_annotation_mv_by_conversation_outcome_d`
  ON CLUSTER 'conversations'
  AS {DATABASE}.`moment_annotation_by_conversation_outcome`
  ENGINE = Distributed(
    '{cluster}',
    '{DATABASE}',
    'moment_annotation_by_conversation_outcome',
    toUnixTimestamp(conversation_start_time)
  );
```

## Backfill (180 days, no `POPULATE`)

INSI-4097 backfilled full history in one pass from `moment_annotation_d`. For CLO on large customers, use the same **`INSERT INTO storage_table SELECT ...`** mechanism but **chunk by time** to control workload.

```sql
INSERT INTO {DATABASE}.`moment_annotation_by_conversation_outcome`
SELECT
    conversation_id,
    conversation_start_time,
    moment_template_id,
    JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') AS outcome_string_value,
    JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') AS outcome_number_value,
    JSONExtractBool(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') AS outcome_bool_value,
    multiIf(
        JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value'), 'string',
        JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value'), 'number',
        JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value'), 'boolean',
        ''
    ) AS outcome_value_type,
    update_time
FROM {DATABASE}.`moment_annotation_d`
WHERE moment_type = 14
  AND conversation_start_time >= {chunk_start}
  AND conversation_start_time < {chunk_end};
```

Guidance:

- Create the trigger MV **before** backfill so live rows are not missed.
- Limit each run to the **last 180 days** overall; start with **monthly** chunks and shrink if memory or duration spikes.
- Run **one customer DB and one chunk at a time** for large tenants.
- Record completed ranges so a failed chunk can be retried independently.
- Prefer off-peak windows; tune `max_execution_time` and `max_memory_usage` per chunk.
- Keep the Insights config flag **off** until backfill validation passes.

### Overlap between live trigger and backfill

INSI-4097 relied on `uniqState()` set-union idempotency for overlapping live and backfill rows. CLO uses **`ReplacingMergeTree(update_time)`**, which deduplicates eventually by sort key, not immediately.

Mitigation:

- Backfill closed partitions first; handle the current month last.
- Compare source vs storage counts per partition before enabling the flag.
- Accept eventual dedup, or agree on a stricter cutoff if reviewers require immediate uniqueness.

## Comparison to INSI-4097

| Aspect | INSI-4097 (`metadata_moment_value_count`) | CLO proposal |
|---|---|---|
| Storage engine | `ReplicatedAggregatingMergeTree` | `ReplicatedReplacingMergeTree(update_time)` |
| MV shape | Projection + `uniqState` aggregate | Narrow row projection (no `GROUP BY`) |
| Backfill scope | Full history, one pass | **180 days**, chunked for large customers |
| Overlap idempotency | `uniqState` union | ReplacingMergeTree eventual dedup |
| Rollout scope | Fix broken MV fleet-wide | **Selected large customers** first |
| Distributed local table | Points at storage, not trigger | Same |

## Why not `POPULATE`

ClickHouse [discourages `POPULATE`](https://clickhouse.com/docs/sql-reference/statements/create/view) on live tables: concurrent source inserts can be missed, and large scans are hard to throttle or resume. `TO` and `POPULATE` cannot be combined. Manual backfill into the storage table is the supported production path.

## Operational safeguards

- Staging first: measure duration, rows read, memory, disk growth per chunk.
- Prod after **6pm PST**; dry-run before schedule and immediately before apply.
- One MV/trigger operation at a time per runbook precedent.
- Monitor ClickHouse via Groundcover; pause if queue or latency spikes.
- Add to `check_missing_materialized_views.py` target list once shipped.
- No TTL initially; monthly partitions are the first size control.

## Open questions

1. Confirm version column for latest CLO value: `update_time` vs `create_time`.
2. Shard-safe backfill: INSI-4097 inserted from `moment_annotation_d` in one connection; confirm that remains correct for chunked 180-day CLO backfill on large shards.
3. Whether overlap duplicates before merge are acceptable for PI queries, or whether queries need `FINAL`/another dedup strategy.
4. Exact chunk sizing per large customer based on measured rows/bytes.
5. Whether `init_db` for new customer onboarding should create empty storage + trigger (no historical backfill needed for new tenants).

## Recommendation

Adopt the **INSI-4097 `TO target_table` pattern** for CLO:

- storage table + trigger MV without `POPULATE`;
- manual 180-day backfill into storage;
- distributed table over storage;
- per-customer flag enablement after validation.

Prototype in staging, then roll out to selected large customers before any broader fleet migration.

## References

- [INSI-4097 production runbook (`metadata_moment_value_count`)](https://github.com/cresta/clickhouse-schema/blob/fdc0b755e2297698fd9a544ff8456567bd6e8c39/conversations/scripts/run_queries_in_existing_database/history_queries/queries_fix_metadata_moment_value_count_replicated.sql)
- [Inline metadata MV reference (`moment_annotation_mv_by_metadata`)](https://github.com/cresta/clickhouse-schema/blob/main/conversations/migrations/20230824160348_init_db.up.sql#L569-L588)
- [Detailed MV investigation](clo-mv-clickhouse-creation-investigation.md)
- [Earlier inline-MV proposal (superseded)](clo-mv-without-populate-proposal.md)
- [ClickHouse CREATE VIEW / materialized views](https://clickhouse.com/docs/sql-reference/statements/create/view)
- [ClickHouse backfilling guidance](https://clickhouse.com/docs/data-modeling/backfilling)
