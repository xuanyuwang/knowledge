#!/usr/bin/env python3
"""
One-time script to backfill process scorecards from Postgres to ClickHouse.

Reads process scorecards (template type=2) from PG director tables,
computes scoring fields on-the-fly from director.scores + template revision,
then writes to CH scorecard_d and score_d distributed tables.

Data sources:
  - director.scorecards filtered by template_id (from scorecard_templates type=2)
  - director.scores + director.scorecard_template_revisions (compute percentage_value, weight, etc.)
  - app.users (is_dev_user flag)

Targets:
  - CH scorecard_d (distributed → scorecard local table)
  - CH score_d (distributed → score local table)

Usage:
  # 1. Set PG and CH connection info
  export PG_CONN=$(AWS_REGION=us-west-2 cresta-cli connstring -i --read-only us-west-2-prod us-west-2-prod oportun)
  export CH_HOST=clickhouse-conversations.us-west-2-prod.internal.cresta.ai
  export CH_PASSWORD='...'
  export CH_DATABASE=oportun_us_west_2

  # 2. Dry run
  python3 backfill_process_scorecards.py --customer oportun --profile us-west-2 --dry-run

  # 3. Test with a small number
  python3 backfill_process_scorecards.py --customer oportun --profile us-west-2 --limit 5

  # 4. Execute full backfill
  python3 backfill_process_scorecards.py --customer oportun --profile us-west-2

  # 5. Verify
  python3 backfill_process_scorecards.py --customer oportun --profile us-west-2 --verify-only

Requirements:
  pip install psycopg2-binary clickhouse-driver
"""

import argparse
import json
import os
import sys
from datetime import datetime, timezone

import psycopg2
import psycopg2.extras
from clickhouse_driver import Client as CHClient

from validate_scoring import parse_template, compute_criterion_percentage, is_manually_scored

# ── Configuration ──────────────────────────────────────────────────────────────

CH_HOST = os.environ.get("CH_HOST", "")
CH_PORT = int(os.environ.get("CH_PORT", "9440"))
CH_USER = os.environ.get("CH_USER", "admin")
CH_PASSWORD = os.environ.get("CH_PASSWORD", "")
CH_DATABASE = os.environ.get("CH_DATABASE", "")

START_DATE = os.environ.get("START_DATE", "2025-01-01")
END_DATE = os.environ.get("END_DATE", "2026-03-06")

BATCH_SIZE = 1000

# ── Constants ──────────────────────────────────────────────────────────────────

DEFAULT_TIME = datetime(1970, 1, 1, tzinfo=timezone.utc)


# ── Helpers ───────────────────────────────────────────────────────────────────

def ts(val):
    if val is None:
        return DEFAULT_TIME
    if val.tzinfo is None:
        return val.replace(tzinfo=timezone.utc)
    return val


def nullable_float(val, default=-1.0):
    return val if val is not None else default


def nullable_str(val, default=""):
    return val if val is not None else default


def nullable_bool(val, default=False):
    return val if val is not None else default


# ── Core logic ────────────────────────────────────────────────────────────────

def fetch_process_template_ids(pg_cur, customer, profile):
    """Fetch process scorecard template IDs (type=2) for a customer/profile."""
    pg_cur.execute(
        "SELECT DISTINCT resource_id FROM director.scorecard_templates WHERE customer = %s AND profile = %s AND type = 2",
        (customer, profile))
    return [r[0] for r in pg_cur.fetchall()]


def count_scorecards(pg_cur, customer, profile, template_ids, start_date, end_date):
    """Count process scorecards. Uses (customer, profile, conversation_id, template_id) index."""
    pg_cur.execute("""
        SELECT COUNT(*) FROM director.scorecards
        WHERE customer = %s AND profile = %s AND template_id = ANY(%s)
        AND created_at >= %s AND created_at < %s
    """, (customer, profile, template_ids, start_date, end_date))
    return pg_cur.fetchone()[0]


