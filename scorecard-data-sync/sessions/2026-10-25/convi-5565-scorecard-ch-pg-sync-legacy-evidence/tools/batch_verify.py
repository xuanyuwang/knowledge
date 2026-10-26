#!/usr/bin/env python3
"""
Batch verification of scorecard data sync between PostgreSQL and ClickHouse.

Queries both databases and compares scorecard + score data in bulk.

Usage:
    # Uses cresta-cli for PG connection (default: chat-staging cox-sales)
    ./batch_verify.py --since 2026-02-01

    # With explicit PG connection string
    ./batch_verify.py --since 2026-02-01 --pg-conn "postgres://..."

    # Verify specific scorecards
    ./batch_verify.py --ids "id1,id2,id3"

Environment variables:
    CH_HOST     ClickHouse host (default: clickhouse-conversations.chat-staging.internal.cresta.ai)
    CH_PORT     ClickHouse port (default: 8443)
    CH_USER     ClickHouse user
    CH_PASS     ClickHouse password
    AWS_REGION  AWS region for cresta-cli (default: us-west-2)
"""

import argparse
import os
import subprocess
import sys
from datetime import datetime

import clickhouse_connect
import psycopg2


def get_pg_conn_string(environment="chat-staging", cluster="chat-staging", database="cox-sales"):
    """Get PG connection string from cresta-cli."""
    env = os.environ.copy()
    env.setdefault("AWS_REGION", "us-west-2")
    result = subprocess.run(
        ["cresta-cli", "connstring", "-i", "--read-only", environment, cluster, database],
        capture_output=True, text=True, env=env,
    )
    if result.returncode != 0:
        print(f"Error getting connstring: {result.stderr}", file=sys.stderr)
        sys.exit(1)
    return result.stdout.strip()


def connect_pg(conn_str):
    """Connect to PostgreSQL."""
    conn = psycopg2.connect(conn_str)
    conn.set_session(readonly=True)
    return conn


def connect_ch(database="cox_sales"):
    """Connect to ClickHouse."""
    host = os.environ.get("CH_HOST", "clickhouse-conversations.chat-staging.internal.cresta.ai")
    port = int(os.environ.get("CH_PORT", "8443"))
    user = os.environ.get("CH_USER", "")
    password = os.environ.get("CH_PASS", "")
    if not user or not password:
        print("Error: CH_USER and CH_PASS environment variables are required", file=sys.stderr)
        sys.exit(1)
    return clickhouse_connect.get_client(
        host=host, port=port, username=user, password=password,
        database=database, secure=True,
    )


def query_pg_scorecards(pg_conn, customer, profile, since=None, ids=None):
    """Query scorecards from PostgreSQL."""
    cur = pg_conn.cursor()
    if ids:
        placeholders = ",".join(["%s"] * len(ids))
        cur.execute(f"""
            SELECT resource_id, created_at, updated_at, submitted_at,
                   score, template_id, template_revision, agent_user_id,
                   creator_user_id, submitter_user_id
            FROM director.scorecards
            WHERE customer = %s AND profile = %s AND resource_id IN ({placeholders})
            ORDER BY created_at
        """, [customer, profile] + ids)
    else:
        cur.execute("""
            SELECT resource_id, created_at, updated_at, submitted_at,
                   score, template_id, template_revision, agent_user_id,
                   creator_user_id, submitter_user_id
            FROM director.scorecards
            WHERE customer = %s AND profile = %s AND created_at >= %s
            ORDER BY created_at
        """, (customer, profile, since))

    columns = [desc[0] for desc in cur.description]
    rows = cur.fetchall()
    cur.close()
    return [dict(zip(columns, row)) for row in rows]


def query_pg_scores(pg_conn, scorecard_ids):
    """Query scores from PostgreSQL for given scorecard IDs."""
    if not scorecard_ids:
        return {}
    cur = pg_conn.cursor()
    placeholders = ",".join(["%s"] * len(scorecard_ids))
    cur.execute(f"""
        SELECT scorecard_id, resource_id, criterion_identifier,
               numeric_value, ai_value, text_value, not_applicable, ai_scored
        FROM director.scores
        WHERE scorecard_id IN ({placeholders})
        ORDER BY scorecard_id, criterion_identifier
    """, scorecard_ids)
    columns = [desc[0] for desc in cur.description]
    rows = cur.fetchall()
    cur.close()

    scores_by_scorecard = {}
    for row in rows:
        d = dict(zip(columns, row))
        sid = d["scorecard_id"]
        scores_by_scorecard.setdefault(sid, []).append(d)
    return scores_by_scorecard


