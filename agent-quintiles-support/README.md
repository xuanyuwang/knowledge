# Agent Quintiles Support

**Created:** 2026-02-17
**Updated:** 2026-02-19

## Overview

Support dividing agents into **quintiles** (5 buckets) in addition to the existing **tiers** (3 buckets: top / average / bottom). The current API `RetrieveQAScoreStats` supports grouping by agent tier; we want to add quintiles and expose them on:

- **Performance page**: New "Quintile Rank" column in tables that have an Agents tab.
- **Leaderboard page**: Colored icons per quintile for agents.
- **Coaching Hub page**: Colored icons per quintile for agents.

Medium-sized project across BE (go-servers) and FE (director). Use worktrees when working in go-servers or director.

## Key Findings

### 1. Backend: RetrieveQAScoreStats and tiers

- **API**: `RetrieveQAScoreStats` (not "RetrieveQAStats") in **insights-server**.
- **Relevant files**:
  - `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go` – main handler, tier aggregation.
  - `go-servers/shared/utils/partition.go` – `PartitionUsingVolumeAndMetric(slice, metric, volume, cutoffs)`.
- **Current tier logic**:
  - Three tiers: `BOTTOM_AGENTS`, `AVERAGE_AGENTS`, `TOP_AGENTS` (see `agentTiers` in `retrieve_qa_score_stats.go`).
  - Partition uses **volume-weighted cutoffs**: sort by score (ascending), then split by cumulative **conversation/scorecard volume** at cutoffs.
  - Cutoffs for 3 tiers: `[0.25, 0.75]` (i.e. `bottomAgentPercentage` and `bottomAgentPercentage + AvgAgentPercentage` from outcome_stats – 0.25, 0.5).
  - Code: `utils.PartitionUsingVolumeAndMetric(scoreSlice, scoreMetric, conversationVolumeMetric, []float32{0.25, 0.75})` → 3 partitions, then `createTieredScoreObject(partition, groupedBy, agentTiers[i])`.
- **Proto**: `cresta-proto/cresta/v1/analytics/qa_stats.proto` – `QAScoreGroupBy` has `AgentTier agent_tier = 6`. `outcome_stats.proto` defines `enum AgentTier { TOP_AGENTS=1, AVERAGE_AGENTS=2, BOTTOM_AGENTS=3 }`.
- **Quintile approach (BE)** – **score bands (not volume percentiles):** Quintiles are defined by score ranges (0–100): Q1 = 80+, Q2 = 60–79, Q3 = 40–59, Q4 = 20–39, Q5 = 0–19. Backend stores score as 0–1; add `quintile_rank` (int32 1–5) to `QAScoreGroupBy` and set it via `ScoreToQuintileRank(score)` when returning per-agent scores. See `implementation-plan.md`.  
  - Add optional `quintile_rank` (int32 1–5) to `QAScoreGroupBy`; when response is grouped by AGENT, set it per score using `ScoreToQuintileRank(score)`.
  _(Supersedes previous partition-based options.)_
  1. **Option A – Per-agent quintile in response** (deprecated): When the response is grouped by AGENT, compute quintile per (time_range, team, group, criterion) using the same partition logic with cutoffs `[0.2, 0.4, 0.6, 0.8]`, and attach a new field (e.g. `agent_quintile` or `quintile_rank` 1–5) to each `QAScore`/`QAScoreGroupBy`. Requires proto change (new field) and BE to populate it when returning per-agent scores.
  2. **Option B – New group-by type**: Add `QA_ATTRIBUTE_TYPE_AGENT_QUINTILE` and return aggregated stats per quintile (similar to agent tier). FE would still need per-agent quintile for tables; could call both “by agent” and “by quintile” and join, or we still need per-agent quintile somewhere.
  **Recommendation**: Option A – add optional `quintile_rank` (int32 1–5) or `agent_quintile` enum to `QAScoreGroupBy`, compute in BE when building per-agent results using `PartitionUsingVolumeAndMetric` with 4 cutoffs, assign quintile 1 = lowest score, 5 = highest (or reverse by product preference).

### 2. Frontend: Performance page tables with Agents tab

- **Performance page**: `director-app/src/components/insights/qa-insights/performance/Performance.tsx`. Renders:
  - `PerformanceProgression` – table with agents (and criteria/time).
  - `LeaderboardByScorecardTemplateItem` – Agent / Team tabs; agent table from `useLeaderboardByScorecardTemplateData` → `createAllRows(qaScoreStats, ...)`.
  - `LeaderboardPerCriterion` – per-criterion leaderboards.
