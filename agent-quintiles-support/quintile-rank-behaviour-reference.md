# Quintile Rank — Behaviour Reference

**Created:** 2026-03-10
**Updated:** 2026-03-11

## What is Quintile Rank?

Agents are ranked by their QA score and divided into 5 equal groups (quintiles). Q1 = top 20% (best performers), Q5 = bottom 20%. The rank appears as a number (1-5) and/or a trophy icon (gold for Q1, silver for Q2, bronze for Q3) on various pages.

Quintile rank is **relative** — it depends on which agents, templates, and time range are included in the calculation. Different pages may show different quintiles for the same agent if they use different filters.

---

## Behaviour Summary

### Where quintile rank appears

| Page | Table/Location | What's shown | Quintile scope |
|------|---------------|-------------|----------------|
| **Leaderboard** | Agent Leaderboard | Column (1-5) + trophy icon on name | Overall — all agents in page filters |
| **Leaderboard** | Agent Leaderboard by Metric | Trophy icon on name | Same as above (shared data) |
| **Performance** | Leaderboard by Criteria (2nd table) | Column (1-5) + trophy icon on name | Overall — all agents in page filters |
| **Performance** | Leaderboard per Criteria (3rd table) | Column (1-5) + trophy icon on name | **Per-criterion** — agents ranked within selected criterion only |
| **Coaching Hub** | Recent Coaching Activities | Trophy icon on name + tooltip | Overall — last 7 days, all agents (or filtered) |
| **Coaching Plan** | Header badge | Trophy icon + "Xth quintile" text | Overall — last 7 days, all agents |

### Cross-page comparison

| | Leaderboard | Perf: by Criteria | Perf: per Criterion | Coaching Hub (default) | Coaching Plan |
|---|---|---|---|---|---|
| **Templates** | Page filters | Page filters | Page filters | All | All |
| **Agents** | Page filters | Page filters | Page filters | All | All |
| **Time range** | Page date picker | Page date picker | Page date picker | Last 7 days | Last 7 days |
| **Score type** | Criteria Adherence | Criteria Adherence | Criteria Adherence | Criteria Adherence | Criteria Adherence |
| **Ranking scope** | Overall | Overall | Per selected criterion | Overall | Overall |

### When will quintile ranks match across pages?

**Coaching Hub (default filters) = Coaching Plan**: After CONVI-6389 alignment, these two produce the same quintile when Coaching Hub has no filters applied. Both rank all agents over last 7 days using criteria adherence scoring.

**Leaderboard = Performance (by Criteria)**: These match when the same filters and date range are used.

**Leaderboard/Performance vs Coaching pages**: Will match only if the date range is set to last 7 days and no template/agent filters are applied.

**Performance (per Criterion)**: Intentionally different — ranks agents within a single selected criterion, not overall. Will almost never match other pages.

### How outcomes are handled

Scorecard template items have an `excludeFromQAScores` flag:

- **Excluded** (e.g., AHT, Conversion): Do not contribute to the QA score used for quintile ranking
- **Included** (e.g., CSAT mapped to 0/1, behavioral criteria): Contribute to the weighted average score

This means the quintile is based only on scoreable criteria, not on raw outcome metrics. See `outcome-quintile-investigation.md` for detailed analysis with real data examples.

---

## Technical Details

### How the QA score is computed

The score used for quintile ranking is: `SUM(percentage_value * float_weight) / SUM(float_weight)` from the ClickHouse `score_d` table (when `scoreResource = SCORECARD_SCORE`).

The `getScoreableCriteria()` function in the BE filters out items with `excludeFromQAScores: true` by injecting a `WHERE criterion_id IN (...)` clause. This filtering only applies when:
- `criterionIdentifiers` is empty in the request, AND
- The request does not group by `CRITERION`

If the FE explicitly passes criterion IDs, the filtering is bypassed.

### `scoreResource` values

| Value | ClickHouse table | Description |
|-------|-----------------|-------------|
| `UNSPECIFIED` (0) | `score_d` | Default — same as SCORECARD_SCORE for query purposes |
| `SCORECARD_SCORE` (1) | `score_d` | Per-criterion weighted average ("Criteria Adherence") |
| `SCORECARD` (2) | `scorecard_d` | Overall scorecard score (includes autofail) |

Default `filtersState.scoreResource` is `SCORECARD_SCORE`, hardcoded in `getInitialFiltersState()` (`usePerformanceFilters.tsx`).

### Per-page code reference

<details>
<summary><strong>1. Agent Leaderboard</strong></summary>

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
| **scoreResource** | From `qaScoreFiltersState.scoreResource` (default: `SCORECARD_SCORE`) |
| **includePeerUserStats** | `true` |
| **enableAutofailScoring** | `true` |
| **Separate quintile call?** | **No** — quintile extracted directly from the main AGENT-only response |

**Quintile extraction** (lines 133-146): Builds `agentToQuintileRank` map from `currentScore.data?.qaScoreResult.scores`, keyed by `score.groupedBy?.user?.name`. Passed to both Agent Leaderboard table (column) and Agent Leaderboard by Metric table (icons).

