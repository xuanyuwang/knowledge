# CONVI-6219: Comprehensive Directionality Fix — Implementation Plan

**Created:** 2026-03-17
**Updated:** 2026-03-26
**Status:** Planning (PR #26517 converted to draft for rework)
**Approach:** Option B — Unified shared directionality, separate data sources

## Problem

Both `RetrieveQAScoreStats` and `RetrieveUserOutcomeStats` assume higher score = better. For TIME/CHURN outcomes (e.g., AHT, churn rate), lower values mean better performance. The coaching service already handles this correctly. We need to fix both APIs using shared logic.

### Affected Functions

| API | Function | What it does | Fix needed |
|-----|----------|-------------|------------|
| RetrieveUserOutcomeStats | `aggregateTopAgentsInternalUserOutcomeStats()` | Tier partitioning (TOP/AVG/BOTTOM) | Negate metric for lower-is-better |
| RetrieveQAScoreStats | `aggregateTopAgentsResponse()` | Tier partitioning (TOP/AVG/BOTTOM) | Negate metric for lower-is-better (per-criterion group — already works) |
| RetrieveQAScoreStats | `setQuintileRankForPerAgentScores()` | Quintile ranking Q1-Q5 | **Negate in CH query** (see revised approach below) |

## PR Review Findings (2026-03-26)

PR #26517 was reviewed and two issues were identified:

1. **Skipped error checking** on `BuildLowerIsBetterCriterionSet` — should log a `Warnf` like the other error paths.
2. **Quintile path doesn't handle mixed criteria** — `setQuintileRankForPerAgentScores` applies a single sort direction to ALL scores, but FE can request a template containing both lower-is-better and higher-is-better criteria.

### Root Cause: Aggregation Happens in ClickHouse

When FE requests `group_by=[AGENT]` (the quintile path), the CH query aggregates across ALL criteria:

```sql
-- From testdata/clickhouse_RetrieveQAScoreStats_GroupByAgent_request.sql
SELECT
    agent_user_id,
    SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0) AS weighted_percentage_sum,
    SUM(float_weight) FILTER (WHERE percentage_value >= 0) AS weight_sum,
    COUNT(DISTINCT conversation_id) AS total_conversation_count,
    COUNT(DISTINCT scorecard_id) AS total_scorecard_count
FROM scorecard_score JOIN filtered_scorecard ON ...
GROUP BY agent_user_id
```

The weighted average (`SUM(pv * fw) / SUM(fw)`) already blends all criteria into a single score per agent. There is no `criterion_id` in the SELECT or GROUP BY — so **we cannot apply per-criterion directionality after the query returns**.

Compare with `GroupByAgentCriterion`:
```sql
SELECT agent_user_id, criterion_id, SUM(percentage_value * float_weight) ...
GROUP BY agent_user_id, criterion_id
```

### Key Insight

- **Quintile rank is only used when `group_by=[AGENT]`** (no CRITERION in group-by).
- In this case CH aggregates across all criteria → no criterion info in result.
- Negation **must** happen in the CH query itself, before `SUM()`.

## Revised Approach: Option 1 — Ranking Column in CH Query

Add a separate `ranking_weighted_percentage_sum` column that negates lower-is-better criteria, while keeping `weighted_percentage_sum` unchanged for display.

### SQL Change

When lower-is-better criteria are known, the CH query becomes:

```sql
SELECT
    agent_user_id,
    -- Display score (unchanged)
    SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0) AS weighted_percentage_sum,
    SUM(float_weight) FILTER (WHERE percentage_value >= 0) AS weight_sum,
    -- Ranking score (negates lower-is-better criteria)
    SUM(
      CASE WHEN criterion_id IN ('lower-is-better-crit-1', 'lower-is-better-crit-2')
        THEN -percentage_value * float_weight
        ELSE percentage_value * float_weight
      END
    ) FILTER (WHERE percentage_value >= 0) AS ranking_weighted_percentage_sum,
    COUNT(DISTINCT conversation_id) AS total_conversation_count,
    COUNT(DISTINCT scorecard_id) AS total_scorecard_count
FROM scorecard_score JOIN filtered_scorecard ON ...
GROUP BY agent_user_id
```

When no lower-is-better criteria exist (common case), skip the extra column entirely.

### Where `scoreSelectQuery` is Constructed

The `SUM(percentage_value * float_weight)` expression is a hardcoded string in 3 query builder functions in `retrieve_qa_score_stats_clickhouse.go`:

| Function | Lines | Notes |
|----------|-------|-------|
| `qaScoreStatsClickhouseQuery` | 151-160 | Main path, only inner query |
| `qaScoreStatsClickhouseQueryWithMoment` | 300-305 | Deprecated, has inner + outer query (`SUM(weighted_percentage_sum)` at line 334) |
| `qaScoreStatsClickhouseQueryWithMetadataView` | 469-474 | Same structure as WithMoment, inner + outer query (`SUM(weighted_percentage_sum)` at line 503) |

**Important**: The WithMoment and WithMetadataView variants have a two-level structure:
1. Inner CTE `scorecard_score_per_conversation`: computes `weighted_percentage_sum` per (conversation, scorecard, [groupByKeys])
2. Outer SELECT: `SUM(weighted_percentage_sum)` across conversations

Both levels need the ranking column propagated.

### Data Flow Changes

```
                          CURRENT                              REVISED
                          -------                              -------
CH Query:                 GROUP BY agent_user_id               Same, + ranking column
                          → weighted_percentage_sum             → weighted_percentage_sum (display)
                          → weight_sum                          → weight_sum
                                                               → ranking_weighted_percentage_sum (ranking)

qaScoreStatsRow:          weightedPercentageSum                + rankingWeightedPercentageSum
                          weightSum

Scan (line 661):          &row.weightedPercentageSum, ...      + &row.rankingWeightedPercentageSum

convertCHResponse:        Score = wp / ws (display)            Score = wp / ws (display)
                                                               RankingScore = rwp / ws (for quintile only)

setQuintileRank:          sort by Score                        sort by RankingScore
                          assign Q1-Q5                         assign Q1-Q5
```

## Implementation Steps (Revised)

### Step 1: shared/scoring/directionality.go ✅ (PR #26430, merged)

Already done — `BuildLowerIsBetterCriterionSet`, `IsLowerBetterOutcomeType`, `IsLowerBetterOutcomeTypeEnum`.

### Step 2: Pass lower-is-better criteria to CH query builders

In `readQaScoreStatsFromClickhouse()`:
1. Accept `lowerIsBetterCriteria set.Set[string]` as a new parameter
2. Pass it to the 3 query builder functions
3. Each builder generates the extra `ranking_weighted_percentage_sum` column when the set is non-empty

Function signature changes:
```go
func qaScoreStatsClickhouseQuery(
    ...,
    lowerIsBetterCriteria set.Set[string],  // NEW
) (string, []any)
```

### Step 3: Build `scoreSelectQuery` with ranking column

When `len(lowerIsBetterCriteria) > 0`:

```go
// Build criterion IN clause for CASE WHEN
criterionIDs := lowerIsBetterCriteria.Slice()
placeholders := strings.Repeat("?, ", len(criterionIDs)-1) + "?"
rankingExpr := fmt.Sprintf(`SUM(
    CASE WHEN criterion_id IN (%s)
        THEN -percentage_value * float_weight
        ELSE percentage_value * float_weight
    END
) FILTER (WHERE percentage_value >= 0) AS ranking_weighted_percentage_sum`, placeholders)

scoreSelectQuery = baseScoreSelectQuery + ",\n" + rankingExpr
// Add criterionIDs to args
```

For WithMoment / WithMetadataView variants, also add to the outer SELECT:
```sql
SUM(ranking_weighted_percentage_sum) AS ranking_weighted_percentage_sum
```

For `scorecardTable` (scorecard-level scores, no criterion_id), skip the ranking column — directionality only applies at criterion level.

### Step 4: Update `qaScoreStatsRow` and scan logic

```go
type qaScoreStatsRow struct {
    clickhouseKeyTimeRow
    weightedPercentageSum         float64
    weightSum                     float64
    totalConversationCount        uint64
    totalScorecardCount           uint64
    rankingWeightedPercentageSum  *float64  // nil when no lower-is-better criteria
}
```

In the scan loop (line 661), conditionally append `&row.rankingWeightedPercentageSum` when the ranking column is present. Need to pass a flag or use the presence of `lowerIsBetterCriteria` to know whether to scan the extra column.

### Step 5: Propagate ranking score to response

In `convertCHResponseToQaScoreStatsResponse`, compute the ranking score:
```go
rankingScore := safeDivideFloat(row.weightedPercentageSum, row.weightSum)  // default = display score
if row.rankingWeightedPercentageSum != nil {
    rankingScore = safeDivideFloat(*row.rankingWeightedPercentageSum, row.weightSum)
}
```

Two options for carrying the ranking score:
- **A**: Add a field to `QAScore` proto (e.g. `ranking_score`) — proto change, but clean
- **B**: Store in a side map (`map[agentUserID]float64`) passed to `setQuintileRankForPerAgentScores` — no proto change

Option B is simpler and doesn't expose internals to the FE.

### Step 6: Update `setQuintileRankForPerAgentScores`

```go
func setQuintileRankForPerAgentScores(
    response *analyticspb.RetrieveQAScoreStatsResponse,
    rankingScores map[string]float64,  // agentUserID -> ranking score (nil = use display score)
)
```

Sort and assign quintiles using `rankingScores` instead of `Score`:
```go
sort.SliceStable(agentScores, func(i, j int) bool {
    return getRankingScore(agentScores[i], rankingScores) > getRankingScore(agentScores[j], rankingScores)
})
```

Always sort descending — the negation in the CH query already flips lower-is-better criteria.

### Step 7: Fix `aggregateTopAgentsResponse` (tier path)

The tier path groups by criterion first (`scoreMap`), so it handles mixed criteria correctly. Keep the current approach from PR #26517 (negate in Go per criterion group). No CH query change needed for this path since it uses `group_by=[AGENT_TIER, CRITERION]`.

### Step 8: Fix error handling

Add `Warnf` for `BuildLowerIsBetterCriterionSet` error (review comment 1):
```go
lowerIsBetterCriteria, lErr := scoring.BuildLowerIsBetterCriterionSet(...)
if lErr != nil {
    a.logger.Warnf(ctx, "failed to build lower-is-better criteria set: %v", lErr)
}
```

### Step 9: Tests

| Test | What it verifies |
|------|-----------------|
| Golden SQL test | New golden `.sql` file with `ranking_weighted_percentage_sum` column |
| `TestSetQuintileRankForPerAgentScores_LowerIsBetter` | Low AHT agents get Q1 using ranking scores |
| `TestSetQuintileRankForPerAgentScores_MixedCriteria` | Mixed template: ranking correctly handles both directions |
| `TestAggregateTopAgentsResponse_LowerIsBetter` | Low-score agents in TOP tier (unchanged from current PR) |
| Existing quintile tests | Pass `nil` ranking scores, behavior unchanged |

## How Each API Determines Directionality

### RetrieveUserOutcomeStats — via Metadata Framework

```
FieldDefinition (metadata framework)
  -> GetFieldMetadata().GetUserOutcomesFieldMetadataConfig().GetOutcomeType()
  -> enumspb.OutcomeTypeEnum (TIME, CHURN = lower is better)
```

Already fetched in `filterFieldDefinitionsByFrequency()` — no extra DB queries.

### RetrieveQAScoreStats — via Scorecard Template -> Moment Template chain

```
Scorecard Template (JSONB)
  -> scoring.ParseScorecardTemplateStructure(template)
  -> criterion.GetAutoQA().Triggers (where trigger.Type == "metadata")
  -> trigger.ResourceName  (moment resource name)
  -> Parse moment template ID from resource name
  -> Query director.moment_templates table (customer_id, profile_id, moment_template_id)
  -> Deserialize payload JSONB -> dbmomentpb.MomentData
  -> ConversationOutcomeMomentData.Type  OR  ConversationMetadataMomentData.OutcomeType
  -> commonpb.OutcomeType (TIME, CHURN = lower is better)
```

Requires +1 batched DB query for moment_templates. Scorecard templates may already be fetched.

## PR Strategy (Revised 2026-03-26)

```
PR 1: shared/scoring/directionality.go (foundation)
  |   -> #26430 (merged ✅)
  |
  +-- PR 2: QAScoreStats + UserOutcomeStats combined
  |   -> #26517 (draft, needs rework for CH query ranking column)
  |   -> Wire ranking column into CH query for quintile path
  |   -> Wire BuildLowerIsBetterCriterionSet into QA tier (keep current approach)
  |   -> Wire IsLowerBetterOutcomeTypeEnum into UserOutcomeStats tier
  |
  +-- PR 3: coaching service refactor (pure refactor, uses shared)
      -> #26431 (merged ✅)
```

## Complexity & Risk Assessment

| Aspect | Assessment |
|--------|-----------|
| DB queries | +1 batched query for moment_templates in QAScoreStats. UserOutcomeStats unchanged. |
| CH query change | Only adds extra SELECT column when lower-is-better criteria exist. No impact on non-directionality queries. |
| Blast radius | Tier path: per-criterion negate in Go (safe, groups by criterion). Quintile path: ranking column in CH (only affects sort, not display score). |
| Edge cases | Mixed directionality template: correctly handled by CH CASE WHEN. Scorecard-level queries (no criterion_id): skip ranking column. Empty lower-is-better set: no extra column, no behavior change. |
| Proto changes | None — ranking score carried via side map, not exposed to FE. |
| Rollback | Revert PR #26517 reverted changes. Shared package (#26430) has no side effects. |

## File Changes Summary

| File | Change | PR |
|------|--------|----|
| `shared/scoring/directionality.go` (NEW) | Shared helpers | 1 (#26430 ✅) |
| `insights-server/.../retrieve_qa_score_stats_clickhouse.go` | Add `ranking_weighted_percentage_sum` column to 3 query builders, update scan logic | 2 (#26517) |
| `insights-server/.../retrieve_qa_score_stats.go` | Pass ranking scores to `setQuintileRankForPerAgentScores`, fix error handling | 2 (#26517) |
| `insights-server/.../retrieve_qa_score_stats_test.go` | New golden SQL, new lower-is-better + mixed-criteria tests | 2 (#26517) |
| `insights-server/.../retrieve_user_outcome_stats.go` | Wire `IsLowerBetterOutcomeTypeEnum` into tier aggregation | 2 (#26517) |
| `apiserver/internal/coaching/...` | Refactor to use shared helpers | 3 (#26431 ✅) |
