# Session Note - 2026-07-18 - Claude Code - Time Range Filter Investigation

**Started:** 2026-07-18
**Tool:** Claude Code
**Project:** `analytics`
**Goal:** Map every Performance Insights and Leaderboard UI component to the timestamp column its time range filter targets

## Source Context

- **Primary repo:** `director` (frontend), `go-servers` (backend), `cresta-proto` (API contracts)
- **Repo path:** `/Users/xuanyu.wang/repos/director`, `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/cresta-proto`
- **Branch:** `main`

## Inputs Reviewed

- Director frontend: Performance Insights page components, Leaderboard page components, filter hooks, API hooks
- go-servers backend: analytics handler implementations in `insights-server/internal/analyticsimpl/`
- cresta-proto: `cresta/v1/analytics/analytics_service.proto` for TimeRange field definitions
- ClickHouse column name mapping in `common_clickhouse.go`

## Phase 1: UI Components → API Calls

### Performance Insights

**Conversations Tab** — entry point: `features/insights/performance/performance-conversations/PerformanceConversations.tsx`
- Filter hook: `usePerformanceFilters()` → manages `submitDateRange` (week-rounded) and `submitDateRangeInternal` (exact)
- ALL components call `RetrieveQAScoreStats` via `useQAScoreStats()`:
  - StatsGraphContainer (score trend chart)
  - ConversationCountChart
  - PerformanceProgression (heatmap)
  - LeaderboardByScorecardTemplateItem
  - LeaderboardPerCriterion
- Time range field: `filterByTimeRange: toTimeRangeIncludingDays(startDate, endDate)`

**Agent Outcomes Tab** — entry point: `features/insights/performance/performance-agent-outcomes/PerformanceAgentOutcomes.tsx`
- ALL components call `RetrieveUserOutcomeStats` via `useUserOutcomeStats()`:
  - GraphStatsMetric, GraphStatsSummary
  - AgentOutcomesPerformanceProgression
  - AgentOutcomesLeaderboard, AgentOutcomesPeriodLeaderboard
- Time range field: `filter.timeRange: toTimeRangeIncludingDays(startDate, endDate)`

### Leaderboard

Entry point: `features/insights/leaderboard/Leaderboard.tsx` → three tabs sharing `useLeaderboardsFilters()`

**Agent/Team Tabs** — use a mix of APIs:
- useConversationStats → RetrieveConversationStats
- useAgentStats → RetrieveAgentStats
- useAssistanceStats → RetrieveAssistanceStats
- useHintStats → RetrieveHintStats
- useLiveAssistStats → RetrieveLiveAssistStats
- useKnowledgeAssistStats → RetrieveKnowledgeAssistStats
- useGetQAStats → RetrieveQAScoreStats
- useOutcomeStatsData → RetrieveQAScoreStats
- Team tab differs only in groupBy (ATTRIBUTE_TYPE_GROUP vs ATTRIBUTE_TYPE_AGENT)

**Manager Tab** — uses:
- useManagerStats → RetrieveManagerStats
- useQAScoreStats → RetrieveQAScoreStats (scorecard evaluations)
- useCoachingSessionStats → RetrieveCoachingSessionStats
- useCommentingStats → RetrieveCommentStats
- useLiveAssistStats → RetrieveLiveAssistStats

## Phase 2: API Calls → Timestamp Columns

Central filter logic: `parseClickhouseFilter()` and `buildTimeRangeConditionAndArgs()` in `common_clickhouse.go`

| Backend API | Timestamp Column | ClickHouse Table |
|-------------|-----------------|------------------|
| RetrieveQAScoreStats | `scorecard_time` (mapped to `conversation_time` in scorecard_score table) | scorecard_score / score |
| RetrieveScorecardStats | `scorecard_submit_time` (explicit replacement from `scorecard_time`) | scorecard_d |
| RetrieveConversationStats | `conversation_start_time` (default) or `conversation_end_time` | conversation_d |
| RetrieveAgentStats | `conversation_start_time` | conversation_d |
| RetrieveAssistanceStats | `conversation_start_time` | conversation_d |
| RetrieveHintStats | `conversation_start_time` | conversation_d |
| RetrieveLiveAssistStats | `conversation_start_time` | conversation_d |
| RetrieveCommentStats | `scorecard_submit_time` | scorecard_d |
| RetrieveCoachingSessionStats | `manager_submitted_at` | director.coaching_sessions (**PostgreSQL**) |
| RetrieveUserOutcomeStats | `outcome_time` | user_outcome_field_value_d |
| RetrieveManagerStats | `event_time` (mapped from `conversation_start_time`) | analytics_event_d |
| RetrieveKnowledgeAssistStats | `conversation_start_time` | message_d / action_annotation_d / moment_annotation_d |

Column name mapping dict (`tableColumnNameMapping`): maps `conversation_start_time` → `scorecard_time` in score tables, → `conversation_time` in scorecard_score table, → `event_time` in analytics_event_d.

## Phase 3: Response Processing → Display

- StatsGraphContainer and ConversationCountChart issue a delta request (previous period) for comparison arrows
- PerformanceProgression combines criterion + time_range groupBy for heatmap cells
- Leaderboard summary cards reuse response-level aggregates from the same grouped requests as the table
- Agent Outcomes components issue multiple queries with different groupBy combinations and merge results client-side

## Key Findings

1. **Performance Insights Conversations tab is internally consistent** — all components filter by `scorecard_time` (conversation start time of the scored conversation)
2. **Leaderboard mixes timestamp semantics** — conversation/agent/assistance metrics filter by `conversation_start_time`, while QA scores filter by `scorecard_time` — conceptually similar but can diverge
3. **Manager tab mixes submit-time and conversation-time** — QA scores use `scorecard_time`, comments use `scorecard_submit_time`
4. **`scorecard_time` ≠ `scorecard_submit_time`** — this is the most important distinction; a scorecard submitted today for last week's conversation will appear under different date ranges depending on which column is filtered

## Phase 2 (continued): Remaining APIs

Traced in second pass:
- **RetrieveUserOutcomeStats** → `outcome_time` on `user_outcome_field_value_d` (ClickHouse)
- **RetrieveManagerStats** → `event_time` on `analytics_event_d` (ClickHouse) — column mapping remaps `conversation_start_time` → `event_time`
- **RetrieveCoachingSessionStats** → `manager_submitted_at` on `director.coaching_sessions` (**PostgreSQL**, not ClickHouse)
- Confirmed RetrieveHintStats, RetrieveLiveAssistStats, RetrieveKnowledgeAssistStats all use `conversation_start_time`

## Column-Level Table Mapping

Refined all table components to column-level granularity. Key insight: the Agent/Team Leaderboard table merges 8+ API responses into one row — different columns in the same row filter on different timestamp columns. The Manager Leaderboard is the most heterogeneous: 5 APIs, 4 different timestamp columns, and one API (coaching sessions) queries PostgreSQL instead of ClickHouse.

Full column-level mapping in `deliverables/time-range-timestamp-mapping.md`.

## Follow-ups

- Verify how `scorecard_time` is populated for process scorecards (no conversation)
- Check whether any frontend component passes `conversationTimeRangeField` to switch between start/end time
- Verify timezone handling differences between PostgreSQL (coaching sessions) and ClickHouse (everything else)
