# Time Range Filter → Timestamp Column Mapping

This document maps every UI component on the Performance Insights and Leaderboard pages — down to individual table columns — to the exact timestamp column(s) the backend uses when applying the user-selected time range filter.

## How to Read This Document

- **Time range** refers to the date picker on the page.
- **Timestamp column** is the database column that the backend's WHERE clause filters on.
- The time range is always applied as `[start, end)` — inclusive start, exclusive end.
- Tables on these pages merge data from **multiple independent API calls**. Different columns in the same table row may filter on different timestamp columns.

## Timestamp Column Reference

| Timestamp Column | Database | Table(s) | Semantic Meaning |
|-----------------|----------|----------|------------------|
| `scorecard_time` | ClickHouse | `score`, `scorecard` (also mapped as `conversation_time` in `scorecard_score`) | The conversation start time associated with the scorecard |
| `scorecard_submit_time` | ClickHouse | `scorecard_d` | When the evaluator submitted the scorecard |
| `conversation_start_time` | ClickHouse | `conversation_d`, `moment_annotation_d`, `action_annotation_d`, `conversation_event_d` | When the conversation started |
| `conversation_end_time` | ClickHouse | `conversation_d` | When the conversation ended (only used when explicitly requested via `conversation_time_range_field`) |
| `event_time` | ClickHouse | `analytics_event_d` | When the analytics event (e.g., page visit) occurred |
| `outcome_time` | ClickHouse | `user_outcome_field_value_d` | When the outcome was recorded |
| `manager_submitted_at` | **PostgreSQL** | `director.coaching_sessions` | When the manager submitted the coaching session |

## Frontend Time Range Flow

Both pages share the same pattern:

1. User picks dates via `DateRangeFilter` → stored as `filtersState.submitDateRange` (`{ startDate, endDate }` Dayjs objects)
2. Two derived values:
   - `submitDateRange` — rounded to week boundaries for aggregation
   - `submitDateRangeInternal` — exact selected range
3. Converted to API request fields:
   - **QA/Scorecard APIs**: `filterByTimeRange: toTimeRangeIncludingDays(startDate, endDate)` → proto `TimeRange { start_timestamp, end_timestamp }`
   - **Insights APIs**: `dateRange: [dayRangeStart, dayRangeEnd]` → converted to `filterByTimeRange` in SDK layer
   - **User Outcome APIs**: `filter.timeRange: toTimeRangeIncludingDays(startDate, endDate)`

---

## Backend API → Timestamp Column Summary

| Backend API | Primary Table | Timestamp Column | Database |
|-------------|--------------|------------------|----------|
| RetrieveQAScoreStats | scorecard_score / score | `scorecard_time` | ClickHouse |
| RetrieveScorecardStats | scorecard_d | `scorecard_submit_time` | ClickHouse |
| RetrieveConversationStats | conversation_d | `conversation_start_time` (default) | ClickHouse |
| RetrieveAgentStats | conversation_d | `conversation_start_time` | ClickHouse |
| RetrieveAssistanceStats | conversation_d | `conversation_start_time` | ClickHouse |
| RetrieveHintStats | moment_annotation_d / action_annotation_d | `conversation_start_time` | ClickHouse |
| RetrieveLiveAssistStats | action_annotation_d | `conversation_start_time` | ClickHouse |
| RetrieveKnowledgeAssistStats | message_d / action_annotation_d / moment_annotation_d | `conversation_start_time` | ClickHouse |
| RetrieveCommentStats | scorecard_d | `scorecard_submit_time` | ClickHouse |
| RetrieveCoachingSessionStats | director.coaching_sessions | `manager_submitted_at` | **PostgreSQL** |
| RetrieveManagerStats | analytics_event_d | `event_time` | ClickHouse |
| RetrieveUserOutcomeStats | user_outcome_field_value_d | `outcome_time` | ClickHouse |

---

## Performance Insights Page

### Conversations Tab

