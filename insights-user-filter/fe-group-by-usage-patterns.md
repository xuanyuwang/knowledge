# FE Analytics API GroupBy Usage Patterns

**Created:** 2026-02-26
**Updated:** 2026-02-26

## Summary

This document catalogs how the director FE codebase sets `groupByAttributeTypes` when calling Analytics APIs (RetrieveConversationStats, RetrieveAgentStats, RetrieveHintStats, RetrieveQAScoreStats, etc.). The goal is to identify which calls use NO agent grouping (summary/overview) vs. which calls use agent grouping (per-agent tables).

## Key Types

There are two parallel type systems for group-by:
- **`AttributeStructure[]`** — used by non-QA APIs (RetrieveConversationStats, RetrieveAgentStats, RetrieveHintStats, RetrieveAssistanceStats, etc.)
  - Values: `ATTRIBUTE_TYPE_AGENT`, `ATTRIBUTE_TYPE_GROUP`, `ATTRIBUTE_TYPE_TIME_RANGE`, `ATTRIBUTE_TYPE_POLICY`, etc.
- **`QAAttributeType[]`** — used by QA APIs (RetrieveQAScoreStats)
  - Values: `QA_ATTRIBUTE_TYPE_AGENT`, `QA_ATTRIBUTE_TYPE_GROUP`, `QA_ATTRIBUTE_TYPE_TIME_RANGE`, `QA_ATTRIBUTE_TYPE_CRITERION`, `QA_ATTRIBUTE_TYPE_AGENT_TIER`

## Common GroupBy Constants

Defined in `packages/director-app/src/components/insights/utils.tsx`:

```ts
// Per-agent AND per-team (used by Agent Leaderboard page)
export const ATTRIBUTE_STRUCTURE_AGENT_TEAM: AttributeStructure[] = [
  { attributeType: AttributeType.ATTRIBUTE_TYPE_AGENT },
  { attributeType: AttributeType.ATTRIBUTE_TYPE_GROUP },
];

// Time-range only (used by charts/trends)
export const ATTRIBUTE_STRUCTURE_TIME_RANGE: AttributeStructure[] = [
  { attributeType: AttributeType.ATTRIBUTE_TYPE_TIME_RANGE },
];
```

---

## 1. Leaderboard Pages

### 1a. Agent Leaderboard Page
**File:** `packages/director-app/src/features/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx`

**Non-QA APIs** — all use `ATTRIBUTE_STRUCTURE_AGENT_TEAM` = `[AGENT, GROUP]`:
| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useAgentStats(insightsRequestParams)` | `[AGENT, GROUP]` | Active days per agent |
| `useConversationStats(insightsRequestParams)` | `[AGENT, GROUP]` | Convo volume per agent + for summary cards |
| `useConversationStats(insightsRequestParamsWithAA)` | `[AGENT, GROUP]` | Convos powered by AA per agent |
| `useHintStats(groupByTeamParamsHintStats)` | `[AGENT, GROUP]` | Hint stats per agent |
| `useAssistanceStats(insightsRequestParams)` | `[AGENT, GROUP]` | Assistance used per agent |
| `useLiveAssistStats(insightsRequestParams)` | `[AGENT, GROUP]` | Live assist per agent |
| `useKnowledgeAssistStats(insightsRequestParams)` | `[AGENT, GROUP]` | Knowledge assist per agent |

**IMPORTANT:** The `Statistics` summary cards at the top (Convo Vol, Active agents, Active days, Performance, AHT, Assistance used, Engagement Rate, etc.) receive the **same data** as the agent table below. They use `.data?.averageConversationCountPerFrequencyPerUser`, `.data?.averageActiveAgentCountPerFrequency`, etc. -- these are **pre-aggregated fields in the response** that exist regardless of groupBy. The groupBy only affects the per-entity breakdown rows in `resultGroups`.

**QA API** — uses `[QA_ATTRIBUTE_TYPE_AGENT]`:
| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useGetQAStats(qaScoreFiltersState, QA_ATTRIBUTE_STRUCTURE)` | `[AGENT]` | QA score per agent (with `includePeerUserStats: true`) |

