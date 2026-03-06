#!/usr/bin/env python3
"""
Validate Python scoring logic against historic.scorecard_scores.

Fetches a sample of scorecards from Spirit, computes percentage_value/weight/max_value
from director.scores + template revision, and compares against historic.scorecard_scores.
"""

import json
import math
import os
import sys
from datetime import datetime, timezone

import psycopg2

DELTA = 0.01  # float comparison tolerance (matches Go's Delta)


# ── Template parsing ──────────────────────────────────────────────────────────

def parse_template(template_json):
    """Parse template JSON, return dict of criterion_id -> criterion_info."""
    if isinstance(template_json, str):
        tmpl = json.loads(template_json)
    else:
        tmpl = template_json

    version = tmpl.get("version", 1)
    criteria = {}

    if version == 1:
        for item in tmpl.get("criteria", []):
            _extract_criteria(item, None, criteria)
    else:  # version 2
        for item in tmpl.get("items", []):
            _extract_from_node(item, None, criteria)

    return criteria


def _extract_from_node(item, chapter_id, criteria):
    """Recursively extract criteria from a v2 template node."""
    if "items" in item and item.get("items") is not None and (len(item.get("items", [])) > 0 or item.get("type") is None):
        # It's a chapter
        for child in item.get("items", []):
            _extract_from_node(child, item.get("identifier"), criteria)
    else:
        _extract_criteria(item, chapter_id, criteria)


def _extract_criteria(item, chapter_id, criteria):
    """Extract a single criterion and its branch children."""
    ctype = item.get("type")
    if ctype is None:
        return

    settings = item.get("settings") or {}
    identifier = item["identifier"]

    criteria[identifier] = {
        "type": ctype,
        "weight": item.get("weight", 0),
        "max_value": _get_max_value(item),
        "value_scores": settings.get("scores"),  # list of {value, score} or None
        "exclude_from_qa": settings.get("excludeFromQAScores", False),
        "is_multi_select": settings.get("enableMultiSelect", False),
        "is_per_message": item.get("perMessage", False),
        "is_outcome": _is_outcome(item),
        "chapter_id": chapter_id,
    }

    # Also extract children from branches
    for branch in item.get("branches") or []:
        for child in branch.get("children") or []:
            _extract_criteria(child, chapter_id, criteria)


def _get_max_value(item):
    """Get max value for a criterion (matches Go GetMaxValue/GetCriterionMaxScore)."""
    ctype = item.get("type", "")
    settings = item.get("settings") or {}

    # Sentence, date, user types have max_value = 0
    if ctype in ("sentence", "date", "user"):
        return 0

    # If value_scores mappings exist, max_score = max(score) from mappings
    value_scores = settings.get("scores")
    if value_scores and len(value_scores) > 0:
        return max(vs["score"] for vs in value_scores)

    # numeric-radios: use range.max (default 5)
    if ctype == "numeric-radios":
        range_settings = settings.get("range")
        if range_settings:
            return range_settings.get("max", 5)
        return 5

    # labeled-radios / dropdown-numeric-values: max of options[].value
    options = settings.get("options")
    if options and len(options) > 0:
        return max(opt["value"] for opt in options)

    return 0


def _is_outcome(item):
    """Check if criterion is an outcome criterion (metadata trigger)."""
    auto_qa = item.get("auto_qa")
    if not auto_qa:
        return False
    triggers = auto_qa.get("triggers")
    if not triggers or len(triggers) == 0:
        return False
    return triggers[0].get("type") == "metadata"


# ── Score computation ─────────────────────────────────────────────────────────