All components call `RetrieveQAScoreStats` via `useQAScoreStats()`. Every piece of data on this tab filters on `scorecard_time`.

#### Score Trend Chart (StatsGraphContainer)

| Data | API | Timestamp Column |
|------|-----|-----------------|
| Score trend line | RetrieveQAScoreStats (groupBy: TIME_RANGE) | `scorecard_time` |
| Delta comparison | RetrieveQAScoreStats (previous period via `getDeltaDateRange()`) | `scorecard_time` |

#### Conversation Count Chart

| Data | API | Timestamp Column |
|------|-----|-----------------|
| Conversation count bars | RetrieveQAScoreStats (groupBy: TIME_RANGE) | `scorecard_time` |
| Unfiltered comparison | RetrieveQAScoreStats (no attribute filter) | `scorecard_time` |
| Delta comparison | RetrieveQAScoreStats (previous period) | `scorecard_time` |

#### Performance Progression (Heatmap Table)

| Column | API | Timestamp Column |
|--------|-----|-----------------|
| Criterion/Chapter name | Static from template | N/A |
| # of Scorecards | RetrieveQAScoreStats (groupBy: TIME_RANGE, whole template) | `scorecard_time` |
| Org Target | ListTargets (separate non-analytics API) | N/A |
| Top Agents | RetrieveQAScoreStats (top 25% agent tier) | `scorecard_time` |
| Average | RetrieveQAScoreStats (groupBy: TIME_RANGE) | `scorecard_time` |
| Date period cells | RetrieveQAScoreStats (groupBy: CRITERION + TIME_RANGE) | `scorecard_time` |

#### Leaderboard by Scorecard Template

| Column | API | Timestamp Column |
|--------|-----|-----------------|
| Agent/Team name | From QA score response | N/A |
| # of Scorecards | RetrieveQAScoreStats (groupBy: AGENT/GROUP) | `scorecard_time` |
| Average score | RetrieveQAScoreStats (groupBy: AGENT/GROUP, autofail enabled) | `scorecard_time` |
| Quintile Rank | RetrieveQAScoreStats (groupBy: AGENT) | `scorecard_time` |
| Per-criterion cells | RetrieveQAScoreStats (groupBy: AGENT/GROUP + CRITERION) | `scorecard_time` |

#### Leaderboard per Criterion

| Column | API | Timestamp Column |
|--------|-----|-----------------|
| Agent/Team name | From QA score response | N/A |
| # of Conversations/Scorecards | RetrieveQAScoreStats aggregated | `scorecard_time` |
| Average | RetrieveQAScoreStats weighted average | `scorecard_time` |
| Quintile Rank | RetrieveQAScoreStats (groupBy: AGENT) | `scorecard_time` |
| Date period cells | RetrieveQAScoreStats (groupBy: AGENT/GROUP + TIME_RANGE) | `scorecard_time` |

### Agent Outcomes Tab

All components call `RetrieveUserOutcomeStats` via `useUserOutcomeStats()`. Every piece of data on this tab filters on `outcome_time`.

#### Outcome Metric Charts (GraphStatsMetric)

| Data | API | Timestamp Column |
|------|-----|-----------------|
| Outcome trend lines | RetrieveUserOutcomeStats (groupBy: FIELD + TIME_RANGE) | `outcome_time` |

#### Outcome Summary Cards (GraphStatsSummary)

| Data | API | Timestamp Column |
|------|-----|-----------------|
| Current period value | RetrieveUserOutcomeStats (groupBy: FIELD) | `outcome_time` |
| Previous period delta | RetrieveUserOutcomeStats (previous period) | `outcome_time` |

#### Outcome Performance Progression (Heatmap)