def fetch_scorecards_batch(pg_cur, customer, profile, template_ids, start_date, end_date, limit, offset):
    """Fetch a batch of process scorecards. Uses (customer, profile, conversation_id, template_id) index."""
    pg_cur.execute("""
        SELECT
          customer, profile, resource_id, conversation_id,
          agent_user_id, creator_user_id, template_id, template_revision,
          coaching_plan_id, created_at, updated_at, last_updater_user_id,
          submitted_at, submitter_user_id, score, ai_scored_at,
          manually_scored, auto_failed, acknowledged_at, acknowledge_comment,
          process_interaction_at, usecase_id
        FROM director.scorecards
        WHERE customer = %s AND profile = %s AND template_id = ANY(%s)
        AND created_at >= %s AND created_at < %s
        ORDER BY created_at
        LIMIT %s OFFSET %s
    """, (customer, profile, template_ids, start_date, end_date, limit, offset))
    rows = pg_cur.fetchall()
    scorecards = []
    for r in rows:
        scorecards.append({
            "customer": r[0],
            "profile": r[1],
            "resource_id": r[2],
            "conversation_id": r[3] or "",
            "agent_user_id": r[4],
            "creator_user_id": nullable_str(r[5]),
            "template_id": r[6],
            "template_revision": r[7],
            "coaching_plan_id": nullable_str(r[8]),
            "created_at": ts(r[9]),
            "updated_at": ts(r[10]),
            "last_updater_user_id": nullable_str(r[11]),
            "submitted_at": ts(r[12]),
            "submitter_user_id": nullable_str(r[13]),
            "score": r[14],
            "ai_scored_at": ts(r[15]),
            "manually_scored": nullable_bool(r[16]),
            "auto_failed": nullable_bool(r[17]),
            "acknowledged_at": ts(r[18]),
            "acknowledge_comment": nullable_str(r[19]),
            "process_interaction_at": ts(r[20]),
            "usecase_id": nullable_str(r[21]),
        })
    return scorecards


def fetch_director_scores(pg_cur, customer, profile, scorecard_ids):
    """Fetch raw scores from director.scores for given scorecards."""
    pg_cur.execute("""
        SELECT resource_id, scorecard_id, criterion_identifier, numeric_value,
               ai_value, text_value, not_applicable, ai_scored, auto_failed
        FROM director.scores
        WHERE customer = %s AND profile = %s AND scorecard_id = ANY(%s)
    """, (customer, profile, scorecard_ids))
    scores_by_scorecard = {}
    for r in pg_cur.fetchall():
        score = {
            "score_id": r[0],
            "scorecard_id": r[1],
            "criterion_identifier": r[2],
            "numeric_value": r[3],
            "ai_value": r[4],
            "text_value": nullable_str(r[5]),
            "not_applicable": r[6] or False,
            "ai_scored": r[7] or False,
            "auto_failed": r[8] or False,
        }
        scores_by_scorecard.setdefault(score["scorecard_id"], []).append(score)
    return scores_by_scorecard


def fetch_template_revisions(pg_cur, customer, profile, template_rev_pairs):
    """Fetch and parse template revisions. Returns dict of (template_id, revision) -> parsed criteria."""
    if not template_rev_pairs:
        return {}
    cache = {}
    for tmpl_id, rev in template_rev_pairs:
        if (tmpl_id, rev) in cache:
            continue
        pg_cur.execute("""
            SELECT template FROM director.scorecard_template_revisions
            WHERE customer = %s AND profile = %s AND template_id = %s AND resource_id = %s
        """, (customer, profile, tmpl_id, rev))
        row = pg_cur.fetchone()
        if row:
            cache[(tmpl_id, rev)] = parse_template(row[0])
        else:
            cache[(tmpl_id, rev)] = {}
    return cache


