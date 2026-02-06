# RetrieveLiveAssistStats: Detailed Analysis and Implementation Plan

## Executive Summary

**RetrieveLiveAssistStats is a DUAL-PURPOSE API** that serves fundamentally different use cases with opposite filtering requirements:

1. **Agent/Team Leaderboards**: Track agents **RECEIVING** live assistance → needs `listAgentOnly = true`
2. **Manager Leaderboard**: Track managers **GIVING** live assistance → needs `listAgentOnly = false`

**Recommended Solution**: Update to use `ParseUserFilterForAnalytics` with **CONDITIONAL** `listAgentOnly` based on the use case.

---

## Table of Contents

1. [Frontend Usage Analysis](#frontend-usage-analysis)
2. [Backend Implementation Analysis](#backend-implementation-analysis)
3. [The Dual-Purpose Challenge](#the-dual-purpose-challenge)
4. [Clickhouse Query Analysis](#clickhouse-query-analysis)
5. [Solution Options](#solution-options)
6. [Recommended Implementation](#recommended-implementation)
7. [Testing Strategy](#testing-strategy)

---

## Frontend Usage Analysis

### 1. Agent Leaderboard (`AgentLeaderboardPage.tsx:96`)

```typescript
const agentLiveAssistStats = useLiveAssistStats(
  insightsRequestParams,
  !preSelectionOfFiltersCompleted
);
```

**Context**:
- Shows agents who raised hands or received whispers
- Metrics displayed: `totalWhisperedToAgentCount`, `totalRaisedHandCount`
- User filter: Agents (from agent selector)
- Group by: Agent

**Expected behavior**: Filter to **agent-only users** (no Agent+Manager combos)

**Rationale**: This leaderboard shows agent performance metrics. Including Agent+Manager users would:
- Skew metrics (managers receive fewer whispers)
- Mix pure agents with multi-role users
- Violate the semantic of "Agent Leaderboard"

**Required `listAgentOnly`**: `true` ✅

---

### 2. Team Leaderboard (`TeamLeaderboardPage.tsx:114`)

```typescript
const agentLiveAssistStats = useLiveAssistStats(groupByTeamParamsForAgentStats);
```

**Context**:
- Shows teams of agents and their whisper metrics
- Metrics displayed: Same as Agent Leaderboard, aggregated by team
- User filter: Teams/Groups (from team selector)
- Group by: Group (Team)

**Expected behavior**: Filter to **agent-only users** within teams

**Rationale**: Same as Agent Leaderboard - we want pure agents' metrics, not mixed with managers

**Required `listAgentOnly`**: `true` ✅

---

### 3. Manager Leaderboard (`ManagerLeaderboardPage.tsx:115-117`)

```typescript
const managerLiveAssistStats = useLiveAssistStats(
  managerStatsRequestParams,
  filterByManagerUsers.userNames.length === 0
);
```

**Context**:
- Shows managers who gave whispers to agents
- Metrics displayed: `totalWhisperByManagerCount`
- User filter: Managers (from manager selector)
- Group by: Agent (manager user ID aliased as agent_user_id in query)

**Expected behavior**: Include **users with Manager role** (can have multiple roles like Agent+Manager)

**Rationale**:
- Manager leaderboard shows manager activities
- Many managers also have Agent role (Agent+Manager)
- We WANT to include Agent+Manager users here
- Filtering to manager-only would exclude most managers!

**Required `listAgentOnly`**: `false` ✅

---

## Backend Implementation Analysis

### Current Implementation (Buggy)

```go
// Line 48-59: ListUsersMappedToGroups
userNameToGroupNamesMap, groupsToAggregate, users, err = shared.ListUsersMappedToGroups(
    ctx,
    a.userClientRegistry,
    a.internalUserServiceClientRegistry,
    parent.CustomerID,
    parent.ProfileID,
    req.FilterByAttribute.GetGroups(),
    hasAgentAsGroupByKey,
    includeDirectGroupMembershipsOnly,
    a.enableListUsersCache,
    *a.listUsersCache,
    false, // listAgentOnly ❌ BUG: Always false!
)

// Line 108-116: MoveFiltersToUserFilter
req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(
    ctx,
    a.userClientRegistry,
    a.internalUserServiceClientRegistry,
    parent.CustomerID,
    parent.ProfileID,
    req.FilterByAttribute,
    includeDirectGroupMembershipsOnly,
    false, // listAgentOnly ❌ BUG: Always false!
)
```

**Problem**: `listAgentOnly` is hardcoded to `false` for ALL use cases.

**Impact**:
- ✅ Manager Leaderboard works correctly (needs `false`)
- ❌ Agent Leaderboard includes Agent+Manager users (should be `true`)
- ❌ Team Leaderboard includes Agent+Manager users (should be `true`)

---

## The Dual-Purpose Challenge

### Why This API is Different

All other Analytics APIs we've refactored have **single-purpose** semantics:
- RetrieveSuggestionStats → Agents using suggestions
- RetrieveConversationStats → Agents handling conversations
- RetrieveQAScoreStats → Agents being evaluated

But RetrieveLiveAssistStats tracks **TWO distinct activities** with **TWO distinct user types**:

| Activity | User Type | Metric | listAgentOnly |
|----------|-----------|--------|---------------|
| Agents **receiving** whispers | Agents | `totalWhisperedToAgentCount` | `true` |
| Managers **giving** whispers | Managers | `totalWhisperByManagerCount` | `false` |

### The Query's Clever Design

The Clickhouse query is designed to handle both in a single query:

```sql
WITH raised_hands_and_whispers AS (
    SELECT
        agent_user_id,
        manager_user_id,
        has_raised_hand,
        has_whisper
    FROM action_annotation_d
    WHERE
        -- User filter applies to BOTH agent_user_id AND manager_user_id (line 22-27)
        (agent_user_id IN (...)) OR (manager_user_id IN (...))
),
agent_live_assist_stats AS (
    -- Groups by agent_user_id
    -- Shows agents receiving help
    SELECT ..., COUNT(...) AS whispered_to_agent_count, 0 AS whisper_by_manager_count
    FROM raised_hands_and_whispers
    GROUP BY agent_user_id, ...
),
manager_live_assist_stats AS (
    -- Groups by manager_user_id (aliased as agent_user_id)
    -- Shows managers giving help
    SELECT ..., 0 AS whispered_to_agent_count, COUNT(...) AS whisper_by_manager_count
    FROM raised_hands_and_whispers
    GROUP BY manager_user_id AS agent_user_id, ...
)
SELECT * FROM agent_live_assist_stats
UNION ALL
SELECT * FROM manager_live_assist_stats
```

**Key Insight (lines 22-27 in Clickhouse file)**:
```go
if strings.Contains(c.condition, agentUserIDColumn) {
    cm := strings.ReplaceAll(c.condition, agentUserIDColumn, "manager_user_id")
    c.condition = strings.Join([]string{c.condition, cm}, " OR ")
    c.args = append(c.args, c.arg)
}
```

**Any user filter on `agent_user_id` is automatically applied to BOTH `agent_user_id` AND `manager_user_id`**:
```sql
-- Input filter: agent_user_id IN (user1, user2)
-- Becomes:
(agent_user_id IN (user1, user2)) OR (manager_user_id IN (user1, user2))
```

This is why a single API can serve both purposes!

---

## Clickhouse Query Analysis

### Response Structure

```protobuf
message LiveAssistStatsResult {
  Attribute attribute = 1;
  repeated LiveAssistStats live_assist_stats = 2;
  int32 total_raised_hand_count = 3;
  int32 total_raised_hand_answered_count = 4;
  int32 total_whispered_to_agent_count = 5;    // ← Agent metric
  int32 total_whisper_by_manager_count = 6;     // ← Manager metric
}
```

**Both metrics are always returned**, but frontends use different ones:
- Agent/Team Leaderboards: Use `total_whispered_to_agent_count`
- Manager Leaderboard: Use `total_whisper_by_manager_count`

### Query Execution Flow

```
1. Filter user list (WHERE clause)
   - Agent Leaderboard: agents-only users → [agent1, agent2, agent3]
   - Manager Leaderboard: users with manager role → [manager1, agent+manager2]

2. Query expands filter to BOTH agent_user_id AND manager_user_id
   - WHERE (agent_user_id IN (...)) OR (manager_user_id IN (...))

3. Two CTEs produce results:
   - agent_live_assist_stats: Groups by agent_user_id
   - manager_live_assist_stats: Groups by manager_user_id

4. UNION ALL combines both result sets

5. Frontend picks relevant metrics
```

---

## Solution Options

### Option 1: Add Request Parameter (Recommended)

**Approach**: Add a boolean parameter to the proto to distinguish use cases.

**Proto Change**:
```protobuf
message RetrieveLiveAssistStatsRequest {
  // ... existing fields
  bool filter_to_agents_only = N;  // New field
}
```

**Backend Logic**:
```go
// Determine listAgentOnly based on request parameter
listAgentOnly := req.GetFilterToAgentsOnly()  // Default: false (backward compatible)

// Use in ListUsersMappedToGroups
userNameToGroupNamesMap, groupsToAggregate, users, err = shared.ListUsersMappedToGroups(
    ctx,
    a.userClientRegistry,
    a.internalUserServiceClientRegistry,
    parent.CustomerID,
    parent.ProfileID,
    req.FilterByAttribute.GetGroups(),
    hasAgentAsGroupByKey,
    includeDirectGroupMembershipsOnly,
    a.enableListUsersCache,
    *a.listUsersCache,
    listAgentOnly,  // ← Use request parameter
)

// Use in MoveFiltersToUserFilter
req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(
    ctx,
    a.userClientRegistry,
    a.internalUserServiceClientRegistry,
    parent.CustomerID,
    parent.ProfileID,
    req.FilterByAttribute,
    includeDirectGroupMembershipsOnly,
    listAgentOnly,  // ← Use request parameter
)
```

**Frontend Changes**:
```typescript
// Agent Leaderboard
const agentLiveAssistStats = useLiveAssistStats(
  { ...insightsRequestParams, filterToAgentsOnly: true },  // ← Set to true
  !preSelectionOfFiltersCompleted
);

// Team Leaderboard
const agentLiveAssistStats = useLiveAssistStats({
  ...groupByTeamParamsForAgentStats,
  filterToAgentsOnly: true  // ← Set to true
});

// Manager Leaderboard (no change needed)
const managerLiveAssistStats = useLiveAssistStats(
  managerStatsRequestParams,  // filterToAgentsOnly defaults to false
  filterByManagerUsers.userNames.length === 0
);
```

**Pros**:
- ✅ Backward compatible (defaults to `false`)
- ✅ Explicit intent in API contract
- ✅ Single API endpoint maintained
- ✅ Minimal code changes
- ✅ Frontend controls filtering behavior

**Cons**:
- ⚠️ Requires frontend changes
- ⚠️ Requires proto change

---

### Option 2: Split into Two APIs

**Approach**: Create separate APIs for different purposes.

**New APIs**:
- `RetrieveAgentLiveAssistStats` (listAgentOnly = true)
- `RetrieveManagerLiveAssistStats` (listAgentOnly = false)

**Pros**:
- ✅ Clear separation of concerns
- ✅ Type-safe - can't misuse
- ✅ Easier to maintain

**Cons**:
- ❌ Breaking change
- ❌ Duplicate code
- ❌ More APIs to maintain
- ❌ Frontend needs significant changes

---

### Option 3: Infer from GroupBy or Filters

**Approach**: Detect intent based on request parameters.

**Logic**:
```go
// Infer listAgentOnly based on heuristics
listAgentOnly := false  // Default for manager use case

// If grouping by agents or groups → assume agent leaderboard
if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) ||
   shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
    listAgentOnly = true
}

// If user filter contains only users with MANAGER role → assume manager leaderboard
// (Would need to check user roles)
```

**Pros**:
- ✅ No proto changes
- ✅ No frontend changes
- ✅ Transparent to callers

**Cons**:
- ❌ Heuristics are fragile
- ❌ Hard to maintain
- ❌ May not cover all cases
- ❌ Hidden magic behavior

---

### Option 4: Don't Update (Status Quo)

**Approach**: Keep current implementation with `listAgentOnly = false`.

**Pros**:
- ✅ No changes needed
- ✅ No breaking changes

**Cons**:
- ❌ Bug remains: Agent/Team leaderboards include Agent+Manager users
- ❌ Inconsistent with other refactored APIs
- ❌ Incorrect metrics for agent leaderboards

---

## Recommended Implementation

### Phase 1: Backend Implementation

**Step 1: Update to use ParseUserFilterForAnalytics**

Create branch: `convi-XXXX-fix-retrieveliveasiststats`

**File**: `retrieve_live_assist_stats.go`

```go
func (a AnalyticsServiceImpl) RetrieveLiveAssistStats(
    ctx context.Context, req *analyticspb.RetrieveLiveAssistStatsRequest,
) (*analyticspb.RetrieveLiveAssistStatsResponse, error) {
    // ... validation ...

    parent, err := shared.ParseCustomerProfile(req.Parent)
    if err != nil {
        return nil, err
    }

    var (
        users                             []*userpb.User
        groupsToAggregate                 []*userpb.Group
        userNameToGroupNamesMap           map[string][]string
        hasAgentAsGroupByKey              = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
        includeDirectGroupMembershipsOnly = shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
    )

    if a.enableParseUserFilterForAnalytics {
        // NEW: Use ParseUserFilterForAnalytics
        // Use req.GetFilterToAgentsOnly() to determine listAgentOnly
        listAgentOnly := req.GetFilterToAgentsOnly()  // Default: false
        shouldMoveFiltersToUserFilter := req.FilterByAttribute != nil &&
            (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers)
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
            listAgentOnly,  // ← Use request parameter
            req.GetIncludePeerUserStats(),
            shouldMoveFiltersToUserFilter,
            excludeDeactivatedUsers,
        )
        if err != nil {
            return nil, err
        }

        // Extract results
        userNameToGroupNamesMap = result.UserNameToGroupNamesMap
        groupsToAggregate = result.GroupsToAggregate
        users = result.UsersFromGroups
        req.FilterByAttribute.Users = result.FinalUsers
        req.FilterByAttribute.Groups = result.FinalGroups

        // Early return if no users
        if len(result.FinalUsers) == 0 {
            return &analyticspb.RetrieveLiveAssistStatsResponse{}, nil
        }
    } else {
        // OLD: Keep legacy implementation
        req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(...)
        // ... rest of old code ...
    }

    // Continue with query execution
    if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
        return a.retrieveLiveAssistStatsInternalForGroupsInSingleQuery(ctx, req, parent, hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly, users, groupsToAggregate, userNameToGroupNamesMap)
    }
    return a.retrieveLiveAssistStatsInternal(ctx, req, parent, users, includeDirectGroupMembershipsOnly)
}

// Update internal function to skip redundant MoveFiltersToUserFilter
func (a AnalyticsServiceImpl) retrieveLiveAssistStatsInternal(
    ctx context.Context,
    req *analyticspb.RetrieveLiveAssistStatsRequest,
    parent *shared.CustomerProfile,
    users []*userpb.User,
    includeDirectGroupMembershipsOnly bool,
) (*analyticspb.RetrieveLiveAssistStatsResponse, error) {
    _, err := postgres.ParseArgs(&req.Frequency, req.Metadata)
    if err != nil {
        return nil, err
    }

    if !a.enableParseUserFilterForAnalytics {
        // OLD: Only call MoveFiltersToUserFilter if using legacy path
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
                return &analyticspb.RetrieveLiveAssistStatsResponse{}, nil
            }
        }
    }
    // If using new path, skip MoveFiltersToUserFilter (already done in ParseUserFilterForAnalytics)

    return a.readLiveAssistStatsFromClickhouse(ctx, req, users)
}
```

**Step 2: Proto Change**

**File**: `cresta-proto/cresta/v1/analytics/live_assist_stats.proto`

```protobuf
message RetrieveLiveAssistStatsRequest {
  // ... existing fields ...

  // If true, filters to users with ONLY the agent role (excludes Agent+Manager users).
  // Used for Agent and Team Leaderboards to track agents receiving assistance.
  // If false (default), includes users with manager role (can have multiple roles).
  // Used for Manager Leaderboard to track managers giving assistance.
  bool filter_to_agents_only = N;  // Choose next available field number
}
```

### Phase 2: Frontend Implementation

**File**: `director/packages/director-app/src/pages/insights/leaderboard/agent-leaderboard/AgentLeaderboardPage.tsx`

```typescript
// Line 96: Add filterToAgentsOnly parameter
const agentLiveAssistStats = useLiveAssistStats(
  { ...insightsRequestParams, filterToAgentsOnly: true },
  !preSelectionOfFiltersCompleted
);
```

**File**: `director/packages/director-app/src/pages/insights/leaderboard/team-leaderboard/TeamLeaderboardPage.tsx`

```typescript
// Line 114: Add filterToAgentsOnly parameter
const agentLiveAssistStats = useLiveAssistStats({
  ...groupByTeamParamsForAgentStats,
  filterToAgentsOnly: true
});
```

**File**: Manager Leaderboard (no changes needed - defaults to false)

### Phase 3: Testing

**Backend Tests**:

1. **Test Agent Leaderboard scenario** (filterToAgentsOnly = true)
   - Request with agent filter + filterToAgentsOnly=true
   - Verify only agent-only users returned
   - Verify Agent+Manager users excluded

2. **Test Team Leaderboard scenario** (filterToAgentsOnly = true)
   - Request with group filter + filterToAgentsOnly=true
   - Verify only agent-only users in groups returned

3. **Test Manager Leaderboard scenario** (filterToAgentsOnly = false)
   - Request with manager filter + filterToAgentsOnly=false
   - Verify users with manager role returned (including Agent+Manager)

4. **Test backward compatibility** (filterToAgentsOnly not set)
   - Request without filterToAgentsOnly field
   - Verify defaults to false (manager behavior)

**Integration Tests**:
- Verify Agent Leaderboard shows correct users
- Verify Team Leaderboard shows correct users
- Verify Manager Leaderboard still works
- Verify metrics are correct for each leaderboard

---

## Testing Strategy

### Unit Tests

```go
func TestRetrieveLiveAssistStats(t *testing.T) {
    tests := []struct {
        name               string
        filterToAgentsOnly bool
        groundTruth        []*internaluserpb.LiteUser
        expectedUsers      []string
    }{
        {
            name:               "Agent Leaderboard - filters to agent-only",
            filterToAgentsOnly: true,
            groundTruth: []*internaluserpb.LiteUser{
                {UserId: "agent1", Roles: []string{"AGENT"}},
                {UserId: "agent-manager2", Roles: []string{"AGENT", "MANAGER"}},
                {UserId: "agent3", Roles: []string{"AGENT"}},
            },
            expectedUsers: []string{"agent1", "agent3"},  // Excludes agent-manager2
        },
        {
            name:               "Manager Leaderboard - includes agent+manager",
            filterToAgentsOnly: false,
            groundTruth: []*internaluserpb.LiteUser{
                {UserId: "manager1", Roles: []string{"MANAGER"}},
                {UserId: "agent-manager2", Roles: []string{"AGENT", "MANAGER"}},
                {UserId: "agent3", Roles: []string{"AGENT"}},
            },
            expectedUsers: []string{"manager1", "agent-manager2"},  // Includes agent-manager2
        },
        {
            name:               "Default behavior (backward compat)",
            filterToAgentsOnly: false,  // Default when field not set
            groundTruth: []*internaluserpb.LiteUser{
                {UserId: "agent1", Roles: []string{"AGENT"}},
                {UserId: "agent-manager2", Roles: []string{"AGENT", "MANAGER"}},
            },
            expectedUsers: []string{"agent1", "agent-manager2"},  // Includes all
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mocks
            // Call RetrieveLiveAssistStats
            // Verify expectedUsers
        })
    }
}
```

---

## Summary

### The Problem
RetrieveLiveAssistStats is a dual-purpose API:
- **Agent/Team Leaderboards**: Need `listAgentOnly = true` (currently wrong)
- **Manager Leaderboard**: Need `listAgentOnly = false` (currently correct)

### The Solution
Add `filter_to_agents_only` parameter to proto:
- Agent/Team frontends: Set to `true`
- Manager frontend: Defaults to `false` (no change needed)
- Backend: Use parameter to determine `listAgentOnly`

### Implementation Steps
1. ✅ Add proto field `filter_to_agents_only`
2. ✅ Update backend to use `ParseUserFilterForAnalytics` with conditional `listAgentOnly`
3. ✅ Update Agent Leaderboard frontend to set `filterToAgentsOnly: true`
4. ✅ Update Team Leaderboard frontend to set `filterToAgentsOnly: true`
5. ✅ Manager Leaderboard: No changes (defaults to false)
6. ✅ Add comprehensive tests
7. ✅ Feature flag for safe rollout

### Timeline
- Backend changes: 4-6 hours
- Proto change + regenerate: 1 hour
- Frontend changes: 2-3 hours
- Testing: 3-4 hours
- Total: 10-14 hours (1.5-2 days)

### Risk Assessment
- **Low risk**: Backward compatible (defaults to false)
- **Safe rollout**: Feature flag controlled
- **Clear intent**: Explicit parameter in API contract
- **Easy rollback**: Can disable feature flag
