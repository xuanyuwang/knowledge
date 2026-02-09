#!/bin/zsh
# Backfill scorecards for all customers in a cluster
# Usage: ./createjob.sh <cluster>
# Example: ./createjob.sh us-east-1-prod

set -e

CLUSTER=${1:-us-east-1-prod}
CONTEXT="${CLUSTER}_dev"
START_TIME="2026-01-01T00:00:00Z"
END_TIME="2026-02-01T00:00:00Z"
JOB_NAME="batch-reindex-all-$(date +%s)"

echo "============================================================"
echo "Backfill Scorecards - All Customers"
echo "============================================================"
echo "Cluster:    ${CLUSTER}"
echo "Context:    ${CONTEXT}"
echo "Start time: ${START_TIME}"
echo "End time:   ${END_TIME}"
echo "Job name:   ${JOB_NAME}"
echo "============================================================"

# Step 1: Generate job YAML from cronjob template
echo "Creating job template..."
kubectl create job --from=cronjob/cron-batch-reindex-conversations \
  ${JOB_NAME} \
  -n cresta-cron \
  --context=${CONTEXT} \
  --dry-run=client -o yaml > /tmp/reindex-job.yaml

# Step 2: Set environment variables (no RUN_ONLY_FOR_CUSTOMER_IDS = all customers)
echo "Setting environment variables..."
kubectl set env --local -f /tmp/reindex-job.yaml \
  REINDEX_START_TIME="${START_TIME}" \
  REINDEX_END_TIME="${END_TIME}" \
  -o yaml > /tmp/reindex-job-with-env.yaml

# Step 3: Apply the job
echo "Applying job..."
kubectl apply -f /tmp/reindex-job-with-env.yaml --context=${CONTEXT}

echo ""
echo "Job created: ${JOB_NAME}"
echo ""
echo "To view logs (run immediately, jobs auto-delete after ~10s):"
echo "  kubectl logs -n cresta-cron --context=${CONTEXT} -l job-name=${JOB_NAME} -f"