| Column | API | Timestamp Column |
|--------|-----|-----------------|
| Outcome field name | Static from config | N/A |
| Top Agents (WEIGHTED_AVG mode) | RetrieveUserOutcomeStats (groupBy: FIELD + AGENT_TIER) | `outcome_time` |
| Average (WEIGHTED_AVG mode) | RetrieveUserOutcomeStats (groupBy: FIELD) | `outcome_time` |
| Total (SUM mode) | RetrieveUserOutcomeStats (groupBy: FIELD) | `outcome_time` |
| Org Target | RetrieveUserOutcomeStats (7-day window) | `outcome_time` |
| Date period cells | RetrieveUserOutcomeStats (groupBy: FIELD + TIME_RANGE) | `outcome_time` |

#### Outcome Leaderboard

| Column | API | Timestamp Column |
|--------|-----|-----------------|
| Agent/Team name | From outcome response | N/A |
| Per-outcome-field cells | RetrieveUserOutcomeStats (groupBy: USER/GROUP + FIELD) | `outcome_time` |

#### Outcome Period Leaderboard

| Column | API | Timestamp Column |
|--------|-----|-----------------|
| Agent/Team name | From outcome response | N/A |
| Date period cells | RetrieveUserOutcomeStats (groupBy: USER/GROUP + TIME_RANGE) | `outcome_time` |
| Aggregated column | RetrieveUserOutcomeStats (groupBy: USER/GROUP) | `outcome_time` |

---

## Leaderboard Page

### Agent Leaderboard Tab

This table merges data from **8+ independent API calls**. Each column group uses a different timestamp column.

#### Summary Cards (Statistics)

| Card | API Hook | Backend API | Timestamp Column |
|------|----------|-------------|-----------------|
| Conversation Volume (per agent per day) | useConversationStats | RetrieveConversationStats | `conversation_start_time` |
| Active Agents (per day) | useAgentStats | RetrieveAgentStats | `conversation_start_time` |
| Active Days (per agent) | useAgentStats | RetrieveAgentStats | `conversation_start_time` |
| QA Score Average | useGetQAStats | RetrieveQAScoreStats | `scorecard_time` |
| Engagement Rate | useGetHintStatsByHintType | RetrieveHintStats | `conversation_start_time` |
| Assistance Used | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| Raised Hands Answered | useLiveAssistStats | RetrieveLiveAssistStats | `conversation_start_time` |
| Gen AI Answers | useKnowledgeAssistStats | RetrieveKnowledgeAssistStats | `conversation_start_time` |

#### Agent Leaderboard Table — Per-Column Mapping

