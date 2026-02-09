#!/bin/zsh
# Re-run backfill for cvs and oportun for a single day
# Usage: ./rerun_single_day.sh <date> [--dry-run]
# Example: ./rerun_single_day.sh 2026-01-01 --dry-run

set -e

DATE=${1:-"2026-01-01"}
DRY_RUN=${2:-""}
CLUSTER="us-west-2-prod"
CONTEXT="${CLUSTER}_dev"
CUSTOMERS="cvs,oportun"

# Calculate start and end times
START_TIME="${DATE}T00:00:00Z"
# Calculate next day
NEXT_DATE=$(date -j -f "%Y-%m-%d" -v+1d "$DATE" "+%Y-%m-%d" 2>/dev/null || date -d "$DATE + 1 day" "+%Y-%m-%d")
END_TIME="${NEXT_DATE}T00:00:00Z"

# Create suffix from date (e.g., jan01)
MONTH=$(date -j -f "%Y-%m-%d" "$DATE" "+%b" 2>/dev/null | tr '[:upper:]' '[:lower:]' || date -d "$DATE" "+%b" | tr '[:upper:]' '[:lower:]')
DAY=$(date -j -f "%Y-%m-%d" "$DATE" "+%d" 2>/dev/null || date -d "$DATE" "+%d")
SUFFIX="${MONTH}${DAY}"

JOB_NAME="batch-reindex-day-${SUFFIX}-$(date +%s)"

echo "============================================================"
echo "Re-run Backfill for Single Day"
echo "============================================================"
echo "Date:      ${DATE}"
echo "Start:     ${START_TIME}"
echo "End:       ${END_TIME}"
echo "Cluster:   ${CLUSTER}"
echo "Customers: ${CUSTOMERS}"
echo "Job name:  ${JOB_NAME}"
echo "Dry run:   ${DRY_RUN:-no}"
echo "============================================================"

# Step 1: Generate job YAML from cronjob template
echo "Creating job template..."
kubectl create job --from=cronjob/cron-batch-reindex-conversations \
    ${JOB_NAME} \
    -n cresta-cron \
    --context=${CONTEXT} \
    --dry-run=client -o yaml > /tmp/reindex-job-${SUFFIX}.yaml

# Step 2: Set environment variables
echo "Setting environment variables..."
kubectl set env --local -f /tmp/reindex-job-${SUFFIX}.yaml \
    REINDEX_START_TIME="${START_TIME}" \
    REINDEX_END_TIME="${END_TIME}" \
    RUN_ONLY_FOR_CUSTOMER_IDS="${CUSTOMERS}" \
    -o yaml > /tmp/reindex-job-${SUFFIX}-final.yaml

if [[ "${DRY_RUN}" == "--dry-run" ]]; then
    echo "[DRY-RUN] Would create job: ${JOB_NAME}"
    echo ""
    echo "YAML preview:"
    grep -A2 "REINDEX_START_TIME\|REINDEX_END_TIME\|RUN_ONLY_FOR_CUSTOMER_IDS" /tmp/reindex-job-${SUFFIX}-final.yaml || true
    exit 0
fi

# Step 3: Apply the job
echo "Applying job..."
kubectl apply -f /tmp/reindex-job-${SUFFIX}-final.yaml --context=${CONTEXT}

echo ""
echo "============================================================"
echo "Job created: ${JOB_NAME}"
echo ""
echo "Monitor with:"
echo "  kubectl logs -n cresta-cron --context=${CONTEXT} -l job-name=${JOB_NAME} -f"
echo "============================================================"
