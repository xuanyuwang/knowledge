# Backfill Scorecards

**Created:** 2026-02-07
**Updated:** 2026-02-10
**Status:** Completed
**Linear:** [CONVI-6209](https://linear.app/cresta/issue/CONVI-6209)

## Overview

Backfilled scorecards for all customers across all 8 production clusters for January 2026.

## Result Summary

| Phase | Scope | Result |
|-------|-------|--------|
| Initial backfill | 260 jobs across 8 clusters | 256/260 completed |
| Retry | marriott, united-east | Completed on simple retry |
| Sequential single-day | cvs, oportun (31 days each) | All 31 days completed |

**Final: All customers backfilled for January 2026.**

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
| `createjob.sh` | Create backfill job for all customers in a cluster |
| `rerun_failed.sh` | Retry specific failed customers |
| `rerun_single_day.sh` | Run one day for cvs/oportun |
| `rerun_all_days.sh` | Run all 31 days in parallel (caused DB stress) |
| `rerun_sequential.py` | Run days sequentially with tracking and resume |
| `check_status.py` | Check workflow status via Temporal CLI |

### Key Usage

```bash
# Sequential run with resume support
python3 rerun_sequential.py              # Run (skips completed days)
python3 rerun_sequential.py --status     # Show progress
python3 rerun_sequential.py --reset 15   # Reset a day to re-run
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

## Tracking Files

| File | Contents |
|------|----------|
| `backfill_tracking.json` | 260 workflow IDs from initial bulk backfill |
| `sequential_tracking.json` | Day-by-day progress for cvs/oportun |

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

## Log History

| Date | Summary |
|------|---------|
| 2026-02-07 | Initial backfill: 260 jobs across 8 clusters |
| 2026-02-08 | Status check: 256/260 done, 4 failed, retried |
| 2026-02-09 | Split approaches for cvs/oportun, started sequential |
| 2026-02-10 | Sequential run completed: all 31 days done |

## References

- `go-servers/cron/task-runner/tasks/batch-reindex-conversations/README.md`
- `cresta-proto/cresta/nonpublic/job/internal_job_service.proto`
- `cresta-proto/cresta/nonpublic/temporal/ingestion/reindex_conversations.proto`
