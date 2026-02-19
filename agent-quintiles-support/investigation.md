# Investigation: Agent Quintiles

**Date:** 2026-02-17

## 1. How to divide agents into quintiles in RetrieveQAScoreStats (BE)

### API and files

- **API**: `AnalyticsService.RetrieveQAScoreStats` (cresta-proto), implemented in **go-servers/insights-server**.
- **Main file**: `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`.
- **ClickHouse path**: `retrieve_qa_score_stats_clickhouse.go` (used for some request shapes).

### Current tier implementation

- **Tiers**: `BOTTOM_AGENTS`, `AVERAGE_AGENTS`, `TOP_AGENTS` (see `agentTiers` slice and `createTieredScoreObject`).
- **Partition**: `utils.PartitionUsingVolumeAndMetric(scoreSlice, scoreMetric, conversationVolumeMetric, []float32{bottomAgentPercentage, bottomAgentPercentage + AvgAgentPercentage})`.
  - Constants (from outcome_stats): `topAgentPercentage=0.25`, `AvgAgentPercentage=0.5`, `bottomAgentPercentage=0.25` → cutoffs **0.25, 0.75** → 3 partitions.
- **Algorithm** (`go-servers/shared/utils/partition.go`):
  - Sort by metric (score) ascending, then by volume.
  - Cumulative volume; at each cutoff (e.g. 0.25 * totalVolume) find index → split into N+1 partitions (N = len(cutoffs)).
  - Partitions are non-overlapping and ordered by score.

### Quintile design (BE)

- **Score bands (0–100)**: Q1 = 80+, Q2 = 60–79, Q3 = 40–59, Q4 = 20–39, Q5 = 0–19. Backend score is 0–1; use same thresholds (e.g. ≥0.8 → 1).
- **Where to attach quintile**: When the response is **grouped by AGENT** (per-agent scores), we have a list of `QAScore` per (time_range, team, group, criterion). For each such group:
  1. Get the slice of scores (one per agent).
  2. Call `PartitionUsingVolumeAndMetric(scores, scoreMetric, volumeMetric, []float32{0.2, 0.4, 0.6, 0.8})` → 5 partitions.
  3. For each agent score, set `GroupedBy.QuintileRank` = 1..5 according to which partition it falls into (1 = first partition = lowest scores, 5 = highest).
- **Proto**: Add `int32 quintile_rank = 7` to `QAScoreGroupBy` (OUTPUT_ONLY). 1 = best (80+), 5 = lowest (≤19).
- **Code paths**: Same aggregation path that today builds per-agent scores (before any tier aggregation) should compute quintiles and set the new field. The tier aggregation path (e.g. `aggregateTopAgentsResponse`) is for when we group *by* tier; for per-agent we don’t aggregate by tier, we just attach quintile to each agent row.

**Concrete insertion points:** See `implementation-plan.md` (helper `setQuintileRankForPerAgentScores`, call before appendGroupMemberships; ClickHouse: set in convertCHResponseToQaScoreStatsResponse loop).

### Note on RetrieveManualQAStats

- There is also `RetrieveManualQAStats` (manual QA stats). The ask was specifically for "RetrieveQAStats" → interpreted as **RetrieveQAScoreStats** (QA score stats). Manual QA stats are a different surface; quintiles can be scoped to RetrieveQAScoreStats only unless product asks to extend.

---

## 2. How to add Quintile Rank column on Performance page tables

### Performance page structure

- **Route**: Insights → Performance (`/insights/performance`).
- **Component**: `director-app/src/components/insights/qa-insights/performance/Performance.tsx`.
- **Tables with agent data**:
  1. **PerformanceProgression** – `performance-progression/PerformanceProgression.tsx` – uses `useColumnsForPerformanceProgression`, rows from `createPerformanceProgressionRows` / agent-based data. Check if it has an "agents" view; if so, add column there.
  2. **LeaderboardByScorecardTemplateItem** – Agent tab: data from `useLeaderboardByScorecardTemplateData` → `createAllRows(qaScoreStats, ..., 'agent-table')`; columns from `useColumnsFromScorecardTemplate`. This is the main "leaderboard by scorecard" table with Agent/Team tabs.
  3. **LeaderboardPerCriterion** – per-criterion tables; may also show agents.

