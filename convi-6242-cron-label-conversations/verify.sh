#!/bin/zsh
# Verify backfill results for conversation_with_labels.
# Runs ClickHouse queries to check for duplicates, row counts, and Active Days.
#
# Usage:
#   ./verify.sh <cluster> <customer> <start_date> <end_date>
#
# Examples:
#   ./verify.sh us-east-1-prod alaska-air 2026-01-01 2026-02-19
#
# Prerequisites:
#   - VPN connected
#   - kubectl access to <cluster>_dev
#
# Created: 2026-02-18

set -e

# ---- Args ----

CLUSTER=${1:?"Usage: $0 <cluster> <customer> <start_date> <end_date>"}
CUSTOMER=${2:?"Missing customer"}
START_DATE=${3:?"Missing start_date (YYYY-MM-DD)"}
END_DATE=${4:?"Missing end_date (YYYY-MM-DD)"}

CONTEXT="${CLUSTER}_dev"
CH_NAMESPACE="clickhouse"
DATABASE="conversations"

# ---- Discover ClickHouse pod ----

CH_POD=$(kubectl get pods -n ${CH_NAMESPACE} --context=${CONTEXT} \
    -l "clickhouse.altinity.com/app=chop" \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [[ -z "${CH_POD}" ]]; then
    CH_POD=$(kubectl get pods -n ${CH_NAMESPACE} --context=${CONTEXT} \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
fi

if [[ -z "${CH_POD}" ]]; then
    echo "ERROR: Could not find ClickHouse pod"
    exit 1
fi

run_query() {
    local desc="$1"
    local sql="$2"

    echo ""
    echo "------------------------------------------------------------"
    echo "CHECK: ${desc}"
    echo "------------------------------------------------------------"
    echo "SQL: ${sql}"
    echo ""
    kubectl exec -n ${CH_NAMESPACE} --context=${CONTEXT} ${CH_POD} -- \
        clickhouse-client --query="${sql}" 2>/dev/null || echo "  ERROR running query"
}

# ---- Display ----

echo "============================================================"
echo "Verify conversation_with_labels Backfill"
echo "============================================================"
echo "Cluster:    ${CLUSTER}"
echo "Customer:   ${CUSTOMER}"
echo "Date range: ${START_DATE} to ${END_DATE}"
echo "CH Pod:     ${CH_POD}"
echo "============================================================"

# ---- Check 1: Pending mutations ----

run_query "Pending mutations (should be 0)" \
    "SELECT database, table, mutation_id, is_done, parts_to_do FROM system.mutations WHERE table = 'conversation_with_labels' AND is_done = 0 FORMAT PrettyCompact"

# ---- Check 2: Total row count in conversation_with_labels ----

run_query "Row count in conversation_with_labels" \
    "SELECT count() as label_rows FROM ${DATABASE}.conversation_with_labels_d WHERE customer_id = '${CUSTOMER}' AND conversation_end_time >= '${START_DATE}' AND conversation_end_time < '${END_DATE}'"

# ---- Check 3: Total conversation count in conversation_d (reference) ----

run_query "Row count in conversation_d (reference)" \
    "SELECT count() as conv_rows FROM ${DATABASE}.conversation_d WHERE customer_id = '${CUSTOMER}' AND ended_at >= '${START_DATE}' AND ended_at < '${END_DATE}' AND ended_at IS NOT NULL AND ended_at > '1970-01-01'"

# ---- Check 4: Duplicate conversation_ids (same conv, multiple rows) ----

run_query "Duplicate conversation_ids (should be 0 rows)" \
    "SELECT conversation_id, count() as cnt FROM ${DATABASE}.conversation_with_labels_d FINAL WHERE customer_id = '${CUSTOMER}' AND conversation_end_time >= '${START_DATE}' AND conversation_end_time < '${END_DATE}' GROUP BY conversation_id HAVING cnt > 1 LIMIT 20 FORMAT PrettyCompact"

# ---- Check 5: Duplicate conversation_ids WITHOUT FINAL (raw duplicates from stale ORDER BY) ----

run_query "Raw duplicate conversation_ids without FINAL (pre-merge duplicates)" \
    "SELECT conversation_id, groupArray(agent_user_id) as agents, groupArray(toStartOfHour(conversation_end_time)) as end_hours, count() as cnt FROM ${DATABASE}.conversation_with_labels WHERE customer_id = '${CUSTOMER}' AND conversation_end_time >= '${START_DATE}' AND conversation_end_time < '${END_DATE}' GROUP BY conversation_id HAVING cnt > 1 LIMIT 10 FORMAT PrettyCompact"

# ---- Check 6: Active Days sample (per-agent daily breakdown for a recent week) ----

run_query "Active Days per agent (last 7 days sample)" \
    "SELECT agent_user_id, toDate(conversation_end_time) as day, count() as convs FROM ${DATABASE}.conversation_with_labels_d FINAL WHERE customer_id = '${CUSTOMER}' AND conversation_end_time >= now() - INTERVAL 7 DAY GROUP BY agent_user_id, day ORDER BY day DESC, convs DESC LIMIT 30 FORMAT PrettyCompact"

# ---- Check 7: Usecase distribution ----

run_query "Usecase distribution" \
    "SELECT usecase_id, count() as cnt FROM ${DATABASE}.conversation_with_labels_d FINAL WHERE customer_id = '${CUSTOMER}' AND conversation_end_time >= '${START_DATE}' AND conversation_end_time < '${END_DATE}' GROUP BY usecase_id ORDER BY cnt DESC FORMAT PrettyCompact"

# ---- Summary ----

echo ""
echo "============================================================"
echo "Verification complete."
echo ""
echo "What to look for:"
echo "  1. No pending mutations"
echo "  2. label_rows ~ conv_rows (labels should cover most conversations)"
echo "  3. No duplicate conversation_ids (with or without FINAL)"
echo "  4. Active Days > 0 for agents with conversations"
echo "  5. Usecase distribution looks reasonable"
echo "============================================================"
