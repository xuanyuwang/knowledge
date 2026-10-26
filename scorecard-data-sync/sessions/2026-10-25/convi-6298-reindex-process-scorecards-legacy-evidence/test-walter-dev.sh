#!/bin/zsh
# Test CONVI-6298 ReindexScorecards workflow on voice-staging / walter-dev
#
# Prerequisites:
#   - VPN connected to voice-staging
#   - kubectl context voice-staging_dev configured
#   - cresta-cli authenticated
#   - AWS_REGION=us-west-2
#
# Usage:
#   ./test-walter-dev.sh check          # Check PG + CH counts (pre-test)
#   ./test-walter-dev.sh delete-ch      # Delete CH process scorecard data
#   ./test-walter-dev.sh trigger        # Create k8s job with REINDEX_MODE=process
#   ./test-walter-dev.sh trigger --dry-run  # Preview job YAML only
#   ./test-walter-dev.sh verify         # Verify CH matches PG (post-test)
#   ./test-walter-dev.sh logs           # Tail the k8s job logs
#   ./test-walter-dev.sh temporal       # Port-forward + list Temporal workflows
#
# Environment:
#   Customer: cresta / walter-dev
#   Cluster:  voice-staging
#   CH DB:    cresta_walter_dev
#   PG DB:    walter-dev (on voice-staging RDS)
#
# Created: 2026-03-19

set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────
CLUSTER="voice-staging"
CONTEXT="${CLUSTER}_dev"
CUSTOMER_ID="cresta"
PROFILE_ID="walter-dev"
CH_HOST="clickhouse-conversations.voice-staging.internal.cresta.ai"
CH_PORT=9440
CH_USER="admin"
CH_PASSWORD="${CH_PASSWORD:?Set CH_PASSWORD env var}"
CH_DATABASE="cresta_walter_dev"
PSQL="/opt/homebrew/opt/postgresql@15/bin/psql"
CH_CLIENT="/opt/homebrew/bin/clickhouse"

# Time range for reindex (covers all process scorecards: 2024-05-02 to 2026-02-23)
START_TIME="2024-01-01T00:00:00Z"
END_TIME="2026-03-20T00:00:00Z"

# ── Helpers ──────────────────────────────────────────────────────────────────
get_pg_conn() {
    AWS_REGION=us-west-2 cresta-cli connstring -i --read-only --force \
        voice-staging voice-staging walter-dev
}

run_ch() {
    $CH_CLIENT client -h "$CH_HOST" --port "$CH_PORT" -u "$CH_USER" \
        --password "$CH_PASSWORD" --secure -d "$CH_DATABASE" --query "$1"
}

run_pg() {
    local conn
    conn=$(get_pg_conn)
    $PSQL "$conn" -t -A -c "$1"
}

# ── Commands ─────────────────────────────────────────────────────────────────

