# Analytics APIs Refactoring Summary

## Overview

This document consolidates the refactoring work for multiple Analytics Service APIs to use the new unified `ParseUserFilterForAnalytics` function with correct `listAgentOnly` settings.

## Problem Statement

### The Bug

All the refactored APIs previously used `listAgentOnly = false` in their user filtering logic:
- `ListUsersMappedToGroups(..., false)`
- `MoveFiltersToUserFilter(..., false)` or `MoveGroupFilterToUserFilterForQA(..., false)`

This caused **Agent+Manager users to be incorrectly included** in agent activity tracking. Since these APIs track agent activities (not manager activities), they should only include users with the Agent role, excluding Agent+Manager combinations.

### The Fix

Replace the 3-step filtering process with a single `ParseUserFilterForAnalytics` call using `listAgentOnly = true`:

**Old 3-step process:**
1. `ApplyResourceACL` - Apply resource-based access control
2. `ListUsersMappedToGroups` - Map users to their groups
3. `MoveFiltersToUserFilter` - Convert group filters to user filters

**New unified approach:**
- `ParseUserFilterForAnalytics` - Handles all three steps with correct `listAgentOnly` setting

## Refactored APIs

### 1. GenAI Feature Usage APIs (6 APIs)

These APIs track agent usage of GenAI features and all follow the same pattern.

| API | JIRA | PR | Use Case |
|-----|------|-----|----------|
| RetrieveSuggestionStats | CONVI-6015 | #25125 | Agent/Team Leaderboards - AI Suggestions |
| RetrieveSummarizationStats | CONVI-6016 | #25126 | Agent/Team Leaderboards - Summarization |
| RetrieveSmartComposeStats | CONVI-6017 | #25127 | Agent/Team Leaderboards - Smart Compose |
| RetrieveNoteTakingStats | CONVI-6018 | #25128 | Agent/Team Leaderboards - Note Taking |
| RetrieveGuidedWorkflowStats | CONVI-6019 | #25129 | Agent/Team Leaderboards - Guided Workflows |
| RetrieveKnowledgeBaseStats | CONVI-6008 | #25130 | Agent/Team Leaderboards - KB Search |

**Common characteristics:**
- Track agent usage of GenAI features
- Used in Agent and Team Leaderboards
- Single-purpose: agents using features
- Solution: `listAgentOnly = true`

**Clickhouse tables:**
- `suggestion_d`, `summarization_d`, `smart_compose_d`, `note_taking_d`, `guided_workflow_d`, `kb_search_d`
- Key column: `agent_user_id` (the agent using the feature)

### 2. Knowledge Assist API

| API | JIRA | PR | Use Case |
|-----|------|-----|----------|
| RetrieveKnowledgeAssistStats | CONVI-6020 | #25100 | Agent/Team Leaderboards - Knowledge Assist |

**Characteristics:**
- Tracks agent usage of Knowledge Assist feature
- Used in Agent and Team Leaderboards
- Single-purpose: agents receiving knowledge assistance
- Solution: `listAgentOnly = true`

**Clickhouse table:**
- `knowledge_assist_search_d`
- Key column: `agent_user_id` (the agent using knowledge assist)

### 3. QA Score Stats API

| API | JIRA | PR | Use Case |
|-----|------|-----|----------|
| RetrieveQAScoreStats | CONVI-6010 | #25143 | Agent/Team Leaderboards, QA Insights |

**Characteristics:**
- Tracks QA scores for agents being evaluated
- Used in Agent/Team Leaderboards, QA Insights, Coaching Hub
- Single-purpose: agents being evaluated (not evaluators)
- Solution: `listAgentOnly = true`
- **Special note**: Uses `MoveGroupFilterToUserFilterForQA` instead of `MoveFiltersToUserFilter`

**Clickhouse table:**
- `scorecard_score_d`
- Key column: `agent_user_id` (the agent being evaluated)

