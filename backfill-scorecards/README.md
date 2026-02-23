# Backfill Scorecards

**Created:** 2026-02-07
**Updated:** 2026-02-23 (all backfills complete — appeal cleanup fully done)
**Linear:** [CONVI-6209](https://linear.app/cresta/issue/CONVI-6209)

## Overview

Scorecard backfill tooling and tracking. Organized by backfill run.

## Completed: Appeal Scorecard Cleanup — All Customers (2026-02-21 – 2026-02-23)

**Status:** Completed. 95/95 customers deleted + backfilled across all 8 clusters.
**Reason:** [PR #25653](https://github.com/cresta/go-servers/pull/25653) (CONVI-6227) filters out appeal request scorecards during reindex, but reindex only INSERTs — old appeal data lingers in ClickHouse. All customers need cleanup: delete old data + re-backfill.

### Deletion Results

| Cluster | Customers | Completed | Scorecards Removed | Scores Removed | Notes |
|---------|-----------|-----------|-------------------|----------------|-------|
| voice-prod | 17 | 17/17 | ~18.9M | ~178.9M | |
| us-east-1-prod | 44 | 44/44 | ~22.1M | ~154.8M | |
| us-west-2-prod | 30 | 30/30 | ~27.2M | ~297.6M | oportun used chunked 1-day deletes |
| chat-prod | 3 | 3/3 | ~2.0M | ~18.0M | |
| schwab-prod | 1 | 1/1 | ~267K | ~1.2M | |
| eu-west-2-prod | 0 | - | 0 | 0 | No data in range |
| ap-southeast-2-prod | 0 | - | 0 | 0 | No data in range |
| ca-central-1-prod | 0 | - | 0 | 0 | No data in range |
| **Total** | **95** | **95/95** | **~70.5M** | **~650.5M** | |

### Backfill Results

| Cluster | Backfill Complete | Notes |
|---------|-------------------|-------|
| us-west-2-prod | 30/30 | oportun: 51-day sequential backfill |
| us-east-1-prod | 44/44 | united-east, marriott, spirit: windowed parallel backfill |
| voice-prod | 17/17 | hilton: windowed parallel; vivint, holidayinn-transfers: full-range |
| chat-prod | 3/3 | |
| schwab-prod | 1/1 | |
| **Total** | **95/95** | |

### Windowed Backfill for Large Customers

4 customers failed full-range backfill (heartbeat timeout). Re-run with windowed backfills, initially sequential then switched to fully parallel on the weekend:

| Customer | Cluster | Window Size | Windows | Strategy | Duration |
|----------|---------|-------------|---------|----------|----------|
| spirit | us-east-1-prod | 10-day | 5 | Sequential → parallel | ~14h total |
| hilton | voice-prod | 5-day | 9 | Sequential → parallel | ~18h total |
| marriott | us-east-1-prod | 10-day | 5 | Sequential → parallel | ~20h total |
| united-east | us-east-1-prod | 5-day | 10 | Sequential → parallel | ~24h total |

All completed by 2026-02-23 with zero failures. Heartbeat details showed ~400-460K conversations per 5-day window for united-east, ~640K for marriott 10-day windows.

## Completed: Mutual of Omaha (2026-02-19)

**Status:** Completed
**Reason:** [PR #25653](https://github.com/cresta/go-servers/pull/25653) (CONVI-6227) — filter out appeal request scorecards during reindex. Needed to delete old ClickHouse data (which included appeal scorecards) and re-backfill with the new code.

| Parameter | Value |
|-----------|-------|
| Cluster | voice-prod |
| Customer IDs | `mutualofomaha,mutualofomaha-sandbox` |
| Date range | 2026-01-01 to 2026-02-20 |
| Approach | Delete ClickHouse data, then 5 parallel 10-day backfill jobs |

**Pre-delete → Post-backfill:**

| Table | Before | After | Removed (appeal scorecards) |
|-------|--------|-------|-----------------------------|
| `scorecard` | 330,294 | 298,791 | 31,503 |
| `score` | 2,881,300 | 2,609,114 | 272,186 |

## Completed: Jan 2026 All Clusters (2026-02-07 - 2026-02-10)

**Status:** Completed

Backfilled scorecards for all customers across all 8 production clusters for January 2026.
Artifacts in `jan-2026-all-clusters/`.

| Phase | Scope | Result |
|-------|-------|--------|
| Initial backfill | 260 jobs across 8 clusters | 256/260 completed |
| Retry | marriott, united-east | Completed on simple retry |
| Sequential single-day | cvs, oportun (31 days each) | All 31 days completed |

## Approach

### Phase 1: Bulk Backfill (2026-02-07)

Created one k8s job per cluster from `cron-batch-reindex-conversations` cronjob template. Each job processes all customers in the cluster for the full month (Jan 1 - Feb 1).

```bash
./createjob.sh <cluster>  # for each of 8 clusters
```

| Cluster | Jobs |
|---------|------|
| us-east-1-prod | 86 |
| us-west-2-prod | 80 |
| voice-prod | 49 |
| chat-prod | 28 |
| eu-west-2-prod | 6 |
| schwab-prod | 5 |
| ap-southeast-2-prod | 4 |
| ca-central-1-prod | 2 |
| **Total** | **260** |

**Result:** 256/260 completed. 4 large customers failed.

### Phase 2: Retry Failed Customers (2026-02-08)

4 customers failed due to high conversation volume:

| Customer | Cluster | Failure | Conversations |
|----------|---------|---------|---------------|
| marriott | us-east-1-prod | Heartbeat timeout | 1.3M |
| united-east | us-east-1-prod | Heartbeat timeout | 578K |
| cvs | us-west-2-prod | DB deadlock (40P01) | 4.2M |
| oportun | us-west-2-prod | DB deadlock (40P01) | 2.6M |

- **marriott, united-east:** Completed on simple retry.
- **cvs, oportun:** Continued failing. Tried splitting into 10-day windows — still failed with heartbeat timeout.

### Phase 3: Sequential Single-Day Backfill (2026-02-09 - 2026-02-10)

For cvs and oportun, the full month and even 10-day windows were too large. Running all 31 days in parallel caused DB stress, resulting in external cancellation.

**Solution:** Process one day at a time, sequentially, waiting for completion before starting the next.

Built `rerun_sequential.py` with:
- JSON-based progress tracking (`sequential_tracking.json`) for resume after interruption
- Automatic port-forward management
- Temporal workflow discovery and polling
- `--status` and `--reset DAY` commands

**Result:** All 31 days completed for both cvs and oportun.

## Scripts

| Script | Purpose |
|--------|---------|
| `backfill.sh` | General-purpose backfill (accepts cluster, customers, date range) |
| `list_ch_databases.sh` | Discover all ClickHouse databases with scorecard data on a cluster |
| `cluster_cleanup.py` | Orchestrate appeal cleanup per cluster: discover → delete → backfill with tracking |

Previous run scripts are in `jan-2026-all-clusters/`.

### Usage

```bash
# Backfill specific customers
./backfill.sh <cluster> <customers> <start_date> <end_date>
./backfill.sh voice-prod "mutualofomaha,mutualofomaha-sandbox" 2026-01-01 2026-02-20

# Backfill all customers in a cluster
./backfill.sh us-east-1-prod all 2026-01-01 2026-02-01

# Dry run (preview without applying)
./backfill.sh voice-prod "mutualofomaha,mutualofomaha-sandbox" 2026-01-01 2026-02-20 --dry-run
```

## Job Tracking

### Temporal CLI

```bash
# Port-forward to Temporal
kubectl --context=${CLUSTER}_dev -n temporal port-forward svc/temporal-frontend-headless 7233:7233

# List workflows
temporal workflow list --namespace ingestion --address localhost:7233 \
  --query 'WorkflowId STARTS_WITH "reindexconversations-cvs"'

# Describe a workflow
temporal workflow describe --namespace ingestion --address localhost:7233 \
  --workflow-id "WORKFLOW_ID"
```

### InternalJobService (gRPC)

See `cresta-proto/cresta/nonpublic/job/internal_job_service.proto` for `GetJob` and `ListJobs` APIs.

## Directory Structure

```
backfill-scorecards/
├── backfill.sh                          # General-purpose backfill script
├── list_ch_databases.sh                 # ClickHouse database discovery
├── cluster_cleanup.py                   # Appeal cleanup orchestration
├── README.md
├── log/                                 # Daily progress logs
├── tracking/                            # Per-cluster JSON tracking files
│   ├── voice-prod.json
│   ├── us-east-1-prod.json
│   └── ...
├── jan-2026-all-clusters/               # Previous run: all customers, Jan 2026
│   ├── createjob.sh, rerun_*.sh/py      # Run-specific scripts
│   ├── backfill_tracking*.json           # Workflow tracking
│   ├── sequential_tracking.json          # Day-by-day tracking for cvs/oportun
│   └── logs-*.txt                        # Job output logs
└── mutualofomaha-jan-feb-2026/           # Previous run: Mutual of Omaha
```

## Clusters

| Cluster | Temporal Namespace |
|---------|-------------------|
| us-east-1-prod | ingestion |
| us-west-2-prod | ingestion |
| chat-prod | ingestion |
| voice-prod | ingestion |
| ap-southeast-2-prod | ingestion |
| ca-central-1-prod | ingestion |
| eu-west-2-prod | ingestion |
| schwab-prod | ingestion |

## Lessons Learned

1. **Large customers need smaller time windows.** CVS (4.2M conversations/month) and oportun (2.6M) can't process a full month in one workflow — heartbeat timeout or DB deadlock.
2. **Don't run all days in parallel.** 60 concurrent reindex workflows caused enough DB stress that operations canceled them.
3. **Sequential with tracking is the right approach.** One day at a time with JSON-based progress tracking allows safe resume after VPN drops, timeouts, or interruptions.
4. **Temporal namespace is `ingestion`**, not `cresta` or `jobmanagement`.
5. **Reindex only INSERTs — it does not DELETE old ClickHouse data.** If the reindex code changes what it writes (e.g., filtering out appeal scorecards), old stale rows remain. Must delete from ClickHouse before re-backfilling.
6. **Delete from local tables with `ON CLUSTER`, not distributed tables.** ClickHouse `ALTER TABLE ... DELETE` must target local tables (e.g., `scorecard`) with `ON CLUSTER 'conversations'`, not distributed tables (`scorecard_d`).
7. **ON CLUSTER mutations can block the entire cluster.** A large DELETE (e.g., oportun 232M rows) creates mutations on all 9 nodes. If one mutation takes too long, it blocks all subsequent mutations on those nodes. Use `KILL MUTATION ON CLUSTER` to unblock.
8. **ClickHouse mutations only affect existing parts.** New inserts (from backfill) create new parts that are not affected by pending delete mutations. Safe to proceed with backfill while mutations are still completing.
9. **Database names use SanitizeDatabaseName(customer_id + "_" + profile_id).** Replace non-alphanumeric chars (except `_`) with `_`. Cannot reverse from database name to customer ID. Use `backfill_tracking.json` as authoritative mapping.
10. **Reindex workflows for large customers can run for hours.** hilton 5-day window: ~2h. united-east 5-day: up to 6h. marriott/spirit 10-day: ~3h. Set script timeouts accordingly (8h+), or use fire-and-forget with later verification.
11. **Windowed backfill sizes depend on conversation volume, not scorecard count.** The bottleneck is reading conversations from Postgres, not writing to ClickHouse. Customers with >50K scorecards/day typically need 5-day or smaller windows; <30K scorecards/day can use 10-day windows.
12. **Parallel windows are safe on weekends.** Running all windows for a customer in parallel doesn't cause issues when traffic is low. Sequential is safer on weekdays; switch to parallel on weekends to save time. Use `temporal workflow describe` heartbeat details to monitor per-workflow progress (ReindexedConversations / TotalConversationCount).

## Problems & Strategy Evolution

### Problem 1: Full-range backfill fails for large customers

**Symptom:** Temporal workflow heartbeat timeout for customers with high conversation volume (oportun, cvs, hilton, united-east, marriott, spirit).

**Root cause:** The reindex workflow reads all conversations from Postgres for the given date range. For large customers (millions of conversations over 51 days), the activity takes too long between heartbeat signals, and Temporal kills it.

**Strategy changes:**
1. Full 51-day range → **10-day parallel windows** → still failed for cvs/oportun (DB deadlock from concurrent workflows)
2. 10-day parallel → **1-day sequential** for cvs/oportun → worked (Jan 2026 backfill)
3. For appeal cleanup, tried **5-day and 10-day sequential windows** based on per-customer volume analysis:
   - \>80K scorecards/day → 5-day windows (hilton, united-east)
   - <30K scorecards/day → 10-day windows (marriott, spirit)
4. Window sizes were correct (workflows completed at attempt=1), but **script timeout was too short** (1h) — workflows took 2-6h per window. Fixed by increasing to 8h.

### Problem 2: Oportun too large for single ClickHouse DELETE

**Symptom:** `ALTER TABLE ... DELETE ON CLUSTER` for oportun (15.6M scorecards + 232.9M scores) blocked the entire cluster's mutation queue, exceeding the 600s mutation timeout.

**Strategy changes:**
1. Single full-range delete → **chunked 1-day deletes** (51 iterations), each deleting one day's data then waiting for mutations to clear
2. Added a **full-range sweep** after daily deletes to catch any stragglers
3. Then ran **1-day sequential backfill** (51 jobs) to re-insert clean data

### Problem 3: ON CLUSTER mutations blocking unrelated customers

**Symptom:** Small databases (cvs-sandbox: 4 scorecards) were stuck for 10+ minutes. The `wait_for_mutations` check saw pending mutations and blocked, but the mutations belonged to other operations (reindex `UPDATE _row_exists=0` from unrelated tables).

**Strategy changes:**
1. Realized `system.mutations` shows ALL mutations for a database, not just ours
2. **Killed stuck mutations** with `KILL MUTATION ON CLUSTER 'conversations'`
3. Issued **fresh ON CLUSTER deletes** and proceeded to backfill immediately — safe because ClickHouse mutations only affect parts that existed at mutation creation time; new inserts from backfill are unaffected

### Problem 4: Direct DELETE only affects local shard

**Symptom:** Attempted to bypass ON CLUSTER issues by deleting directly (without `ON CLUSTER`). Counts on distributed tables didn't go to zero — only 1 of 3 shards was cleaned.

**Root cause:** Tables are `ReplicatedReplacingMergeTree` with data sharded across 3 shards × 3 replicas. A direct delete hits only the local shard.

**Resolution:** Must always use `ON CLUSTER 'conversations'` for deletes. Accepted the mutation replication overhead and used `SETTINGS replication_wait_for_inactive_replica_timeout = 0` to avoid blocking on slow replicas.

### Problem 5: Unmapped customer databases

**Symptom:** `cluster_cleanup.py` didn't know about some customers because they were added after the Jan 2026 `backfill_tracking.json` was created.

**Resolution:** Queried ClickHouse directly (`SELECT DISTINCT customer_id, profile_id FROM <db>.scorecard`) to discover the mapping, then added them to the tracking JSON. Affected: gardner-white (voice-prod), chime (us-east-1-prod), cba-japan (us-west-2-prod).

### Key Principle

**Start conservative, scale up.** The pattern that consistently worked:
1. Try full range → fails for large customers
2. Try moderate windows (5-10 day) → works for most, but need long timeout
3. Fall back to 1-day sequential for the very largest (oportun, cvs)

The bottleneck is always **Postgres read time** (conversation volume), not ClickHouse write time. Window size should be chosen based on conversation count, not scorecard count.

## ClickHouse Operations

### Connection

```bash
# Direct connection (requires VPN)
/opt/homebrew/bin/clickhouse client \
  -h clickhouse-conversations.<cluster>.internal.cresta.ai \
  --port 9440 -u admin --password '<password>' --secure

# Get password
kubectl --context=<cluster>_dev -n clickhouse \
  get secrets clickhouse-cluster --template '{{.data.admin_password}}' | base64 -d
```

### Delete + Backfill Pattern

Each customer has its own database (e.g., `mutualofomaha_voice`). Tables: `scorecard`, `score`, `scorecard_score`.

```sql
-- 1. Count before delete
SELECT count() FROM mutualofomaha_voice.scorecard
WHERE scorecard_time >= '2026-01-01' AND scorecard_time < '2026-02-20';

-- 2. Delete from local tables (NOT _d distributed tables)
ALTER TABLE mutualofomaha_voice.scorecard ON CLUSTER 'conversations'
DELETE WHERE scorecard_time >= '2026-01-01' AND scorecard_time < '2026-02-20'
SETTINGS replication_wait_for_inactive_replica_timeout = 0;

ALTER TABLE mutualofomaha_voice.score ON CLUSTER 'conversations'
DELETE WHERE scorecard_time >= '2026-01-01' AND scorecard_time < '2026-02-20'
SETTINGS replication_wait_for_inactive_replica_timeout = 0;

-- 3. Check mutations completed
SELECT * FROM system.mutations WHERE database = 'mutualofomaha_voice' AND is_done = 0;

-- 4. Run backfill
-- ./backfill.sh voice-prod "mutualofomaha,mutualofomaha-sandbox" 2026-01-01 2026-02-20
```

## Log History

| Date | Summary |
|------|---------|
| 2026-02-07 | Initial backfill: 260 jobs across 8 clusters |
| 2026-02-08 | Status check: 256/260 done, 4 failed, retried |
| 2026-02-09 | Split approaches for cvs/oportun, started sequential |
| 2026-02-10 | Sequential run completed: all 31 days done |
| 2026-02-19 | Reorganized project; Mutual of Omaha backfill completed (delete + 5 parallel jobs) |
| 2026-02-20 | Created appeal cleanup rollout tooling (list_ch_databases.sh, cluster_cleanup.py) |
| 2026-02-21 | Executed appeal cleanup across all 8 clusters: 95/95 deleted, oportun chunked 1-day deletes + sequential backfill |
| 2026-02-22 | Backfill verification: 89/95 complete. 4 large customers re-running with windowed backfills, switched to parallel |
| 2026-02-23 | All backfills complete (95/95). Appeal scorecard cleanup fully done |

## References

- `go-servers/cron/task-runner/tasks/batch-reindex-conversations/README.md`
- `cresta-proto/cresta/nonpublic/job/internal_job_service.proto`
- `cresta-proto/cresta/nonpublic/temporal/ingestion/reindex_conversations.proto`