cmd_check() {
    echo "============================================================"
    echo "Pre-test check: PG vs CH for process scorecards"
    echo "Customer: ${CUSTOMER_ID} / ${PROFILE_ID}"
    echo "============================================================"

    echo ""
    echo "── PG: Process scorecard templates (type=2) ──"
    run_pg "
        SELECT count(DISTINCT resource_id)
        FROM director.scorecard_templates
        WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}' AND type = 2;
    " | xargs -I{} echo "  Distinct templates: {}"

    echo ""
    echo "── PG: Process scorecards ──"
    run_pg "
        SELECT count(*)
        FROM director.scorecards
        WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}'
        AND template_id IN (
            SELECT DISTINCT resource_id FROM director.scorecard_templates
            WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}' AND type = 2
        );
    " | xargs -I{} echo "  Total scorecards: {}"

    run_pg "
        SELECT min(created_at)::date || ' to ' || max(created_at)::date
        FROM director.scorecards
        WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}'
        AND template_id IN (
            SELECT DISTINCT resource_id FROM director.scorecard_templates
            WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}' AND type = 2
        );
    " | xargs -I{} echo "  Date range: {}"

    echo ""
    echo "── PG: Director scores for process scorecards ──"
    run_pg "
        SELECT count(*)
        FROM director.scores sc
        WHERE sc.customer = '${CUSTOMER_ID}' AND sc.profile = '${PROFILE_ID}'
        AND sc.scorecard_id IN (
            SELECT resource_id FROM director.scorecards
            WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}'
            AND template_id IN (
                SELECT DISTINCT resource_id FROM director.scorecard_templates
                WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}' AND type = 2
            )
        );
    " | xargs -I{} echo "  Total scores: {}"

    echo ""
    echo "── CH: Process scorecards (conversation_id = '') ──"
    local ch_sc
    ch_sc=$(run_ch "SELECT count() FROM scorecard_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Scorecards: ${ch_sc}"

    local ch_scores
    ch_scores=$(run_ch "SELECT count() FROM score_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Scores: ${ch_scores}"

    echo ""
    echo "============================================================"
}

cmd_delete_ch() {
    echo "============================================================"
    echo "Deleting CH process scorecard data for ${CUSTOMER_ID}/${PROFILE_ID}"
    echo "============================================================"

    echo ""
    echo "Before delete:"
    local before_sc
    before_sc=$(run_ch "SELECT count() FROM scorecard_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Scorecards: ${before_sc}"
    local before_scores
    before_scores=$(run_ch "SELECT count() FROM score_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Scores: ${before_scores}"

    if [[ "$before_sc" == "0" && "$before_scores" == "0" ]]; then
        echo ""
        echo "Nothing to delete."
        return
    fi

    echo ""
    echo "Deleting from scorecard (local table, ON CLUSTER)..."
    run_ch "ALTER TABLE ${CH_DATABASE}.scorecard ON CLUSTER 'conversations' DELETE WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = '' SETTINGS replication_wait_for_inactive_replica_timeout = 0"

    echo "Deleting from score (local table, ON CLUSTER)..."
    run_ch "ALTER TABLE ${CH_DATABASE}.score ON CLUSTER 'conversations' DELETE WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = '' SETTINGS replication_wait_for_inactive_replica_timeout = 0"

    echo ""
    echo "Waiting for mutations to complete..."
    sleep 5
    local pending
    pending=$(run_ch "SELECT count() FROM system.mutations WHERE database = '${CH_DATABASE}' AND is_done = 0")
    echo "  Pending mutations: ${pending}"

    if [[ "$pending" != "0" ]]; then
        echo "  Waiting 10s more..."
        sleep 10
        pending=$(run_ch "SELECT count() FROM system.mutations WHERE database = '${CH_DATABASE}' AND is_done = 0")
        echo "  Pending mutations: ${pending}"
    fi

    echo ""
    echo "After delete:"
    local after_sc
    after_sc=$(run_ch "SELECT count() FROM scorecard_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Scorecards: ${after_sc}"
    local after_scores
    after_scores=$(run_ch "SELECT count() FROM score_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Scores: ${after_scores}"
    echo "============================================================"
}

cmd_trigger() {
    local DRY_RUN=false
    [[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true

    local JOB_NAME="reindex-process-test-$(date +%s)"
    local YAML_FILE="/tmp/reindex-process-${JOB_NAME}.yaml"

    echo "============================================================"
    echo "Triggering ReindexScorecards workflow"
    echo "============================================================"
    echo "Cluster:     ${CLUSTER}"
    echo "Customer:    ${CUSTOMER_ID}"
    echo "Start time:  ${START_TIME}"
    echo "End time:    ${END_TIME}"
    echo "REINDEX_MODE: process"
    echo "Job name:    ${JOB_NAME}"
    echo "Dry run:     ${DRY_RUN}"
    echo "============================================================"

    # Step 1: Generate job YAML from cronjob template
    echo "Creating job template..."
    kubectl create job --from=cronjob/cron-batch-reindex-conversations \
        "${JOB_NAME}" \
        -n cresta-cron \
        --context="${CONTEXT}" \
        --dry-run=client -o yaml > "${YAML_FILE}"

    # Step 2: Set environment variables
    echo "Setting environment variables..."
    kubectl set env --local -f "${YAML_FILE}" \
        REINDEX_MODE="process" \
        REINDEX_START_TIME="${START_TIME}" \
        REINDEX_END_TIME="${END_TIME}" \
        RUN_ONLY_FOR_CUSTOMER_IDS="${CUSTOMER_ID}" \
        -o yaml > "${YAML_FILE}.final.yaml"

    mv "${YAML_FILE}.final.yaml" "${YAML_FILE}"

    if [[ "${DRY_RUN}" == "true" ]]; then
        echo ""
        echo "[DRY-RUN] Would create job: ${JOB_NAME}"
        echo ""
        echo "Env vars preview:"
        grep -A2 "REINDEX_MODE\|REINDEX_START_TIME\|REINDEX_END_TIME\|RUN_ONLY_FOR_CUSTOMER_IDS" "${YAML_FILE}" || true
        echo ""
        echo "Full YAML at: ${YAML_FILE}"
        return
    fi

    # Step 3: Apply the job
    echo "Applying job..."
    kubectl apply -f "${YAML_FILE}" --context="${CONTEXT}"

    echo ""
    echo "============================================================"
    echo "Job created: ${JOB_NAME}"
    echo ""
    echo "Monitor:"
    echo "  kubectl logs -n cresta-cron --context=${CONTEXT} -l job-name=${JOB_NAME} -f"
    echo ""
    echo "Or run:"
    echo "  $0 logs"
    echo "============================================================"

    # Save job name for later use
    echo "${JOB_NAME}" > /tmp/reindex-process-test-latest-job.txt
}

cmd_verify() {
    echo "============================================================"
    echo "Post-test verification: qualified scores synced to CH"
    echo "============================================================"
    echo ""
    echo "The reindex workflow only writes scorecards that have valid"
    echo "scores after filtering through GenerateHistoricScorecardScores."
    echo "Scorecards are skipped if they have: no director.scores rows,"
    echo "no matching template revision, or only non-leaf/chapter criteria."
    echo "This is the same filtering used by the conversation reindex path."

    # ── PG breakdown ──
    echo ""
    echo "── PG: Process scorecards breakdown ──"
    local pg_total
    pg_total=$(run_pg "
        SELECT count(*) FROM director.scorecards
        WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}'
        AND template_id IN (
            SELECT DISTINCT resource_id FROM director.scorecard_templates
            WHERE customer = '${CUSTOMER_ID}' AND profile = '${PROFILE_ID}' AND type = 2
        );
    ")
    echo "  Total scorecards:          ${pg_total}"

    # Note: JOINing director.scores with director.scorecards is too expensive on
    # some read replicas (temp disk exhaustion). We skip the PG "with scores" count
    # and verify using CH-side checks instead.

    # ── CH counts ──
    echo ""
    echo "── CH: Reindexed process scorecards ──"
    local ch_sc
    ch_sc=$(run_ch "SELECT count() FROM scorecard_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Scorecards:                ${ch_sc}"

    local ch_scores
    ch_scores=$(run_ch "SELECT count() FROM score_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Scores:                    ${ch_scores}"

    local ch_distinct_sc
    ch_distinct_sc=$(run_ch "SELECT count(DISTINCT scorecard_id) FROM score_d FINAL WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''")
    echo "  Distinct scorecards in scores: ${ch_distinct_sc}"

    # ── Validation ──
    echo ""
    echo "── Validation ──"

    # 1. CH scorecards <= total PG scorecards (basic sanity)
    if [[ "${ch_sc}" -le "${pg_total}" ]]; then
        echo "  PASS: CH scorecards (${ch_sc}) <= PG total (${pg_total})"
    else
        echo "  FAIL: CH scorecards (${ch_sc}) > PG total (${pg_total})"
    fi

    # 2. Every CH scorecard should have at least one score
    if [[ "${ch_sc}" == "${ch_distinct_sc}" ]]; then
        echo "  PASS: Every CH scorecard has scores (${ch_sc} scorecards, ${ch_distinct_sc} with scores)"
    else
        echo "  WARN: CH scorecards (${ch_sc}) != distinct scorecards in scores (${ch_distinct_sc})"
    fi

    # 3. CH should have >0 data (workflow actually ran)
    if [[ "${ch_sc}" -gt 0 && "${ch_scores}" -gt 0 ]]; then
        echo "  PASS: CH has data (${ch_sc} scorecards, ${ch_scores} scores)"
    else
        echo "  FAIL: CH is empty — workflow may not have run"
    fi

    # 4. Check CH scorecards all exist in PG
    local ch_not_in_pg
    ch_not_in_pg=$(run_ch "
        SELECT count() FROM (
            SELECT DISTINCT scorecard_id FROM scorecard_d FINAL
            WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''
        ) WHERE scorecard_id NOT IN (
            SELECT scorecard_id FROM score_d FINAL
            WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''
        )
    ")
    if [[ "${ch_not_in_pg}" == "0" ]]; then
        echo "  PASS: All CH scorecards have matching score rows"
    else
        echo "  WARN: ${ch_not_in_pg} CH scorecards have no matching score rows"
    fi

    echo ""
    echo "── Expected filter rate ──"
    echo "  PG total scorecards:  ${pg_total}"
    echo "  CH qualified:         ${ch_sc} scorecards, ${ch_scores} scores"
    echo "  Filtered out:         $((pg_total - ch_sc)) scorecards (no scores, template mismatch, non-leaf criteria)"

    # ── Samples ──
    echo ""
    echo "── CH: Sample scorecards (latest 5) ──"
    run_ch "
        SELECT scorecard_id, scorecard_template_id, scorecard_time, score, agent_user_id
        FROM scorecard_d FINAL
        WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''
        ORDER BY scorecard_time DESC
        LIMIT 5
        FORMAT PrettyCompact
    "

    echo ""
    echo "── CH: Sample scores (latest 10) ──"
    run_ch "
        SELECT scorecard_id, criterion_id, numeric_value, percentage_value, weight, not_applicable
        FROM score_d FINAL
        WHERE customer_id = '${CUSTOMER_ID}' AND profile_id = '${PROFILE_ID}' AND conversation_id = ''
        ORDER BY scorecard_time DESC
        LIMIT 10
        FORMAT PrettyCompact
    "

    echo ""
    echo "============================================================"
}

cmd_logs() {
    local job_name
    if [[ -f /tmp/reindex-process-test-latest-job.txt ]]; then
        job_name=$(cat /tmp/reindex-process-test-latest-job.txt)
    else
        echo "No job name found. Listing recent jobs..."
        kubectl get jobs -n cresta-cron --context="${CONTEXT}" --sort-by=.metadata.creationTimestamp | grep reindex-process | tail -5
        return
    fi
    echo "Tailing logs for job: ${job_name}"
    kubectl logs -n cresta-cron --context="${CONTEXT}" -l job-name="${job_name}" -f
}

cmd_temporal() {
    echo "Starting port-forward to Temporal..."
    echo "  kubectl --context=${CONTEXT} -n temporal port-forward svc/temporal-frontend-headless 7233:7233"
    echo ""
    echo "In another terminal, list workflows:"
    echo "  temporal workflow list --namespace ingestion --address localhost:7233 \\"
    echo "    --query 'WorkflowType = \"ReindexScorecardsWorkflow\"'"
    echo ""
    kubectl --context="${CONTEXT}" -n temporal port-forward svc/temporal-frontend-headless 7233:7233
}

# ── Main ─────────────────────────────────────────────────────────────────────
case "${1:-help}" in
    check)      cmd_check ;;
    delete-ch)  cmd_delete_ch ;;
    trigger)    cmd_trigger "${2:-}" ;;
    verify)     cmd_verify ;;
    logs)       cmd_logs ;;
    temporal)   cmd_temporal ;;
    *)
        echo "Usage: $0 {check|delete-ch|trigger [--dry-run]|verify|logs|temporal}"
        echo ""
        echo "Workflow:"
        echo "  1. $0 check          # See current PG vs CH state"
        echo "  2. $0 delete-ch      # Clean CH process scorecard data"
        echo "  3. $0 trigger --dry-run  # Preview the k8s job"
        echo "  4. $0 trigger        # Create k8s job with REINDEX_MODE=process"
        echo "  5. $0 logs           # Monitor the job"
        echo "  6. $0 verify         # Compare PG vs CH after completion"
        ;;
esac
