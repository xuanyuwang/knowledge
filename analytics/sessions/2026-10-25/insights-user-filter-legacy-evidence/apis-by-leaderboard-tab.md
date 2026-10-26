# Analytics APIs Categorized by Leaderboard Tab

## Summary
This document categorizes Analytics Service APIs by which Leaderboard tab uses them. This helps determine the correct `listAgentOnly` parameter for each API when applying the `ParseUserFilterForAnalytics` refactoring.

## Category 1: Agent Leaderboard Only
**Purpose:** Track agent activities
**User Filtering:** Should use `listAgentOnly = true` to filter to agent-only users
**Status:** ✅ Completed

| API Name | Request Proto | Proto Lines | Hooks | Refactored |
|----------|---------------|-------------|-------|------------|
| `RetrieveAgentStats` | `RetrieveAgentStatsRequest` | 2152-2195 | `useAgentStats` | ✅ CONVI-5173 |

## Category 2: Agent + Team Leaderboards (Shared)
**Purpose:** Track agent activities, aggregated by agent or team
**User Filtering:** Should use `listAgentOnly = true` to filter to agent-only users
**Status:** ✅ Completed

| API Name | Request Proto | Proto Lines | Hooks | Refactored |
|----------|---------------|-------------|-------|------------|
| `RetrieveConversationStats` | `RetrieveConversationStatsRequest` | 1771-1831 | `useConversationStats` | ✅ CONVI-6005 |
| `RetrieveAssistanceStats` | `RetrieveAssistanceStatsRequest` | 1870-1919 | `useAssistanceStats` (legacy) | ❌ Legacy |
| `RetrieveSuggestionStats` | `RetrieveSuggestionStatsRequest` | 2520-2564 | `useSuggestionStats` (via split APIs) | ✅ CONVI-6015 |
| `RetrieveSummarizationStats` | `RetrieveSummarizationStatsRequest` | 2593-2637 | `useSummarizationStats` (via split APIs) | ✅ CONVI-6016 |
| `RetrieveSmartComposeStats` | `RetrieveSmartComposeStatsRequest` | 2687-2731 | `useSmartComposeStats` (via split APIs) | ✅ CONVI-6017 |
| `RetrieveNoteTakingStats` | `RetrieveNoteTakingStatsRequest` | 2763-2807 | `useNoteTakingStats` (via split APIs) | ✅ CONVI-6018 |
| `RetrieveGuidedWorkflowStats` | `RetrieveGuidedWorkflowStatsRequest` | 2840-2868 | `useGuidedWorkflowStats` (via split APIs) | ✅ CONVI-6019 |
| `RetrieveKnowledgeBaseStats` | `RetrieveKnowledgeBaseStatsRequest` | 2898-2926 | `useKnowledgeBaseStats` (via split APIs) | ✅ CONVI-6008 |
| `RetrieveHintStats` | `RetrieveHintStatsRequest` | 2051-2111 | `useHintStats`, `useGetHintStatsByHintType` | ✅ CONVI-6007 |
| `RetrieveKnowledgeAssistStats` | `RetrieveKnowledgeAssistStatsRequest` | 2910-2950 | `useKnowledgeAssistStats` | ✅ CONVI-6020 |

## Category 3: Manager Leaderboard Only
**Purpose:** Track manager activities (coaching, commenting, scorecards)
**User Filtering:** Should use `listAgentOnly = false` - managers can have multiple roles
**Status:** Not yet updated (SHOULD NOT UPDATE YET per user request)

| API Name | Request Proto | Proto Lines | Hooks |
|----------|---------------|-------------|-------|
| `RetrieveCoachingSessionStats` | `RetrieveCoachingSessionStatsRequest` | 2432-2456 | `useCoachingSessionStats` |
| `RetrieveCommentStats` | `RetrieveCommentStatsRequest` | 2350-2374 | `useCommentingStats` |
| `RetrieveScorecardStats` | `RetrieveScorecardStatsRequest` | 2391-2415 | `useScorecardStats` |

## Category 4: Multi-Tab (Agent + Team + Manager)
**Purpose:** Track activities across different user types
**User Filtering:** Complex - used in multiple contexts with different requirements
**Status:** ✅ Completed

| API Name | Request Proto | Proto Lines | Hooks | Used In | Refactored |
|----------|---------------|-------------|-------|---------|------------|
| `RetrieveLiveAssistStats` | `RetrieveLiveAssistStatsRequest` | 2278-2321 | `useLiveAssistStats` | Agent, Team, Manager | ✅ CONVI-6009 |

## Category 5: Other Pages (Performance, Leaderboard-by-Metric)
**Purpose:** QA scoring and performance tracking
**User Filtering:** Context-dependent
**Status:** ✅ Completed

| API Name | Request Proto | Proto Lines | Hooks | Used In | Refactored |
|----------|---------------|-------------|-------|---------|------------|
| `RetrieveQAScoreStats` | `RetrieveQAScoreStatsRequest` | 3084-3144 | `useQAScoreStats`, `useGetQAStats` | Agent, Team, Leaderboard-by-Metric, Performance | ✅ CONVI-6010 |

## Implementation Strategy

### Phase 1: Agent + Team Leaderboards (Category 2) - Priority 1
**Target:** 10 APIs that track agent activities
**Approach:** Apply `ParseUserFilterForAnalytics` with `listAgentOnly = true`
**Rationale:** These APIs all track agent performance metrics and should filter to agent-only users

**Suggested Order:**
1. `RetrieveConversationStats` - Core conversation metrics
2. `RetrieveHintStats` - Core assistance metrics
3. `RetrieveKnowledgeAssistStats` - Knowledge assist metrics
4. Split Assistance APIs (6 APIs together):
   - `RetrieveSuggestionStats`
   - `RetrieveSummarizationStats`
   - `RetrieveSmartComposeStats`
   - `RetrieveNoteTakingStats`
   - `RetrieveGuidedWorkflowStats`
   - `RetrieveKnowledgeBaseStats`

### Phase 2: Multi-Tab APIs (Categories 4 & 5) - Priority 2
**Target:** `RetrieveLiveAssistStats`, `RetrieveQAScoreStats`
**Approach:** Analyze each context carefully, may need different behavior per tab
**Rationale:** These are used across multiple tabs with potentially different filtering needs

### Phase 3: Manager Leaderboard (Category 3) - Priority 3 (DEFER)
**Target:** 3 Manager-only APIs
**Approach:** Apply `ParseUserFilterForAnalytics` with `listAgentOnly = false`
**Rationale:** Track manager activities, not restricted to agent-only users
**Status:** User requested to defer this category for now

## listAgentOnly Flag Guidance

| Category | listAgentOnly Value | Reason |
|----------|-------------------|--------|
| Agent + Team Leaderboards | `true` | Tracks agent performance, should filter to users with ONLY agent role |
| Manager Leaderboard | `false` | Tracks manager activities, managers can have multiple roles |
| Multi-tab APIs | Context-dependent | Needs analysis - may need different values per usage context |

## Notes

- **Already Completed:** RetrieveAgentStats updated in CONVI-5173 with `listAgentOnly = true`
- **Feature Flag:** All updates gated behind `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS`
- **Split Assistance APIs:** When `enableSplitAssistanceStats` feature flag is enabled, the 6 split APIs replace the single `RetrieveAssistanceStats` API
