# Backfill Scorecards

**Created:** 2026-02-07
**Updated:** 2026-02-21 (appeal cleanup executed across all clusters)
**Linear:** [CONVI-6209](https://linear.app/cresta/issue/CONVI-6209)

## Overview

Scorecard backfill tooling and tracking. Organized by backfill run.

## Completed: Appeal Scorecard Cleanup — All Customers (2026-02-21)

**Status:** Completed (95/95 customers across 8 clusters)
**Reason:** [PR #25653](https://github.com/cresta/go-servers/pull/25653) (CONVI-6227) filters out appeal request scorecards during reindex, but reindex only INSERTs — old appeal data lingers in ClickHouse. All customers need cleanup: delete old data + re-backfill.

**Results:**

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

**Quick start (for retries/resets):**
```bash
# Check progress
python3 cluster_cleanup.py <cluster> --status

# Reset a failed customer
python3 cluster_cleanup.py <cluster> --reset <customer>

# Run cleanup (delete + backfill per customer)
python3 cluster_cleanup.py <cluster> clickhouse-conversations.<cluster>.internal.cresta.ai '<password>'
```

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
| 2026-02-21 | Executed appeal cleanup across all 8 clusters: 94/95 completed, oportun deferred |

## References

- `go-servers/cron/task-runner/tasks/batch-reindex-conversations/README.md`
- `cresta-proto/cresta/nonpublic/job/internal_job_service.proto`
- `cresta-proto/cresta/nonpublic/temporal/ingestion/reindex_conversations.proto`
