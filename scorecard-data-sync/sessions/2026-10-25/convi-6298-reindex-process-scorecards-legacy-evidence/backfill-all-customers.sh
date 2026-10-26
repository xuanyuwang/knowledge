#!/bin/zsh
# Backfill process scorecards for all customers on a given prod cluster.
#
# Usage:
#   ./backfill-all-customers.sh <cluster> [phase]
#
# Examples:
#   ./backfill-all-customers.sh voice-prod 2026      # Phase 1: 2026 only
#   ./backfill-all-customers.sh voice-prod pre-2026   # Phase 2: everything before 2026
#   ./backfill-all-customers.sh voice-prod all         # Full backfill: 2020-now
#   ./backfill-all-customers.sh voice-prod --dry-run   # Preview only
#   ./backfill-all-customers.sh voice-prod 2026 --dry-run
#
# Clusters: voice-prod, chat-prod, us-west-2-prod, us-east-1-prod,
#           ap-southeast-2-prod, eu-west-2-prod, ca-central-1-prod, schwab-prod
#
# Created: 2026-03-28

set -euo pipefail

# ── Args ─────────────────────────────────────────────────────────────────────
CLUSTER="${1:?Usage: $0 <cluster> [phase] [--dry-run]}"
PHASE="${2:-2026}"
DRY_RUN=false

# Check for --dry-run in any position
for arg in "$@"; do
    [[ "$arg" == "--dry-run" ]] && DRY_RUN=true
done

# If phase is --dry-run, default to 2026
[[ "$PHASE" == "--dry-run" ]] && PHASE="2026"

CONTEXT="${CLUSTER}_dev"

# ── Time ranges by phase ─────────────────────────────────────────────────────
case "$PHASE" in
    2026)
        START_TIME="2026-01-01T00:00:00Z"
        END_TIME="2026-03-29T00:00:00Z"
        ;;
    pre-2026)
        START_TIME="2020-01-01T00:00:00Z"
        END_TIME="2026-01-01T00:00:00Z"
        ;;
    all)
        START_TIME="2020-01-01T00:00:00Z"
        END_TIME="2026-03-29T00:00:00Z"
        ;;
    *)
        echo "Unknown phase: $PHASE (use: 2026, pre-2026, all)"
        exit 1
        ;;
esac

JOB_NAME="reindex-process-${PHASE}-$(date +%s)"

# ── Summary ──────────────────────────────────────────────────────────────────
echo "============================================================"
echo "Backfill Process Scorecards — All Customers"
echo "============================================================"
echo "Cluster:    ${CLUSTER}"
echo "Context:    ${CONTEXT}"
echo "Phase:      ${PHASE}"
echo "Start time: ${START_TIME}"
echo "End time:   ${END_TIME}"
echo "Job name:   ${JOB_NAME}"
echo "Dry run:    ${DRY_RUN}"
echo "Clean up:   true (idempotent)"
echo "Scope:      ALL customers (no RUN_ONLY_FOR_CUSTOMER_IDS)"
echo "============================================================"

# ── Step 1: Generate job YAML from cronjob template ──────────────────────────
echo ""
echo "Step 1: Generating job YAML from cron-batch-reindex-conversations..."
YAML_FILE="/tmp/${JOB_NAME}.yaml"

kubectl create job --from=cronjob/cron-batch-reindex-conversations \
    "${JOB_NAME}" \
    -n cresta-cron \
    --context="${CONTEXT}" \
    --dry-run=client -o yaml > "${YAML_FILE}"

# ── Step 2: Set environment variables ────────────────────────────────────────
echo "Step 2: Setting environment variables..."
kubectl set env --local -f "${YAML_FILE}" \
    REINDEX_MODE="process" \
    REINDEX_START_TIME="${START_TIME}" \
    REINDEX_END_TIME="${END_TIME}" \
    REINDEX_SCORECARDS_CLEAN_UP_BEFORE_WRITE="true" \
    -o yaml > "${YAML_FILE}.final.yaml"

mv "${YAML_FILE}.final.yaml" "${YAML_FILE}"

# ── Dry run: show and exit ───────────────────────────────────────────────────
if [[ "${DRY_RUN}" == "true" ]]; then
    echo ""
    echo "[DRY-RUN] Would create job: ${JOB_NAME}"
    echo ""
    echo "Env vars in YAML:"
    grep -E "name: (REINDEX_|RUN_ONLY)" "${YAML_FILE}" -A1 || true
    echo ""
    echo "Full YAML at: ${YAML_FILE}"
    echo ""
    echo "To apply:"
    echo "  kubectl apply -f ${YAML_FILE} --context=${CONTEXT}"
    exit 0
fi

# ── Step 3: Confirm and apply ────────────────────────────────────────────────
echo ""
echo "About to create job for ALL customers on ${CLUSTER}."
read -q "REPLY?Proceed? [y/N] " || { echo "\nAborted."; exit 1; }
echo ""

echo "Step 3: Applying job..."
kubectl apply -f "${YAML_FILE}" --context="${CONTEXT}"

echo ""
echo "============================================================"
echo "Job created: ${JOB_NAME}"
echo ""
echo "Monitor:"
echo "  kubectl logs -n cresta-cron --context=${CONTEXT} -l job-name=${JOB_NAME} -f"
echo ""
echo "Check status:"
echo "  kubectl get job ${JOB_NAME} -n cresta-cron --context=${CONTEXT}"
echo "============================================================"

# Save for reference
echo "${JOB_NAME}" > /tmp/reindex-process-backfill-latest-${CLUSTER}.txt
echo "Job name saved to /tmp/reindex-process-backfill-latest-${CLUSTER}.txt"
