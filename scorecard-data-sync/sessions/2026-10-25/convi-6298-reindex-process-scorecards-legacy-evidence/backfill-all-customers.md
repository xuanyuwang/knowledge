# Backfill Process Scorecards for All Customers

**Created**: 2026-03-28
**Updated**: 2026-03-28

## Goal

Backfill all process scorecards into ClickHouse for every customer, across all prod clusters.

## Strategy

**Phase 1**: Backfill 2026 data for all customers (small dataset, quick)
- Time range: `2026-01-01T00:00:00Z` to `2026-03-29T00:00:00Z`
- Process scorecards are relatively rare — expect fast completion
- Measure execution time to estimate Phase 2

**Phase 2**: Backfill all data before 2026
- Time range: `2020-01-01T00:00:00Z` to `2026-01-01T00:00:00Z`
- Scope determined by Phase 1 timing

## Prod Clusters

| Cluster | Context | Notes |
|---------|---------|-------|
| `voice-prod` | `voice-prod_dev` | |
| `chat-prod` | `chat-prod_dev` | |
| `us-west-2-prod` | `us-west-2-prod_dev` | |
| `us-east-1-prod` | `us-east-1-prod_dev` | |
| `ap-southeast-2-prod` | `ap-southeast-2-prod_dev` | APAC |
| `eu-west-2-prod` | `eu-west-2-prod_dev` | EU |
| `ca-central-1-prod` | `ca-central-1-prod_dev` | Canada |
| `schwab-prod` | `schwab-prod_dev` | Schwab |

## Execution Method

Use the same approach as staging/sandbox tests: create a k8s job from the `cron-batch-reindex-conversations` cronjob template with:
- `REINDEX_MODE=process` — only process scorecards
- `REINDEX_START_TIME` / `REINDEX_END_TIME` — time range
- **No** `RUN_ONLY_FOR_CUSTOMER_IDS` — run for all customers
- `REINDEX_SCORECARDS_CLEAN_UP_BEFORE_WRITE=true` — clean write (idempotent)

The cron job iterates over all customers in the cluster, dispatching a `JOB_TYPE_REINDEX_SCORECARDS` Temporal workflow for each.

## Phase 1: 2026 Backfill

### Commands per Cluster

```bash
# Set variables
START_TIME="2026-01-01T00:00:00Z"
END_TIME="2026-03-29T00:00:00Z"
JOB_NAME="reindex-process-2026-$(date +%s)"

# For each cluster:
CLUSTER="voice-prod"  # voice-prod, chat-prod, us-west-2-prod, us-east-1-prod, ap-southeast-2-prod, eu-west-2-prod, ca-central-1-prod, schwab-prod
CONTEXT="${CLUSTER}_dev"

# 1. Generate job YAML
kubectl create job --from=cronjob/cron-batch-reindex-conversations \
    "${JOB_NAME}" \
    -n cresta-cron \
    --context="${CONTEXT}" \
    --dry-run=client -o yaml > /tmp/${JOB_NAME}.yaml

# 2. Set env vars (no RUN_ONLY_FOR_CUSTOMER_IDS = all customers)
kubectl set env --local -f /tmp/${JOB_NAME}.yaml \
    REINDEX_MODE="process" \
    REINDEX_START_TIME="${START_TIME}" \
    REINDEX_END_TIME="${END_TIME}" \
    REINDEX_SCORECARDS_CLEAN_UP_BEFORE_WRITE="true" \
    -o yaml > /tmp/${JOB_NAME}-final.yaml

# 3. Apply
kubectl apply -f /tmp/${JOB_NAME}-final.yaml --context="${CONTEXT}"

# 4. Monitor
kubectl logs -n cresta-cron --context="${CONTEXT}" -l job-name="${JOB_NAME}" -f
```

## Phase 1 Execution Log

| Cluster | Job Name | Duration | Customers | Notes |
|---------|----------|----------|-----------|-------|
| voice-prod | reindex-process-2026-voice-prod-1774719360 | 4s | all | Complete |
| chat-prod | reindex-process-2026-chat-prod-1774719363 | 6s | all | Complete |
| us-west-2-prod | reindex-process-2026-us-west-2-prod-1774719365 | 9s | all | Complete |
| us-east-1-prod | reindex-process-2026-us-east-1-prod-1774719367 | 5s | 90 | Complete (largest cluster) |
| ap-southeast-2-prod | reindex-process-2026-ap-southeast-2-prod-1774719369 | 4s | all | Complete |
| eu-west-2-prod | reindex-process-2026-eu-west-2-prod-1774719374 | 4s | all | Complete |
| ca-central-1-prod | reindex-process-2026-ca-central-1-prod-1774719377 | 4s | all | Complete |
| schwab-prod | reindex-process-2026-schwab-prod-1774719378 | 4s | all | Complete |

Note: Duration above is the cron job dispatch time (creating Temporal workflows). The actual reindex work runs asynchronously in Temporal.

## Phase 2: Pre-2026 Backfill

To be planned after Phase 1 completes. Will use:
- `START_TIME=2020-01-01T00:00:00Z`
- `END_TIME=2026-01-01T00:00:00Z`

Duration estimate: TBD based on Phase 1 timing.
