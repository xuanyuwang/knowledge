# Backfill Scorecards Investigation

**Created:** 2026-02-07
**Updated:** 2026-02-07

## Goal

Backfill scorecards for all customers for January 2026.

## Scripts

### `backfill_all.py` - Create jobs for all customers

Creates k8s jobs from the cron-batch-reindex-conversations cronjob for all customers across clusters, then collects temporal workflow IDs from logs.

```bash
# Edit config.json with your clusters and customers first

# Dry run (shows what would be done)
python3 backfill_all.py --config config.json --dry-run

# Run for all customers (waits for logs)
python3 backfill_all.py --config config.json

# Run for specific cluster/customer
python3 backfill_all.py --config config.json --cluster us-east-1-prod --customer sunbit

# Skip log collection (faster, but no workflow IDs)
python3 backfill_all.py --config config.json --skip-logs

# Custom time range
python3 backfill_all.py --config config.json \
    --start-time "2026-01-01T00:00:00Z" \
    --end-time "2026-02-01T00:00:00Z"
```

Output: `backfill_tracking.json` with job info including temporal workflow IDs.

### `check_status.py` - Check job status via Temporal

Reads tracking file and queries Temporal for workflow status.

```bash
# Prerequisites: port-forward to Temporal
kubectl --context=us-east-1-prod_dev -n temporal port-forward svc/temporal-frontend-headless 7233:7233

# Check all jobs
python3 check_status.py --tracking backfill_tracking.json

# Check specific cluster
python3 check_status.py --tracking backfill_tracking.json --cluster us-east-1-prod

# Save updated status
python3 check_status.py --tracking backfill_tracking.json --output backfill_tracking_updated.json
```

### `config.json` - Customer configuration

```json
{
  "clusters": [
    {
      "name": "us-east-1-prod",
      "customers": [
        {"id": "sunbit", "profile": "default"},
        {"id": "customer2", "profile": "default"}
      ]
    }
  ]
}
```

## Job Tracking Options

### Option 1: InternalJobService (gRPC API)

The `InternalJobService` in `cresta/nonpublic/job/internal_job_service.proto` provides APIs to query job status:

- `GetJob` - Get a specific job by name
- `ListJobs` - List jobs with filters

**Job resource name format:** `customers/{customer_id}/profiles/{profile_id}/jobs/{job_id}`

**Job types relevant:**
- `JOB_TYPE_REINDEX_CONVERSATIONS = 13`
- `JOB_TYPE_BACKFILL_SCORECARDS = 1`

**Job states:**
- `PENDING` - Waiting to start
- `RUNNING` - Currently executing
- `SUCCEEDED` - Completed successfully
- `FAILED` - Failed with error
- `PARTIALLY_SUCCEEDED` - Completed with issues
- `CANCELLED` / `CANCELLING` - Cancelled
- `TIMED_OUT` - Did not complete in time

**Usage with grpcurl:**
```bash
# Get token
CLUSTER="us-east-1-prod"
CUSTOMER="sunbit"
TOKEN=$(cresta-cli cresta-token $CLUSTER $CUSTOMER --json | jq -r .accessToken)

# Get job by name
grpcurl -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "customers/sunbit/profiles/default/jobs/JOB_ID"}' \
  grpc-cresta-api.${CLUSTER}.internal.cresta.ai:443 \
  cresta.nonpublic.job.InternalJobService/GetJob

# List jobs
grpcurl -H "Authorization: Bearer $TOKEN" \
  -d '{"parent": "customers/sunbit/profiles/default", "filters": {"job_type": [{"job_type": 13}]}}' \
  grpc-cresta-api.${CLUSTER}.internal.cresta.ai:443 \
  cresta.nonpublic.job.InternalJobService/ListJobs
```

### Option 2: Temporal CLI

The `temporal` CLI can query workflow status directly from Temporal.

**Installation:**
```bash
brew install temporal
```

**Temporal UI URLs (VPN required):**
- us-west-2-prod: https://temporal.us-west-2-prod.internal.cresta.ai/
- us-east-1-prod: https://temporal.us-east-1-prod.internal.cresta.ai/
- chat-prod: https://temporal.chat-prod.internal.cresta.ai/
- voice-prod: https://temporal.voice-prod.internal.cresta.ai/

**Port-forward to access Temporal:**
```bash
kubectl --context=${CLUSTER}_dev -n temporal port-forward svc/temporal-frontend-headless 7233:7233
```

**Query workflows:**
```bash
# List workflows by ID prefix
temporal workflow list --address localhost:7233 --namespace ingestion \
  --query 'WorkflowId STARTS_WITH "reindexconversations"'

# List running workflows only
temporal workflow list --address localhost:7233 --namespace ingestion \
  --query 'WorkflowId STARTS_WITH "reindexconversations" AND ExecutionStatus = "Running"'

# List completed workflows
temporal workflow list --address localhost:7233 --namespace ingestion \
  --query 'ExecutionStatus = "Completed" AND StartTime > "2026-01-01T00:00:00Z"'

# Describe a specific workflow
temporal workflow describe --address localhost:7233 --namespace ingestion \
  --workflow-id "reindexconversations-sunbit-us-east-1-xxxxx"

# Show workflow history/events
temporal workflow show --address localhost:7233 --namespace ingestion \
  --workflow-id "WORKFLOW_ID"

# Output as JSON for scripting
temporal workflow list --address localhost:7233 --namespace ingestion \
  -q 'WorkflowId STARTS_WITH "reindexconversations"' \
  -o json
```

