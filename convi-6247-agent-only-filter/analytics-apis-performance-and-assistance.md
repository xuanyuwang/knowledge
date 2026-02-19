# Analytics Service APIs Used by Performance and Agent Assistance Pages

**Created:** 2025-02-17

Lists which Analytics Service APIs the Director frontend uses on the **Performance** and **Agent Assist** (Assistance Insights) pages. Leaderboard APIs are documented in [insights-user-filter/apis-by-leaderboard-tab.md](../insights-user-filter/apis-by-leaderboard-tab.md).

**Frontend paths (director repo):**
- **Performance:** `packages/director-app/src/components/insights/qa-insights/performance/` and sibling qa-insights components
- **Agent Assist:** `packages/director-app/src/components/insights/assistance/`, `src/features/insights/assistance/`

---

## Performance Page

**Purpose:** QA score trends, conversation count, average handle time, score line charts, performance progression, leaderboard-by-criterion, QA conversation examples.

| Analytics API | Request message | FE hook(s) | Used in |
|---------------|-----------------|------------|---------|
| **RetrieveQAScoreStats** | RetrieveQAScoreStatsRequest | `useQAScoreStats`, `useQAScoreStatsRequestParams` | PerformanceProgression, LeaderboardPerCriterion, ConversationCountChart, StatsGraphContainer (useStatsData), ScoreLineChartGraph, ScoreInsightsMetric, QAICell, LeaderboardByScorecardTemplateItem, useTopAgentsQAScoreStats |
| **RetrieveConversationStats** | RetrieveConversationStatsRequest | `useConversationStats`, `useInsightsRequestParams` | ConversationCountChart, AverageHandleTimeInsightsMetric, AverageHandleTimeChartGraph |

**Note:** QA conversation examples drawer uses `useRetrieveQAConversationsRequestParams` / `useRetrieveAllQAConversations` → **RetrieveQAConversations**. If that RPC supports `filter_to_agents_only`, the FE should pass it when we add the page-wide filter.

---

## Agent Assist (Assistance Insights) Page

**Purpose:** Assistance used stats, hint stats by type, knowledge assist, conversation stats, agent stats; summary/suggestions/smart compose/notes/KB/guided workflow breakdowns; silence-hold hints; behavior hints.

| Analytics API | Request message | FE hook(s) | Used in |
|---------------|-----------------|------------|---------|
| **RetrieveHintStats** | RetrieveHintStatsRequest | `useHintStats`, `useGetHintStatsByHintType`, `useGetSilenceHoldHintStats` | EngagementCell, EngagementByBehaviorTable, SpecificHintBreakdownLeaderboard, useGetPolicyNamesForHintType, AssistanceUsedPage (useGetHintStatsByHintType), AssistanceInsightsCarouselMenu, AssistanceInsightsContainer (useGetHintStatsByHintType, useGetSilenceHoldHintStats), SilenceHoldHintsPage |
| **RetrieveAssistanceStats** (legacy) | RetrieveAssistanceStatsRequest | `useAssistanceStats` | SummaryUsedLeaderboardByType, AssistanceUsedPage, AssistanceInsightsContainer, useGetAllAssistanceStats |
| Split assistance APIs | Various | `useAssistanceStatsWithSplitAPIs` | Same as above when `enableSplitAssistanceStats` is on (Suggestion, Summarization, SmartCompose, NoteTaking, GuidedWorkflow, KnowledgeBase) |
| **RetrieveKnowledgeAssistStats** | RetrieveKnowledgeAssistStatsRequest | `useKnowledgeAssistStats` | GenAIAnswersLeaderboardByType, AssistanceUsedPage, AssistanceInsightsCarouselMenu, AssistanceInsightsContainer |
| **RetrieveConversationStats** | RetrieveConversationStatsRequest | `useConversationStats` | AssistanceUsedPage, AssistanceInsightsContainer |
| **RetrieveAgentStats** | RetrieveAgentStatsRequest | `useAgentStats` | AssistanceUsedPage |

**Not used on Agent Assist:** RetrieveLiveAssistStats, RetrieveQAScoreStats, RetrieveCoachingSessionStats, RetrieveCommentStats, RetrieveScorecardStats (those are Leaderboard-only in this context).

---

## Summary

- **Performance** uses: RetrieveQAScoreStats, RetrieveConversationStats (and optionally RetrieveQAConversations for the drawer).
- **Agent Assist** uses: RetrieveHintStats, RetrieveAssistanceStats / split APIs, RetrieveKnowledgeAssistStats, RetrieveConversationStats, RetrieveAgentStats.

All of these are already included in [insights-user-filter/apis-by-leaderboard-tab.md](../insights-user-filter/apis-by-leaderboard-tab.md); there are no additional analytics service APIs used only by Performance or Agent Assist. When adding the page-wide filter (CONVI-6247), pass `filterToAgentsOnly` / `listAgentOnly` from filter state into the request options for these hooks on Performance and Agent Assist as well.