</details>

<details>
<summary><strong>2. Performance — Leaderboard by Criteria (2nd table)</strong></summary>

**File:** `components/insights/qa-insights/leaderboard-by-scorecard-template-item/LeaderboardByScorecardTemplateItem.tsx` (lines 61-72)

Makes **two** API calls:

**Call 1 — Per-criterion data (columns):**
```typescript
const requestParams = useQAScoreStatsRequestParams(filtersState, [AGENT|GROUP, CRITERION], {
  excludeNonScorable: false,
  additionalFilterByAttribute: additionalFilters,
});
```
- `groupBy: [AGENT, CRITERION]` — produces per-agent-per-criterion scores
- Quintile on this response is ranked across ALL agent x criterion rows (meaningless) — **not used**

**Call 2 — Overall score for quintile (dedicated AGENT-only call):**
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
| **scoreResource** | From `filtersState.scoreResource` (default: `SCORECARD_SCORE`) |
| **enableAutofailScoring** | `true` |
| **Separate quintile call?** | **Yes** — Call 2 is dedicated for quintile |

**Quintile extraction** (`utils.ts` lines 171-193): Builds `quintileRankByRowId` map from Call 2's response, keyed by `score.groupedBy.user?.name`.

</details>

<details>
<summary><strong>3. Performance — Leaderboard per Criteria (3rd table)</strong></summary>

**File:** `components/insights/qa-insights/leaderboard-per-criterion/LeaderboardPerCriterion.tsx` (lines 170-180)

Makes **two** API calls:

**Call 1 — Daily breakdown for the selected criterion:**
```typescript
const requestParams = useQAScoreStatsRequestParams(filtersState, [AGENT|GROUP, TIME_RANGE], {
  additionalFilterByAttribute: additionalFilters,  // includes criterionIdentifiers: [selectedCriterion]
});
```

**Call 2 — Quintile only (AGENT-only, same criterion):**
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
| **enableAutofailScoring** | `false` |
| **Separate quintile call?** | **Yes** |
| **Skipped when** | No criterion selected, feature flag off, or team view |

**Note:** Since `criterionIdentifiers` is explicitly set, the BE's `getScoreableCriteria()` filtering is **bypassed**. In practice, the dropdown only shows scoreable criteria.

</details>

<details>
<summary><strong>4. Coaching Hub — Recent Coaching Activities</strong></summary>

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
| **Time range** | Last 7 days (hardcoded) |
| **criterionIdentifiers** | `[]` |
| **scoreResource** | Explicit `SCORECARD_SCORE` |
| **groupMembershipFilter** | Respects `directTeamMembership` setting |
| **includePeerUserStats** | `true` |

</details>

<details>
<summary><strong>5. Coaching Plan — Header Badge (aligned in CONVI-6389)</strong></summary>

**File:** `features/coaching-workflow/agent-coaching/agent-coaching-plan/agent-coaching-plan-header/QuintileRankBadge.tsx`

**PR:** [#17263](https://github.com/cresta/director/pull/17263)

```typescript
useQAScoreStats({
  filterByAttribute: {
    criterionIdentifiers: [],
    groupMembershipFilter: {
      type: GroupMembershipFilterGroupMembershipType.ALL_GROUP_MEMBERSHIPS,
    },
  },
  groupByAttributeTypes: [QAAttributeType.QA_ATTRIBUTE_TYPE_AGENT],
  filterByTimeRange: toTimeRangeIncludingDays(startDate, endDate),
  includePeerUserStats: true,
  scoreResource: QA_SCORE_RESOURCE_SCORECARD_SCORE,
}, { skip: !enableQuintileRank });
```

| Parameter | Value |
|-----------|-------|
| **groupBy** | `[AGENT]` |
| **Templates** | `undefined` (all templates) |
| **Users** | No filter (all agents) |
| **Time range** | Last 7 days (hardcoded) |
| **criterionIdentifiers** | `[]` |
| **scoreResource** | Explicit `SCORECARD_SCORE` |
| **groupMembershipFilter** | `ALL_GROUP_MEMBERSHIPS` |
| **includePeerUserStats** | `true` |

**Change summary (CONVI-6389):** Removed explicit `scorecardTemplateNames` prop. Added `scoreResource`, `includePeerUserStats`, `criterionIdentifiers`, `groupMembershipFilter` to match Coaching Hub defaults.

</details>

### Outcome filtering details

See `outcome-quintile-investigation.md` for full analysis. Key points:

- `getScoreableCriteria()` (`retrieve_qa_score_stats.go` lines 700-720) parses template JSON, collects IDs where `IsExcludeFromQAScores() == false`
- Injected as `req.FilterByAttribute.CriterionIdentifiers` (line 211) → becomes `WHERE criterion_id IN (...)` on ClickHouse
- `percentage_value` is NOT always 0-1: AHT stores raw seconds (59-2998), but is excluded by the flag
- Bypass conditions: explicit `criterionIdentifiers` in request, or grouping by `CRITERION`
