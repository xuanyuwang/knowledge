#!/bin/zsh
# Re-run backfill for the 4 failed customers
# Usage: ./rerun_failed.sh [--dry-run]

set -e

DRY_RUN=${1:-""}
START_TIME="2026-01-01T00:00:00Z"
END_TIME="2026-02-01T00:00:00Z"

# Failed customers
declare -A FAILED_CUSTOMERS
FAILED_CUSTOMERS=(
    ["us-east-1-prod"]="marriott,united-east"
    ["us-west-2-prod"]="cvs,oportun"
)

echo "============================================================"
echo "Re-run Backfill for Failed Customers"
echo "============================================================"
echo "Start time: ${START_TIME}"
echo "End time:   ${END_TIME}"
echo "Dry run:    ${DRY_RUN:-no}"
echo ""

for CLUSTER in "${(@k)FAILED_CUSTOMERS}"; do
    CUSTOMERS="${FAILED_CUSTOMERS[$CLUSTER]}"
    CONTEXT="${CLUSTER}_dev"
    JOB_NAME="batch-reindex-retry-$(date +%s)"

    echo "============================================================"
    echo "Cluster:   ${CLUSTER}"
    echo "Customers: ${CUSTOMERS}"
    echo "Job name:  ${JOB_NAME}"
    echo "============================================================"

    # Step 1: Generate job YAML from cronjob template
    echo "Creating job template..."
    kubectl create job --from=cronjob/cron-batch-reindex-conversations \
        ${JOB_NAME} \
        -n cresta-cron \
        --context=${CONTEXT} \
        --dry-run=client -o yaml > /tmp/reindex-job-${CLUSTER}.yaml

    # Step 2: Set environment variables with specific customers
    echo "Setting environment variables..."
    kubectl set env --local -f /tmp/reindex-job-${CLUSTER}.yaml \
        REINDEX_START_TIME="${START_TIME}" \
        REINDEX_END_TIME="${END_TIME}" \
        RUN_ONLY_FOR_CUSTOMER_IDS="${CUSTOMERS}" \
        -o yaml > /tmp/reindex-job-${CLUSTER}-final.yaml

    if [[ "${DRY_RUN}" == "--dry-run" ]]; then
        echo "[DRY-RUN] Would create job: ${JOB_NAME}"
        echo ""
        continue
    fi

    # Step 3: Apply the job
    echo "Applying job..."
    kubectl apply -f /tmp/reindex-job-${CLUSTER}-final.yaml --context=${CONTEXT}

    echo ""
    echo "Job created: ${JOB_NAME}"
    echo "To view logs:"
    echo "  kubectl logs -n cresta-cron --context=${CONTEXT} -l job-name=${JOB_NAME} -f"
    echo ""

    # Wait a bit between clusters to avoid potential conflicts
    sleep 2
done

echo "============================================================"
echo "Done! Monitor the jobs with:"
echo "  kubectl get jobs -n cresta-cron --context=us-east-1-prod_dev | grep retry"
echo "  kubectl get jobs -n cresta-cron --context=us-west-2-prod_dev | grep retry"
echo "============================================================"