- **Data source**: `useQAScoreStats` → `RetrieveQAScoreStats` with `groupBy` including `QAAttributeType.QA_ATTRIBUTE_TYPE_AGENT`. Rows come from `qaScoreResult.scores`; each score has `groupedBy.user` and score values but **no tier/quintile on per-agent rows today** (tier only appears when grouping by agent_tier).
- **Adding Quintile Rank column**: Once BE returns `quintile_rank` (or equivalent) on each per-agent score in `groupedBy`, add a column to:
  - Performance Progression agent view (if it shows per-agent rows),
  - `LeaderboardByScorecardTemplateItem` agent table (columns from `useColumnsFromScorecardTemplate` / `useLeaderboardByScorecardTemplateData`),
  - `LeaderboardPerCriterion` agent tables.
  Column definition: display `groupedBy.quintileRank` (or similar) as "Quintile Rank" (e.g. 1–5 or "Q1"–"Q5").

### 3. Frontend: Leaderboard page and Coaching Hub – quintile icons

- **Leaderboard (Insights)**:
  - `AgentLeaderboardPage` uses `useAgentStats` (agent stats API) and `useGetQAStats` (RetrieveQAScoreStats). Rows are built from `agentStats.data?.resultsGroupedByAttributeTypes` and `score.data?.qaScoreResult.scores` (see `AgentLeaderboard.tsx` – `getCorrectRowFromLeaderboardQAGroupBy`). So each row has access to QA score and could have `groupedBy.quintileRank` once BE adds it.
  - Add a small icon (or badge) column or overlay next to agent name; color by quintile (e.g. 5 colors for Q1–Q5). Need to pass quintile from the score into the row and a shared `QuintileIcon` component.
- **Coaching Hub**:
  - `CoachingHub.tsx` → `CriteriaGoalSnapshot` (target progress) and `RecentCoachingActivities`. Agent lists may come from `getCoachingOverviewsWithQAStats` and target/QA stats. `CriteriaGoalSnapshot` uses `useTargetQAStats` → `qaScoreResult.scores` keyed by criterion; if we need per-agent quintile there, we need either (1) a per-agent QA stats call that includes quintile, or (2) Coaching Hub to use the same RetrieveQAScoreStats (by agent) and read quintile from `groupedBy`. Same icon component: color by quintile.
- **Icon approach**: One shared component (e.g. `QuintileRankIcon` or `AgentQuintileBadge`) that takes quintile 1–5 and renders a small colored dot/circle or icon; define 5 colors (e.g. red → yellow → green or a 5-step palette). Use on Leaderboard agent rows and Coaching Hub agent lists.

## Implementation plan

See **`implementation-plan.md`** for the concrete BE-first plan. Summary:

- **Quintile definition:** Score bands (0–100): Q1 = 80+, Q2 = 60–79, Q3 = 40–59, Q4 = 20–39, Q5 = 0–19. Backend uses 0–1 scale.
- **Phase 1 (BE):** Proto add `quintile_rank`; implement `ScoreToQuintileRank(score)`; set `GroupedBy.QuintileRank` in both retrieve_qa_score_stats.go and retrieve_qa_score_stats_clickhouse.go when returning per-agent scores; add tests.
- **Phase 2 (FE):** Performance page column; Leaderboard + Coaching Hub quintile icons (to be refined when FE requirements are set).
- **Worktrees:** Use separate worktrees for go-servers and director when making changes.

## Status

Active – BE done (go-servers PR #25795), FE Phase 2 in progress. Agent Leaderboard quintile column implemented in director-quintiles. Performance page + Coaching Hub follow-up next.

## Log History

| Date       | Summary |
|-----------|---------|
| 2026-02-17 | Project created; investigation and plan documented. Quintiles defined as score bands: Q1 80+, Q2 60–79, Q3 40–59, Q4 20–39, Q5 0–19. Concrete BE implementation plan added (implementation-plan.md). |
| 2026-02-18 | Proto PR merged (cresta-proto #7874, v2.0.534). BE implemented: `ScoreToQuintileRank`, `setQuintileRankForPerAgentScores`, 14 tests. go-servers PR: [#25795](https://github.com/cresta/go-servers/pull/25795). Agent tier logic documented (`agent-tier-logic.md`). |
| 2026-02-19 | FE Phase 2: Added `quintileRank` to internal types, transformer, `LeaderboardRow`, and Agent Leaderboard column (Q1–Q5 plain text). 5 files in director-quintiles. Scope: Agent Leaderboard only; Performance page + Coaching Hub deferred. |

## Related

- **Backend**: `~/repos/go-servers` (use worktree)
- **Frontend**: `~/repos/director` (use worktree)
- **Proto**: cresta-proto (analytics qa_stats, outcome_stats)
- **Existing tier**: 3 tiers (top 25%, middle 50%, bottom 25% by volume-weighted score)