def query_ch_scorecards(ch_conn, customer, profile, scorecard_ids):
    """Query scorecards from ClickHouse (FINAL)."""
    if not scorecard_ids:
        return {}
    placeholders = ",".join([f"'{sid}'" for sid in scorecard_ids])
    result = ch_conn.query(f"""
        SELECT
            scorecard_id,
            scorecard_create_time,
            scorecard_last_update_time,
            scorecard_submit_time,
            score,
            scorecard_template_id,
            scorecard_template_revision,
            agent_user_id,
            creator_user_id,
            submitter_user_id,
            update_time
        FROM scorecard_d FINAL
        WHERE customer_id = '{customer}' AND profile_id = '{profile}'
          AND scorecard_id IN ({placeholders})
    """)
    scorecards = {}
    for row in result.result_rows:
        scorecards[row[0]] = {
            "scorecard_id": row[0],
            "create_time": row[1],
            "last_update_time": row[2],
            "submit_time": row[3],
            "score": row[4],
            "template_id": row[5],
            "template_revision": row[6],
            "agent_user_id": row[7],
            "creator_user_id": row[8],
            "submitter_user_id": row[9],
            "update_time": row[10],
        }
    return scorecards


def query_ch_scores(ch_conn, customer, profile, scorecard_ids):
    """Query scores from ClickHouse (FINAL)."""
    if not scorecard_ids:
        return {}
    placeholders = ",".join([f"'{sid}'" for sid in scorecard_ids])
    result = ch_conn.query(f"""
        SELECT
            scorecard_id,
            score_id,
            criterion_id,
            numeric_value,
            ai_value,
            text_value,
            not_applicable,
            ai_scored,
            scorecard_submit_time,
            update_time
        FROM score_d FINAL
        WHERE customer_id = '{customer}' AND profile_id = '{profile}'
          AND scorecard_id IN ({placeholders})
        ORDER BY scorecard_id, criterion_id
    """)
    scores_by_scorecard = {}
    for row in result.result_rows:
        d = {
            "scorecard_id": row[0],
            "score_id": row[1],
            "criterion_id": row[2],
            "numeric_value": row[3],
            "ai_value": row[4],
            "text_value": row[5],
            "not_applicable": row[6],
            "ai_scored": row[7],
            "submit_time": row[8],
            "update_time": row[9],
        }
        scores_by_scorecard.setdefault(row[0], []).append(d)
    return scores_by_scorecard


def is_zero_time(t):
    """Check if a datetime is zero/default."""
    if t is None:
        return True
    if isinstance(t, datetime):
        return t.year <= 1970
    return True


def compare_scorecard(pg_sc, ch_sc, pg_scores, ch_scores):
    """Compare one scorecard between PG and CH. Returns list of issues."""
    sid = pg_sc["resource_id"]
    issues = []

    # Check existence in CH
    if ch_sc is None:
        issues.append("NOT IN CH")
        return issues

    # Check submitted state
    pg_submitted = pg_sc["submitted_at"] is not None
    ch_submitted = not is_zero_time(ch_sc["submit_time"])

    if pg_submitted and not ch_submitted:
        issues.append(f"submitted in PG ({pg_sc['submitted_at']}) but not in CH")
    elif not pg_submitted and ch_submitted:
        issues.append(f"not submitted in PG but CH has submit_time={ch_sc['submit_time']}")

    # Check overall score
    pg_score = pg_sc["score"]
    ch_score = ch_sc["score"]
    if pg_score is not None and abs(float(pg_score) - float(ch_score)) > 0.01:
        issues.append(f"score mismatch: PG={pg_score} CH={ch_score}")

    # Check score count
    pg_sc_scores = pg_scores.get(sid, [])
    ch_sc_scores = ch_scores.get(sid, [])
    if len(pg_sc_scores) != len(ch_sc_scores):
        issues.append(f"score count: PG={len(pg_sc_scores)} CH={len(ch_sc_scores)}")

    # Check individual scores
    ch_score_map = {s["criterion_id"]: s for s in ch_sc_scores}
    for pg_s in pg_sc_scores:
        crit = pg_s["criterion_identifier"]
        ch_s = ch_score_map.get(crit)
        if ch_s is None:
            issues.append(f"criterion {crit[:8]}.. missing in CH")
            continue

        pg_num = pg_s["numeric_value"]
        ch_num = ch_s["numeric_value"]
        if pg_num is not None and abs(float(pg_num) - float(ch_num)) > 0.01:
            issues.append(f"criterion {crit[:8]}.. numeric: PG={pg_num} CH={ch_num}")

        if pg_submitted and is_zero_time(ch_s["submit_time"]):
            issues.append(f"criterion {crit[:8]}.. missing submit_time in CH")

    # Check CH has scores not in PG
    pg_crit_set = {s["criterion_identifier"] for s in pg_sc_scores}
    for ch_s in ch_sc_scores:
        if ch_s["criterion_id"] not in pg_crit_set:
            issues.append(f"criterion {ch_s['criterion_id'][:8]}.. in CH but not PG")

    return issues