**Special logic:**
```go
// When only grouping by groups (not agents), use group filter for user mapping
if !hasAgentAsGroupByKey && hasGroupAsGroupByKey && len(groups) > 0 {
    groupsForMapping = groups  // Use group filter
} else {
    groupsForMapping = []      // Don't filter by groups
}
```

## Standard Refactoring Pattern

All APIs follow the same implementation pattern:

### 1. Main API Function

```go
func (a AnalyticsServiceImpl) RetrieveXXXStats(
    ctx context.Context, req *analyticspb.RetrieveXXXStatsRequest,
) (*analyticspb.RetrieveXXXStatsResponse, error) {
    // ... validation and setup ...

    var (
        users                             []*userpb.User
        groupsToAggregate                 []*userpb.Group
        userNameToGroupNamesMap           map[string][]string
        hasAgentAsGroupByKey              = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
        includeDirectGroupMembershipsOnly = shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
    )

    if a.enableParseUserFilterForAnalytics {
        // New implementation using ParseUserFilterForAnalytics
        listAgentOnly := true // Agent activity tracking
        shouldMoveFiltersToUserFilter := req.FilterByAttribute != nil &&
            (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) ||
            shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP)
        excludeDeactivatedUsers := req.FilterByAttribute != nil && req.FilterByAttribute.GetExcludeDeactivatedUsers()

        result, err := ParseUserFilterForAnalytics(
            ctx,
            a.userClientRegistry,
            a.internalUserServiceClientRegistry,
            a.configClient,
            a.resourceACLHelperProvider.Get(),
            parent.CustomerID,
            parent.ProfileID,
            req.FilterByAttribute.GetUsers(),
            req.FilterByAttribute.GetGroups(),
            hasAgentAsGroupByKey,
            includeDirectGroupMembershipsOnly,
            a.enableListUsersCache,
            *a.listUsersCache,
            listAgentOnly,
            req.GetIncludePeerUserStats(),
            shouldMoveFiltersToUserFilter,
            excludeDeactivatedUsers,
        )
        if err != nil {
            return nil, err
        }

        // Extract values from result
        userNameToGroupNamesMap = result.UserNameToGroupNamesMap
        groupsToAggregate = result.GroupsToAggregate
        users = result.UsersFromGroups

        // Reset if not grouping by agent or group
        if !shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) &&
            !shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
            users = []*userpb.User{}
            groupsToAggregate = []*userpb.Group{}
            userNameToGroupNamesMap = map[string][]string{}
        }
        req.FilterByAttribute.Users = result.FinalUsers
        req.FilterByAttribute.Groups = result.FinalGroups

        // Early return if no users to query
        if len(result.FinalUsers) == 0 {
            return &analyticspb.RetrieveXXXStatsResponse{}, nil
        }
    } else {
        // Old implementation preserved for backward compatibility
        req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(...)
        // ... rest of old 3-step process ...
    }

    // Continue with Clickhouse query logic
    // ...
}
```

### 2. Internal Function (Skip Redundant Filtering)

```go
func (a AnalyticsServiceImpl) retrieveXXXStatsInternal(
    ctx context.Context, req *analyticspb.RetrieveXXXStatsRequest,
) (*analyticspb.RetrieveXXXStatsResponse, error) {
    // ... setup ...

    if !a.enableParseUserFilterForAnalytics {
        // Legacy path: Move group filter to user filter
        if req.FilterByAttribute != nil && (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
            req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(
                ctx,
                a.userClientRegistry,
                a.internalUserServiceClientRegistry,
                parent.CustomerID,
                parent.ProfileID,
                req.FilterByAttribute,
                includeDirectGroupMembershipsOnly,
                false, // listAgentOnly
            )
            if err != nil {
                return nil, err
            }
            if len(req.FilterByAttribute.Users) == 0 {
                return &analyticspb.RetrieveXXXStatsResponse{}, nil
            }
        }
    }
    // If flag enabled, skip the redundant MoveFiltersToUserFilter call

    // Continue with Clickhouse query
    // ...
}
```

## Feature Flag

