#!/bin/bash
# List all ClickHouse databases on a cluster that have scorecard data in the date range.
#
# Usage:
#   ./list_ch_databases.sh <ch_host> <ch_password>
#
# Output:
#   database_name | scorecard_count | score_count
#
# Example:
#   ./list_ch_databases.sh clickhouse-conversations.voice-prod.internal.cresta.ai 'password'
#
# Created: 2026-02-20

set -e

if [[ $# -lt 2 ]]; then
    echo "Usage: ./list_ch_databases.sh <ch_host> <ch_password>"
    echo ""
    echo "  ch_host     - ClickHouse host (e.g., clickhouse-conversations.voice-prod.internal.cresta.ai)"
    echo "  ch_password - ClickHouse admin password"
    echo ""
    echo "Lists all databases with scorecard/score data in 2026-01-01 to 2026-02-21."
    exit 1
fi

CH_HOST="$1"
CH_PASSWORD="$2"
CH_CLIENT="/opt/homebrew/bin/clickhouse client"
START_DATE="2026-01-01"
END_DATE="2026-02-21"

echo "============================================================"
echo "ClickHouse Database Discovery"
echo "============================================================"
echo "Host:       ${CH_HOST}"
echo "Date range: ${START_DATE} to ${END_DATE}"
echo "============================================================"
echo ""

# Get all databases that have a scorecard table
DATABASES=$($CH_CLIENT -h "$CH_HOST" --port 9440 -u admin --password "$CH_PASSWORD" --secure \
    --query "
        SELECT DISTINCT database
        FROM system.tables
        WHERE name = 'scorecard' AND database NOT IN ('system', 'default', 'INFORMATION_SCHEMA', 'information_schema')
        ORDER BY database
    " 2>/dev/null)

if [[ -z "$DATABASES" ]]; then
    echo "No databases found with scorecard tables."
    exit 0
fi

printf "%-50s | %15s | %15s\n" "database" "scorecard_count" "score_count"
printf "%-50s-+-%15s-+-%15s\n" "$(printf '%0.s-' {1..50})" "$(printf '%0.s-' {1..15})" "$(printf '%0.s-' {1..15})"

TOTAL_SC=0
TOTAL_SCORE=0
DB_COUNT=0

while IFS= read -r db; do
    [[ -z "$db" ]] && continue

    SC_COUNT=$($CH_CLIENT -h "$CH_HOST" --port 9440 -u admin --password "$CH_PASSWORD" --secure \
        --query "
            SELECT count()
            FROM ${db}.scorecard
            WHERE scorecard_time >= '${START_DATE}' AND scorecard_time < '${END_DATE}'
        " 2>/dev/null || echo "0")

    SCORE_COUNT=$($CH_CLIENT -h "$CH_HOST" --port 9440 -u admin --password "$CH_PASSWORD" --secure \
        --query "
            SELECT count()
            FROM ${db}.score
            WHERE scorecard_time >= '${START_DATE}' AND scorecard_time < '${END_DATE}'
        " 2>/dev/null || echo "0")

    # Only print databases with data
    if [[ "$SC_COUNT" -gt 0 ]] || [[ "$SCORE_COUNT" -gt 0 ]]; then
        printf "%-50s | %15s | %15s\n" "$db" "$SC_COUNT" "$SCORE_COUNT"
        TOTAL_SC=$((TOTAL_SC + SC_COUNT))
        TOTAL_SCORE=$((TOTAL_SCORE + SCORE_COUNT))
        DB_COUNT=$((DB_COUNT + 1))
    fi
done <<< "$DATABASES"

echo ""
printf "%-50s | %15s | %15s\n" "TOTAL (${DB_COUNT} databases)" "$TOTAL_SC" "$TOTAL_SCORE"
echo "============================================================"