**Outcome Stats** (from `useOutcomeStatsData`):
| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(requestParams)` | `[CRITERION]` | Outcome overview (no agent) |
| `useQAScoreStats(requestParamsForAgent)` | `[AGENT, CRITERION]` | Outcomes per agent |
| `useQAScoreStats(requestParamsForAgentAndTimeRange)` | `[AGENT, TIME_RANGE, CRITERION]` | Outcomes per agent per time |

### 1b. Team Leaderboard Page
**File:** `packages/director-app/src/features/insights/leaderboard/team-leaderboard/TeamLeaderboardPage.tsx`

**Non-QA APIs** — all use `ATTRIBUTE_STRUCTURE_BY_GROUP` = `[GROUP]`:
| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useAgentStats(groupByTeamParamsForAgentStats)` | `[GROUP]` | Active agents/days per team |
| `useConversationStats(groupByTeamParams)` | `[GROUP]` | Convo volume per team |
| `useHintStats(groupByTeamParamsHintStats)` | `[GROUP]` | Hint stats per team |
| `useAssistanceStats(groupByTeamParams)` | `[GROUP]` | Assistance per team |
| `useKnowledgeAssistStats(groupByTeamParams)` | `[GROUP]` | Knowledge assist per team |
| `useLiveAssistStats(groupByTeamParamsForAgentStats)` | `[GROUP]` | Live assist per team |

**QA API** — uses `[QA_ATTRIBUTE_TYPE_GROUP]`:
| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useGetQAStats(qaScoreFiltersState, QA_ATTRIBUTE_STRUCTURE_GROUP)` | `[GROUP]` | QA score per team |

### 1c. Manager Leaderboard Page
**File:** `packages/director-app/src/features/insights/leaderboard/manager-leaderboard/ManagerLeaderboardPage.tsx`

**Non-QA APIs** — all use `ATTRIBUTE_STRUCTURE_BY_AGENT` = `[AGENT]`:
| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useManagerStats(managerStatsRequestParamsForAgentStats)` | `[AGENT]` | Manager stats per user |
| `useScorecardStats(managerStatsRequestParams)` | `[AGENT]` | Scorecard stats per manager |
| `useCoachingSessionStats(managerStatsRequestParams)` | `[AGENT]` | Coaching session per manager |
| `useCommentingStats(managerStatsRequestParams)` | `[AGENT]` | Comment stats per manager |
| `useLiveAssistStats(managerStatsRequestParams)` | `[AGENT]` | Live assist per manager |

---

## 2. Performance Page

**File:** `packages/director-app/src/components/insights/qa-insights/performance-conversations/PerformanceConversations.tsx`

This page uses the `usePerformanceFilters` hook and renders several sub-components. Crucially, **the Performance page has NO non-QA analytics API calls at its top level** -- all its data comes from QA APIs.

### 2a. Performance Score Chart (ScoreLineChartGraph)
**File:** `packages/director-app/src/components/insights/qa-insights/score-line-chart/ScoreLineChartGraph.tsx`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(requestParams)` | `[TIME_RANGE]` | Score over time (NO agent grouping) |
| `useQAScoreStats(requestParamsManual)` | `[TIME_RANGE]` | Manual scores over time |
| `useQAScoreStats(requestParamsAuto)` | `[TIME_RANGE]` | Auto scores over time |

### 2b. Performance Score Metric Card (ScoreInsightsMetric)
**File:** `packages/director-app/src/components/insights/qa-insights/score-line-chart/ScoreInsightsMetric.tsx`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(requestParams)` | `[TIME_RANGE]` | Current period score (NO agent grouping) |
| `useQAScoreStats(requestParamsDelta)` | `[TIME_RANGE]` | Previous period score for delta |
| `useQAScoreStats(requestParamsForAll)` | `[TIME_RANGE]` | All-agents score (no user filter, for single-agent view comparison) |

