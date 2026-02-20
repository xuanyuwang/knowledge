#!/bin/zsh
# Re-run backfill for cvs and oportun sequentially - one day at a time
# Usage: ./rerun_sequential.sh [start_day] [end_day]
# Example: ./rerun_sequential.sh 2 31  # Run Jan 2-31

set -e

START_DAY=${1:-2}
END_DAY=${2:-31}
CLUSTER="us-west-2-prod"
CONTEXT="${CLUSTER}_dev"
CUSTOMERS="cvs,oportun"
TEMPORAL_NS="ingestion"

echo "============================================================"
echo "Sequential Backfill for cvs/oportun"
echo "============================================================"
echo "Cluster:   ${CLUSTER}"
echo "Customers: ${CUSTOMERS}"
echo "Days:      Jan ${START_DAY} to Jan ${END_DAY}"
echo "============================================================"
echo ""

# Function to wait for workflows to complete
wait_for_workflows() {
    local workflow_ids=("$@")
    local max_wait=3600  # 1 hour max per day
    local elapsed=0
    local check_interval=30

    echo "Waiting for ${#workflow_ids[@]} workflow(s) to complete..."

    while [[ $elapsed -lt $max_wait ]]; do
        # Check status of all workflows
        local all_done=true
        local statuses=""

        for wf_id in "${workflow_ids[@]}"; do
            local status=$(temporal workflow describe \
                --namespace ${TEMPORAL_NS} \
                --address localhost:7233 \
                --workflow-id "$wf_id" \
                --output json 2>/dev/null | \
                python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('workflowExecutionInfo',{}).get('status','UNKNOWN'))" 2>/dev/null || echo "UNKNOWN")

            statuses="${statuses}${wf_id##*-}: ${status##*_}, "

            if [[ "$status" == *"RUNNING"* ]]; then
                all_done=false
            elif [[ "$status" == *"FAILED"* ]] || [[ "$status" == *"TIMED_OUT"* ]]; then
                echo "  WARNING: Workflow failed - $wf_id: $status"
            fi
        done

        if $all_done; then
            echo "  All workflows completed!"
            return 0
        fi

        echo "  [${elapsed}s] Status: ${statuses%%, }"
        sleep $check_interval
        elapsed=$((elapsed + check_interval))
    done

    echo "  TIMEOUT: Workflows did not complete within ${max_wait}s"
    return 1
}

# Start port-forward in background
pkill -f "port-forward.*7233" 2>/dev/null || true
sleep 2
kubectl --context=${CONTEXT} -n temporal port-forward svc/temporal-frontend-headless 7233:7233 &
PF_PID=$!
sleep 5

# Trap to cleanup port-forward on exit
trap "kill $PF_PID 2>/dev/null" EXIT

for DAY in $(seq $START_DAY $END_DAY); do
    DATE=$(printf "2026-01-%02d" $DAY)

    if [[ $DAY -eq 31 ]]; then
        NEXT_DATE="2026-02-01"
    else
        NEXT_DATE=$(printf "2026-01-%02d" $((DAY + 1)))
    fi

    START_TIME="${DATE}T00:00:00Z"
    END_TIME="${NEXT_DATE}T00:00:00Z"
    SUFFIX="jan$(printf "%02d" $DAY)"
    JOB_NAME="batch-reindex-seq-${SUFFIX}-$(date +%s)"

    echo ""
    echo "============================================================"
    echo "Day $DAY: ${DATE}"
    echo "  Start: ${START_TIME}"
    echo "  End:   ${END_TIME}"
    echo "  Job:   ${JOB_NAME}"
    echo "============================================================"

    # Create and apply job
    kubectl create job --from=cronjob/cron-batch-reindex-conversations \
        ${JOB_NAME} \
        -n cresta-cron \
        --context=${CONTEXT} \
        --dry-run=client -o yaml > /tmp/reindex-job-${SUFFIX}.yaml

    kubectl set env --local -f /tmp/reindex-job-${SUFFIX}.yaml \
        REINDEX_START_TIME="${START_TIME}" \
        REINDEX_END_TIME="${END_TIME}" \
        RUN_ONLY_FOR_CUSTOMER_IDS="${CUSTOMERS}" \
        -o yaml > /tmp/reindex-job-${SUFFIX}-final.yaml

    kubectl apply -f /tmp/reindex-job-${SUFFIX}-final.yaml --context=${CONTEXT}
    echo "  Job created: ${JOB_NAME}"

    # Wait for job to spawn workflows (k8s job completes quickly)
    sleep 10

    # Find the workflows that were just created
    echo "  Looking for spawned workflows..."
    WORKFLOW_IDS=()

    # Query for workflows started in the last minute for cvs and oportun
    while IFS= read -r wf_id; do
        if [[ -n "$wf_id" ]]; then
            WORKFLOW_IDS+=("$wf_id")
            echo "  Found: $wf_id"
        fi
    done < <(temporal workflow list \
        --namespace ${TEMPORAL_NS} \
        --address localhost:7233 \
        --query "ExecutionStatus = \"Running\" AND (WorkflowId STARTS_WITH \"reindexconversations-cvs-us-west-2\" OR WorkflowId STARTS_WITH \"reindexconversations-oportun-us-west-2\")" \
        --output json 2>/dev/null | \
        python3 -c "
import json, sys
from datetime import datetime, timedelta, timezone
data = json.load(sys.stdin)
now = datetime.now(timezone.utc)
cutoff = now - timedelta(minutes=2)
for wf in data:
    start_str = wf.get('startTime', '')
    if start_str:
        # Parse ISO format
        start = datetime.fromisoformat(start_str.replace('Z', '+00:00'))
        if start > cutoff:
            print(wf.get('execution', {}).get('workflowId', ''))
" 2>/dev/null)

    if [[ ${#WORKFLOW_IDS[@]} -eq 0 ]]; then
        echo "  WARNING: No workflows found, checking completed..."
        # Maybe they completed very quickly
        sleep 5
        continue
    fi

    # Wait for workflows to complete
    wait_for_workflows "${WORKFLOW_IDS[@]}"

    echo "  Day $DAY complete!"
done

echo ""
echo "============================================================"
echo "All days completed!"
echo "============================================================"
