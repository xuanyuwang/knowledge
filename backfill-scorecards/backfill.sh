#!/bin/zsh
# General-purpose scorecard backfill script
#
# Usage:
#   ./backfill.sh <cluster> <customers> <start_date> <end_date> [--dry-run]
#
# Arguments:
#   cluster     - Kubernetes cluster (e.g., voice-prod, us-east-1-prod)
#   customers   - Comma-separated customer IDs (e.g., "mutualofomaha,mutualofomaha-sandbox")
#                 Use "all" to backfill all customers in the cluster
#   start_date  - Start date in YYYY-MM-DD format (e.g., 2026-01-01)
#   end_date    - End date in YYYY-MM-DD format, exclusive (e.g., 2026-02-20)
#   --dry-run   - Preview the job YAML without applying
#
# Examples:
#   ./backfill.sh voice-prod "mutualofomaha,mutualofomaha-sandbox" 2026-01-01 2026-02-20
#   ./backfill.sh voice-prod "mutualofomaha,mutualofomaha-sandbox" 2026-01-01 2026-02-20 --dry-run
#   ./backfill.sh us-east-1-prod all 2026-01-01 2026-02-01
#
# Created: 2026-02-19

set -e

if [[ $# -lt 4 ]]; then
    echo "Usage: ./backfill.sh <cluster> <customers> <start_date> <end_date> [--dry-run]"
    echo ""
    echo "  cluster     - e.g., voice-prod, us-east-1-prod"
    echo "  customers   - comma-separated IDs, or \"all\" for all customers"
    echo "  start_date  - YYYY-MM-DD (inclusive)"
    echo "  end_date    - YYYY-MM-DD (exclusive)"
    echo ""
    echo "Example:"
    echo "  ./backfill.sh voice-prod \"mutualofomaha,mutualofomaha-sandbox\" 2026-01-01 2026-02-20"
    exit 1
fi

CLUSTER=$1
CUSTOMERS=$2
START_DATE=$3
END_DATE=$4
DRY_RUN=false
[[ "${5}" == "--dry-run" ]] && DRY_RUN=true

CONTEXT="${CLUSTER}_dev"
START_TIME="${START_DATE}T00:00:00Z"
END_TIME="${END_DATE}T00:00:00Z"

# Create a short suffix from the customer and date range
SHORT_CUSTOMER=$(echo "${CUSTOMERS}" | cut -d',' -f1 | cut -c1-10)
SUFFIX="${SHORT_CUSTOMER}-$(echo ${START_DATE} | tr -d '-')-$(echo ${END_DATE} | tr -d '-')"
JOB_NAME="batch-reindex-${SUFFIX}-$(date +%s)"

echo "============================================================"
echo "Backfill Scorecards"
echo "============================================================"
echo "Cluster:    ${CLUSTER}"
echo "Context:    ${CONTEXT}"
echo "Customers:  ${CUSTOMERS}"
echo "Start time: ${START_TIME}"
echo "End time:   ${END_TIME}"
echo "Job name:   ${JOB_NAME}"
echo "Dry run:    ${DRY_RUN}"
echo "============================================================"

# Step 1: Generate job YAML from cronjob template
echo "Creating job template..."
kubectl create job --from=cronjob/cron-batch-reindex-conversations \
    ${JOB_NAME} \
    -n cresta-cron \
    --context=${CONTEXT} \
    --dry-run=client -o yaml > /tmp/reindex-${SUFFIX}.yaml

# Step 2: Set environment variables
echo "Setting environment variables..."
if [[ "${CUSTOMERS}" == "all" ]]; then
    # No customer filter = all customers
    kubectl set env --local -f /tmp/reindex-${SUFFIX}.yaml \
        REINDEX_START_TIME="${START_TIME}" \
        REINDEX_END_TIME="${END_TIME}" \
        -o yaml > /tmp/reindex-${SUFFIX}-final.yaml
else
    kubectl set env --local -f /tmp/reindex-${SUFFIX}.yaml \
        REINDEX_START_TIME="${START_TIME}" \
        REINDEX_END_TIME="${END_TIME}" \
        RUN_ONLY_FOR_CUSTOMER_IDS="${CUSTOMERS}" \
        -o yaml > /tmp/reindex-${SUFFIX}-final.yaml
fi

if [[ "${DRY_RUN}" == "true" ]]; then
    echo ""
    echo "[DRY-RUN] Would create job: ${JOB_NAME}"
    echo ""
    echo "Env vars preview:"
    grep -A2 "REINDEX_START_TIME\|REINDEX_END_TIME\|RUN_ONLY_FOR_CUSTOMER_IDS" /tmp/reindex-${SUFFIX}-final.yaml || true
    echo ""
    echo "Full YAML at: /tmp/reindex-${SUFFIX}-final.yaml"
    exit 0
fi

# Step 3: Apply the job
echo "Applying job..."
kubectl apply -f /tmp/reindex-${SUFFIX}-final.yaml --context=${CONTEXT}

echo ""
echo "============================================================"
echo "Job created: ${JOB_NAME}"
echo ""
echo "Monitor:"
echo "  kubectl logs -n cresta-cron --context=${CONTEXT} -l job-name=${JOB_NAME} -f"
echo ""
echo "Temporal (after port-forward):"
echo "  kubectl --context=${CONTEXT} -n temporal port-forward svc/temporal-frontend-headless 7233:7233"
echo "  temporal workflow list --namespace ingestion --address localhost:7233 \\"
echo "    --query 'WorkflowId STARTS_WITH \"reindexconversations-${SHORT_CUSTOMER}\"'"
echo "============================================================"