def compute_criterion_percentage(criterion_info, scores):
    """Compute percentage_value and weight for a criterion's scores.

    Returns list of (percentage_value, weight) tuples, one per score.
    None percentage means no percentage computed (excluded/NA/no value).
    """
    if not scores:
        return [(None, 0)] * len(scores) if scores else []

    # Check not_applicable and valid numeric values
    any_na = any(s.get("not_applicable") for s in scores)
    has_numeric = any(s.get("numeric_value") is not None for s in scores)

    if any_na or not has_numeric:
        return [(None, 0)] * len(scores)

    ci = criterion_info
    weight = ci["weight"]

    if ci["exclude_from_qa"]:
        return [(None, 0)] * len(scores)

    if ci["is_multi_select"]:
        return _compute_multi_select(ci, scores, weight)
    elif ci["is_per_message"]:
        return _compute_per_message(ci, scores, weight)
    elif ci["is_outcome"] and (not ci["value_scores"] or len(ci["value_scores"]) == 0):
        return _compute_outcome(ci, scores, weight)
    else:
        return _compute_default(ci, scores, weight)


def _compute_multi_select(ci, scores, weight):
    """Multi-select: percentage = (num_scores * selected_score) / sum_of_all_scores."""
    value_scores = ci["value_scores"]
    if not value_scores:
        return [(None, 0)] * len(scores)

    sum_score = sum(vs["score"] for vs in value_scores)
    if sum_score <= 0:
        return [(None, 0)] * len(scores)

    n = len(scores)
    results = []
    for s in scores:
        nv = s.get("numeric_value")
        if nv is None:
            results.append((None, 0))
            continue
        selected = _map_score_value(nv, value_scores)
        if selected is None:
            results.append((None, 0))
            continue
        pct = (n * selected) / sum_score
        results.append((pct, weight / n))
    return results


def _compute_per_message(ci, scores, weight):
    """Per-message: each score gets percentage = mapped_value / max_score, weight split."""
    max_score = ci["max_value"]
    if max_score <= 0:
        return [(None, 0)] * len(scores)

    valid_results = []
    for s in scores:
        pct = _map_to_percentage(s, ci, max_score)
        valid_results.append(pct)

    valid_count = sum(1 for p in valid_results if p is not None)
    if valid_count == 0:
        return [(None, 0)] * len(scores)

    return [(p, weight / valid_count if p is not None else 0) for p in valid_results]


def _compute_outcome(ci, scores, weight):
    """Outcome criterion without value_scores: percentage = numeric_value directly."""
    if len(scores) != 1:
        return [(None, 0)] * len(scores)
    s = scores[0]
    nv = s.get("numeric_value") if s.get("numeric_value") is not None else s.get("ai_value")
    if nv is None:
        return [(None, 0)]
    return [(nv, weight)]


def _compute_default(ci, scores, weight):
    """Default single-value criterion: percentage = mapped_value / max_score."""
    if len(scores) != 1:
        return [(None, 0)] * len(scores)

    max_score = ci["max_value"]
    if max_score <= 0:
        return [(None, 0)]

    pct = _map_to_percentage(scores[0], ci, max_score)
    return [(pct, weight if pct is not None else 0)]


def _map_to_percentage(score, ci, max_score):
    """Map a score to percentage = mapped_value / max_score."""
    nv = score.get("numeric_value")
    if nv is None:
        nv = score.get("ai_value")
    if nv is None:
        return None

    mapped = _map_score_value(nv, ci.get("value_scores"))
    if mapped is None:
        return None
    if mapped > max_score:
        return None  # would be error in Go
    return mapped / max_score


def _map_score_value(numeric_value, value_scores):
    """Map numeric_value through value_scores mapping. Returns mapped score or raw value."""
    if not value_scores or len(value_scores) == 0:
        return numeric_value
    for vs in value_scores:
        if abs(numeric_value - vs["value"]) < DELTA:
            return vs["score"]
    return None  # not found in mapping


def is_manually_scored(score):
    """Matches Go isManuallyScored logic."""
    ai_scored = score.get("ai_scored", False)
    if not ai_scored:
        return True
    ai_value = score.get("ai_value")
    numeric_value = score.get("numeric_value")
    if ai_value is None:
        return numeric_value is not None
    return numeric_value is None or numeric_value != ai_value


