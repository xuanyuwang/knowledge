# CONVI-6219: Fix directionality for time and churn outcome type

**Status:** Implemented — PR [#26332](https://github.com/cresta/go-servers/pull/26332) (draft)
**Branch:** `convi-6219-fix-directionality-for-time-and-churn-outcome-type`

## Ticket Summary

Coaching pages already handle inverse directionality for TIME and CHURN outcome types (lower = better). The same logic needs to be applied in:

1. **Performance Insights -> Top Agents** (agent tier partitioning for user outcome stats) — **Fixed in this PR**
2. **Quintile Rank** (e.g., lower AHT = quintile rank 1) — **Requires proto change, tracked separately**

## Root Cause

### Problem 1: Top Agents partitioning ignores directionality (FIXED)

`aggregateTopAgentsInternalUserOutcomeStats()` partitions agents into tiers (TOP/AVG/BOTTOM) using `PartitionUsingVolumeAndMetric()`. The partition function sorts ascending by metric value, then splits into 3 partitions:
- First partition (lowest values) -> `BOTTOM_AGENTS`
- Middle partition -> `AVG_AGENTS`
- Last partition (highest values) -> `TOP_AGENTS`

For TIME/CHURN outcomes, **lower values are better**, so agents with low AHT were incorrectly placed in `BOTTOM_AGENTS` instead of `TOP_AGENTS`.

### Problem 2: No quintile rank for user outcome stats (NOT IN SCOPE)

`UserOutcomeStatsGroupBy` proto does NOT have a `quintile_rank` field. Requires a proto change in `cresta-proto` — tracked separately.

## What Was Changed

### Files modified

| File | Change |
|------|--------|
| `insights-server/internal/analyticsimpl/retrieve_user_outcome_stats.go` | Added `isLowerBetterOutcomeTypeEnum()` helper using `enumspb.OutcomeTypeEnum`; extended `filterFieldDefinitionsByFrequency()` to return lower-is-better set; updated `aggregateTopAgentsInternalUserOutcomeStats()` to negate metric for lower-is-better fields |
| `insights-server/internal/analyticsimpl/retrieve_user_outcome_stats_query.go` | Added `LowerIsBetterFieldDefinitions set.Set[string]` to `UserOutcomeQuerySpec` |
| `insights-server/internal/analyticsimpl/retrieve_user_outcome_stats_transform_test.go` | Added 7 new tests: 5 for `isLowerBetterOutcomeTypeEnum`, 2 for `aggregateTopAgentsInternalUserOutcomeStats` (higher-is-better and lower-is-better) |

### How it works

1. `filterFieldDefinitionsByFrequency()` already fetches full `FieldDefinition` objects from the metadata framework. Extended it to also build a `set.Set[string]` of field definition names where `outcome_type` is TIME or CHURN.
2. The set is stored on `UserOutcomeQuerySpec.LowerIsBetterFieldDefinitions` and threaded through to `aggregateTopAgentsInternalUserOutcomeStats()`.
3. For each outcome field group, if the field is lower-is-better, the metric value is negated before partitioning. This flips the ascending sort so that agents with the lowest real values end up in `TOP_AGENTS`.

### Key design decisions

- Used `enumspb.OutcomeTypeEnum` (from `cresta-proto`) instead of `commonpb.OutcomeType` for consistency with the rest of insights-server.
- Added nil guard on `lowerIsBetterFieldDefinitions` interface before calling `.Has()` for defensive coding.
- `getValueMetric` closure is created inside the loop (per outcome field group) since `lowerIsBetter` varies per group.
- `conversationVolumeMetric` stays outside the loop since it doesn't depend on the field.

### Data flow

```
RetrieveUserOutcomeStats request
  -> filterFieldDefinitionsByFrequency()
     -> Fetches FieldDefinitions from metadata framework
     -> Each has: GetFieldMetadata().GetUserOutcomesFieldMetadataConfig().GetOutcomeType()
     -> Builds set of field_definition_names where outcome_type is TIME or CHURN
     -> Returns (filteredNames, lowerIsBetterSet, err)
  -> UserOutcomeQuerySpec.LowerIsBetterFieldDefinitions = lowerIsBetterSet
  -> getInternalAggregatedOutcomeStats(ctx, spec)
     -> aggregateTopAgentsInternalUserOutcomeStats(stats, mode, spec.LowerIsBetterFieldDefinitions)
        -> Per outcome field group: negate metric if lower-is-better
        -> PartitionUsingVolumeAndMetric() now correctly assigns tiers
```

## Tests

All 16 tests pass (7 new + 9 existing):
- `TestIsLowerBetterOutcomeTypeEnum`: TIME and CHURN return true; SALE, RETENTION, UNSPECIFIED return false
- `TestAggregateTopAgentsInternalUserOutcomeStats`: higher-is-better places high-value agents in TOP; lower-is-better places low-value agents in TOP

## Background: Field Definitions

Field definitions are metadata framework resources describing individual outcome metrics (e.g., "Average Handle Time", "Sale Rate"). Each has a `UserOutcomesFieldMetadataConfig` containing `outcome_type` (SALE, CHURN, TIME, etc.) which determines directionality. They belong to the `user_outcomes` resource type and are fetched via the metadata framework `GetResourceType()` RPC.
