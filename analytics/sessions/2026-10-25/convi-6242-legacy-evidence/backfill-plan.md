# Backfill Plan: cron-label-conversations 2026 Data

**Created:** 2026-02-18
**Updated:** 2026-02-18

## Key Question: Do We Need to Delete Existing Data?

**YES.** Deletion is mandatory before re-inserting.

### Why

The `conversation_with_labels` ClickHouse table uses `ReplicatedReplacingMergeTree` with this ORDER BY:

```sql
ORDER BY (toStartOfHour(conversation_end_time), agent_user_id, conversation_id, customer_id, profile_id)
```

`ReplacingMergeTree` deduplicates rows that share the **same ORDER BY values**. Two of these columns are mutable:

| Column | Mutable? | Problem |
|--------|----------|---------|
| `toStartOfHour(conversation_end_time)` | Yes | `ended_at` changes as conversation closes |
| `agent_user_id` | Yes | Agent can be re-assigned |
| `conversation_id` | No | Stable |
| `customer_id` | No | Stable |
| `profile_id` | No | Stable |

When we re-insert a conversation whose agent or end_time has changed, the new row has **different ORDER BY values** from the old row. ClickHouse treats it as a new row, creating a **duplicate** instead of a replacement.

### What Happens Without Deletion

If we simply re-run the cron without deleting:
- Old row: `(hour(02:00), agent_A, conv_123, cust, profile)` — stale
- New row: `(hour(03:00), agent_B, conv_123, cust, profile)` — correct
- Both rows exist → queries that aggregate by agent see inflated counts
- `FINAL` keyword doesn't help because ORDER BY values differ

### Schema Migration (PR clickhouse-schema#172) Doesn't Eliminate Delete

Even after changing ORDER BY to `(conversation_start_time, conversation_id, customer_id, profile_id)`:
- **Future** inserts would deduplicate correctly (immutable keys only)
- **Existing** stale rows written under the old ORDER BY still persist
- Backfill still requires deleting old rows first

## Deletion Strategy

### Option 1: DELETE by Date Range per Customer (Recommended)

Run a ClickHouse DELETE for each customer's 2026 data before re-running the cron.

```sql
-- Run against the LOCAL table (conversation_with_labels), NOT the distributed table (_d)
-- Must run on each shard/replica, or use ON CLUSTER
ALTER TABLE conversation_with_labels ON CLUSTER 'conversations'
DELETE WHERE
  customer_id = '{customer_id}'
  AND conversation_end_time >= '2026-01-01 00:00:00'
  AND conversation_end_time < '2026-02-19 00:00:00'
SETTINGS replication_wait_for_inactive_replica_timeout = 0;
```

**Pros:**
- Precise, only affects target data
- Can verify row counts before/after
- Safe — doesn't touch other customers or date ranges

**Cons:**
- Must wait for mutations to complete before re-inserting (check `system.mutations`)
- Need to run per-customer or use a broader WHERE

### Option 2: DELETE by Date Range for All Customers

If backfilling all customers in a cluster:

```sql
ALTER TABLE conversation_with_labels ON CLUSTER 'conversations'
DELETE WHERE
  conversation_end_time >= '2026-01-01 00:00:00'
  AND conversation_end_time < '2026-02-19 00:00:00'
SETTINGS replication_wait_for_inactive_replica_timeout = 0;
```

**Pros:** Simpler, one statement per cluster
**Cons:** Deletes data for all customers including correct rows — must re-insert everything

### Verifying Mutation Completion

```sql
-- Check pending mutations
SELECT * FROM system.mutations
WHERE table = 'conversation_with_labels' AND is_done = 0;

-- Wait until no pending mutations before proceeding with re-insert
```

## Backfill Execution

### Cron Env Vars for Backfill

The cron supports these env vars to control time range and customer:

| Env Var | Purpose | Example |
|---------|---------|---------|
| `LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_START_AT_RANGE_START` | Override start time (RFC3339) | `2026-01-01T00:00:00Z` |
| `LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_END_AT_RANGE_END` | Override end time (RFC3339) | `2026-02-19T00:00:00Z` |
| `LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_BATCH_SIZE` | Batch duration (default 24h) | `24h` |
| `FILTER_CUSTOMER_IN_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE` | Single customer filter | `alaska-air` |
| `ENABLE_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE` | Must be `true` | `true` |

