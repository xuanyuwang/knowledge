# percentage_value Bug in scorecard_score.go

**Created:** 2026-04-22
**Customer:** cresta, **Profile:** walter-dev
**Template:** `019db651-4170-75b1-a431-0a859426cb04`

## Symptom

RetrieveQAStats returns `100` for the template, displayed as 10000%. Other templates return scores in 0-1 range.

## Data

- Scorecard: `019db653-1579-71b8-b931-838867257254` (manually scored, score=100)
- Score: `019db653-15e4-7d50-8eb6-9ff5e9f9c78f`
  - `numeric_value = 2`, `max_value = 2`, `percentage_value = 100`
  - Expected `percentage_value = 1.0`

Template criterion (`019db651-658f-7698-9e4a-26bacee63088`):
- Type: labeled-radios, weight: 1
- Options: Yes=0, No=1, N/A=2 (isNA=true)
- ValueScores: `[{value:0, score:1}, {value:1, score:0}, {value:2, score:9}]`

## Root Cause

In `shared/clickhouse/conversations/scorecard_score.go:304`:
```go
percentageValue = (numericValue / maxValue) * 100
```

Introduced in commit `b9eafa34eb` (PR #26913, merged 2026-04-21) — "remove historic analytics integration (Phase 2 + 3)".

### Two problems

1. **`* 100` is wrong** — RetrieveQAStats expects `percentage_value` in 0-1 range, not 0-100. The old historic path stored it as 0-1. For `numeric_value=2, maxValue=2`: `(2/2) * 100 = 100` instead of `1.0`.

2. **Doesn't use `MapScoreValue` / `GetCriterionMaxScore`** — For criteria with ValueScores (like labeled-radios), the raw `numericValue` isn't the score. It needs to be mapped through ValueScores first, and `maxScore` should come from `GetCriterionMaxScore()`, not `GetMaxValue()`.

### Correct calculation (from `scorecard_scores_dao.go:mapToPercentageScore`)

```
MapScoreValue(2, criterion) → finds {value:2, score:9} → scoreValue = 9
GetCriterionMaxScore(criterion) → max of {1, 0, 9} = 9
percentage = scoreValue / maxScore = 9 / 9 = 1.0
```

## Timeline

- Before April 21: scores written through historic/kafka path → `percentage_value` in 0-1 range (correct)
- After April 21 (PR #26913): scores written through new `scorecard_score.go` path → `percentage_value = (numericValue / maxValue) * 100` (broken)

## RetrieveQAStats Calculation Trace

From `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`:

```sql
SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0) AS weighted_percentage_sum,
SUM(float_weight) FILTER (WHERE percentage_value >= 0) AS weight_sum
```

Then: `Score = weighted_percentage_sum / weight_sum = (100 * 1) / 1 = 100`

Expected: `(1.0 * 1) / 1 = 1.0`

## Data on PostgreSQL vs ClickHouse

### PostgreSQL: `director.scores` (source of truth)

| Column | Type | Description |
|--------|------|-------------|
| `numeric_value` | nullable float64 | Criterion value — a lookup key into `settings.options`, NOT the score itself. E.g., 0="Yes", 1="No", 2="N/A" |
| `ai_value` | nullable float64 | AI-predicted criterion value (same domain as numeric_value) |
| `text_value` | nullable string | Free-text value |
| `not_applicable` | nullable bool | Whether score is N/A |
| `ai_scored` | nullable bool | Whether AI scored this |
| `auto_failed` | nullable bool | Whether auto-fail triggered |
| `criterion_identifier` | string | Which criterion this score belongs to |
| `scorecard_id` | string | FK to scorecards table |

Key point: `numeric_value` stores the **raw criterion value** (e.g., 2), not the mapped score (e.g., 9). The mapping from value→score lives in the template's `settings.scores` array (ValueScores).

### ClickHouse: `score` table (analytics projection)

| Column | Type | Description |
|--------|------|-------------|
| `numeric_value` | float64 | Same as PG, but uses `-1` sentinel for NULL |
| `ai_value` | float64 | Same as PG, but uses `-1` sentinel for NULL |
| `percentage_value` | float64 | **Computed**: mapped score ÷ max score, in 0-1 range. Uses `-1` sentinel for NULL. This is what analytics queries aggregate. |
| `weight` | int32 | From template criterion |
| `float_weight` | float64 | From template criterion |
| `max_value` | float64 | From template criterion |
| `not_applicable` | bool | From PG |
| `ai_scored` | bool | From PG |
| `manually_scored` | bool | **Computed** per-score |
| `auto_failed` | bool | From PG |

Key point: `percentage_value` is the critical computed field. Analytics queries (e.g., RetrieveQAStats) aggregate it as:
```sql
SUM(percentage_value * float_weight) / SUM(float_weight)
```

### PostgreSQL: `historic.scorecard_scores` (intermediate table, removed by b9eafa34eb)

This was a middleware table in the `historic` schema that pre-computed the derived fields before writing to ClickHouse. Its schema mirrors the ClickHouse `score` table closely: it has `percentage_value`, `weight`, `float_weight`, `max_value`, `manually_scored` — all computed from PG source + template.

## Before b9eafa34eb: Calculation Flow

```
director.scores (PG)
       │
       ▼
GenerateHistoricScorecardScores()          ← scoring/scorecard_scores_dao.go
       │
       ├─ GetValidScorecardScores()        ← validate scores against template tree (branches/children)
       ├─ GroupCriterionScores()            ← group by criterion_identifier, skip chapters
       │
       ▼  For each criterion:
createSingleHistoricScorecardScores()
       │
       ├─ ComputeCriterionPercentageScore()
       │    │
       │    ├─ If any score is NotApplicable → return nil (no percentage)
       │    ├─ If no valid NumericValue      → return nil (no percentage)
       │    │
       │    └─ mapToPercentageScore()
       │         │
       │         ├─ MapScoreValue(numericValue, criterion)
       │         │    └─ Looks up ValueScores: {value:2} → {score:9}
       │         │       (If no ValueScores, returns numericValue as-is)
       │         │
       │         ├─ GetCriterionMaxScore(criterion)
       │         │    └─ Max of all ValueScores: max(1, 0, 9) = 9
       │         │       (If no ValueScores, returns criterion.MaxValue)
       │         │
       │         └─ percentage = scoreValue / maxScore    ← 0-1 range
       │              e.g., 9 / 9 = 1.0
       │
       ├─ isManuallyScored(score)           ← per-score logic:
       │    └─ !ai_scored → true
       │       !ai_value.Valid → numeric_value.Valid
       │       else → numeric_value != ai_value
       │
       └─ Build hmodel.ScorecardScores:
            NumericValue     = PG numeric_value (preserved)
            PercentageValue  = computed (0-1 range)
            Weight           = criterion.GetWeight()
            FloatWeight      = criterion.GetWeight()
            MaxValue         = criterion.GetMaxValue()
            ManuallyScored   = per-score computation
       │
       ▼
historic.scorecard_scores (PG intermediate table)
       │
       ▼
BuildScoreRows()                           ← clickhouse/conversations/conversation.go
       │
       ├─ NumericValue:     -1 if NULL, else raw value
       ├─ AiValue:          -1 if NULL, else raw value
       ├─ PercentageValue:  -1 if NULL or (AiValue AND NumericValue both NULL),
       │                     else value from historic table (already 0-1)
       └─ Pass-through:     Weight, FloatWeight, MaxValue, ManuallyScored, etc.
       │
       ▼
ClickHouse score table
```

**Concrete example with the bug template:**

Template: ValueScores = `[{value:0, score:1}, {value:1, score:0}, {value:2, score:9}]`

```
PG: numeric_value = 2
  → MapScoreValue(2, criterion) finds {value:2, score:9} → scoreValue = 9
  → GetCriterionMaxScore() = max(1, 0, 9) = 9
  → percentage = 9 / 9 = 1.0
  → historic.percentage_value = 1.0
  → CH percentage_value = 1.0 ✅
```

## After b9eafa34eb: Calculation Flow (BROKEN)

```
director.scores (PG)
       │
       ▼
BuildScoreRowsFromDirectorScores()         ← clickhouse/conversations/scorecard_score.go
       │
       ├─ Get criterion from template:
       │    weight     = criterion.GetWeight()
       │    floatWeight= criterion.GetWeight()
       │    maxValue   = criterion.GetMaxValue()     ← WRONG denominator
       │
       ├─ percentageValue = (numericValue / maxValue) * 100    ← WRONG
       │    ├─ No MapScoreValue() call — raw value used directly
       │    ├─ Uses GetMaxValue() not GetCriterionMaxScore()
       │    └─ Multiplies by 100 (CH expects 0-1 range)
       │
       ├─ NumericValue:     0 if NULL (was -1)
       ├─ AiValue:          0 if NULL (was -1)
       ├─ ManuallyScored:   scorecard.ManuallyScored (was per-score)
       │
       ├─ No NotApplicable / missing-value skip for percentage
       ├─ No GetValidScorecardScores() validation
       │
       └─ Build ScoreRow directly
       │
       ▼
ClickHouse score table
```

**Same example with the bug template:**

Template: ValueScores = `[{value:0, score:1}, {value:1, score:0}, {value:2, score:9}]`, MaxValue = 2

```
PG: numeric_value = 2
  → NO MapScoreValue() call — uses raw numericValue = 2
  → maxValue = criterion.GetMaxValue() = 2 (NOT GetCriterionMaxScore() which returns 9)
  → percentageValue = (2 / 2) * 100 = 100
  → CH percentage_value = 100 ❌ (expected 1.0)
```

### Summary of Errors in New Path

| Field | Old (correct) | New (broken) | Impact |
|-------|---------------|--------------|--------|
| `percentage_value` | `MapScoreValue(numericValue) / GetCriterionMaxScore()` in 0-1 range | `(numericValue / GetMaxValue()) * 100` | Wrong value, wrong scale. All analytics scores broken for criteria with ValueScores. |
| `numeric_value` sentinel | `-1` for NULL | `0` for NULL | `0` is a valid criterion value; downstream can't distinguish NULL from "value 0" |
| `ai_value` sentinel | `-1` for NULL | `0` for NULL | Same issue |
| `manually_scored` | Per-score: `isManuallyScored(score)` | Per-scorecard: `scorecard.ManuallyScored` | Wrong granularity — individual score override status lost |
| NotApplicable skip | If NotApplicable → no percentage computed | Always computes percentage | N/A scores pollute aggregation |
| Score validation | `GetValidScorecardScores()` filters invalid/branch-excluded scores | No validation | Invalid or branch-excluded scores written to CH |

## Fix

Replace the naive calculation in `scorecard_score.go` with the proper `mapToPercentageScore` logic that:
1. Calls `MapScoreValue()` to map raw value through ValueScores
2. Uses `GetCriterionMaxScore()` for the denominator
3. Returns result in 0-1 range (no `* 100`)