def compute_scores_for_scorecard(sc, director_scores, criteria):
    """Compute percentage_value, weight, float_weight, max_value, manually_scored for each score.

    Returns list of dicts with computed fields merged with raw score data.
    """
    if not director_scores or not criteria:
        return []

    # Group scores by criterion
    grouped = {}
    for s in director_scores:
        grouped.setdefault(s["criterion_identifier"], []).append(s)

    computed_scores = []
    for crit_id, crit_scores in grouped.items():
        ci = criteria.get(crit_id)
        if ci is None:
            continue  # chapter or unknown criterion

        pct_results = compute_criterion_percentage(ci, crit_scores)
        for i, s in enumerate(crit_scores):
            pct, w = pct_results[i] if i < len(pct_results) else (None, 0)
            computed_scores.append({
                **s,
                "percentage_value": pct,
                "weight": int(w),
                "float_weight": w,
                "max_value": float(ci["max_value"]),
                "manually_scored": is_manually_scored(s),
                # Fields from scorecard context
                "customer_id": sc["customer"],
                "profile_id": sc["profile"],
                "usecase_id": sc.get("usecase_id", ""),
                "scorecard_template_id": sc["template_id"],
                "scorecard_template_revision": sc["template_revision"],
                "conversation_id": sc.get("conversation_id", ""),
                "agent_id": sc["agent_user_id"],
                "conversation_duration_bins_id": 0,
                "is_voice_mail": False,
            })
    return computed_scores


def fetch_dev_users(pg_cur, customer_id, agent_user_ids):
    if not agent_user_ids:
        return {}
    pg_cur.execute(
        "SELECT user_id, is_dev_user FROM app.users WHERE customer_id = %s AND user_id = ANY(%s)",
        (customer_id, list(agent_user_ids)))
    return {r[0]: r[1] for r in pg_cur.fetchall()}


def build_ch_scorecard_row(sc, is_dev_user):
    return (
        nullable_str(sc["acknowledge_comment"]),
        sc["agent_user_id"],
        sc["ai_scored_at"],
        sc["auto_failed"],
        nullable_str(sc["coaching_plan_id"]),
        0,                                              # conversation_duration_bins_id (process=0)
        0,                                              # conversation_duration_secs (process=0)
        sc["conversation_id"],                          # empty for process
        nullable_str(sc["creator_user_id"]),
        sc["customer"],
        is_dev_user,
        False,                                          # is_voice_mail (process=false)
        nullable_str(sc["last_updater_user_id"]),
        sc["manually_scored"],
        sc["profile"],
        sc["score"] if sc["score"] is not None else -1.0,
        sc["acknowledged_at"],
        sc["created_at"],
        sc["resource_id"],
        sc["updated_at"],
        sc["submitted_at"],
        sc["template_id"],
        sc["template_revision"],
        sc["process_interaction_at"],                   # scorecard_time = process_interaction_at
        nullable_str(sc["submitter_user_id"]),
        datetime.now(timezone.utc),
        nullable_str(sc["usecase_id"]),
    )


def build_ch_score_row(sc, css, is_dev_user):
    """Build CH score_d row from scorecard + computed score."""
    percentage_value = -1.0
    if (css["percentage_value"] is not None and
            (css["ai_value"] is not None or css["numeric_value"] is not None)):
        percentage_value = css["percentage_value"]

    return (
        nullable_str(sc["acknowledge_comment"]),
        css["agent_id"],
        sc["ai_scored_at"],
        css["ai_scored"],
        nullable_float(css["ai_value"]),
        css["auto_failed"],
        nullable_str(sc["coaching_plan_id"]),
        css["conversation_duration_bins_id"],
        0,                                              # conversation_duration_secs (process=0)
        DEFAULT_TIME,                                   # conversation_start_time (process=default)
        css["conversation_id"],
        nullable_str(sc["creator_user_id"]),
        css["criterion_identifier"],
        css["customer_id"],
        css["float_weight"],
        is_dev_user,
        css["is_voice_mail"],
        "",                                             # language_code (process="")
        nullable_str(sc["last_updater_user_id"]),
        css["manually_scored"],
        css["max_value"],
        css["not_applicable"],
        nullable_float(css["numeric_value"]),
        percentage_value,
        css["profile_id"],
        css["score_id"],
        sc["acknowledged_at"],
        sc["created_at"],
        sc["resource_id"],
        sc["updated_at"],
        sc["score"] if sc["score"] is not None else -1.0,
        sc["submitted_at"],
        css["scorecard_template_id"],
        css["scorecard_template_revision"],
        sc["created_at"],                               # scorecard_time = created_at for score table
        nullable_str(sc["submitter_user_id"]),
        css["text_value"],
        datetime.now(timezone.utc),
        nullable_str(sc["usecase_id"]),
        css["weight"],
    )


