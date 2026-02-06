# RetrieveLiveAssistStats Backend Implementation Summary

## JIRA
CONVI-6009

## Branch
`convi-6009-fix-retrieveliveassiststats`

## Overview

Updated RetrieveLiveAssistStats to use the new unified `ParseUserFilterForAnalytics` function with the `filter_to_agents_only` request parameter to correctly filter users based on use case.

## Problem

RetrieveLiveAssistStats is a **dual-purpose API** that serves both Agent/Team Leaderboards and Manager Leaderboard:

| Use Case | Metric | Filter Required |
|----------|--------|----------------|
| **Agent/Team Leaderboards** | `total_whispered_to_agent_count` (agents receiving whispers) | Pure agents only (`listAgentOnly = true`) |
| **Manager Leaderboard** | `total_whisper_by_manager_count` (managers giving whispers) | All managers (`listAgentOnly = false`) |

**Current Bug**: The API always uses `listAgentOnly = false` for all use cases, causing Agent+Manager users to incorrectly appear on Agent/Team Leaderboards.

## Solution

### 1. Proto Change (Already Complete)

The `filter_to_agents_only` field was already added to `RetrieveLiveAssistStatsRequest` in cresta-proto v1.0.2667:

```protobuf
message RetrieveLiveAssistStatsRequest {
  // ... existing fields ...

  // If true, filter results to users who have ONLY the Agent role (no other roles).
  // This excludes users with Manager, QA Admin, or other additional roles.
  bool filter_to_agents_only = 9 [(google.api.field_behavior) = OPTIONAL];
}
```

### 2. Backend Implementation

Updated `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_live_assist_stats.go` with the standard refactoring pattern:

#### Main API Function (`RetrieveLiveAssistStats`)

Added feature flag to enable new vs legacy implementation:

```go
if a.enableParseUserFilterForAnalytics {
    // New implementation using ParseUserFilterForAnalytics
    listAgentOnly := req.GetFilterToAgentsOnly() // Use request parameter

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
        listAgentOnly, // ← Uses request parameter
        req.GetIncludePeerUserStats(),
        shouldMoveFiltersToUserFilter,
        excludeDeactivatedUsers,
    )
    // ... extract results and update request ...
} else {
    // Legacy implementation (old 3-step process)
    // ... preserved for backward compatibility ...
}
```

#### Internal Function (`retrieveLiveAssistStatsInternal`)

Skips redundant filtering when feature flag is enabled:

```go
if !a.enableParseUserFilterForAnalytics {
    // Legacy path: Move group filter to user filter
    if req.FilterByAttribute != nil && ... {
        req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(
            ctx,
            a.userClientRegistry,
            a.internalUserServiceClientRegistry,
            parent.CustomerID,
            parent.ProfileID,
            req.FilterByAttribute,
            includeDirectGroupMembershipsOnly,
            false, // listAgentOnly - BUG: Always false!
        )
        // ... handle error ...
    }
}
// If flag enabled, skip the redundant MoveFiltersToUserFilter call
```

## Key Implementation Details

### The Dual-Purpose Query

RetrieveLiveAssistStats has special Clickhouse query logic that applies user filters to **both** `agent_user_id` AND `manager_user_id` using OR:

```go
// In retrieve_live_assist_stats_clickhouse.go:20-27
for i := 0; i < len(aaConditions); i++ {
    c := aaConditions[i]
    if strings.Contains(c.condition, agentUserIDColumn) {
        cm := strings.ReplaceAll(c.condition, agentUserIDColumn, "manager_user_id")
        c.condition = strings.Join([]string{c.condition, cm}, " OR ")
        c.args = append(c.args, c.arg)
        aaConditions[i] = c
    }
}
```

This transforms:
```sql
-- Input
WHERE agent_user_id IN ('alice', 'bob')

-- Output
WHERE (agent_user_id IN ('alice', 'bob')) OR (manager_user_id IN ('alice', 'bob'))
```

This allows a single query to:
- Count whispers received by agents (agent_user_id matches)
- Count whispers given by managers (manager_user_id matches)

### Why `filter_to_agents_only` is Critical

When `filter_to_agents_only = true`:
- Input users: `[alice (agent), bob (agent+manager), charlie (agent)]`
- After filtering: `[alice, charlie]` (bob excluded - has multiple roles)
- Query: `WHERE (agent_user_id IN ('alice', 'charlie')) OR (manager_user_id IN ('alice', 'charlie'))`
- Result: Agent Leaderboard shows only pure agents ✅

When `filter_to_agents_only = false`:
- Input users: `[alice (manager), bob (agent+manager)]`
- After filtering: `[alice, bob]` (includes users with manager role)
- Query: `WHERE (agent_user_id IN ('alice', 'bob')) OR (manager_user_id IN ('alice', 'bob'))`
- Result: Manager Leaderboard shows all managers ✅

## Feature Flag

**Flag**: `enableParseUserFilterForAnalytics`
**Environment Variable**: `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS`
**Default**: `false` (uses legacy implementation)

