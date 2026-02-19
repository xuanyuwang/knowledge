#!/bin/zsh
# Backfill conversation_with_labels for a specific customer and date range.
# Creates a k8s job from the cron-label-conversations cronjob template with backfill env vars.
#
# Usage:
#   ./backfill.sh <cluster> <customer> <start_date> <end_date> [--dry-run]
#
# Examples:
#   ./backfill.sh us-east-1-prod alaska-air 2026-01-01 2026-02-19 --dry-run
#   ./backfill.sh us-east-1-prod alaska-air 2026-01-01 2026-02-19
#   ./backfill.sh us-east-1-prod "" 2026-01-01 2026-02-19    # all customers
#
# Prerequisites:
#   - VPN connected
#   - kubectl access to <cluster>_dev
#   - PR go-servers#25706 deployed (ended_at filter fix)
#
# Created: 2026-02-18

set -e

# ---- Args ----

CLUSTER=${1:?"Usage: $0 <cluster> <customer> <start_date> <end_date> [--dry-run]"}
CUSTOMER=${2:-""}
START_DATE=${3:?"Missing start_date (YYYY-MM-DD)"}
END_DATE=${4:?"Missing end_date (YYYY-MM-DD)"}
DRY_RUN=${5:-""}

CONTEXT="${CLUSTER}_dev"
START_TIME="${START_DATE}T00:00:00Z"
END_TIME="${END_DATE}T00:00:00Z"

# Job name: include customer (or "all") and timestamp for uniqueness
if [[ -n "${CUSTOMER}" ]]; then
    JOB_NAME="backfill-labels-${CUSTOMER}-$(date +%s)"
else
    JOB_NAME="backfill-labels-all-$(date +%s)"
fi

# ---- Display ----

echo "============================================================"
echo "Backfill conversation_with_labels"
echo "============================================================"
echo "Cluster:    ${CLUSTER}"
echo "Context:    ${CONTEXT}"
echo "Customer:   ${CUSTOMER:-all}"
echo "Start time: ${START_TIME}"
echo "End time:   ${END_TIME}"
echo "Job name:   ${JOB_NAME}"
echo "Dry run:    ${DRY_RUN:-no}"
echo "============================================================"

# ---- Step 1: Generate job YAML from cronjob template ----

echo "Creating job template from cron-label-conversations..."
kubectl create job --from=cronjob/cron-label-conversations \
    ${JOB_NAME} \
    -n cresta-cron \
    --context=${CONTEXT} \
    --dry-run=client -o yaml > /tmp/label-job-${JOB_NAME}.yaml

# ---- Step 2: Set environment variables ----

echo "Setting environment variables..."

# Build env var list
ENV_VARS=(
    "ENABLE_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE=true"
    "LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_START_AT_RANGE_START=${START_TIME}"
    "LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_END_AT_RANGE_END=${END_TIME}"
)
if [[ -n "${CUSTOMER}" ]]; then
    ENV_VARS+=("FILTER_CUSTOMER_IN_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE=${CUSTOMER}")
fi

kubectl set env --local -f /tmp/label-job-${JOB_NAME}.yaml \
    "${ENV_VARS[@]}" \
    -o yaml > /tmp/label-job-${JOB_NAME}-final.yaml

# ---- Step 3: Dry-run or apply ----

if [[ "${DRY_RUN}" == "--dry-run" ]]; then
    echo ""
    echo "[DRY-RUN] Would create job: ${JOB_NAME}"
    echo ""
    echo "Env vars preview:"
    grep -A1 "ENABLE_LABEL_CONVERSATIONS\|LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE\|FILTER_CUSTOMER" \
        /tmp/label-job-${JOB_NAME}-final.yaml || true
    echo ""
    echo "Full YAML: /tmp/label-job-${JOB_NAME}-final.yaml"
    exit 0
fi

echo "Applying job..."
kubectl apply -f /tmp/label-job-${JOB_NAME}-final.yaml --context=${CONTEXT}

echo ""
echo "============================================================"
echo "Job created: ${JOB_NAME}"
echo ""
echo "Monitor with:"
echo "  kubectl logs -n cresta-cron --context=${CONTEXT} -l job-name=${JOB_NAME} -f"
echo ""
echo "Wait for completion:"
echo "  kubectl wait --for=condition=complete job/${JOB_NAME} -n cresta-cron --context=${CONTEXT} --timeout=3600s"
echo "============================================================"
