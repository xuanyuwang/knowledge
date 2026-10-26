#!/bin/zsh
# Delete existing conversation_with_labels data for a customer and date range.
# Executes ALTER TABLE DELETE via kubectl exec into a ClickHouse pod.
#
# Must run BEFORE backfill to avoid duplicate rows (see backfill-plan.md).
#
# Usage:
#   ./delete_existing.sh <cluster> <customer> <start_date> <end_date> [--dry-run]
#   ./delete_existing.sh <cluster> --all <start_date> <end_date> [--dry-run]
#
# Examples:
#   ./delete_existing.sh us-east-1-prod alaska-air 2026-01-01 2026-02-19 --dry-run
#   ./delete_existing.sh us-east-1-prod alaska-air 2026-01-01 2026-02-19
#   ./delete_existing.sh us-east-1-prod --all 2026-01-01 2026-02-19
#
# Prerequisites:
#   - VPN connected
#   - kubectl access to <cluster>_dev
#   - ClickHouse pod accessible in the clickhouse namespace
#
# Created: 2026-02-18

set -e

# ---- Args ----

CLUSTER=${1:?"Usage: $0 <cluster> <customer|--all> <start_date> <end_date> [--dry-run]"}
CUSTOMER=${2:?"Missing customer (or --all)"}
START_DATE=${3:?"Missing start_date (YYYY-MM-DD)"}
END_DATE=${4:?"Missing end_date (YYYY-MM-DD)"}
DRY_RUN=${5:-""}

CONTEXT="${CLUSTER}_dev"
CH_NAMESPACE="clickhouse"
DATABASE="conversations"

# ---- Discover ClickHouse pod ----

echo "Discovering ClickHouse pod in ${CH_NAMESPACE} namespace..."
CH_POD=$(kubectl get pods -n ${CH_NAMESPACE} --context=${CONTEXT} \
    -l "clickhouse.altinity.com/app=chop" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [[ -z "${CH_POD}" ]]; then
    echo "ERROR: No ClickHouse pod found. Trying alternative label..."
    CH_POD=$(kubectl get pods -n ${CH_NAMESPACE} --context=${CONTEXT} \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
fi

if [[ -z "${CH_POD}" ]]; then
    echo "ERROR: Could not find ClickHouse pod in namespace ${CH_NAMESPACE}"
    echo "You can specify it manually:"
    echo "  CH_POD=<pod-name> $0 $@"
    exit 1
fi

echo "Using ClickHouse pod: ${CH_POD}"

# ---- Build DELETE query ----

if [[ "${CUSTOMER}" == "--all" ]]; then
    WHERE_CLAUSE="conversation_end_time >= '${START_DATE} 00:00:00' AND conversation_end_time < '${END_DATE} 00:00:00'"
    SCOPE="ALL customers"
else
    WHERE_CLAUSE="customer_id = '${CUSTOMER}' AND conversation_end_time >= '${START_DATE} 00:00:00' AND conversation_end_time < '${END_DATE} 00:00:00'"
    SCOPE="customer=${CUSTOMER}"
fi

DELETE_SQL="ALTER TABLE ${DATABASE}.conversation_with_labels ON CLUSTER 'conversations' DELETE WHERE ${WHERE_CLAUSE} SETTINGS replication_wait_for_inactive_replica_timeout = 0"

COUNT_SQL="SELECT count() FROM ${DATABASE}.conversation_with_labels_d WHERE ${WHERE_CLAUSE}"

CHECK_MUTATIONS_SQL="SELECT database, table, mutation_id, command, is_done, parts_to_do FROM system.mutations WHERE table = 'conversation_with_labels' AND is_done = 0 FORMAT PrettyCompact"

# ---- Display ----

echo ""
echo "============================================================"
echo "Delete conversation_with_labels data"
echo "============================================================"
echo "Cluster:    ${CLUSTER}"
echo "Scope:      ${SCOPE}"
echo "Date range: ${START_DATE} to ${END_DATE}"
echo "CH Pod:     ${CH_POD}"
echo "Dry run:    ${DRY_RUN:-no}"
echo "============================================================"

# ---- Count existing rows ----

echo ""
echo "Counting existing rows..."
ROW_COUNT=$(kubectl exec -n ${CH_NAMESPACE} --context=${CONTEXT} ${CH_POD} -- \
    clickhouse-client --query="${COUNT_SQL}" 2>/dev/null || echo "ERROR")

echo "Rows to delete: ${ROW_COUNT}"

if [[ "${ROW_COUNT}" == "0" ]]; then
    echo "No rows to delete. Exiting."
    exit 0
fi

# ---- Dry-run or execute ----

echo ""
echo "DELETE SQL:"
echo "  ${DELETE_SQL}"
echo ""

if [[ "${DRY_RUN}" == "--dry-run" ]]; then
    echo "[DRY-RUN] Would delete ${ROW_COUNT} rows."
    echo ""
    echo "To execute for real, run without --dry-run"
    exit 0
fi

# Confirm
echo "WARNING: This will delete ${ROW_COUNT} rows from conversation_with_labels."
echo -n "Type 'yes' to proceed: "
read CONFIRM
if [[ "${CONFIRM}" != "yes" ]]; then
    echo "Aborted."
    exit 1
fi

# Execute DELETE
echo "Executing DELETE..."
kubectl exec -n ${CH_NAMESPACE} --context=${CONTEXT} ${CH_POD} -- \
    clickhouse-client --query="${DELETE_SQL}"

echo "DELETE mutation submitted."

# ---- Wait for mutation ----

echo ""
echo "Checking mutation status..."
sleep 2

kubectl exec -n ${CH_NAMESPACE} --context=${CONTEXT} ${CH_POD} -- \
    clickhouse-client --query="${CHECK_MUTATIONS_SQL}" 2>/dev/null || true

echo ""
echo "============================================================"
echo "Mutation submitted. Monitor with:"
echo "  kubectl exec -n ${CH_NAMESPACE} --context=${CONTEXT} ${CH_POD} -- \\"
echo "    clickhouse-client --query=\"${CHECK_MUTATIONS_SQL}\""
echo ""
echo "Wait until is_done=1 for all parts before running backfill."
echo "============================================================"