def main():
    parser = argparse.ArgumentParser(description="Batch verify scorecard PG↔CH sync")
    parser.add_argument("--since", default="2026-02-01", help="Start date (default: 2026-02-01)")
    parser.add_argument("--ids", help="Comma-separated scorecard IDs (overrides --since)")
    parser.add_argument("--customer", default="cox")
    parser.add_argument("--profile", default="sales")
    parser.add_argument("--pg-conn", help="PG connection string (default: from cresta-cli)")
    parser.add_argument("--env", default="chat-staging", help="Environment for cresta-cli")
    parser.add_argument("--cluster", default="chat-staging", help="Cluster for cresta-cli")
    parser.add_argument("--database", default="cox-sales", help="Database for cresta-cli")
    parser.add_argument("--verbose", "-v", action="store_true", help="Show details for each scorecard")
    args = parser.parse_args()

    # Connect to PG
    pg_conn_str = args.pg_conn or get_pg_conn_string(args.env, args.cluster, args.database)
    print("Connecting to PostgreSQL...")
    pg_conn = connect_pg(pg_conn_str)
    print("PostgreSQL connected")

    # Connect to CH
    ch_database = args.customer.replace("-", "_") + "_" + args.profile.replace("-", "_")
    print(f"Connecting to ClickHouse (database: {ch_database})...")
    ch_conn = connect_ch(ch_database)
    print("ClickHouse connected")

    # Query PG scorecards
    ids = args.ids.split(",") if args.ids else None
    print(f"\nQuerying PostgreSQL scorecards (since={args.since})...")
    pg_scorecards = query_pg_scorecards(pg_conn, args.customer, args.profile, since=args.since, ids=ids)
    print(f"Found {len(pg_scorecards)} scorecards in PostgreSQL")

    if not pg_scorecards:
        print("No scorecards to verify.")
        return

    scorecard_ids = [sc["resource_id"] for sc in pg_scorecards]

    # Query PG scores
    print("Querying PostgreSQL scores...")
    pg_scores = query_pg_scores(pg_conn, scorecard_ids)
    total_pg_scores = sum(len(v) for v in pg_scores.values())
    print(f"Found {total_pg_scores} scores across {len(pg_scores)} scorecards")

    # Query CH scorecards (in batches to avoid query size limits)
    print("\nQuerying ClickHouse scorecards...")
    batch_size = 200
    ch_scorecards = {}
    ch_scores = {}
    for i in range(0, len(scorecard_ids), batch_size):
        batch = scorecard_ids[i:i + batch_size]
        ch_scorecards.update(query_ch_scorecards(ch_conn, args.customer, args.profile, batch))
        ch_scores.update(query_ch_scores(ch_conn, args.customer, args.profile, batch))
    print(f"Found {len(ch_scorecards)} scorecards in ClickHouse")
    total_ch_scores = sum(len(v) for v in ch_scores.values())
    print(f"Found {total_ch_scores} scores in ClickHouse")

    # Compare
    print("\n" + "=" * 70)
    print("VERIFICATION RESULTS")
    print("=" * 70)

    passed = 0
    failed = 0
    missing_in_ch = 0
    submitted_issues = 0
    failure_details = []

    for pg_sc in pg_scorecards:
        sid = pg_sc["resource_id"]
        ch_sc = ch_scorecards.get(sid)
        issues = compare_scorecard(pg_sc, ch_sc, pg_scores, ch_scores)

        if issues:
            failed += 1
            is_submitted = pg_sc["submitted_at"] is not None
            if "NOT IN CH" in issues:
                missing_in_ch += 1
            if is_submitted:
                submitted_issues += 1
            failure_details.append((sid, pg_sc, issues))
            if args.verbose:
                status = "SUBMITTED" if is_submitted else "draft"
                print(f"  [FAIL] {sid} ({status}): {'; '.join(issues)}")
        else:
            passed += 1
            if args.verbose:
                print(f"  [PASS] {sid}")

    # Summary
    submitted_count = sum(1 for sc in pg_scorecards if sc["submitted_at"] is not None)
    draft_count = len(pg_scorecards) - submitted_count

    print(f"\n{'=' * 70}")
    print("SUMMARY")
    print(f"{'=' * 70}")
    print(f"Total scorecards:     {len(pg_scorecards)} ({submitted_count} submitted, {draft_count} draft)")
    print(f"Passed:               {passed}")
    print(f"Failed:               {failed}")
    print(f"  - Missing in CH:    {missing_in_ch}")
    print(f"  - Submitted w/issue:{submitted_issues}")
    print(f"Success rate:         {passed * 100 / len(pg_scorecards):.1f}%")

    if failure_details:
        print(f"\n{'=' * 70}")
        print("FAILURES")
        print(f"{'=' * 70}")
        for sid, pg_sc, issues in failure_details:
            status = "SUBMITTED" if pg_sc["submitted_at"] is not None else "draft"
            print(f"  {sid} ({status}):")
            for issue in issues:
                print(f"    - {issue}")

    print(f"\n{'=' * 70}")
    if failed == 0:
        print("ALL VALIDATIONS PASSED")
    else:
        print(f"SOME VALIDATIONS FAILED ({failed}/{len(pg_scorecards)})")
    print(f"{'=' * 70}")

    pg_conn.close()
    ch_conn.close()

    sys.exit(1 if failed > 0 else 0)


if __name__ == "__main__":
    main()
