#!/bin/zsh
# Re-run backfill for cvs and oportun for all days in January 2026
# Usage: ./rerun_all_days.sh [--dry-run]

set -e

DRY_RUN=${1:-""}
CLUSTER="us-west-2-prod"
CONTEXT="${CLUSTER}_dev"
CUSTOMERS="cvs,oportun"

echo "============================================================"
echo "Re-run Backfill for All Days in January 2026"
echo "============================================================"
echo "Cluster:   ${CLUSTER}"
echo "Customers: ${CUSTOMERS}"
echo "Days:      31 (2026-01-01 to 2026-01-31)"
echo "Dry run:   ${DRY_RUN:-no}"
echo "============================================================"
echo ""

# Skip Jan 1 since it's already done
for DAY in $(seq 2 31); do
    DATE=$(printf "2026-01-%02d" $DAY)
    NEXT_DAY=$((DAY + 1))

    if [[ $DAY -eq 31 ]]; then
        NEXT_DATE="2026-02-01"
    else
        NEXT_DATE=$(printf "2026-01-%02d" $NEXT_DAY)
    fi

    START_TIME="${DATE}T00:00:00Z"
    END_TIME="${NEXT_DATE}T00:00:00Z"

    SUFFIX="jan$(printf "%02d" $DAY)"
    JOB_NAME="batch-reindex-day-${SUFFIX}-$(date +%s)"

    echo "------------------------------------------------------------"
    echo "Day $DAY: ${DATE}"
    echo "  Start: ${START_TIME}"
    echo "  End:   ${END_TIME}"
    echo "  Job:   ${JOB_NAME}"

    # Step 1: Generate job YAML from cronjob template
    kubectl create job --from=cronjob/cron-batch-reindex-conversations \
        ${JOB_NAME} \
        -n cresta-cron \
        --context=${CONTEXT} \
        --dry-run=client -o yaml > /tmp/reindex-job-${SUFFIX}.yaml

    # Step 2: Set environment variables
    kubectl set env --local -f /tmp/reindex-job-${SUFFIX}.yaml \
        REINDEX_START_TIME="${START_TIME}" \
        REINDEX_END_TIME="${END_TIME}" \
        RUN_ONLY_FOR_CUSTOMER_IDS="${CUSTOMERS}" \
        -o yaml > /tmp/reindex-job-${SUFFIX}-final.yaml

    if [[ "${DRY_RUN}" == "--dry-run" ]]; then
        echo "  [DRY-RUN] Would create job"
        continue
    fi

    # Step 3: Apply the job
    kubectl apply -f /tmp/reindex-job-${SUFFIX}-final.yaml --context=${CONTEXT}
    echo "  Created: ${JOB_NAME}"

    # Small delay between jobs
    sleep 2
done

echo ""
echo "============================================================"
echo "Done! Created jobs for 30 days (Jan 2-31)"
echo "Jan 1 was already completed earlier."
echo ""
echo "Monitor with:"
echo "  kubectl get jobs -n cresta-cron --context=${CONTEXT} | grep 'batch-reindex-day'"
echo "============================================================"
