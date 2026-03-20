# CONVI-6219: Comprehensive Directionality Fix — Implementation Plan

**Created:** 2026-03-17
**Updated:** 2026-03-19
**Status:** Planning
**Approach:** Option B — Unified shared directionality, separate data sources

## Problem

Both `RetrieveQAScoreStats` and `RetrieveUserOutcomeStats` assume higher score = better. For TIME/CHURN outcomes (e.g., AHT, churn rate), lower values mean better performance. The coaching service already handles this correctly. We need to fix both APIs using shared logic.

### Affected Functions

| API | Function | What it does | Fix needed |
|-----|----------|-------------|------------|
| RetrieveUserOutcomeStats | `aggregateTopAgentsInternalUserOutcomeStats()` | Tier partitioning (TOP/AVG/BOTTOM) | Negate metric for lower-is-better (PR #26332, draft) |
| RetrieveQAScoreStats | `aggregateTopAgentsResponse()` | Tier partitioning (TOP/AVG/BOTTOM) | Same negate pattern |
| RetrieveQAScoreStats | `setQuintileRankForPerAgentScores()` | Quintile ranking Q1-Q5 | Flip sort order for lower-is-better |

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

### Coaching Service (reference, already correct)

Uses the same chain as QAScoreStats. Helpers live in `apiserver/internal/coaching/action_retrieve_coaching_progresses.go`:
- `extractCriterionOutcomeMomentResourceNames(templateJSON)` -> `map[criterionID]momentResourceName`
- `getOutcomeTypesFromMomentTemplates(db, customerID, profileID, momentTemplateIDs)` -> `map[momentTemplateID]OutcomeType`
- `isLowerBetterOutcomeType(outcomeType)` -> bool

## Existing Code to Reuse

### Already in shared packages
- `shared/scoring/scorecard_template_parser.go` — `ParseScorecardTemplateStructure()`, `GetCriteriaSlice()`, `AutoQAConfig` with triggers
- `shared/scoring/scorecard_templates.go` — trigger type constants (`TriggerTypeMetadata`)
- `shared/scoring/scorecard_template_dao.go` — `ResolveScorecardTemplateTriggers()` queries moment_templates (but returns display names only, not outcome types)
- `shared/qa/scorecard_template.go` — `ListCurrentScorecardTemplateIDs()`, `ExtractScorecardTemplateIDs()`

### Already in insights-server
- `a.appsDB.DB(ctx)` — PostgreSQL access (same DB as coaching)
- `qa.ListCurrentScorecardTemplateIDs()` — already called in `readQaScoreStatsRespectingTemplateQaScoreConfig()`
- `qa.ExtractScorecardTemplateIDs()` — parses template IDs from request
- `isLowerBetterOutcomeTypeEnum()` — already in PR #26332 for UserOutcomeStats (uses `enumspb.OutcomeTypeEnum`)

## Implementation Steps

### Step 1: Create `shared/scoring/directionality.go`

Move coaching helpers to shared + add the `enumspb` variant:

```go
// shared/scoring/directionality.go
package scoring

// --- commonpb.OutcomeType (used by QA scores, coaching) ---

// IsLowerBetterOutcomeType returns true for TIME and CHURN outcome types.
func IsLowerBetterOutcomeType(outcomeType commonpb.OutcomeType) bool {
    switch outcomeType {
    case commonpb.OutcomeType_TIME, commonpb.OutcomeType_CHURN:
        return true
    default:
        return false
    }
}

// --- enumspb.OutcomeTypeEnum (used by user outcome stats / metadata framework) ---

// IsLowerBetterOutcomeTypeEnum returns true for TIME and CHURN outcome type enums.
func IsLowerBetterOutcomeTypeEnum(outcomeType enumspb.OutcomeTypeEnum) bool {
    switch outcomeType {
    case enumspb.OutcomeTypeEnum_OUTCOME_TYPE_TIME, enumspb.OutcomeTypeEnum_OUTCOME_TYPE_CHURN:
        return true
    default:
        return false
    }
}

// --- Criterion -> Moment -> OutcomeType chain (used by QA scores, coaching) ---

// ExtractCriterionOutcomeMomentResourceNames parses a scorecard template JSON
// and returns map[criterionID] -> moment resource name for metadata triggers.
func ExtractCriterionOutcomeMomentResourceNames(templateJSON string) map[string]string

// GetOutcomeTypesFromMomentTemplates queries moment_templates and returns
// map[momentTemplateID] -> OutcomeType.
func GetOutcomeTypesFromMomentTemplates(
    db *gorm.DB, customerID, profileID string, momentTemplateIDs []string,
) (map[string]commonpb.OutcomeType, error)

// BuildLowerIsBetterCriterionSet is a composite helper: given scorecard templates,
// resolves the full chain and returns a set of criterion IDs where lower score = better.
func BuildLowerIsBetterCriterionSet(
    db *gorm.DB,
    profileName customerpb.ProfileNameInterface,
    templates map[string]*model.ScorecardTemplates,
) (set.Set[string], error) {
    // 1. For each template, parse and extract criterion -> moment resource name
    // 2. Collect unique moment resource names, parse moment template IDs
    // 3. Batch query moment_templates for outcome types
    // 4. For each criterion, check if its moment's outcome type is lower-is-better
    // 5. Return set of criterion IDs
}
```

Also add `shared/scoring/directionality_test.go` with unit tests.

### Step 2: Update coaching service to use shared helpers

Pure refactor in `apiserver/internal/coaching/action_retrieve_coaching_progresses.go`:
- Replace `extractCriterionOutcomeMomentResourceNames()` -> `scoring.ExtractCriterionOutcomeMomentResourceNames()`
- Replace `getOutcomeTypesFromMomentTemplates()` -> `scoring.GetOutcomeTypesFromMomentTemplates()`
- Replace `isLowerBetterOutcomeType()` -> `scoring.IsLowerBetterOutcomeType()`
- Delete the local copies

### Step 3: Update UserOutcomeStats (PR #26332) to use shared helper

In `insights-server/internal/analyticsimpl/retrieve_user_outcome_stats.go`:
- Replace local `isLowerBetterOutcomeTypeEnum()` -> `scoring.IsLowerBetterOutcomeTypeEnum()`
- Delete the local copy

### Step 4: Wire directionality into RetrieveQAScoreStats

In `retrieve_qa_score_stats.go`, in `retrieveQAScoreStatsInternal()`:

```go
// Build lower-is-better criterion set
// (reuse templates if already fetched for RespectTemplateQaScoreConfig)
scorecardTemplateIDs := qa.ExtractScorecardTemplateIDs(req.GetFilterByAttribute().GetScorecardTemplates())
templates, err := qa.ListCurrentScorecardTemplateIDs(
    a.appsDB.DB(ctx), profileName, usecaseNames, scorecardTemplateIDs, false)
if err != nil { ... }
lowerIsBetterCriteria, err := scoring.BuildLowerIsBetterCriterionSet(
    a.appsDB.DB(ctx), profileName, templates)
if err != nil { ... }

// Pass to tier and quintile functions
```

**Note**: The `RespectTemplateQaScoreConfig` path already fetches templates. Refactor to reuse that result to avoid a duplicate query.

### Step 5: Fix `aggregateTopAgentsResponse()`

Same pattern as UserOutcomeStats:

```go
func (a *AnalyticsServiceImpl) aggregateTopAgentsResponse(
    resp *analyticspb.RetrieveQAScoreStatsResponse,
    lowerIsBetterCriteria set.Set[string],  // NEW PARAM
) *analyticspb.RetrieveQAScoreStatsResponse
```

Each partition group has a single criterion (grouped by criterion_id). For each group:
- Check if criterion is in the lower-is-better set
- If so, negate the score metric before `PartitionUsingVolumeAndMetric()`

### Step 6: Fix `setQuintileRankForPerAgentScores()`

```go
func setQuintileRankForPerAgentScores(
    resp *analyticspb.RetrieveQAScoreStatsResponse,
    lowerIsBetterCriteria set.Set[string],  // NEW PARAM
)
```

The current implementation ranks ALL agent scores together. With directionality:

**Approach**: Check if any score's criterion is lower-is-better. If so, flip the sort for those scores.

In practice, the FE calls with a single criterion filter, so all scores share the same directionality. Implementation:
1. Collect per-agent scores (where `GroupedBy.User != nil`)
2. Determine if the criterion is lower-is-better (check first score's criterion, or check all and handle mixed)
3. Sort ascending (instead of descending) if lower-is-better
4. `utils.AssignRankGroups(n, 5, scoreFunction)` works the same — it just needs the sort to be correct
5. Assign quintile ranks

**Mixed directionality edge case**: If scores span criteria with different directionalities, log a warning and fall back to default (descending) sort. This shouldn't happen in practice.

### Step 7: Tests

| Test | Location | What it verifies |
|------|----------|-----------------|
| `TestIsLowerBetterOutcomeType` | `shared/scoring/directionality_test.go` | TIME/CHURN -> true, SALE/RETENTION/etc -> false |
| `TestIsLowerBetterOutcomeTypeEnum` | `shared/scoring/directionality_test.go` | Same for enumspb variant |
| `TestExtractCriterionOutcomeMomentResourceNames` | `shared/scoring/directionality_test.go` | Parses template JSON, extracts metadata trigger resource names |
| `TestBuildLowerIsBetterCriterionSet` | `shared/scoring/directionality_test.go` | End-to-end with mock DB: templates with TIME/CHURN/SALE criteria |
| `TestAggregateTopAgentsResponse_LowerIsBetter` | `insights-server/.../retrieve_qa_score_stats_test.go` | Low-score agents in TOP tier for TIME criteria |
| `TestSetQuintileRankForPerAgentScores_LowerIsBetter` | `insights-server/.../retrieve_qa_score_stats_test.go` | Low-score agents get Q1 for TIME criteria |

## PR Strategy (Revised 2026-03-20)

```
PR 1: shared/scoring/directionality.go (foundation)
  |   -> #26430 (in review)
  |
  +-- PR 2: QAScoreStats + UserOutcomeStats combined
  |   -> Wire BuildLowerIsBetterCriterionSet into QA tier + quintile
  |   -> Wire IsLowerBetterOutcomeTypeEnum into UserOutcomeStats tier
  |   -> Supersedes draft #26332 (close it)
  |
  +-- PR 3: coaching service refactor (pure refactor, uses shared)
```

PRs 2, 3 are independent after PR 1 merges.

**Why close #26332?** Draft PR introduced a local `isLowerBetterOutcomeTypeEnum()` in insights-server. The combined PR will use the shared `scoring.IsLowerBetterOutcomeTypeEnum()` instead.

## Complexity & Risk Assessment

| Aspect | Assessment |
|--------|-----------|
| DB queries | +1 batched query for moment_templates in QAScoreStats. UserOutcomeStats unchanged (no extra queries). |
| Blast radius | Coaching: pure refactor, no behavior change. UserOutcomeStats: swap local helper for shared, no behavior change. QAScoreStats: new functionality, only affects TIME/CHURN criteria. |
| Edge cases | Mixed directionality in single quintile request (unlikely in practice). Criteria without metadata triggers (non-outcome) default to higher-is-better. Nil/empty template maps. |
| Proto changes | None needed. |
| Rollback | Each PR is independently revertable. Shared package has no side effects if consumers aren't updated. |

## File Changes Summary

| File | Change | PR |
|------|--------|----|
| `shared/scoring/directionality.go` (NEW) | `IsLowerBetterOutcomeType`, `IsLowerBetterOutcomeTypeEnum`, `ExtractCriterionOutcomeMomentResourceNames`, `GetOutcomeTypesFromMomentTemplates`, `BuildLowerIsBetterCriterionSet` | 1 (#26430) |
| `shared/scoring/directionality_test.go` (NEW) | Unit tests for all shared helpers | 1 (#26430) |
| `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go` | Wire `BuildLowerIsBetterCriterionSet` into `retrieveQAScoreStatsInternal`, pass to `aggregateTopAgentsResponse` and `setQuintileRankForPerAgentScores`, fix sort/negate logic | 2 (combined) |
| `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_test.go` | New tests for tier + quintile with lower-is-better criteria | 2 (combined) |
| `insights-server/internal/analyticsimpl/retrieve_user_outcome_stats.go` | Replace local `isLowerBetterOutcomeTypeEnum` with `scoring.IsLowerBetterOutcomeTypeEnum` | 2 (combined) |
| `insights-server/internal/analyticsimpl/retrieve_user_outcome_stats_transform_test.go` | Move `TestIsLowerBetterOutcomeTypeEnum` to shared (or keep as integration test calling shared) | 2 (combined) |
| `apiserver/internal/coaching/action_retrieve_coaching_progresses.go` | Replace local helpers with shared calls, delete local copies | 3 (coaching refactor) |