CH_INSERT_SCORECARD = """INSERT INTO scorecard_d (
    acknowledge_comment, agent_user_id, ai_score_time, auto_failed,
    coaching_plan_id, conversation_duration_bins_id, conversation_duration_secs,
    conversation_id, creator_user_id, customer_id, is_dev_user, is_voice_mail,
    last_updator_user_id, manually_scored, profile_id, score,
    scorecard_acknowledge_time, scorecard_create_time, scorecard_id,
    scorecard_last_update_time, scorecard_submit_time, scorecard_template_id,
    scorecard_template_revision, scorecard_time, submitter_user_id,
    update_time, usecase_id
) VALUES"""

CH_INSERT_SCORE = """INSERT INTO score_d (
    acknowledge_comment, agent_user_id, ai_score_time, ai_scored, ai_value,
    auto_failed, coaching_plan_id, conversation_duration_bins_id,
    conversation_duration_secs, conversation_start_time, conversation_id,
    creator_user_id, criterion_id, customer_id, float_weight, is_dev_user,
    is_voice_mail, language_code, last_updator_user_id, manually_scored,
    max_value, not_applicable, numeric_value, percentage_value, profile_id,
    score_id, scorecard_acknowledge_time, scorecard_create_time, scorecard_id,
    scorecard_last_update_time, scorecard_score, scorecard_submit_time,
    scorecard_template_id, scorecard_template_revision, scorecard_time,
    submitter_user_id, text_value, update_time, usecase_id, weight
) VALUES"""


CH_SCORECARD_COLUMNS = [
    "acknowledge_comment", "agent_user_id", "ai_score_time", "auto_failed",
    "coaching_plan_id", "conversation_duration_bins_id", "conversation_duration_secs",
    "conversation_id", "creator_user_id", "customer_id", "is_dev_user", "is_voice_mail",
    "last_updator_user_id", "manually_scored", "profile_id", "score",
    "scorecard_acknowledge_time", "scorecard_create_time", "scorecard_id",
    "scorecard_last_update_time", "scorecard_submit_time", "scorecard_template_id",
    "scorecard_template_revision", "scorecard_time", "submitter_user_id",
    "update_time", "usecase_id",
]

CH_SCORE_COLUMNS = [
    "acknowledge_comment", "agent_user_id", "ai_score_time", "ai_scored", "ai_value",
    "auto_failed", "coaching_plan_id", "conversation_duration_bins_id",
    "conversation_duration_secs", "conversation_start_time", "conversation_id",
    "creator_user_id", "criterion_id", "customer_id", "float_weight", "is_dev_user",
    "is_voice_mail", "language_code", "last_updator_user_id", "manually_scored",
    "max_value", "not_applicable", "numeric_value", "percentage_value", "profile_id",
    "score_id", "scorecard_acknowledge_time", "scorecard_create_time", "scorecard_id",
    "scorecard_last_update_time", "scorecard_score", "scorecard_submit_time",
    "scorecard_template_id", "scorecard_template_revision", "scorecard_time",
    "submitter_user_id", "text_value", "update_time", "usecase_id", "weight",
]


def format_row_as_dict(columns, row):
    d = {}
    for col, val in zip(columns, row):
        if isinstance(val, datetime):
            d[col] = val.isoformat()
        else:
            d[col] = val
    return d