### 2c. Conversation Count Chart (ConversationCountChart)
**File:** `packages/director-app/src/components/insights/qa-insights/conversation-count-chart/ConversationCountChart.tsx`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(requestParams)` -- QA path | `QA: [TIME_RANGE]` | Scored convo count over time (NO agent grouping) |
| `useQAScoreStats(unfilteredRequestParams)` -- QA path | `QA: [TIME_RANGE]` | Unfiltered convo count (for ratio) |
| `useConversationStats(convCountRequestParams)` -- non-QA fallback | `[TIME_RANGE]` | Convo count when no scorecard template selected (NO agent grouping) |

### 2d. Performance Progression (Heatmap Table)
**File:** `packages/director-app/src/components/insights/qa-insights/performance-progression/PerformanceProgression.tsx`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(requestParams)` | `[CRITERION, TIME_RANGE]` | Per-criterion per-time scores (NO agent grouping) |
| `useQAScoreStats(requestParamsWholeTemplate)` | `[TIME_RANGE]` | All-criteria aggregate (NO agent grouping) |
| `useTopAgentsQAScoreStats(...)` | `[AGENT_TIER]` | Top/bottom agent scores (uses AGENT_TIER, not AGENT) |
| CSV export via `requestParamsByAgent` | `[AGENT, CRITERION, TIME_RANGE]` | Per-agent breakdown (only for CSV export, not displayed) |

### 2e. Leaderboard By Scorecard Template Item (per-agent/per-team table)
**File:** `packages/director-app/src/components/insights/qa-insights/leaderboard-by-scorecard-template-item/LeaderboardByScorecardTemplateItem.tsx`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(requestParams)` | `[AGENT or GROUP, CRITERION]` | Per-agent (or per-team) per-criterion scores |
| `useQAScoreStats(scorecardRequestParams)` | `[AGENT or GROUP]` | Per-agent (or per-team) overall scorecard score |

### 2f. Leaderboard Per Criterion (per-agent/per-team table with criterion dropdown)
**File:** `packages/director-app/src/components/insights/qa-insights/leaderboard-per-criterion/LeaderboardPerCriterion.tsx`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(requestParams)` | `[AGENT or GROUP, TIME_RANGE]` | Per-agent/team per-time for selected criterion |
| `useQAScoreStats(quintileRequestParams)` | `[AGENT]` | Per-agent quintile ranking for selected criterion |

### 2g. Outcome Stats (useStatsData)
**File:** `packages/director-app/src/components/insights/qa-insights/stats-graph-container/hooks/useStatsData.ts`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(requestParams)` | `[CRITERION, TIME_RANGE]` | Outcome scores over time (NO agent grouping) |
| `useQAScoreStats(deltaRequestParams)` | `[CRITERION, TIME_RANGE]` | Delta period outcome scores (NO agent grouping) |

---

## 3. Assistance Insights Page

**File:** `packages/director-app/src/components/insights/assistance/assistance-insights-container/AssistanceInsightsContainer.tsx`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useConversationStats(requestParams)` | **`[]` (empty)** | Total convo count for overview card (NO grouping at all) |
| `useConversationStats(deltaRequestParams)` | **`[]` (empty)** | Delta total convo count |
| `useConversationStats(requestParamsWithAA)` | **`[]` (empty)** | Total convos with AA |
| `useAssistanceStats(requestParamsByAgentTeam)` | `[AGENT, GROUP]` | Assistance per agent (for leaderboard table) |
| `useAssistanceStats(requestParamsByTeam)` | `[GROUP]` | Assistance per team |
| `useKnowledgeAssistStats(requestParams)` | **`[]` (empty)** | Total KA stats (overview) |
| `useKnowledgeAssistStats(requestParamsByTeam)` | `[GROUP]` | KA per team |
| `useKnowledgeAssistStats(requestParamsByAgentTeam)` | `[AGENT, GROUP]` | KA per agent |
| `useGetHintStatsByHintType(...)` | **`[]` (empty)** | Hint stats overview (NO grouping) |
| `useGetHintStatsByHintType(...)` | `[AGENT, POLICY]` | Hints by agent+policy |
| `useGetHintStatsByHintType(...)` | `[GROUP, POLICY]` | Hints by team+policy |

---

## 4. Coaching Hub