## Testing

Existing tests pass with both flag enabled and disabled:
```bash
go test -run TestRetrieveLiveAssistStats ./internal/analyticsimpl/ -timeout 10m
ok  	github.com/cresta/go-servers/insights-server/internal/analyticsimpl	8.398s
```

Test coverage includes:
- Basic agent stats retrieval
- Group-by agent and time
- Group-by group (team leaderboard)
- Various filter combinations

## Frontend Changes Required

The frontend must set `filter_to_agents_only` based on use case:

### Agent Leaderboard
```typescript
// director/packages/director-app/src/pages/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx
const request: RetrieveLiveAssistStatsRequest = {
  parent: ...,
  filterByAttribute: ...,
  filterByTimeRange: ...,
  frequency: ...,
  groupByAttributeTypes: ...,
  filterToAgentsOnly: true, // ← Must set to true for Agent/Team Leaderboards
};
```

### Team Leaderboard
```typescript
// director/packages/director-app/src/pages/insights/leaderboard/team-leaderboard/TeamLeaderboardPage.tsx
const request: RetrieveLiveAssistStatsRequest = {
  parent: ...,
  filterByAttribute: ...,
  filterByTimeRange: ...,
  frequency: ...,
  groupByAttributeTypes: [AttributeType.ATTRIBUTE_TYPE_GROUP, AttributeType.ATTRIBUTE_TYPE_AGENT],
  filterToAgentsOnly: true, // ← Must set to true for Agent/Team Leaderboards
};
```

### Manager Leaderboard
```typescript
// director/packages/director-app/src/pages/insights/leaderboard/manager-leaderboard/ManagerLeaderboardPage.tsx
const request: RetrieveLiveAssistStatsRequest = {
  parent: ...,
  filterByAttribute: ...,
  filterByTimeRange: ...,
  frequency: ...,
  groupByAttributeTypes: ...,
  // filterToAgentsOnly defaults to false - don't set it
};
```

## Files Modified

### Backend
- `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_live_assist_stats.go`
  - Lines 33-118: Added feature flag with new/old implementation paths (~86 lines)
  - Lines 161-182: Wrapped redundant filter move with flag check (~22 lines)

### Documentation Created
- `/Users/xuanyu.wang/repos/go-servers/.tmp/insights-user-filter/retrieve-live-assist-stats-user-filter-flow.md`
  - Step-by-step explanation of user filter flow to Clickhouse query
- `/Users/xuanyu.wang/repos/go-servers/.tmp/insights-user-filter/retrieve-live-assist-stats-implementation.md`
  - This implementation summary

## Rollout Plan

1. ✅ **Complete backend implementation** - DONE
2. ⏳ **Update frontend to set `filterToAgentsOnly`**
3. ⏳ **Deploy with flag disabled** - Legacy behavior continues
4. ⏳ **Enable flag in staging** - Validate behavior
5. ⏳ **Monitor Agent leaderboards** - Verify correct users appear
6. ⏳ **Enable in production** - Gradual rollout
7. ⏳ **Remove legacy code** - After successful rollout

## Comparison with Other APIs

RetrieveLiveAssistStats differs from the other 11 refactored APIs:

| API Type | Count | listAgentOnly | Frontend Changes |
|----------|-------|---------------|------------------|
| **Single-purpose agent APIs** | 11 | Always `true` (hardcoded) | None required |
| **RetrieveLiveAssistStats** | 1 | From `req.GetFilterToAgentsOnly()` | Must set parameter |

**Why the difference?**
- Single-purpose APIs only serve Agent/Team Leaderboards → Always need pure agents
- RetrieveLiveAssistStats serves 3 use cases → Needs request parameter to distinguish

## Benefits

### 1. Correctness
- Agent/Team Leaderboards show only pure agents (no Agent+Manager users)
- Manager Leaderboard includes all managers (Agent+Manager users allowed)
- Metrics accurately reflect the intended role

### 2. Backward Compatibility
- Feature flag allows safe rollout
- Legacy implementation preserved for rollback
- All existing tests pass

### 3. Consistency
- Uses same `ParseUserFilterForAnalytics` function as other APIs
- Follows same refactoring pattern
- Reduces code duplication

## Related Documentation

- **User Filter Flow**: `.tmp/insights-user-filter/retrieve-live-assist-stats-user-filter-flow.md`
- **Detailed Analysis**: `.tmp/insights-user-filter/retrieve-live-assist-stats-detailed-analysis.md`
- **Consolidated Summary**: `.tmp/insights-user-filter/consolidated-api-refactoring-summary.md`
- **Implementation Plan**: `.tmp/insights-user-filter/consolidated-implementation-plan.md`

## Next Steps

1. Update frontend Agent/Team Leaderboards to set `filterToAgentsOnly: true`
2. Verify end-to-end behavior with integration tests
3. Create PR with backend and frontend changes
4. Deploy and monitor rollout