All refactored APIs use the same feature flag:
- **Flag**: `enableParseUserFilterForAnalytics`
- **Environment Variable**: `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS`
- **Default**: `false` (uses legacy implementation)
- **Purpose**: Allows safe rollout and easy rollback if issues arise

## Benefits

### 1. Correctness
- Agent leaderboards now only show agent-only users (no Agent+Manager combos)
- Metrics accurately reflect pure agent performance
- Consistent filtering across all agent activity APIs

### 2. Code Quality
- Single unified function instead of 3-step process
- Reduced code duplication
- Easier to maintain and understand

### 3. Safety
- Feature flag allows gradual rollout
- Legacy implementation preserved for rollback
- All existing tests pass with new implementation

## Testing

All APIs have comprehensive test coverage:
- Tests pass with both flag enabled and disabled
- Coverage includes: GroupByAgent, GroupByGroup, GroupByTime, various filters
- Integration tests verify Clickhouse queries work correctly

## Frontend Impact

**No frontend changes required** for the refactored APIs. The response format remains identical:
- Agent Leaderboard pages continue to work unchanged
- Team Leaderboard pages continue to work unchanged
- QA Insights pages continue to work unchanged

The only difference is **which users appear in the results**:
- **Before**: Agent-only users + Agent+Manager users
- **After**: Agent-only users only

## Special Cases

### RetrieveLiveAssistStats (NOT refactored)

This API is fundamentally different and requires a **different solution**:

**Why it's different:**
- **Dual-purpose**: Serves both Agent/Team Leaderboards AND Manager Leaderboard
- **Agent/Team Leaderboards**: Track agents receiving help (`totalWhisperedToAgentCount`) - needs `listAgentOnly = true`
- **Manager Leaderboard**: Track managers giving help (`totalWhisperByManagerCount`) - needs `listAgentOnly = false`

**Clickhouse query quirk:**
- Applies user filters to BOTH `agent_user_id` AND `manager_user_id` using OR condition
- This allows a single query to serve both purposes

**Recommended solution:**
- **Option 1**: Split into two separate APIs
- **Option 2**: Add `filter_to_agents_only` request parameter (preferred)
- **Option 3**: Don't update (keep current behavior)

See `.tmp/insights-user-filter/retrieve-live-assist-stats-analysis.md` for detailed analysis.

## Files Modified

### Per-API Files
Each API required changes to one main file:
- `insights-server/internal/analyticsimpl/retrieve_suggestion_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_summarization_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_smart_compose_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_note_taking_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_guided_workflow_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_knowledge_base_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_knowledge_assist_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`

### Common Pattern
Each file had two types of changes:
1. **Main API function**: Add feature flag with new/old paths (~60-100 lines changed)
2. **Internal function**: Wrap redundant filter move with flag check (~20-30 lines changed)

## Rollout Plan

1. **Deploy with flag disabled** - Legacy behavior continues
2. **Enable flag in staging** - Validate behavior with real data
3. **Monitor metrics** - Verify Agent leaderboards show correct users
4. **Enable in production** - Gradual rollout per customer or globally
5. **Remove legacy code** - After successful rollout, clean up old implementation

## Summary Statistics

| Metric | Count |
|--------|-------|
| Total APIs refactored | 8 |
| Total PRs created | 8 |
| Total JIRAs | 8 |
| Refactoring pattern | Standard (7), Special logic (1) |
| Test suites passing | 100% |
| Frontend changes required | 0 |
| Breaking changes | 0 |

## Related Work

This refactoring is part of a larger effort to:
1. Standardize user filtering across all Analytics Service APIs
2. Correctly filter to agent-only users for agent activity tracking
3. Improve code maintainability and consistency
4. Fix long-standing bugs with Agent+Manager users appearing in agent leaderboards

## Next Steps

1. ✅ Complete 8 single-purpose agent activity APIs (DONE)
2. ⏳ Address RetrieveLiveAssistStats dual-purpose API (requires discussion)
3. ⏳ Enable feature flag in staging environment
4. ⏳ Validate metrics and leaderboard data
5. ⏳ Enable feature flag in production
6. ⏳ Remove legacy code after successful rollout