**File:** `packages/director-app/src/features/coaching-workflow/coaching-hub/criteria-goal-snapshot/hooks/useGoalPopoverQAStats.ts`

| API Call | GroupBy | Purpose |
|----------|---------|---------|
| `useQAScoreStats(qaScoreStatsParams)` | `QA: [AGENT]` | Per-agent score for coaching goal popover |

**File:** `packages/director-app/src/features/coaching-workflow/coaching-hub/recent-coaching-activities/RecentCoachingActivities.tsx`

Uses `useRequestForRetrieveQAStats` with various `groupBy` parameters passed from caller.

---

## Key Findings: NO-Agent-Grouping Cases

### Cases where `groupByAttributeTypes` = `[]` (empty / NO grouping):
1. **Assistance Insights overview cards**: `ConversationStats`, `KnowledgeAssistStats`, `HintStats` all called with `ATTRIBUTE_STRUCTURE = []` (empty array) to get **totals** for the overview/summary cards.

### Cases where `groupByAttributeTypes` = `[TIME_RANGE]` only (no agent dimension):
1. **Performance Score Line Chart** (`ScoreLineChartGraph`): `QA: [TIME_RANGE]` -- score trend over time
2. **Performance Score Metric Card** (`ScoreInsightsMetric`): `QA: [TIME_RANGE]` -- total average score
3. **Conversation Count Chart** (`ConversationCountChart`): `QA: [TIME_RANGE]` or non-QA `[TIME_RANGE]` -- convo volume over time
4. **Performance Progression heatmap rows**: `QA: [CRITERION, TIME_RANGE]` -- per-criterion trend
5. **Performance Progression "All criteria" row**: `QA: [TIME_RANGE]` -- all-criteria aggregate
6. **Outcome stats cards** (`useStatsData`): `QA: [CRITERION, TIME_RANGE]` -- outcome trends

### Cases where `groupByAttributeTypes` includes AGENT:
1. **Agent Leaderboard page**: all non-QA APIs use `[AGENT, GROUP]`, QA uses `[AGENT]`
2. **Manager Leaderboard page**: all non-QA APIs use `[AGENT]`
3. **Performance LeaderboardByScorecardTemplateItem**: QA `[AGENT, CRITERION]` or `[AGENT]`
4. **Performance LeaderboardPerCriterion**: QA `[AGENT, TIME_RANGE]` and `[AGENT]` for quintile
5. **Assistance Insights agent leaderboard tables**: `[AGENT, GROUP]`, `[AGENT, POLICY]`
6. **Coaching Hub goal popover**: QA `[AGENT]`

---

## Implications for `listAgentOnly` / `filterToAgentsOnly`

When `groupByAttributeTypes` does NOT include `ATTRIBUTE_TYPE_AGENT`:
- The API returns **aggregated totals** across all users matching the filter
- If managers/supervisors are included in the data, their conversations inflate the totals
- The `filterToAgentsOnly` field on the request filters at the query level (WHERE clause) to exclude non-agent users

Currently `filterToAgentsOnly` is passed in these cases:
- **Agent Leaderboard**: `QA_STATS_INCLUDE_PEER_USER_STATS_OPTIONS = { includePeerUserStats: true, filterToAgentsOnly: true }`
- **Team Leaderboard**: `FILTER_TO_AGENTS_ONLY_OPTIONS = { filterToAgentsOnly: true }`

It is NOT currently passed for:
- Performance page overview cards (ScoreInsightsMetric, ConversationCountChart, PerformanceProgression)
- Assistance Insights overview cards (the `[]` empty groupBy calls)
- Leaderboard Statistics summary cards (they reuse the same request as the agent table)

The Leaderboard Statistics cards (`Statistics.tsx`) consume `conversationStats.data?.averageConversationCountPerFrequencyPerUser`, `agentStats.data?.averageActiveAgentCountPerFrequency` etc. Since these responses come from the SAME request as the agent-level breakdown (which uses `[AGENT, GROUP]`), the aggregate fields are calculated from the per-agent results and already reflect agent-level data. However, they DO include manager users unless `filterToAgentsOnly` is set.