def process_batch(pg_cur, customer, profile, scorecards):
    scorecard_ids = [sc["resource_id"] for sc in scorecards]

    # Fetch director.scores
    scores_by_scorecard = fetch_director_scores(pg_cur, customer, profile, scorecard_ids)

    # Fetch and cache template revisions
    template_rev_pairs = set((sc["template_id"], sc["template_revision"]) for sc in scorecards)
    template_cache = fetch_template_revisions(pg_cur, customer, profile, template_rev_pairs)

    # Fetch dev users
    agent_ids_by_customer = {}
    for sc in scorecards:
        if sc["agent_user_id"]:
            agent_ids_by_customer.setdefault(sc["customer"], set()).add(sc["agent_user_id"])
    dev_users = {}
    for customer_id, agent_ids in agent_ids_by_customer.items():
        dev_users.update(fetch_dev_users(pg_cur, customer_id, agent_ids))

    scorecard_rows = []
    score_rows = []
    no_scores_count = 0
    for sc in scorecards:
        is_dev = dev_users.get(sc["agent_user_id"], False)
        scorecard_rows.append(build_ch_scorecard_row(sc, is_dev))

        dir_scores = scores_by_scorecard.get(sc["resource_id"], [])
        criteria = template_cache.get((sc["template_id"], sc["template_revision"]), {})
        computed = compute_scores_for_scorecard(sc, dir_scores, criteria)
        if not computed:
            no_scores_count += 1
        for css in computed:
            score_rows.append(build_ch_score_row(sc, css, is_dev))

    return scorecard_rows, score_rows, no_scores_count


def run_dry_run(pg_conn, customer, profile, template_ids, start_date, end_date, sample_size=3):
    pg_cur = pg_conn.cursor()

    total = count_scorecards(pg_cur, customer, profile, template_ids, start_date, end_date)
    print(f"Total process scorecards in [{start_date}, {end_date}): {total}")
    if total == 0:
        print("Nothing to backfill.")
        return

    # Fetch sample and show built CH rows
    print(f"\n{'═' * 70}")
    print(f"Sample: first {sample_size} scorecards → CH rows")
    print(f"{'═' * 70}")

    scorecards = fetch_scorecards_batch(pg_cur, customer, profile, template_ids, start_date, end_date, sample_size, 0)
    if not scorecards:
        return

    scorecard_rows, score_rows, no_scores = process_batch(pg_cur, customer, profile, scorecards)

    for i, (sc, sc_row) in enumerate(zip(scorecards, scorecard_rows)):
        print(f"\n── Scorecard {i+1}: {sc['resource_id']} ──")
        print(f"  PG source:")
        print(f"    agent_user_id:        {sc['agent_user_id']}")
        print(f"    template_id:          {sc['template_id']}")
        print(f"    created_at:           {sc['created_at']}")
        print(f"    process_interaction_at:{sc['process_interaction_at']}")
        print(f"    score:                {sc['score']}")
        print(f"\n  CH scorecard_d row:")
        print(json.dumps(format_row_as_dict(CH_SCORECARD_COLUMNS, sc_row), indent=4))

        sc_score_rows = [r for r in score_rows if r[28] == sc["resource_id"]]
        print(f"\n  CH score_d rows ({len(sc_score_rows)} scores):")
        for j, sr in enumerate(sc_score_rows[:3]):
            print(f"    Score {j+1}:")
            print(json.dumps(format_row_as_dict(CH_SCORE_COLUMNS, sr), indent=6))
        if len(sc_score_rows) > 3:
            print(f"    ... and {len(sc_score_rows) - 3} more scores")

    if no_scores:
        print(f"\n  WARNING: {no_scores} sample scorecards have no computed score rows.")


