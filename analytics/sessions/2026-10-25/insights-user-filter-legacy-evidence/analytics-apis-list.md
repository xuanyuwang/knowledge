# Analytics Service APIs Used in Leaderboard and Performance Pages

## Summary
This document lists all Analytics Service APIs used in the Director Leaderboard and Performance pages. These APIs need to be updated with a `filter_to_agents_only` field to filter results to users who have ONLY the Agent role.

## API List

| # | API Name | Request Proto | Proto Lines | Hooks | Used In Pages | Refactored |
|---|----------|---------------|-------------|-------|---------------|------------|
| 1 | `RetrieveAgentStats` | `RetrieveAgentStatsRequest` | 2152-2195 | `useAgentStats` | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-5173) |
| 2 | `RetrieveConversationStats` | `RetrieveConversationStatsRequest` | 1771-1831 | `useConversationStats` | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6005) |
| 3 | `RetrieveAssistanceStats` | `RetrieveAssistanceStatsRequest` | 1870-1919 | `useAssistanceStats` | Agent Leaderboard, Team Leaderboard (legacy, replaced by split APIs) | ❌ (Legacy) |
| 4 | `RetrieveSuggestionStats` | `RetrieveSuggestionStatsRequest` | 2520-2564 | `useSuggestionStats` (via `useAssistanceStatsWithSplitAPIs`) | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6015) |
| 5 | `RetrieveSummarizationStats` | `RetrieveSummarizationStatsRequest` | 2593-2637 | `useSummarizationStats` (via `useAssistanceStatsWithSplitAPIs`) | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6016) |
| 6 | `RetrieveSmartComposeStats` | `RetrieveSmartComposeStatsRequest` | 2687-2731 | `useSmartComposeStats` (via `useAssistanceStatsWithSplitAPIs`) | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6017) |
| 7 | `RetrieveNoteTakingStats` | `RetrieveNoteTakingStatsRequest` | 2763-2807 | `useNoteTakingStats` (via `useAssistanceStatsWithSplitAPIs`) | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6018) |
| 8 | `RetrieveGuidedWorkflowStats` | `RetrieveGuidedWorkflowStatsRequest` | 2840-2868 | `useGuidedWorkflowStats` (via `useAssistanceStatsWithSplitAPIs`) | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6019) |
| 9 | `RetrieveKnowledgeBaseStats` | `RetrieveKnowledgeBaseStatsRequest` | 2898-2926 | `useKnowledgeBaseStats` (via `useAssistanceStatsWithSplitAPIs`) | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6008) |
| 10 | `RetrieveHintStats` | `RetrieveHintStatsRequest` | 2051-2111 | `useHintStats`, `useGetHintStatsByHintType` | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6007) |
| 11 | `RetrieveKnowledgeAssistStats` | `RetrieveKnowledgeAssistStatsRequest` | 2910-2950 | `useKnowledgeAssistStats` | Agent Leaderboard, Team Leaderboard | ✅ (CONVI-6020) |
| 12 | `RetrieveLiveAssistStats` | `RetrieveLiveAssistStatsRequest` | 2278-2321 | `useLiveAssistStats` | Agent Leaderboard, Team Leaderboard, Manager Leaderboard | ✅ (CONVI-6009) |
| 13 | `RetrieveQAScoreStats` | `RetrieveQAScoreStatsRequest` | 3084-3144 | `useQAScoreStats`, `useGetQAStats` | Agent Leaderboard, Team Leaderboard, Leaderboard-by-Metric pages, **Performance Page** | ✅ (CONVI-6010) |
| 14 | `RetrieveCoachingSessionStats` | `RetrieveCoachingSessionStatsRequest` | 2432-2456 | `useCoachingSessionStats` | Manager Leaderboard | ❌ (Not needed) |
| 15 | `RetrieveCommentStats` | `RetrieveCommentStatsRequest` | 2350-2374 | `useCommentingStats` | Manager Leaderboard | ❌ (Not needed) |
| 16 | `RetrieveScorecardStats` | `RetrieveScorecardStatsRequest` | 2391-2415 | `useScorecardStats` | Manager Leaderboard | ❌ (Not needed) |