### K8s Job Creation (Same Pattern as backfill-scorecards)

```bash
CLUSTER="us-east-1-prod"
CUSTOMER="alaska-air"
START="2026-01-01T00:00:00Z"
END="2026-02-19T00:00:00Z"

# Step 1: Create job YAML from cronjob template
kubectl create job "backfill-labels-${CUSTOMER}-$(date +%s)" \
  --from=cronjob/cron-label-conversations \
  -n cresta-cron --context="${CLUSTER}_dev" \
  --dry-run=client -o yaml > /tmp/label-job.yaml

# Step 2: Set env vars
kubectl set env --local -f /tmp/label-job.yaml \
  ENABLE_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE="true" \
  LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_START_AT_RANGE_START="${START}" \
  LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_END_AT_RANGE_END="${END}" \
  FILTER_CUSTOMER_IN_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE="${CUSTOMER}" \
  -o yaml > /tmp/label-job-final.yaml

# Step 3: Apply
kubectl apply -f /tmp/label-job-final.yaml --context="${CLUSTER}_dev"
```

### Sequential Day-by-Day Approach (for Large Customers)

Learned from backfill-scorecards: large customers (e.g., CVS with 4.2M conversations) time out on month-long ranges. Use day-by-day execution with tracking.

```bash
# Single day example
./backfill_single_day.sh alaska-air 2026-01-15
```

## Execution Order

### Prerequisites
1. PR go-servers#25706 (ended_at filter) is merged and deployed
2. PR clickhouse-schema#172 (ORDER BY change) is deployed (recommended but not blocking)

### Steps

```
For each cluster:
  For each customer (or all customers):
    1. DELETE existing rows for 2026 date range
    2. Wait for mutation to complete (check system.mutations)
    3. Create k8s job with time range env vars
    4. Monitor job completion (kubectl logs)
    5. Verify row counts match expected conversation counts
```

### Clusters to Backfill

Depends on which customers are affected. At minimum:
- **Alaska Air** — confirmed affected (us-east-1-prod or whichever cluster they're on)
- Potentially all customers across all clusters if we want clean 2026 data

### Verification

After backfill, verify per customer:

```sql
-- Count rows in conversation_with_labels vs conversation_d
-- They should be close to 1:1 (one label row per conversation)

-- Check for duplicates (same conversation_id, different agent/end_time)
SELECT conversation_id, count() as cnt
FROM conversation_with_labels_d FINAL
WHERE customer_id = '{customer}'
  AND conversation_end_time >= '2026-01-01'
GROUP BY conversation_id
HAVING cnt > 1;

-- Check Active Days for the known affected agent
SELECT agent_user_id, count(DISTINCT toDate(conversation_end_time)) as active_days
FROM conversation_with_labels_d FINAL
WHERE customer_id = 'alaska-air'
  AND agent_user_id = '256f70253da263fe'
  AND conversation_end_time >= '2026-02-10'
  AND conversation_end_time < '2026-02-11';
```

## Scripts to Create

Reference `backfill-scorecards/` for patterns. Scripts needed:

| Script | Purpose | Reference |
|--------|---------|-----------|
| `backfill.sh` | Create k8s job for one customer + date range | `backfill-scorecards/createjob.sh` |
| `backfill_sequential.py` | Day-by-day with progress tracking | `backfill-scorecards/rerun_sequential.py` |
| `delete_existing.sh` | Run ClickHouse DELETE before backfill | New — uses `clickhouse-client` or port-forward |
| `verify.sh` | Post-backfill verification queries | New |

## Risk Mitigation

1. **Dry-run first**: Use `--dry-run=client` on kubectl to preview job YAML
2. **Single customer first**: Backfill Alaska Air alone, verify, then expand
3. **Off-peak hours**: Run during low-traffic window to avoid ClickHouse load
4. **Mutation monitoring**: Don't insert until DELETE mutation completes
5. **Row count verification**: Compare before/after counts