def run_backfill(pg_conn, ch_client, customer, profile, template_ids, start_date, end_date, limit=None):
    pg_cur = pg_conn.cursor()

    total = count_scorecards(pg_cur, customer, profile, template_ids, start_date, end_date)
    effective_total = min(total, limit) if limit else total
    print(f"Found {total} process scorecards in [{start_date}, {end_date})")
    if limit:
        print(f"  --limit {limit}: will process only the first {effective_total}")
    if effective_total == 0:
        print("Nothing to backfill.")
        return

    offset = 0
    total_sc_inserted = 0
    total_score_inserted = 0
    total_no_scores = 0
    all_inserted_ids = []

    while offset < effective_total:
        batch_size = min(BATCH_SIZE, effective_total - offset)
        scorecards = fetch_scorecards_batch(pg_cur, customer, profile, template_ids, start_date, end_date, batch_size, offset)
        if not scorecards:
            break

        scorecard_rows, score_rows, no_scores = process_batch(pg_cur, customer, profile, scorecards)
        total_no_scores += no_scores
        all_inserted_ids.extend(sc["resource_id"] for sc in scorecards)

        if scorecard_rows:
            ch_client.execute(CH_INSERT_SCORECARD, scorecard_rows)
            total_sc_inserted += len(scorecard_rows)
        if score_rows:
            ch_client.execute(CH_INSERT_SCORE, score_rows)
            total_score_inserted += len(score_rows)

        offset += len(scorecards)
        print(f"  Processed {offset}/{effective_total} scorecards "
              f"(+{len(scorecard_rows)} scorecards, +{len(score_rows)} scores)")

    print(f"\nBackfill complete:")
    print(f"  Scorecards inserted: {total_sc_inserted}")
    print(f"  Scores inserted:     {total_score_inserted}")

    if total_no_scores:
        print(f"\n  Note: {total_no_scores} scorecards had no computed score rows (no director.scores or no template match).")

    return all_inserted_ids


def _ch_count_in_batches(ch_client, query_template, ids, batch_size=5000):
    """Run a count query against CH in batches to avoid max query size limit."""
    total = 0
    for i in range(0, len(ids), batch_size):
        batch = ids[i:i + batch_size]
        total += ch_client.execute(query_template, {"ids": batch})[0][0]
    return total


def _ch_distinct_in_batches(ch_client, query_template, ids, batch_size=5000):
    """Run a DISTINCT query against CH in batches."""
    result = set()
    for i in range(0, len(ids), batch_size):
        batch = ids[i:i + batch_size]
        result.update(r[0] for r in ch_client.execute(query_template, {"ids": batch}))
    return result


def run_verify(pg_conn, ch_client, customer, profile, template_ids, start_date, end_date, scorecard_ids=None):
    pg_cur = pg_conn.cursor()

    if scorecard_ids:
        # Small set — use IN clause directly
        pg_sc_count = len(scorecard_ids)
        ch_sc_count = ch_client.execute(
            "SELECT count() FROM scorecard_d FINAL WHERE scorecard_id IN %(ids)s",
            {"ids": scorecard_ids}
        )[0][0]
        ch_score_count = ch_client.execute(
            "SELECT count() FROM score_d FINAL WHERE scorecard_id IN %(ids)s",
            {"ids": scorecard_ids}
        )[0][0]
    else:
        # Large set — count by customer/profile filter (no FINAL needed for approximate count)
        pg_sc_count = count_scorecards(pg_cur, customer, profile, template_ids, start_date, end_date)
        ch_sc_count = ch_client.execute(
            "SELECT count() FROM scorecard_d WHERE customer_id = %(c)s AND profile_id = %(p)s AND conversation_id = ''",
            {"c": customer, "p": profile}
        )[0][0]
        ch_score_count = ch_client.execute(
            "SELECT count() FROM score_d WHERE customer_id = %(c)s AND profile_id = %(p)s AND conversation_id = ''",
            {"c": customer, "p": profile}
        )[0][0]

    scope = f"[{start_date}, {end_date})" if not scorecard_ids else f"{len(scorecard_ids)} specific scorecards"
    print(f"Verification for {scope}:")
    print(f"")
    print(f"  {'':30s} {'PG':>10s} {'CH':>10s}")
    print(f"  {'─' * 55}")
    print(f"  {'Scorecards':30s} {pg_sc_count:>10d} {ch_sc_count:>10d}")
    print(f"  {'Scores':30s} {'':>10s} {ch_score_count:>10d}")

    if not scorecard_ids:
        print(f"\n  Note: CH counts include all process scorecards (conversation_id=''), not filtered by date range.")
        print(f"  CH count >= PG count means backfill is complete.")

    if scorecard_ids and pg_sc_count != ch_sc_count:
        print(f"\n  MISMATCH: expected {pg_sc_count} scorecards in CH, got {ch_sc_count}")
        return False

    print(f"\n  Verification done.")
    return True