### Refactoring Status Summary
- ✅ **12 APIs refactored** to use `ParseUserFilterForAnalytics`
- ❌ **1 Legacy API** not refactored (RetrieveAssistanceStats - replaced by split APIs)
- ❌ **3 Manager APIs** not refactored (not needed for user filter fixes)

### Note on Split Assistance APIs
When the feature flag `enableSplitAssistanceStats` is enabled, the `useAssistanceStatsWithSplitAPIs` hook replaces the single `RetrieveAssistanceStats` API with 6 separate API calls (APIs #4-9 above). This provides more granular assistance statistics.

## Proto File Location
All request messages are defined in: `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/analytics/analytics_service.proto`

## Field to Add

Each request message needs to add the following field:

```proto
// If true, filter results to users who have ONLY the Agent role (no other roles).
// This excludes users with Manager, QA Admin, or other additional roles.
bool filter_to_agents_only = <next_available_field_number> [(google.api.field_behavior) = OPTIONAL];
```

### Field Numbers by Request Message

| Request Message | Next Available Field Number |
|----------------|----------------------------|
| `RetrieveAgentStatsRequest` | 9 (last field is `include_peer_user_stats = 8`) |
| `RetrieveConversationStatsRequest` | 11 (last field is `allow_matching_stale_metadata_values = 10`) |
| `RetrieveAssistanceStatsRequest` | 9 (last field is `include_peer_user_stats = 8`) |
| `RetrieveSuggestionStatsRequest` | 8 (last field is `include_peer_user_stats = 7`) |
| `RetrieveSummarizationStatsRequest` | 8 (last field is `include_peer_user_stats = 7`) |
| `RetrieveSmartComposeStatsRequest` | 8 (last field is `include_peer_user_stats = 7`) |
| `RetrieveNoteTakingStatsRequest` | 8 (last field is `include_peer_user_stats = 7`) |
| `RetrieveGuidedWorkflowStatsRequest` | 8 (last field is `include_peer_user_stats = 7`) |
| `RetrieveKnowledgeBaseStatsRequest` | 8 (last field is `include_peer_user_stats = 7`) |
| `RetrieveHintStatsRequest` | 10 (last field is `include_peer_user_stats = 9`) |
| `RetrieveKnowledgeAssistStatsRequest` | 7 (last field is `metadata = 6`) |
| `RetrieveLiveAssistStatsRequest` | 9 (last field is `include_peer_user_stats = 8`) |
| `RetrieveQAScoreStatsRequest` | 12 (last field is `conversation_time_range_field = 11`) |
| `RetrieveCoachingSessionStatsRequest` | 7 (last field is `metadata = 6`) |
| `RetrieveCommentStatsRequest` | 7 (last field is `metadata = 6`) |
| `RetrieveScorecardStatsRequest` | 7 (last field is `metadata = 6`) |

## Implementation Notes

### Backend Requirements
When `filter_to_agents_only = true`, the backend must:
1. Query all user roles for the filtered users
2. Include only users who have EXACTLY the Agent role (and no other roles)
3. Exclude users who have Agent + Manager, Agent + QA_ADMIN, or any other role combination

### Backward Compatibility
- Field is `OPTIONAL` with default value `false`
- Existing API calls will continue to work unchanged
- No impact on existing functionality unless explicitly set to `true`

## Related Frontend Code

### Leaderboard Pages
- Agent Leaderboard: `/Users/xuanyu.wang/repos/director/packages/director-app/src/pages/insights/leaderboard/agent-leaderboard/`
- Team Leaderboard: `/Users/xuanyu.wang/repos/director/packages/director-app/src/pages/insights/leaderboard/team-leaderboard/`
- Manager Leaderboard: `/Users/xuanyu.wang/repos/director/packages/director-app/src/pages/insights/leaderboard/manager-leaderboard/`

### Performance Page
- Performance: `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/qa-insights/performance/`

### Hooks Location
- Located in `@cresta/director-api` package
- Custom hooks in `/Users/xuanyu.wang/repos/director/packages/director-app/src/pages/insights/leaderboard/hooks/`