### Data shape

- Rows are built from `qaScoreStats.data?.qaScoreResult.scores` (and scorecard-level stats). Each score has `groupedBy: { user, timeRange, team, group, criterionId, agentTier? }`. After BE change: `groupedBy.quintileRank` (or similar).
- **Column addition**: Add a column definition (e.g. in `useColumnsFromScorecardTemplate` or shared column factory) that reads `row.original.groupedBy?.quintileRank` (or from the score object used for that row) and displays "Quintile Rank" or "Q1"–"Q5". Place after agent name or in a consistent position.

### Files to touch (director)

- `components/insights/qa-insights/leaderboard-by-scorecard-template-item/hooks/useColumnsFromScorecardTemplate.tsx` (or equivalent column hook).
- `components/insights/qa-insights/performance-progression/useColumnsForPerformanceProgression.tsx` if Performance Progression has an agent table.
- `components/insights/qa-insights/leaderboard-per-criterion/` if it has agent columns.
- Types: ensure row/score types include `quintileRank` from API types (director-api / web-client generated from proto).

---

## 3. Leaderboard and Coaching Hub – quintile info and colored icons

### Leaderboard (Insights)

- **Agent Leaderboard**: `features/insights/leaderboard/agent-leaderboard/AgentLeaderboard.tsx`, `AgentLeaderboardPage.tsx`.
- **Data**: `agentStats` (useAgentStats – different API) + `score` from `useGetQAStats` (RetrieveQAScoreStats). Rows built from `agentStats.data?.resultsGroupedByAttributeTypes` and `score.data?.qaScoreResult.scores`; matching by user (e.g. `getCorrectRowFromLeaderboardQAGroupBy`). So each row can get the corresponding `QAScore` and thus `groupedBy.quintileRank` once BE adds it.
- **Display**: Add a small icon (e.g. colored circle or badge) per row indicating quintile (1–5). Use a shared component that maps quintile → color (e.g. red=1, orange=2, yellow=3, light green=4, green=5). Place next to agent name or in a dedicated column.

### Leaderboard by metric (agents)

- `leaderboard-by-metric/agent-leaderboard-by-metric/` – `useLeaderboardMetricDataForAgents` builds rows from `agentStats` and `currentScore.data?.qaScoreResult.scores`. Same approach: attach quintile from score’s `groupedBy.quintileRank` to row, render same icon.

### Coaching Hub

- **CoachingHub**: `features/coaching-workflow/coaching-hub/CoachingHub.tsx` – tabs Recent Activities, Progress. **CriteriaGoalSnapshot** uses `useTargetQAStats` → `qaScoreResult.scores` keyed by criterion (aggregate per criterion, not per agent in the snapshot). **RecentCoachingActivities** and other widgets may list agents.
- To show quintile icons for agents on Coaching Hub we need per-agent QA score (with quintile) for the same filters. Options:
  - Call RetrieveQAScoreStats grouped by AGENT (with same filters as Coaching Hub) and cache; then for each displayed agent, look up quintile from that response.
  - Or add a small “agent quintile” API used by Coaching Hub. Prefer reusing RetrieveQAScoreStats (group by AGENT) and passing quintile into the agent list UI.
- **Icon**: Reuse the same QuintileRankIcon component (color by 1–5).

### Shared component

- Create e.g. `QuintileRankIcon` or `AgentQuintileBadge`: props `quintileRank: 1|2|3|4|5`, optional size; render a colored dot/circle. Define a 5-step color palette (accessible and distinct). Use on Performance (optional), Leaderboard agent tables, and Coaching Hub agent lists.

---

## Worktrees

- For BE work: create worktree for go-servers (e.g. `git worktree add ../go-servers-quintiles feature/quintiles`).
- For FE work: create worktree for director (e.g. `git worktree add ../director-quintiles feature/quintiles`).