def main():
    parser = argparse.ArgumentParser(description="Backfill process scorecards from PG to CH")
    parser.add_argument("--customer", required=True, help="Customer ID (e.g., oportun, spirit)")
    parser.add_argument("--profile", required=True, help="Profile ID (e.g., us-west-2, us-east-1)")
    parser.add_argument("--dry-run", action="store_true",
                        help="Fetch a few scorecards, build CH rows, print them (no writes)")
    parser.add_argument("--verify-only", action="store_true", help="Compare PG vs CH counts")
    parser.add_argument("--limit", type=int, default=None,
                        help="Only process N scorecards (useful for testing with real writes)")
    parser.add_argument("--pg-conn", default=os.environ.get("PG_CONN", ""),
                        help="Postgres connection string (or set PG_CONN env var)")
    parser.add_argument("--ch-host", default=CH_HOST, help="ClickHouse host (or set CH_HOST)")
    parser.add_argument("--ch-password", default=os.environ.get("CH_PASSWORD", ""),
                        help="ClickHouse password (or set CH_PASSWORD env var)")
    parser.add_argument("--ch-database", default=CH_DATABASE, help="ClickHouse database (or set CH_DATABASE)")
    parser.add_argument("--start-date", default=START_DATE, help=f"Start date (default: {START_DATE})")
    parser.add_argument("--end-date", default=END_DATE, help=f"End date (default: {END_DATE})")
    parser.add_argument("--batch-size", type=int, default=1000, help="Batch size (default: 1000)")
    args = parser.parse_args()

    global BATCH_SIZE
    BATCH_SIZE = args.batch_size

    pg_connstring = args.pg_conn
    if not pg_connstring:
        print("ERROR: No PG connection string. Set PG_CONN env var or use --pg-conn.")
        sys.exit(1)

    print(f"Connecting to Postgres...", flush=True)
    pg_conn = psycopg2.connect(pg_connstring, connect_timeout=10)
    pg_conn.set_session(readonly=True, autocommit=True)

    pg_cur = pg_conn.cursor()
    template_ids = fetch_process_template_ids(pg_cur, args.customer, args.profile)
    print(f"Found {len(template_ids)} process scorecard templates for {args.customer}/{args.profile}")

    if not template_ids:
        print("No process scorecard templates found. Nothing to do.")
        pg_conn.close()
        return

    if args.dry_run:
        run_dry_run(pg_conn, args.customer, args.profile, template_ids, args.start_date, args.end_date)
        pg_conn.close()
        return

    ch_host = args.ch_host or CH_HOST
    ch_password = args.ch_password or CH_PASSWORD
    ch_database = args.ch_database or CH_DATABASE
    if not ch_host or not ch_password or not ch_database:
        print("ERROR: ClickHouse host, password, and database required.")
        print("  Set CH_HOST, CH_PASSWORD, CH_DATABASE env vars or use CLI args.")
        sys.exit(1)

    print(f"Connecting to ClickHouse ({ch_host}:{CH_PORT}/{ch_database})...")
    ch_client = CHClient(
        host=ch_host, port=CH_PORT, user=CH_USER, password=ch_password,
        database=ch_database, secure=True, verify=False,
    )

    if args.verify_only:
        run_verify(pg_conn, ch_client, args.customer, args.profile, template_ids, args.start_date, args.end_date)
    else:
        inserted_ids = run_backfill(pg_conn, ch_client, args.customer, args.profile, template_ids, args.start_date, args.end_date, limit=args.limit)
        print("\nRunning verification...")
        if args.limit and inserted_ids:
            run_verify(pg_conn, ch_client, args.customer, args.profile, template_ids, args.start_date, args.end_date, scorecard_ids=inserted_ids)
        else:
            run_verify(pg_conn, ch_client, args.customer, args.profile, template_ids, args.start_date, args.end_date)

    pg_conn.close()


if __name__ == "__main__":
    main()
