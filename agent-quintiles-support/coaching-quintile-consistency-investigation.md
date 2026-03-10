# Quintile Rank — Per-Page Request Parameters Reference

**Created:** 2026-03-10

## Overview

Each page that displays quintile rank makes its own `RetrieveQAScoreStats` call. This document is a comprehensive reference for what parameters each page uses, how outcomes are handled, and whether quintile ranks are consistent across pages.

## How Outcomes Are Handled

Quintile ranking uses the `QAScore.score` field, which is a weighted average: `SUM(percentage_value * float_weight) / SUM(float_weight)`. Which criteria/outcomes contribute depends on the `excludeFromQAScores` flag in the scorecard template (see `outcome-quintile-investigation.md` for full details).

- Items with `excludeFromQAScores: true` (e.g., AHT, Conversion) are **filtered out** by `getScoreableCriteria()` at query time
- Items with `excludeFromQAScores: false` or `null` (e.g., CSAT, behavioral criteria) **participate** in the weighted average
- This filtering only applies when `criterionIdentifiers` is empty AND the request does not group by `CRITERION` — if the FE explicitly passes criterion IDs, the filtering is bypassed
- `percentage_value` is NOT always 0-1 (e.g., AHT stores raw seconds), but excluded items don't enter the aggregation

---

## 1. Agent Leaderboard Page

**File:** `features/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx` (lines 115-119)

```typescript
const { currentScore } = useGetQAStats(
  qaScoreFiltersState,
  [QAAttributeType.QA_ATTRIBUTE_TYPE_AGENT],
  { enableAutofailScoring: true, includePeerUserStats: true }
);
```

| Parameter | Value |
|-----------|-------|
| **groupBy** | `[AGENT]` |
| **Templates** | From page filters (`qaScoreFiltersState`) |
| **Users** | From page filters |
| **Time range** | From page date picker |
| **criterionIdentifiers** | `[]` (cleared when `enableAutofailScoring: true`) |
| **scoreResource** | From `qaScoreFiltersState.scoreResource` |
| **includePeerUserStats** | `true` |
| **Separate quintile call?** | **No** — quintile extracted directly from the main AGENT-only response |

**Quintile extraction** (lines 133-146): Builds `agentToQuintileRank` map from `currentScore.data?.qaScoreResult.scores`, keyed by `score.groupedBy?.user?.name`. Passed to both Agent Leaderboard table (column) and Agent Leaderboard by Metric table (icons).

---

## 2. Performance — Leaderboard by Criteria (2nd table)

**File:** `components/insights/qa-insights/leaderboard-by-scorecard-template-item/LeaderboardByScorecardTemplateItem.tsx` (lines 61-72)

Makes **two** API calls:

### Call 1 — Per-criterion data (columns)
```typescript
const requestParams = useQAScoreStatsRequestParams(filtersState, [AGENT|GROUP, CRITERION], {
  excludeNonScorable: false,
  additionalFilterByAttribute: additionalFilters,
});
```
- `groupBy: [AGENT, CRITERION]` — produces per-agent-per-criterion scores
- Quintile on this response is ranked across ALL agent×criterion rows (meaningless) — **not used**

### Call 2 — Overall score for quintile (dedicated AGENT-only call)
```typescript
const scorecardRequestParams = useQAScoreStatsRequestParams(filtersState, [AGENT|GROUP], {
  enableAutofailScoring: true,
  excludeNonScorable: true,
  additionalFilterByAttribute: additionalFilters,
});
```

| Parameter | Value |
|-----------|-------|
| **groupBy** | `[AGENT]` (no CRITERION) |
| **Templates** | From page filters (`filtersState`) |
| **Users** | From page filters |
| **Time range** | From page date picker |
| **criterionIdentifiers** | `[]` (cleared when `enableAutofailScoring: true`) |
| **scoreResource** | From `filtersState.scoreResource` |
| **Separate quintile call?** | **Yes** — Call 2 is dedicated for quintile |

**Quintile extraction** (`utils.ts` lines 171-193): Builds `quintileRankByRowId` map from Call 2's response, keyed by `score.groupedBy.user?.name`. Applied to each row via `row.quintileRank = quintileRankByRowId.get(key)`.

---

## 3. Performance — Leaderboard per Criteria (3rd table)

**File:** `components/insights/qa-insights/leaderboard-per-criterion/LeaderboardPerCriterion.tsx` (lines 170-180)

Makes **two** API calls:

### Call 1 — Daily breakdown for the selected criterion
```typescript
const requestParams = useQAScoreStatsRequestParams(filtersState, [AGENT|GROUP, TIME_RANGE], {
  additionalFilterByAttribute: additionalFilters,  // includes criterionIdentifiers: [selectedCriterion]
});
```
- `groupBy: [AGENT, TIME_RANGE]` — per-agent daily scores for the selected criterion

### Call 2 — Quintile only (AGENT-only, same criterion)
```typescript
const quintileRequestParams = useQAScoreStatsRequestParams(filtersState, [AGENT], {
  additionalFilterByAttribute: additionalFilters,  // includes criterionIdentifiers: [selectedCriterion]
});
```

| Parameter | Value |
|-----------|-------|
| **groupBy** | `[AGENT]` (no TIME_RANGE) |
| **Templates** | From page filters (`filtersState`) |
| **Users** | From page filters |
| **Time range** | From page date picker |
| **criterionIdentifiers** | `[selectedCriterion.value]` — single criterion from dropdown |
| **scoreResource** | Not explicitly set (API default) |
| **Separate quintile call?** | **Yes** — Call 2 is dedicated for quintile |
| **Skipped when** | No criterion selected, feature flag off, or team view |

