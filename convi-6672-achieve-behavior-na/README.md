# CONVI-6672: Achieve Behavior Metric N/A in Performance Insights

**Created:** 2026-04-26
**Updated:** 2026-04-26
**Status:** Investigation complete, pending PG template verification

## Overview

Several behavior criteria in Achieve's "Technical insights Scorecard" show N/A in Performance Insights for the Welcome Call use case (Jan–Apr 2026), even though:
- Opera rules are active and firing
- Scorecards show Yes/No auto-scores in Closed Conversations
- Multiple Opera Time Machine backfills have been run

**Customer:** Achieve (achieve.cresta.com)
**Use case:** Welcome Call
**Template:** Technical insights Scorecard
**Affected criteria:** 6 "Intro Trigger" behaviors (Password Reset, Error 400, 2 Step Verification, Apple ID, Credit Card on File, Web Server Error)
**Working criteria:** Turnbull "unable to load" Error (and others)

## Key Finding

**Root cause: criteria with `percentage_value=-1` and `float_weight=1e-13` in ClickHouse, despite having valid `numeric_value` (0 or 1) and `not_applicable=false`.**

This pattern is caused by `excludeFromQAScores=true` on the criterion in the scorecard template JSON. When this flag is set, the scoring pipeline (`computeScore()`) returns `nil` for the percentage score, causing:
- `percentage_value` → stays at default `-1`
- `float_weight` → stays at effectively `0` (stored as `1e-13`)

Performance Insights filters `percentage_value >= 0` and/or uses weighted averages with `float_weight`, so these criteria appear as N/A.

## Data Evidence

### How we found the pattern

ClickHouse does not store template titles, so we cannot definitively map a template ID to the "Technical insights Scorecard" name. However, we narrowed the search to the 3 templates used for `welcome-call` usecase and found **one template (`019c203e-1bba-71be-9ae7-e093ef0c80b7`) that exhibits a pattern matching the reported issue**: a group of 6 closely-related criteria where 5 have broken scores and 1 works — consistent with the ticket's description of 6 Intro Trigger behaviors showing N/A while other criteria (like Turnbull) work fine.

**This template ID needs to be confirmed against PG** to verify it is indeed "Technical insights Scorecard."

### Welcome Call Templates

| Template ID | Scorecard Count (Jan–Apr 2026) |
|---|---|
| `0198f7bd-5060-70ac-bd00-7d4b5ffd5b62` | 79,225 |
| `1aa9dc76-14f6-4f2c-932c-fa15aa04fdb2` | 77,745 |
| `019c203e-1bba-71be-9ae7-e093ef0c80b7` | 76,652 (pattern match) |

### Criteria Analysis on Template `019c203e`

The template has 6 criteria starting with `019d66xx` that share similar UUIDv7 timestamps (added around the same time). On a sample scorecard (`019dac81-4611-7993-a5f9-720f03a5baf8`):

| Criterion ID | numeric_value | percentage_value | float_weight | Status |
|---|---|---|---|---|
| `019d6604-bcad` | 1 | **-1** | **1e-13** | BROKEN |
| `019d6605-5737` | 1 | 0 | 1 | **WORKING** |
| `019d6605-e0c6` | 1 | **-1** | **1e-13** | BROKEN |
| `019d6606-8ebd` | 1 | **-1** | **1e-13** | BROKEN |
| `019d6606-d625` | 1 | **-1** | **1e-13** | BROKEN |
| `019d6607-12ef` | 1 | **-1** | **1e-13** | BROKEN |

All 6 criteria have valid `numeric_value` (behavior detected or not) and `not_applicable=false`, but only the working one has a computed `percentage_value` and proper `float_weight`.

### Aggregate Breakdown (broken criterion `019d6604-bcad`)

| not_applicable | numeric_value | percentage_value | float_weight | count |
|---|---|---|---|---|
| false | 1 | -1 | 1e-13 | 41,625 |
| true | -1 | -1 | 1e-13 | 21,954 |
| false | 0 | -1 | 1e-13 | 1,096 |

### Aggregate Breakdown (working criterion `019d6605-5737`)

| not_applicable | numeric_value | percentage_value | float_weight | count |
|---|---|---|---|---|
| false | 1 | 0 | 1 | 42,152 |
| true | -1 | -1 | 1e-13 | 22,031 |
| false | 0 | 1 | 1 | 492 |

## Code Path Analysis

### How `percentage_value=-1` is set

1. **`deriveScorecardScoreData()`** in `shared/clickhouse/conversations/scorecard_score.go:452-470`
   - Initializes `percentageValue = -1`
   - Only updates if `percentageScores != nil`

2. **`ComputeCriterionPercentageScore()`** → **`computeScore()`** in `shared/scoring/scorecard_scores_dao.go:712-714`
   ```go
   if criterion.IsExcludeFromQAScores() {
       return nil, nil  // <-- This makes percentageScores nil
   }
   ```

3. When `percentageScores` is nil, `percentageValue` stays `-1` and `floatWeight` stays `0` (becomes `1e-13` in CH).

### Why the working criterion works

The working criterion (`019d6605-5737`) either:
- Does NOT have `excludeFromQAScores=true` in the template JSON, OR
- Is routed through `computeOutcomeCriterionScore()` which **does NOT check** `IsExcludeFromQAScores()` (notable code path difference at `scorecard_scores_dao.go:363`)

## Hypothesis

**Most likely**: The 5 broken criteria have `"excludeFromQAScores": true` (or equivalently, the "Evaluate scores" toggle is OFF) in the scorecard template builder. This was likely set accidentally when the criteria were added, or there's a template builder UI bug that defaulted this incorrectly.

**To verify** (requires PG access):
```sql
-- First confirm which template is "Technical insights Scorecard"
SELECT resource_id, title
FROM director.scorecard_templates
WHERE resource_id IN (
  '019c203e-1bba-71be-9ae7-e093ef0c80b7',
  '0198f7bd-5060-70ac-bd00-7d4b5ffd5b62',
  '1aa9dc76-14f6-4f2c-932c-fa15aa04fdb2'
)
AND revision = 'latest';

-- Then inspect the template JSON for excludeFromQAScores
SELECT resource_id, template
FROM director.scorecard_templates
WHERE resource_id = '<confirmed_template_id>'
  AND revision = 'latest';
```

Then inspect each criterion's `excludeFromQAScores` field in the template JSON.

## Fix Options

1. **Template fix (if `excludeFromQAScores` is wrong):** Update the template to set `excludeFromQAScores=false` on the 5 affected criteria, then re-run the downstream backfill to recompute scores in ClickHouse.

2. **Re-score existing scorecards:** After template fix, existing scorecards need to be re-processed through the scoring pipeline to update `percentage_value` and `float_weight` in ClickHouse.

## Log History

| Date | Summary |
|------|---------|
| 2026-04-26 | Initial investigation: identified root cause pattern in ClickHouse data |