### Temporal Query Syntax

**Operators:** `=`, `!=`, `>`, `>=`, `<`, `<=`, `AND`, `OR`, `()`, `BETWEEN ... AND`, `IN`, `STARTS_WITH`

**Default Search Attributes:**
- `WorkflowId` - Workflow identifier
- `WorkflowType` - Type/name of the workflow
- `ExecutionStatus` - Running, Completed, Failed, Terminated, Canceled, TimedOut
- `StartTime` - When workflow started (ISO 8601 format)
- `CloseTime` - When workflow completed
- `ExecutionTime` - Scheduled execution time

**Example Queries:**
```sql
-- By workflow ID
WorkflowId = 'my-workflow-id'
WorkflowId IN ('id1', 'id2', 'id3')
WorkflowId STARTS_WITH 'reindexconversations-sunbit'

-- By status
ExecutionStatus = 'Running'
ExecutionStatus != 'Completed'

-- Compound
WorkflowId STARTS_WITH 'reindex' AND ExecutionStatus = 'Running'
ExecutionStatus = 'Failed' OR ExecutionStatus = 'TimedOut'

-- Time-based
StartTime > '2026-01-01T00:00:00Z'
StartTime BETWEEN '2026-01-01T00:00:00Z' AND '2026-01-31T23:59:59Z'
```

**Note:** Search attribute names are case sensitive.

**Namespaces:**
- `ingestion` - For both reindex conversations AND backfill scorecards workflows
  - Task queue `reindex_conversations` - reindex workflows
  - Task queue `backfill_scorecards` - backfill scorecards workflows

## Creating Jobs

The `createjob.sh` script creates a k8s job from the `cron-batch-reindex-conversations` cronjob template.

**Script:** `backfill-scorecards/createjob.sh`

**Environment variables:**
- `REINDEX_START_TIME` - Start of time range (ISO 8601)
- `REINDEX_END_TIME` - End of time range (ISO 8601)
- `RUN_ONLY_FOR_CUSTOMER_IDS` - Comma-separated customer IDs

**Example:**
```bash
CLUSTER=us-east-1-prod_dev
CUSTOMER=sunbit

kubectl create job --from=cronjob/cron-batch-reindex-conversations \
  batch-reindex-conversations-${CUSTOMER}-$(date +%s) \
  -n cresta-cron \
  --context=${CLUSTER} \
  --dry-run=client -o yaml > /tmp/reindex-job.yaml

kubectl set env --local -f /tmp/reindex-job.yaml \
  REINDEX_START_TIME="2026-01-01T00:00:00Z" \
  REINDEX_END_TIME="2026-02-01T00:00:00Z" \
  RUN_ONLY_FOR_CUSTOMER_IDS="${CUSTOMER}" \
  -o yaml > /tmp/reindex-job-with-env.yaml

kubectl apply -f /tmp/reindex-job-with-env.yaml --context=${CLUSTER}
```

## Log Message Format

From task logs:
```
Created reindex conversations job: name=%s, execution_id=%s, cluster=%s
```

- `name` = Job resource name (for InternalJobService)
- `execution_id` = Temporal workflow ID

## Workflow ID Format

**Reindex Conversations:**
```
reindexconversations-{customerID}-{profileID}-{uuid}
```
Example: `reindexconversations-sunbit-us-east-1-a60ef966-adf1-4949-bb94-5eb5cd0f65d6`

**Backfill Scorecards:**
```
backfillscorecards-{customerID}-{profileID}-{uuid}
```
Example: `backfillscorecards-sunbit-us-east-1-b71ef123-cde2-5050-cc95-6fc6de0f76e7`

**Query examples:**
```bash
# All reindex workflows for a customer
temporal workflow list --namespace ingestion \
  --query 'WorkflowId STARTS_WITH "reindexconversations-sunbit"'

# All backfill scorecards workflows for a customer
temporal workflow list --namespace ingestion \
  --query 'WorkflowId STARTS_WITH "backfillscorecards-sunbit"'

# All running workflows for a customer
temporal workflow list --namespace ingestion \
  --query 'WorkflowId STARTS_WITH "reindexconversations-sunbit" AND ExecutionStatus = "Running"'
```

## Clusters

| Cluster | Temporal UI |
|---------|-------------|
| us-east-1-prod | temporal.us-east-1-prod.internal.cresta.ai |
| us-west-2-prod | temporal.us-west-2-prod.internal.cresta.ai |
| chat-prod | temporal.chat-prod.internal.cresta.ai |
| voice-prod | temporal.voice-prod.internal.cresta.ai |

## References

- Temporal README: `go-servers/temporal/README.md`
- Job proto: `cresta-proto/cresta/v1/job/job.proto`
- InternalJobService proto: `cresta-proto/cresta/nonpublic/job/internal_job_service.proto`
- Reindex conversations proto: `cresta-proto/cresta/nonpublic/temporal/ingestion/reindex_conversations.proto`

## TODO

- [x] Determine which namespace backfill scorecards workflows use (confirmed: `ingestion`)
- [x] Create tracking scripts for all customers/clusters (`backfill_all.py`, `check_status.py`)
- [x] Document createjob.sh usage
- [x] Test temporal CLI with port-forward once VPN/network is working
- [ ] Populate config.json with actual customer list per cluster