**Quintile extraction** (lines 181-192): Builds `quintileRankByResourceName` map from Call 2's response. This is a **per-criterion quintile** — agents are ranked only by their score on the selected criterion, because `criterionIdentifiers` filters the data before aggregation.

**Note:** Since `criterionIdentifiers` is explicitly set, the BE's `getScoreableCriteria()` filtering is **bypassed** (see `outcome-quintile-investigation.md` section 7). If the dropdown includes an outcome with `excludeFromQAScores: true`, its raw values would be used. In practice, the dropdown only shows scoreable criteria.

---

## 4. Coaching Hub — Recent Coaching Activities

**File:** `features/coaching-workflow/coaching-hub/recent-coaching-activities/RecentCoachingActivities.tsx` (lines 307-332)

```typescript
useQAScoreStats({
  filterByTimeRange: toTimeRangeIncludingDays(last7Days.startDate, last7Days.endDate),
  groupByAttributeTypes: [QAAttributeType.QA_ATTRIBUTE_TYPE_AGENT],
  frequency: Frequency.DAILY,
  includePeerUserStats: true,
  scoreResource: QA_SCORE_RESOURCE_SCORECARD_SCORE,
  filterByAttribute: {
    ...quintileRankFilterByAttribute,
    scorecardTemplates: hasTemplateFilter ? filters.scorecardTemplateNames : undefined,
    criterionIdentifiers: [],
    groupMembershipFilter: { type: filters.directTeamMembership ? DIRECT_ONLY : ALL },
  },
}, { skip: !enableQuintileRank });
```

| Parameter | Value |
|-----------|-------|
| **groupBy** | `[AGENT]` |
| **Templates** | User-selected from page filters, or `undefined` (all) if no filter |
| **Users** | User-selected from page filters, or empty arrays (all) if no filter |
| **Time range** | **Last 7 days** (hardcoded) |
| **criterionIdentifiers** | `[]` (empty — triggers `getScoreableCriteria()` filtering) |
| **scoreResource** | Explicit `QA_SCORE_RESOURCE_SCORECARD_SCORE` |
| **groupMembershipFilter** | Respects `directTeamMembership` setting |
| **includePeerUserStats** | `true` |
| **Separate quintile call?** | **Yes** — dedicated call for quintile only |

---

## 5. Coaching Plan — Header Badge

**File:** `features/coaching-workflow/agent-coaching/agent-coaching-plan/agent-coaching-plan-header/QuintileRankBadge.tsx` (lines 31-42)

```typescript
useQAScoreStats({
  filterByAttribute: {
    scorecardTemplates: scorecardTemplateNames,
  },
  groupByAttributeTypes: [QAAttributeType.QA_ATTRIBUTE_TYPE_AGENT],
  filterByTimeRange: toTimeRangeIncludingDays(startDate, endDate),
}, { skip: !enableQuintileRank || !scorecardTemplateNames.length });
```

`scorecardTemplateNames` from `useCurrentScorecardTemplates()` in `AgentCoachingPlan.tsx` (line 138-140) — all current templates (ACTIVE + INACTIVE).

| Parameter | Value |
|-----------|-------|
| **groupBy** | `[AGENT]` |
| **Templates** | All current templates from `useCurrentScorecardTemplates()` |
| **Users** | **No filter** — ranks against all agents globally |
| **Time range** | **Last 7 days** (hardcoded) |
| **criterionIdentifiers** | Not specified (empty — triggers `getScoreableCriteria()` filtering) |
| **scoreResource** | Not specified (API default) |
| **groupMembershipFilter** | Not specified |
| **includePeerUserStats** | Not specified |
| **Separate quintile call?** | **Yes** — dedicated call |

---

## Cross-Page Comparison

| Parameter | Leaderboard | Perf: by Criteria | Perf: per Criterion | Coaching Hub | Coaching Plan |
|-----------|-------------|-------------------|---------------------|--------------|---------------|
| **groupBy** | `[AGENT]` | `[AGENT]` | `[AGENT]` | `[AGENT]` | `[AGENT]` |
| **Templates** | Page filters | Page filters | Page filters | Page filters or all | All current |
| **Users** | Page filters | Page filters | Page filters | Page filters or all | **None** |
| **Time range** | Page picker | Page picker | Page picker | Last 7 days | Last 7 days |
| **criterionIdentifiers** | `[]` | `[]` | `[selected]` | `[]` | Not specified |
| **scoreResource** | From filters | From filters | API default | Explicit SCORECARD_SCORE | API default |
| **enableAutofailScoring** | `true` | `true` | `false` | N/A | N/A |
| **groupMembershipFilter** | From filters | From filters | From filters | `directTeamMembership` | Not specified |
| **Separate call?** | No | Yes | Yes | Yes | Yes |

## Will Quintile Ranks Match Across Pages?

**Generally no.** Key reasons:

1. **Different user populations** — Coaching Plan has no user filter (all agents globally); Coaching Hub may filter by selected users/teams; Leaderboard/Performance use their own page filters
2. **Different time ranges** — Coaching Hub/Plan hardcode last 7 days; Leaderboard/Performance use the page date picker
3. **Different template sets** — Coaching Plan uses all current templates; other pages use page-selected templates
4. **Different scoreResource** — Coaching Hub explicitly sets `QA_SCORE_RESOURCE_SCORECARD_SCORE`; others vary
5. **Per-criterion vs overall** — Performance per-criterion table ranks agents within a single criterion; all other pages use overall scores

**Most likely to match:** Leaderboard and Performance (by criteria) when the same user uses the same filters and date range — both use page filters consistently.

**Most likely to differ:** Coaching Plan vs everything else — it has no user filter, uses all templates, and uses API defaults for most parameters.