| Column | API Hook | Backend API | Timestamp Column |
|--------|----------|-------------|-----------------|
| **Name** | useConversationStats | RetrieveConversationStats | `conversation_start_time` |
| **Team** | useConversationStats | RetrieveConversationStats | `conversation_start_time` |
| **# of Conversations (per day)** | useConversationStats | RetrieveConversationStats | `conversation_start_time` |
| **Active Days** | useAgentStats | RetrieveAgentStats | `conversation_start_time` |
| **Convos Powered by AA (%)** | useConversationStats (2 calls) | RetrieveConversationStats | `conversation_start_time` |
| **Performance Score** | useGetQAStats | RetrieveQAScoreStats | `scorecard_time` |
| **Average Handle Time** | useConversationStats | RetrieveConversationStats | `conversation_start_time` |
| **Assistance Used (per day)** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Hints Followed** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Gen AI Answers** | useKnowledgeAssistStats | RetrieveKnowledgeAssistStats | `conversation_start_time` |
| **Searches** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Summary** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Suggestions** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Smart Compose** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Guided Workflow** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Notes** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **# of Submitted Scorecards** | useGetQAStats | RetrieveQAScoreStats | `scorecard_time` |
| **Overall Engagement Rate (%)** | Multiple hint hooks | RetrieveHintStats + RetrieveAssistanceStats + RetrieveKnowledgeAssistStats | `conversation_start_time` |
| **Behavioral Hint (%)** | useGetHintStatsByHintType | RetrieveHintStats | `conversation_start_time` |
| **KB Hint (%)** | useGetHintStatsByHintType | RetrieveHintStats | `conversation_start_time` |
| **GWF Hint (%)** | useGetHintStatsByHintType | RetrieveHintStats | `conversation_start_time` |
| **Reminder Hint (# sent)** | useGetHintStatsByHintType | RetrieveHintStats | `conversation_start_time` |
| **Checklist (%)** | useGetHintStatsByHintType | RetrieveHintStats | `conversation_start_time` |
| **Hands Raised** | useLiveAssistStats | RetrieveLiveAssistStats | `conversation_start_time` |
| **Raises Answered %** | useLiveAssistStats | RetrieveLiveAssistStats | `conversation_start_time` |
| **Live Assist (received)** | useLiveAssistStats | RetrieveLiveAssistStats | `conversation_start_time` |
| **Quintile Rank** | useGetQAStats | RetrieveQAScoreStats | `scorecard_time` |
| **Outcome Metrics (dynamic)** | useOutcomeStatsData | RetrieveQAScoreStats | `scorecard_time` |

**By-Metric Heatmap** (AgentLeaderboardByMetric): Same APIs as above but adds `TIME_RANGE` to groupBy for time-series breakdown. The timestamp column for each metric row matches the table above.

### Team Leaderboard Tab

Same API hooks and timestamp columns as Agent tab. Only difference: `groupBy` uses `ATTRIBUTE_TYPE_GROUP` instead of `ATTRIBUTE_TYPE_AGENT`. Additional columns:

| Column | API Hook | Backend API | Timestamp Column |
|--------|----------|-------------|-----------------|
| **# of Active Agents** | useAgentStats | RetrieveAgentStats | `conversation_start_time` |
| **Conversations w. Realtime Summaries** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Summary Used** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |
| **Summary Used %** | useAssistanceStats | RetrieveAssistanceStats | `conversation_start_time` |

### Manager Leaderboard Tab

This table merges data from **5 independent API calls**, each filtering on a different timestamp column.

#### Manager Statistics Summary Cards

| Card | API Hook | Backend API | Timestamp Column |
|------|----------|-------------|-----------------|
| Total Page Views (per manager) | useManagerStats | RetrieveManagerStats | `event_time` |
| Live Convo Page Views (per manager) | useManagerStats | RetrieveManagerStats | `event_time` |
| Closed Convo Page Views (per manager) | useManagerStats | RetrieveManagerStats | `event_time` |
| Scorecards Evaluated (avg per manager) | useQAScoreStats | RetrieveQAScoreStats | `scorecard_time` |
| Coaching Sessions Submitted (per manager) | useCoachingSessionStats | RetrieveCoachingSessionStats | `manager_submitted_at` (PG) |
| Comments Placed (per manager) | useCommentingStats | RetrieveCommentStats | `scorecard_submit_time` |
| Live Assist Given | useLiveAssistStats | RetrieveLiveAssistStats | `conversation_start_time` |

#### Manager Leaderboard Table — Per-Column Mapping

| Column | API Hook | Backend API | Timestamp Column | Database |
|--------|----------|-------------|-----------------|----------|
| **Manager** | useManagerStats | RetrieveManagerStats | `event_time` | ClickHouse |
| **All Page Views** | useManagerStats | RetrieveManagerStats | `event_time` | ClickHouse |
| **Live Convo Page Views** | useManagerStats | RetrieveManagerStats | `event_time` | ClickHouse |
| **Closed Convo Page Views** | useManagerStats | RetrieveManagerStats | `event_time` | ClickHouse |
| **Scorecards Evaluated** | useQAScoreStats | RetrieveQAScoreStats | `scorecard_time` | ClickHouse |
| **Coaching Sessions Submitted** | useCoachingSessionStats | RetrieveCoachingSessionStats | `manager_submitted_at` | **PostgreSQL** |
| **Comments Placed** | useCommentingStats | RetrieveCommentStats | `scorecard_submit_time` | ClickHouse |
| **Live Assist (gave)** | useLiveAssistStats | RetrieveLiveAssistStats | `conversation_start_time` | ClickHouse |

**By-Metric Heatmap** (ManagerLeaderboardByMetric): Same APIs, same timestamp columns, with daily breakdown.

---

## Cross-Page Observations

1. **Performance Insights Conversations tab is fully consistent** — every component filters by `scorecard_time`. No mixed semantics.

2. **Performance Insights Agent Outcomes tab is fully consistent** — every component filters by `outcome_time`.

3. **Agent/Team Leaderboard tables mix two timestamp families in one row**:
   - Most columns: `conversation_start_time` (conversation/agent/assistance/hint/live-assist APIs)
   - QA Score, # of Scorecards, Quintile Rank, Outcome Metrics: `scorecard_time`
   - These are conceptually similar (both represent "when the conversation happened") but can diverge because `scorecard_time` comes from the scorecard's associated conversation, not the conversation table directly.

4. **Manager Leaderboard has the most heterogeneous timestamp mix** — a single table row combines data filtered by four different timestamp columns:
   - `event_time` (page views)
   - `scorecard_time` (scorecards evaluated)
   - `manager_submitted_at` (coaching sessions — from PostgreSQL, not ClickHouse)
   - `scorecard_submit_time` (comments)
   - `conversation_start_time` (live assist)

5. **`scorecard_time` vs `scorecard_submit_time`** — the most important distinction:
   - A scorecard submitted today for a conversation from last week will:
     - **Appear** under last week's date range for QA Score metrics (`scorecard_time`)
     - **Appear** under today's date range for "Comments Placed" (`scorecard_submit_time`) and "Coaching Sessions" (`manager_submitted_at`)
   - This means the Manager Leaderboard can show a manager with 0 scorecards evaluated but >0 comments for the same date range — if they commented on old scorecards within the range.

6. **RetrieveCoachingSessionStats is the only API using PostgreSQL** — all other analytics APIs query ClickHouse. This means coaching session counts may have different latency, consistency, and timezone handling characteristics.

## Process Scorecards

Process scorecards have no associated conversation. The `scorecard_time` field for process scorecards needs verification against the scorecard sync pipeline to determine what value is stored. If set to scorecard creation time, process scorecards behave like `scorecard_submit_time` semantics. If null, they may be excluded from QA Score queries.

## Column Name Mapping Detail

The backend uses `tableColumnNameMapping` (in `common_clickhouse.go`) to remap logical column names per table:

| Logical Column | scorecard_score table | score table | scorecard table | conversation_d | analytics_event_d | user_outcome_field_value_d |
|---------------|----------------------|-------------|-----------------|---------------|-------------------|---------------------------|
| `conversation_start_time` | `conversation_time` | `scorecard_time` | `scorecard_time` | `conversation_start_time` | `event_time` | N/A |
| (time range) | — | — | — | — | — | `outcome_time` (hardcoded) |

This means `RetrieveQAScoreStats` always filters on what is logically "the conversation start time associated with the scorecard", even though the physical ClickHouse column has different names across tables.

## Source Files

### Frontend (Director)
- Performance page: `packages/director-app/src/features/insights/performance/`
- Leaderboard page: `packages/director-app/src/features/insights/leaderboard/`
- API hooks: `packages/director-app/src/components/insights/hooks/`
- Filter hooks: `packages/director-app/src/components/insights/hooks/performance-filters/`

### Backend (go-servers)
- Analytics handlers: `insights-server/internal/analyticsimpl/`
- Column mapping: `insights-server/internal/analyticsimpl/common_clickhouse.go` (lines ~1490-1541)
- Time range filter builder: `insights-server/internal/analyticsimpl/common_clickhouse.go` (`buildTimeRangeConditionAndArgs`, `parseClickhouseFilter`)

### Proto (cresta-proto)
- API definitions: `cresta/v1/analytics/analytics_service.proto`