# ── Validation ────────────────────────────────────────────────────────────────

def validate(pg_conn, customer, profile, num_scorecards=10):
    cur = pg_conn.cursor()

    # Get template IDs
    cur.execute(
        "SELECT DISTINCT resource_id FROM director.scorecard_templates WHERE customer = %s AND profile = %s AND type = 2",
        (customer, profile))
    template_ids = [r[0] for r in cur.fetchall()]
    print(f"Process template IDs: {len(template_ids)}")

    # Fetch sample scorecards
    cur.execute("""
        SELECT resource_id, template_id, template_revision, customer, profile
        FROM director.scorecards
        WHERE customer = %s AND profile = %s AND template_id = ANY(%s)
        ORDER BY created_at
        LIMIT %s
    """, (customer, profile, template_ids, num_scorecards))
    scorecards = cur.fetchall()
    print(f"Sample scorecards: {len(scorecards)}")

    total_compared = 0
    total_match = 0
    total_mismatch = 0
    mismatch_cats = {"na_only": 0, "excl_only": 0, "na_and_excl": 0, "other": 0}

    for sc_id, tmpl_id, tmpl_rev, cust, prof in scorecards:
        print(f"\n{'─' * 60}")
        print(f"Scorecard: {sc_id}")
        print(f"  Template: {tmpl_id} rev={tmpl_rev}")

        # Fetch template revision
        cur.execute("""
            SELECT template FROM director.scorecard_template_revisions
            WHERE customer = %s AND profile = %s AND template_id = %s AND resource_id = %s
        """, (cust, prof, tmpl_id, tmpl_rev))
        row = cur.fetchone()
        if not row:
            print("  WARNING: template revision not found, skipping")
            continue
        criteria = parse_template(row[0])
        print(f"  Criteria in template: {len(criteria)}")

        # Fetch director.scores
        cur.execute("""
            SELECT resource_id, criterion_identifier, numeric_value, ai_value,
                   text_value, not_applicable, ai_scored, auto_failed
            FROM director.scores
            WHERE customer = %s AND profile = %s AND scorecard_id = %s
        """, (cust, prof, sc_id))
        director_scores = []
        for r in cur.fetchall():
            director_scores.append({
                "score_id": r[0],
                "criterion_identifier": r[1],
                "numeric_value": r[2],
                "ai_value": r[3],
                "text_value": r[4],
                "not_applicable": r[5] or False,
                "ai_scored": r[6] or False,
                "auto_failed": r[7] or False,
            })

        # Group by criterion
        grouped = {}
        for s in director_scores:
            grouped.setdefault(s["criterion_identifier"], []).append(s)

        # Compute using Python logic
        computed = {}  # score_id -> {percentage_value, weight, float_weight, max_value, manually_scored}
        for crit_id, crit_scores in grouped.items():
            ci = criteria.get(crit_id)
            if ci is None:
                continue  # chapter score, skip

            pct_results = compute_criterion_percentage(ci, crit_scores)
            for i, s in enumerate(crit_scores):
                pct, w = pct_results[i] if i < len(pct_results) else (None, 0)
                computed[s["score_id"]] = {
                    "percentage_value": pct,
                    "weight": int(w),
                    "float_weight": w,
                    "max_value": float(ci["max_value"]),
                    "manually_scored": is_manually_scored(s),
                }

        # Fetch historic.scorecard_scores for comparison
        cur.execute("""
            SELECT score_id, percentage_value, weight, float_weight, max_value, manually_scored
            FROM historic.scorecard_scores
            WHERE customer_id = %s AND profile_id = %s AND scorecard_id = %s
        """, (cust, prof, sc_id))
        historic_rows = {r[0]: {
            "percentage_value": r[1],
            "weight": r[2],
            "float_weight": r[3],
            "max_value": r[4],
            "manually_scored": r[5],
        } for r in cur.fetchall()}

        print(f"  Director scores: {len(director_scores)}, Historic rows: {len(historic_rows)}")

        # Compare
        for score_id, comp in computed.items():
            hist = historic_rows.get(score_id)
            if not hist:
                print(f"    {score_id}: no historic row (skipped)")
                continue

            total_compared += 1
            mismatches = []

            # percentage_value comparison
            h_pct = hist["percentage_value"]
            c_pct = comp["percentage_value"]
            if h_pct is None and c_pct is None:
                pass  # both null, OK
            elif h_pct is not None and c_pct is not None:
                # historic stores as NullFloat64, compare with tolerance
                if not math.isclose(float(h_pct), float(c_pct), abs_tol=0.001):
                    mismatches.append(f"percentage_value: historic={h_pct} computed={c_pct}")
            elif h_pct is None and c_pct is not None:
                mismatches.append(f"percentage_value: historic=NULL computed={c_pct}")
            elif h_pct is not None and c_pct is None:
                mismatches.append(f"percentage_value: historic={h_pct} computed=NULL")

            # weight comparison
            if hist["weight"] != comp["weight"]:
                mismatches.append(f"weight: historic={hist['weight']} computed={comp['weight']}")

            # float_weight comparison
            if hist["float_weight"] is not None and comp["float_weight"] is not None:
                if not math.isclose(float(hist["float_weight"]), float(comp["float_weight"]), abs_tol=0.001):
                    mismatches.append(f"float_weight: historic={hist['float_weight']} computed={comp['float_weight']}")

            # max_value comparison
            if hist["max_value"] is not None and not math.isclose(float(hist["max_value"]), comp["max_value"], abs_tol=0.001):
                mismatches.append(f"max_value: historic={hist['max_value']} computed={comp['max_value']}")

            # manually_scored comparison
            if hist["manually_scored"] != comp["manually_scored"]:
                mismatches.append(f"manually_scored: historic={hist['manually_scored']} computed={comp['manually_scored']}")

            if mismatches:
                total_mismatch += 1
                # Categorize mismatch
                score_detail = next((s for s in director_scores if s["score_id"] == score_id), None)
                ci_for_score = criteria.get(score_detail["criterion_identifier"]) if score_detail else None
                na = score_detail.get("not_applicable") if score_detail else False
                excl = ci_for_score.get("exclude_from_qa") if ci_for_score else False
                cat = []
                if na: cat.append("N/A")
                if excl: cat.append("excl_qa")
                cat_str = f" [{','.join(cat)}]" if cat else ""
                if na and excl: mismatch_cats["na_and_excl"] += 1
                elif na: mismatch_cats["na_only"] += 1
                elif excl: mismatch_cats["excl_only"] += 1
                else: mismatch_cats["other"] += 1
                print(f"    MISMATCH {score_id}{cat_str}:")
                for m in mismatches:
                    print(f"      {m}")
            else:
                total_match += 1

    print(f"\n{'═' * 60}")
    print(f"RESULTS: {total_compared} scores compared")
    print(f"  Match:    {total_match}")
    print(f"  Mismatch: {total_mismatch}")
    if total_compared > 0:
        print(f"  Accuracy: {total_match/total_compared*100:.1f}%")
    print(f"  Mismatch categories: {mismatch_cats}")
    other = mismatch_cats["other"]
    if other > 0:
        print(f"  WARNING: {other} mismatches NOT explained by N/A or exclude_from_qa!")


if __name__ == "__main__":
    pg_connstring = os.environ.get("PG_CONN", "")
    if not pg_connstring:
        print("Set PG_CONN env var")
        sys.exit(1)

    customer = sys.argv[1] if len(sys.argv) > 1 else "spirit"
    profile = sys.argv[2] if len(sys.argv) > 2 else "us-east-1"
    num = int(sys.argv[3]) if len(sys.argv) > 3 else 10

    conn = psycopg2.connect(pg_connstring, connect_timeout=10)
    conn.set_session(readonly=True, autocommit=True)
    validate(conn, customer, profile, num)
    conn.close()
