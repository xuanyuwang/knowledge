# CONVI-6753: Weight=0 Criteria Showing N/A in Performance Insights

**Status**: Fix implemented, PR pending  
**Customer**: Home Care Delivered  
**Ticket**: [ZD #20340](https://crestasupport.zendesk.com/agent/tickets/20340) / [CONVI-6753](https://linear.app/cresta/issue/CONVI-6753)  
**Affected since**: ~2026-04-28  
**Worktree**: `/Users/xuanyu.wang/repos/go-servers-convi-6753` (branch `convi-6753-fix-weight-zero-float`)

## Problem

Intake + Recur rules annotate correctly in Closed Conversations but show **N/A** (score=0) in Performance Insights. A Performance Config Backfill (4/28-5/7) did not resolve the issue.

## Regression Timeline

| Date | Commit | Event |
|------|--------|-------|
| 2024-04-10 | `82e3134825` | `float_weight` column introduced in PI queries to handle fractional weights that int32 `weight` truncated to 0. |
| pre-2026-04-21 | — | **Old flow**: Go writes `FloatWeight=0` for weight-0 criteria → row lands in PG `historic.scorecard_scores` table → PG column default `0.0000000000001` (1e-13) replaces the Go zero → reindexer reads 1e-13 from PG → ClickHouse gets 1e-13. **Weight=0 criteria worked in PI.** |
| 2026-04-21 | `b9eafa34eb` | PR #26913 merged: removed historic analytics integration (Phase 2+3). |
| 2026-04-22 | `f15c277d9f` | PR #26913 reverted due to breakage. |
| 2026-04-22–23 | `9f467aecb1` → `d1572a310d` | Score row generation rebuilt: new `BuildScoreRowsFromDirectorScores` computes ClickHouse rows directly from director scores, **bypassing the PG `historic.scorecard_scores` table entirely**. |
| ~2026-04-28 | — | Customer starts seeing N/A for weight-0 criteria in PI. |

## Root Cause

The old flow wrote score rows through the `historic.scorecard_scores` PostgreSQL table, which had a **column default** that silently replaced zero weights:

```sql
-- historic-analytics/historic-schema/historic-schema.sql:311
float_weight FLOAT NOT NULL DEFAULT 0.0000000000001

-- historic-analytics/historic-schema/scorecard-scores.sql:46
CASE WHEN crit.weight <> 0 THEN crit.weight ELSE 0.0000000000001 END as float_weight
-- comment: "when weight is 0, float weight set to the default value"
```

The new direct-from-director code path (`BuildScoreRowsFromDirectorScores`) bypasses this table, so the Go-level `0.0` is written straight to ClickHouse. PI queries then compute `SUM(percentage * float_weight) / SUM(float_weight)` = 0/0 = N/A.

### Why the Go code always wrote 0

Both the old and new Go code paths use the same calculation:

```go
// shared/scoring/scorecard_scores_dao.go (identical pre- and post-26913)
weight := float64(criterion.GetWeight())                    // 0 for weight-0 criteria
percentageScores[i].Weight = weight / float64(numberScores) // 0.0
```

The difference was never in the Go logic — it was in whether the PG default `1e-13` rescued the zero before it reached ClickHouse.

## Fix

**File**: `shared/clickhouse/conversations/scorecard_score.go:467-470`

Use `1e-13` to match the historic PG default exactly:

```go
derived.floatWeight = percentageScores[i].Weight
if derived.floatWeight == 0 {
    derived.floatWeight = 1e-13
}
```

**Test**: `TestBuildScoreRowsFromDirectorScores_WeightZeroUsesMinFloat` — verifies a weight-0 criterion with a valid score produces `FloatWeight = 1e-13`.

## After Merge

A **Performance Config Backfill** for 4/28 onward is needed for Home Care Delivered to rewrite ClickHouse score rows with the corrected `float_weight` values.
