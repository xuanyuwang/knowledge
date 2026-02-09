#!/bin/zsh
# Re-run backfill for cvs and oportun with split time ranges
# Usage: ./rerun_split.sh [--dry-run]

set -e

DRY_RUN=${1:-""}
CLUSTER="us-west-2-prod"
CONTEXT="${CLUSTER}_dev"
CUSTOMERS="cvs,oportun"

# Split January into 3 periods (~10 days each)
declare -a TIME_RANGES
TIME_RANGES=(
    "2026-01-01T00:00:00Z|2026-01-11T00:00:00Z|jan01-10"
    "2026-01-11T00:00:00Z|2026-01-21T00:00:00Z|jan11-20"
    "2026-01-21T00:00:00Z|2026-02-01T00:00:00Z|jan21-31"
)

echo "============================================================"
echo "Re-run Backfill for cvs/oportun (Split Time Ranges)"
echo "============================================================"
echo "Cluster:   ${CLUSTER}"
echo "Customers: ${CUSTOMERS}"
echo "Dry run:   ${DRY_RUN:-no}"
echo ""
echo "Time ranges:"
echo "  1. Jan 01-10"
echo "  2. Jan 11-20"
echo "  3. Jan 21-31"
echo ""

for RANGE in "${TIME_RANGES[@]}"; do
    IFS='|' read -r START_TIME END_TIME SUFFIX <<< "$RANGE"
    JOB_NAME="batch-reindex-split-${SUFFIX}-$(date +%s)"

    echo "============================================================"
    echo "Period:     ${SUFFIX}"
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
        continue
    fi

    # Step 3: Apply the job
    echo "Applying job..."
    kubectl apply -f /tmp/reindex-job-${SUFFIX}-final.yaml --context=${CONTEXT}

    echo ""
    echo "Job created: ${JOB_NAME}"
    echo ""

    # Wait between jobs to stagger them
    sleep 5
done

echo "============================================================"
echo "Done! Monitor with:"
echo "  kubectl get jobs -n cresta-cron --context=${CONTEXT} | grep split"
echo "============================================================"
