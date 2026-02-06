# User Filter Implementation Plan: Comprehensive Guide

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Solution Design: Ground Truth Pattern](#solution-design-ground-truth-pattern)
4. [Critical Bug Fix: Issue #10](#critical-bug-fix-issue-10)
5. [Implementation Approach](#implementation-approach)
6. [Testing Strategy](#testing-strategy)
7. [API Compatibility Analysis](#api-compatibility-analysis)
8. [Rollout Plan](#rollout-plan)
9. [Performance Impact](#performance-impact)

---

## Executive Summary

### The Problem

Analytics Service APIs had multiple critical bugs in user filtering logic:

1. **Role Filtering Bug**: Group expansion didn't respect role filters (agent-only), causing data leakage
2. **Union vs Intersection Bug (Issue #10)**: Users and groups were intersected instead of unioned
3. **Semantic Ambiguity**: Empty user lists had inconsistent meanings across different contexts
4. **Security Risk**: Limited access users could potentially see unauthorized data

### The Solution

**Ground Truth Pattern**: A multi-layered filtering approach that:
- Fetches all target-role entities upfront (ground truth)
- Applies ACL filtering with group expansion and union semantics
- Intersects all results with ground truth at every step
- Guarantees no unauthorized or wrong-role data can leak through

### Impact

- **8 APIs refactored** with new unified `ParseUserFilterForAnalytics` function
- **Issue #10 fixed** - Groups now correctly unioned with users
- **Security hardened** - Ground truth prevents data leakage
- **Tests enhanced** - 32 test cases covering all scenarios

---

## Problem Statement

### 1. The Role Filtering Bug

**Context**: Analytics APIs filter entities through a multi-step pipeline:
1. **ACL Filtering**: Restrict entities based on caller's permissions
2. **Group Expansion**: Expand group filters into individual entity lists
3. **Role Filtering**: Filter entities to specific roles (e.g., agents, managers)
4. **Additional Filtering**: Optional deactivation status, time-based filters

**The Bug**: When group expansion (Step 2) called an external service to list entities, it didn't apply role filtering.

**Example**:
```
Request: Get "Agent Stats" for group "Sales Team"
Bug: Returns stats for agents, managers, and visitors in Sales Team
Expected: Returns stats ONLY for agents in Sales Team
```

**Root Cause**: The group expansion step called `ListEntitiesFromGroups(groupIDs)` but didn't pass the `roleFilter` parameter.

**Impact**:
- **Data leakage**: Limited-access users could see entities of wrong roles
- **Wrong metrics**: Aggregations included non-role entities
- **Security issue**: Violated principle of least privilege

### 2. Union vs Intersection Bug (Issue #10)

**Context**: When both `reqUsers` and `reqGroups` are provided in a request, the semantic should be:
```
Final Users = (reqUsers ∪ users-from-reqGroups) ∩ ACL-allowed-users ∩ ground-truth
```

**The Bug**: Current implementation incorrectly computed:
```
Final Users = reqUsers ∩ users-from-reqGroups  ❌
```

**Example**:
```
Request:
  reqUsers = [alice, bob]
  reqGroups = [sales-team] containing [charlie, diana]

Expected (UNION): [alice, bob, charlie, diana]
Current (INTERSECTION): [alice, bob]  ❌ Missing charlie, diana!
```

**Root Cause**: Users from groups were expanded AFTER ACL filtering, causing them not to be included in the final result.

**Impact**:
- Missing users in query results
- Incorrect analytics metrics
- Frontend leaderboards showing incomplete data

### 3. Semantic Ambiguity: Empty User Lists

**Current Semantics**: Empty `req.FilterByAttribute.Users` means different things based on ACL state:
- ACL Disabled: "query all users"
- Root Access: "query all users"
- Limited Access with no managed users: Early return with empty response

**Problems**:
1. **Inconsistent Semantics**: Same input (empty list) has different meanings
2. **Security Risk**: If early return removed accidentally, empty list would query ALL users
3. **Scalability Issue**: To query "all users", must list thousands of user IDs in request

---

## Solution Design: Ground Truth Pattern

### Core Concept

**Fetch ALL entities of the target role ONCE upfront** and use this "ground truth" to filter ALL subsequent steps.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Step 0: Fetch Ground Truth                                  │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ FetchAllEntitiesByRole(roleFilter, statusFilter)        │ │
│ │ → Map<ID, Entity> (ALL entities matching criteria)      │ │
│ └─────────────────────────────────────────────────────────┘ │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 1: Apply ACL                                            │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ ApplyACL(callerID, requestedEntities)                   │ │
│ │ → (managedEntities, isRootAccess)                       │ │
│ └─────────────────────────────────────────────────────────┘ │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 2: Expand Groups & Union                                │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ if len(aclGroupIDs) > 0:                                │ │
│ │   usersFromGroups = ExpandGroups(aclGroupIDs)           │ │
│ │   usersFromGroups = Intersect(usersFromGroups, GT)      │ │
│ │   managedEntities = UNION(managedEntities, usersFromGroups) │ │
│ └─────────────────────────────────────────────────────────┘ │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 3: Intersect with Ground Truth                          │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ if isRootAccess && empty:                               │ │
│ │    managedEntities = AllValues(groundTruth)             │ │
│ │ else:                                                   │ │
│ │    managedEntities = Intersect(managed, groundTruth)    │ │
│ └─────────────────────────────────────────────────────────┘ │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 4: Optional Additional Filters & Intersect              │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ if additionalFiltersNeeded:                             │ │
│ │    result = ApplyFilters(managedEntities, filters)      │ │
│ │    result = Intersect(result, groundTruth) ✅            │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Key Properties

**Safety Guarantee**: No matter how many steps or external service calls, only entities in the ground truth can appear in results.

**Metadata Enrichment**: Ground truth provides canonical entity metadata, ensuring consistency across all outputs.

**Status Filtering**: Ground truth respects status filters (active/inactive), automatically propagating to all steps.

**Root Access Handling**: The `isRootAccess` flag distinguishes:
- **Root access + empty filter**: Return ALL entities from ground truth
- **Limited access + no managed entities**: Return EMPTY (correctly denying access)

### Implementation Pattern

#### Before (Buggy)

```go
func FilterEntitiesPipeline(
    callerID, roleFilter string,
    requestedEntities, requestedGroups []Entity,
) ([]Entity, error) {
    // Step 1: ACL filtering
    managedEntities := ApplyACL(callerID, requestedEntities)
    managedGroups := ApplyACL(callerID, requestedGroups)

    // Step 2: Group expansion (BUG: no role filter!)
    entitiesFromGroups := ExpandGroups(managedGroups)

    // Step 3: Role filtering (too late!)
    if roleFilter != "" {
        managedEntities = FilterByRole(managedEntities, roleFilter)
        // BUG: entitiesFromGroups NOT filtered!
    }

    return append(managedEntities, entitiesFromGroups...), nil
}
```

**Problem**: `entitiesFromGroups` bypasses role filtering.

#### After (Ground Truth Pattern)

```go
func FilterEntitiesPipeline(
    callerID, roleFilter string,
    requestedEntities, requestedGroups []Entity,
    statusFilter StatusFilter,
) ([]Entity, []Entity, error) {
    // Step 0: Fetch ground truth FIRST
    groundTruth := FetchAllEntitiesByRole(roleFilter, statusFilter)

    // Step 1: ACL filtering
    managedEntities, isRootAccess := ApplyACL(callerID, requestedEntities)
    managedGroups, _ := ApplyACL(callerID, requestedGroups)

    // Step 2: Expand groups & union & intersect
    if len(managedGroups) > 0 {
        entitiesFromGroups := ExpandGroups(managedGroups)
        entitiesFromGroups = Intersect(entitiesFromGroups, groundTruth)
        managedEntities = Union(managedEntities, entitiesFromGroups)
    }

    // Step 3: Intersect with ground truth
    if isRootAccess && len(managedEntities) == 0 {
        managedEntities = AllValues(groundTruth) // Root access
    } else {
        managedEntities = Intersect(managedEntities, groundTruth)
    }

    return managedEntities, entitiesFromGroups, nil
}

func Intersect(entities []Entity, groundTruth map[ID]Entity) []Entity {
    result := []Entity{}
    for _, entity := range entities {
        if canonical, exists := groundTruth[entity.ID]; exists {
            result = append(result, canonical) // Use enriched version
        }
    }
    return result
}
```

---

## Critical Bug Fix: Issue #10

### Problem Analysis

**Scenario**: When both `reqUsers` and `reqGroups` are provided, what should the final user list be?

**Expected Semantic**: UNION
```
Final Users = (reqUsers ∪ users-from-reqGroups) ∩ ACL-allowed ∩ ground-truth
```

**Current Buggy Behavior**:
```
Request: reqUsers=[alice, bob], reqGroups=[sales-team (contains charlie, diana)]

Step 1: Apply ACL
  - ACL filters reqUsers → filteredUsers=[alice, bob]
  - ACL filters reqGroups → filteredGroups=[sales-team]

Step 2: Intersect filteredUsers with ground truth
  - Result: finalUsers=[alice, bob]

Step 3: Expand groups to users
  - usersFromGroups=[charlie, diana]

Step 4: ??? No union logic
  - finalUsers stays [alice, bob]
  - usersFromGroups=[charlie, diana]

Result: Only [alice, bob] are used in query (missing charlie, diana!)
```

### Solution: Expand Groups Inside applyResourceACL

**Strategy**: Expand groups to users INSIDE `applyResourceACL` (after ACL filtering but before returning), then UNION with ACL-filtered users.

#### Case Analysis: When to Expand aclGroupIDs

**Case A: aclGroupIDs is non-empty**
- Caller has access to some groups
- **Action**: Expand those groups to users, then UNION with aclUserIDs
- **Example**:
  ```
  reqUsers=[alice], reqGroups=[sales-team]
  aclUserIDs = [alice]
  aclGroupIDs = [sales-team]
  Expand sales-team → [bob, charlie]
  Result: [alice] ∪ [bob, charlie] = [alice, bob, charlie] ✅
  ```

**Case B: aclGroupIDs is empty** (3 sub-cases)

**Case B-1: ACL disabled OR root access**
- Original groups passed through (not filtered by ACL)
- Empty aclGroupIDs because reqGroups was empty
- **Meaning**: No group filtering requested
- **Action**: Ignore aclGroupIDs
- **Result**: Use aclUserIDs (or all users if aclUserIDs is also empty)

**Case B-2: ACL enabled + limited access + reqGroups empty**
- Groups not requested
- GetAllGroupIDs() returns empty `[]`
- **Meaning**: Groups were not requested, no group filtering needed
- **Action**: Ignore aclGroupIDs
- **Result**: Use aclUserIDs

**Case B-3: ACL enabled + limited access + reqGroups non-empty + no access**
- ACL checks which groups caller has access to
- Caller has no access to any requested groups
- GetAllGroupIDs() returns empty `[]`
- **Meaning**: Caller has no access to requested groups
- **Action**: Don't expand (no groups to expand)
- **Result**: Use aclUserIDs (which may also be empty)

#### Implementation

**Modified Signature**:
```go
func applyResourceACL(
    ctx context.Context,
    configClient config.Client,
    customerID string,
    profileID string,  // NEW
    resourceACLHelper auth.ResourceACLHelper,
    filteredUsers []*userpb.User,
    filteredGroups []*userpb.Group,
    includePeerUserStats bool,
    // NEW parameters for group expansion
    userServiceClient userpb.UserServiceClient,  // NEW
    includeDirectGroupMembershipsOnly bool,      // NEW
    excludeDeactivatedUsers bool,                // NEW
    listAgentOnly bool,                          // NEW
    groundTruthUsers map[string]*internaluserpb.LiteUser,  // NEW
) ([]*userpb.User, []*userpb.Group, bool, bool, error)
```

**Core Logic**:
```go
func applyResourceACL(...) {
    // Existing ACL logic
    isACLEnabled := false
    isRootAccess := false

    if customerConfig.GetEnableAdvancedDataAccessControl() {
        isACLEnabled = true

        resourceACL, err := resourceACLHelper.GetResourceACL(ctx, customerID, aclOptions...)
        if err != nil {
            return nil, nil, isACLEnabled, isRootAccess, err
        }

        isRootAccess = resourceACL.IsRootAccess

        if !resourceACL.IsRootAccess {
            // Get ACL-filtered users
            aclUserIDs := resourceACL.GetAllUserIDs()
            filteredUsers = shared.ConvertUserIDsToUsers(customerID, aclUserIDs)

            // Get ACL-filtered groups
            aclGroupIDs := resourceACL.GetAllGroupIDs()
            filteredGroups = shared.ConvertGroupIDsToGroups(customerID, aclGroupIDs)

            // NEW: Expand groups to users and UNION with filteredUsers
            if len(filteredGroups) > 0 {
                usersFromGroups, err := expandGroupsToUsers(
                    ctx,
                    userServiceClient,
                    customerID,
                    profileID,
                    filteredGroups,
                    includeDirectGroupMembershipsOnly,
                    groundTruthUsers,  // Filter to ground truth
                )
                if err != nil {
                    return nil, nil, isACLEnabled, isRootAccess, err
                }

                // UNION: Merge users from groups with aclUserIDs
                filteredUsers = unionUsers(filteredUsers, usersFromGroups)
            }
        }
    }

    return filteredUsers, filteredGroups, isACLEnabled, isRootAccess, nil
}
```

**Helper Functions**:
```go
func expandGroupsToUsers(
    ctx context.Context,
    userServiceClient userpb.UserServiceClient,
    customerID string,
    profileID string,
    groups []*userpb.Group,
    includeDirectGroupMembershipsOnly bool,
    groundTruthUsers map[string]*internaluserpb.LiteUser,
) ([]*userpb.User, error) {
    // Call ListUsersMappedToGroups
    resp, err := userServiceClient.ListUsersMappedToGroups(...)
    if err != nil {
        return nil, err
    }

    // Filter to ground truth
    filteredUsers := []*userpb.User{}
    for _, user := range resp.Users {
        userID, _ := ConvertUserNameToID(user.Name)
        if _, exists := groundTruthUsers[userID]; exists {
            filteredUsers = append(filteredUsers, user)
        }
    }

    return filteredUsers, nil
}

func unionUsers(users1 []*userpb.User, users2 []*userpb.User) []*userpb.User {
    userMap := map[string]*userpb.User{}
    for _, user := range users1 {
        userMap[user.Name] = user
    }
    for _, user := range users2 {
        userMap[user.Name] = user
    }

    result := []*userpb.User{}
    for _, user := range userMap {
        result = append(result, user)
    }
    return result
}
```

### Fixed Behavior

```
Request: reqUsers=[alice, bob], reqGroups=[sales-team (contains charlie, diana)]

Step 1: Apply ACL
  - ACL filters reqUsers → aclUserIDs=[alice, bob]
  - ACL filters reqGroups → aclGroupIDs=[sales-team]
  - Expand sales-team → usersFromGroups=[charlie, diana]
  - UNION: filteredUsers = [alice, bob] ∪ [charlie, diana] = [alice, bob, charlie, diana]

Step 2: Intersect with ground truth
  - finalUsers = [alice, bob, charlie, diana] ∩ ground_truth

Result: All requested users included! ✅
```

---

## Implementation Approach

### Phase 1: Core Function Implementation (Completed)

#### 1.1 Create ParseUserFilterForAnalytics

**File**: `insights-server/internal/analyticsimpl/common_user_filter.go`

**Function Signature**:
```go
func ParseUserFilterForAnalytics(
    ctx context.Context,
    userClientRegistry registry.Registry[userpb.UserServiceClient],
    internalUserServiceClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
    configClient config.Client,
    resourceACLHelper auth.ResourceACLHelper,
    customerID string,
    profileID string,
    reqUsers []*userpb.User,
    reqGroups []*userpb.Group,
    hasAgentAsGroupByKey bool,
    includeDirectGroupMembershipsOnly bool,
    enableListUsersCache bool,
    listUsersCache shared.ListUsersCache,
    listAgentOnly bool,
    includePeerUserStats bool,
    shouldMoveFiltersToUserFilter bool,
    excludeDeactivatedUsers bool,
) (*ParseUserFilterForAnalyticsResult, error)
```

**Returns**:
```go
type ParseUserFilterForAnalyticsResult struct {
    UserNameToGroupNamesMap map[string][]string
    GroupsToAggregate       []*userpb.Group
    UsersFromGroups         []*userpb.User
    FinalUsers              []*userpb.User
    FinalGroups             []*userpb.Group
}
```

#### 1.2 Implementation Steps

**Step 0: Fetch Ground Truth**
```go
// Fetch ALL users matching role and status filters
groundTruthUsers, err := fetchGroundTruthUsers(
    ctx,
    internalUserClientRegistry,
    customerID,
    profileID,
    listAgentOnly,
    excludeDeactivatedUsers,
)
if err != nil {
    return nil, err
}
```

**Step 1: Apply ACL with Group Expansion**
```go
// Apply ACL and expand groups
filteredUsers, filteredGroups, isACLEnabled, isRootAccess, err := applyResourceACL(
    ctx,
    configClient,
    customerID,
    profileID,
    resourceACLHelper,
    reqUsers,
    reqGroups,
    includePeerUserStats,
    userServiceClient,
    includeDirectGroupMembershipsOnly,
    excludeDeactivatedUsers,
    listAgentOnly,
    groundTruthUsers,  // Pass ground truth for filtering
)
```

**Step 2: Handle Root Access**
```go
if isRootAccess && len(filteredUsers) == 0 {
    // Root access with no specific users: use all from ground truth
    filteredUsers = convertGroundTruthToUsers(groundTruthUsers, customerID)
}
```

**Step 3: Intersect with Ground Truth**
```go
filteredUsers = intersectUsersWithGroundTruth(filteredUsers, groundTruthUsers)
```

**Step 4: List Users Mapped to Groups (Optional)**
```go
if hasAgentAsGroupByKey || len(filteredGroups) > 0 {
    userNameToGroupNamesMap, groupsToAggregate, usersFromGroups, err = shared.ListUsersMappedToGroups(
        ctx,
        userClientRegistry,
        internalUserServiceClientRegistry,
        customerID,
        profileID,
        filteredGroups,
        hasAgentAsGroupByKey,
        includeDirectGroupMembershipsOnly,
        enableListUsersCache,
        listUsersCache,
        listAgentOnly,
    )
    if err != nil {
        return nil, err
    }

    // Intersect with ground truth
    usersFromGroups = intersectUsersWithGroundTruth(usersFromGroups, groundTruthUsers)
}
```

**Step 5: Move Filters (Optional)**
```go
if shouldMoveFiltersToUserFilter {
    // Apply additional filtering (deactivation, etc.)
    filteredUsers, err = applyAdditionalFilters(
        ctx,
        userClientRegistry,
        internalUserServiceClientRegistry,
        customerID,
        profileID,
        filteredUsers,
        filteredGroups,
        excludeDeactivatedUsers,
        includeDirectGroupMembershipsOnly,
        listAgentOnly,
    )
    if err != nil {
        return nil, err
    }

    // Intersect with ground truth
    filteredUsers = intersectUsersWithGroundTruth(filteredUsers, groundTruthUsers)
}
```

### Phase 2: API Migration (Completed)

Successfully migrated 8 APIs following the standard pattern:

1. **RetrieveAgentStats** (CONVI-5173) - Reference implementation
2. **RetrieveConversationStats** (CONVI-6005)
3. **RetrieveHintStats** (CONVI-6007)
4. **RetrieveKnowledgeAssistStats** (CONVI-6020)
5. **RetrieveSuggestionStats** (CONVI-6015)
6. **RetrieveSummarizationStats** (CONVI-6016)
7. **RetrieveSmartComposeStats** (CONVI-6017)
8. **RetrieveNoteTakingStats** (CONVI-6018)
9. **RetrieveGuidedWorkflowStats** (CONVI-6019)
10. **RetrieveKnowledgeBaseStats** (CONVI-6008)
11. **RetrieveQAScoreStats** (CONVI-6010)

**Standard Migration Pattern**:
```go
func (a AnalyticsServiceImpl) RetrieveXXXStats(
    ctx context.Context, req *analyticspb.RetrieveXXXStatsRequest,
) (*analyticspb.RetrieveXXXStatsResponse, error) {
    // ... validation ...

    if a.enableParseUserFilterForAnalytics {
        // New implementation using ParseUserFilterForAnalytics
        listAgentOnly := true
        shouldMoveFiltersToUserFilter := /* determine based on filters */
        excludeDeactivatedUsers := req.FilterByAttribute.GetExcludeDeactivatedUsers()

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

        // Extract results
        userNameToGroupNamesMap = result.UserNameToGroupNamesMap
        groupsToAggregate = result.GroupsToAggregate
        users = result.UsersFromGroups
        req.FilterByAttribute.Users = result.FinalUsers
        req.FilterByAttribute.Groups = result.FinalGroups

        // Early return if no users
        if len(result.FinalUsers) == 0 {
            return &analyticspb.RetrieveXXXStatsResponse{}, nil
        }
    } else {
        // Old implementation (backward compatibility)
        // ... existing 3-step process ...
    }

    // Continue with query execution
    // ...
}
```

### Phase 3: Feature Flag and Rollout

**Feature Flag**:
- Environment Variable: `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS`
- Default: `false` (uses legacy implementation)
- Purpose: Safe rollout, easy rollback

**Rollout Strategy**:
1. Deploy with flag disabled (legacy behavior)
2. Enable in staging environment
3. Validate metrics and leaderboard data
4. Enable in production gradually
5. Remove legacy code after successful rollout

---

## Testing Strategy

### Current Test Coverage (32 tests)

#### Category 1: ACL Filtering (5 tests)
- ✅ RootAccessReturnsAllAgents
- ✅ LimitedAccessFiltersToAgentSubset
- ✅ EmptyACLReturnsEmpty
- ✅ NoManagedUsersWithAuthUserAsAgent
- ✅ OnlyNonAgentsReturnsEmpty

#### Category 2: ACL Disabled (3 tests)
- ✅ EmptyRequestReturnsAllAgents
- ✅ WithUserFilterReturnsFilteredAgents
- ✅ WithNonAgentFilterReturnsEmpty

#### Category 3: Agent Filtering (2 tests)
- ✅ ListAgentOnlyTrueFiltersNonAgents
- ✅ ListAgentOnlyFalseKeepsAllUsers

#### Category 4: Group Filtering (2 tests)
- ✅ GroupFilterIntersectedWithACL
- ✅ EmptyGroupFilterWithRootAccess

#### Category 5: Metadata Enrichment (2 tests)
- ✅ EnrichedCorrectly
- ✅ MissingMetadataHandledGracefully

#### Category 6: Edge Cases (4 tests)
- ✅ EmptyRequestWithRootAccess
- ✅ PaginationHandling
- ✅ UserAndGroupFiltersTogether (Updated for Issue #10)
- ✅ EarlyReturnOnEmptyACLUsers

#### Category 7: Include Peer User Stats (2 tests)
- ✅ IncludedWhenFlagTrue
- ✅ ExcludedWhenFlagFalse

#### Category 8: Error Handling (2 tests)
- ✅ ApplyResourceACLFails
- ✅ ListUsersForAnalyticsFails

#### Category 9: Move Filters to User Filter (3 tests)
- ✅ WithGroupsExpansion
- ✅ WithEmptyUsersAndGroups
- ✅ IntersectsWithGroundTruth

#### Category 10: Exclude Deactivated Users (3 tests)
- ✅ TrueFiltersDeactivated
- ✅ FalseIncludesAll
- ✅ WithMoveFilters

#### Category 11: Issue #10 Union Tests (6 new tests)
- ✅ UserAndGroupFiltersTogether (Updated)
- ✅ UsersAndGroupsUnionWithLimitedAccess
- ✅ EmptyUsersWithGroupsReturnsGroupUsers
- ✅ MultipleGroupsUnioned
- ✅ GroupsWithNoAccessReturnsEmpty
- ✅ RootAccessUnionsUsersAndGroups

### Critical Test Scenarios

#### Test 1: Issue #10 UNION vs INTERSECTION (CRITICAL)

**Scenario**: When ACL provides BOTH user filters AND group filters, they should be UNIONED

```go
s.Run("UserAndGroupFiltersTogether", func() {
    // reqUsers=[alice], reqGroups=[sales-team] containing [bob, charlie]
    // Expected: [alice, bob, charlie] (UNION of users and users-from-groups)

    reqUsers := []*userpb.User{
        {Name: userpb.UserName{CustomerID: s.customerID, UserID: "alice"}.String()},
    }
    reqGroups := []*userpb.Group{
        {Name: userpb.GroupName{CustomerID: s.customerID, GroupID: "sales-team"}.String()},
    }

    allAgents := []*internaluserpb.LiteUser{
        {UserId: "alice", Username: "alice", FullName: "Alice Agent"},
        {UserId: "bob", Username: "bob", FullName: "Bob Agent"},
        {UserId: "charlie", Username: "charlie", FullName: "Charlie Agent"},
        {UserId: "diana", Username: "diana", FullName: "Diana Agent"},
    }

    usersFromSalesTeam := []*internaluserpb.LiteUser{
        {UserId: "bob", Username: "bob", FullName: "Bob Agent"},
        {UserId: "charlie", Username: "charlie", FullName: "Charlie Agent"},
    }

    s.setupACLMock(false, reqUsers, reqGroups)
    s.setupAllUsersMock(allAgents, true)
    s.setupListUsersMappedToGroupsMock(usersFromSalesTeam)

    result, err := s.callParseUserFilterForAnalytics(...)

    s.NoError(err)
    s.Len(result.FinalUsers, 3, "Should return UNION: alice + bob + charlie")

    userIDs := make(map[string]bool)
    for _, user := range result.FinalUsers {
        userName, _ := userpb.ParseUserName(user.Name)
        userIDs[userName.UserID] = true
    }
    s.True(userIDs["alice"], "Should include alice from user filter")
    s.True(userIDs["bob"], "Should include bob from sales-team group")
    s.True(userIDs["charlie"], "Should include charlie from sales-team group")
    s.False(userIDs["diana"], "Should NOT include diana (not in filters)")
})
```

**This test will FAIL before the fix** ❌
**This test will PASS after the fix** ✅

#### Test 2: Ground Truth Intersection

**Scenario**: All intermediate results must be intersected with ground truth

```go
s.Run("IntersectsWithGroundTruth", func() {
    // Request includes non-agent users
    // Ground truth only has agents
    // Expected: Non-agents filtered out

    reqUsers := []*userpb.User{
        {Name: userpb.UserName{CustomerID: s.customerID, UserID: "agent1"}.String()},
        {Name: userpb.UserName{CustomerID: s.customerID, UserID: "manager1"}.String()},
    }

    groundTruthAgents := []*internaluserpb.LiteUser{
        {UserId: "agent1", Username: "agent1", FullName: "Agent One"},
        // manager1 NOT in ground truth
    }

    s.setupACLMock(true, reqUsers, []*userpb.Group{})
    s.setupAllUsersMock(groundTruthAgents, true)

    result, err := s.callParseUserFilterForAnalytics(...)

    s.NoError(err)
    s.Len(result.FinalUsers, 1, "Should only include agent1 (manager1 filtered by ground truth)")
    s.Equal("agent1", getUserID(result.FinalUsers[0]))
})
```

#### Test 3: Multiple Groups with Overlapping Users

**Scenario**: UNION of multiple groups with deduplication

```go
s.Run("MultipleGroupsUnioned", func() {
    // reqGroups=[sales-team, eng-team]
    // sales-team: [alice, bob]
    // eng-team: [bob, charlie]
    // Expected: [alice, bob, charlie] (bob appears once)

    reqGroups := []*userpb.Group{
        {Name: userpb.GroupName{CustomerID: s.customerID, GroupID: "sales-team"}.String()},
        {Name: userpb.GroupName{CustomerID: s.customerID, GroupID: "eng-team"}.String()},
    }

    allAgents := []*internaluserpb.LiteUser{
        {UserId: "alice"}, {UserId: "bob"}, {UserId: "charlie"},
    }

    usersFromGroups := []*internaluserpb.LiteUser{
        {UserId: "alice"}, {UserId: "bob"}, {UserId: "bob"}, {UserId: "charlie"},
    }

    s.setupACLMock(false, []*userpb.User{}, reqGroups)
    s.setupAllUsersMock(allAgents, true)
    s.setupListUsersMappedToGroupsMock(usersFromGroups)

    result, err := s.callParseUserFilterForAnalytics(...)

    s.NoError(err)
    s.Len(result.FinalUsers, 3, "Should deduplicate bob")
})
```

### Test Execution Strategy

#### Phase 1: Baseline Testing
```bash
# Run all existing tests
bazel test //insights-server/internal/analyticsimpl:common_user_filter_test
# Expected: All 27 existing tests pass
```

#### Phase 2: Add New Tests
```bash
# Add 6 new tests for Issue #10
# Update UserAndGroupFiltersTogether test
# Expected: 6 new tests FAIL (demonstrate bug)
```

#### Phase 3: Implement Fix
```bash
# Implement Issue #10 fix in applyResourceACL
# Run tests again
# Expected: All 32 tests PASS
```

#### Phase 4: Integration Testing
```bash
# Run all API tests
bazel test //insights-server/internal/analyticsimpl:retrieve_agent_stats_test
bazel test //insights-server/internal/analyticsimpl:retrieve_conversation_stats_test
# ... for all 8 migrated APIs
```

---

## API Compatibility Analysis

### Compatibility Matrix

| # | API | Compatible | Data Source | Special Handling |
|---|-----|-----------|-------------|------------------|
| 1 | RetrieveAgentStats | ✅ Yes | Clickhouse | Reference implementation |
| 2 | RetrieveConversationStats | ✅ Yes | Clickhouse | None |
| 3 | RetrieveAssistanceStats | ✅ Yes | Postgres + CH | Hybrid data source merge |
| 4 | RetrieveHintStats | ✅ Yes | Clickhouse | Multiple grouping patterns |
| 5 | RetrieveKnowledgeAssistStats | ✅ Yes | Clickhouse | None |
| 6 | RetrieveLiveAssistStats | ⚠️ Special | Clickhouse | Dual-purpose (agent/manager) |
| 7 | RetrieveQAScoreStats | ✅ Yes | Clickhouse | Agent tier aggregation |
| 8 | RetrieveCoachingSessionStats | ⚠️ Partial | Postgres (GORM) | Needs GORM refactoring |
| 9 | RetrieveCommentStats | ⚠️ Partial | Postgres (GORM) | Needs GORM refactoring |
| 10 | RetrieveScorecardStats | ✅ Yes | Postgres/CH | Dual data source |

### Special Cases

#### RetrieveLiveAssistStats (Dual-Purpose API)

**Why Special**: Serves two distinct purposes:
- **Agent/Team Leaderboards**: Track agents RECEIVING help (needs `listAgentOnly = true`)
- **Manager Leaderboard**: Track managers GIVING help (needs `listAgentOnly = false`)

**Recommendation**: Add `filter_to_agents_only` parameter to distinguish use cases.

See: `.tmp/insights-user-filter/retrieve-live-assist-stats-analysis.md`

#### GORM-based APIs (CoachingSession, Comment)

**Current Pattern**:
```go
if len(filteredAgents) > 0 {
    query = query.Where("creator_user_id IN ?", fn.MapKeyToSlice(filteredAgents))
}
```

**Updated Pattern**:
```go
if len(filteredAgents) > 0 && !shouldQueryAllUsersInDB {
    query = query.Where("creator_user_id IN ?", fn.MapKeyToSlice(filteredAgents))
}
// If shouldQueryAllUsersInDB=true, skip WHERE clause, filter in app
```

---

## Rollout Plan

### Timeline

| Phase | Duration | Tasks |
|-------|----------|-------|
| Phase 1: Core Implementation | 2-3 weeks | ParseUserFilterForAnalytics, Issue #10 fix, tests |
| Phase 2: API Migration | 3-4 weeks | Migrate 8 APIs, integration tests |
| Phase 3: Testing & QA | 2 weeks | Comprehensive testing, bug fixes |
| Phase 4: Staging Rollout | 1 week | Deploy to staging, validate |
| Phase 5: Production Rollout | 2 weeks | Gradual enable in production |
| Phase 6: Cleanup | 1 week | Remove legacy code |

**Total**: 11-14 weeks

### Rollout Strategy

#### Week 1-3: Development (✅ Completed)
- ✅ Implement ParseUserFilterForAnalytics
- ✅ Implement Issue #10 fix
- ✅ Add comprehensive tests
- ✅ Code review and refinement

#### Week 4-7: API Migration (✅ Completed)
- ✅ Migrate 8 APIs to use new pattern
- ✅ Update internal functions to skip redundant filtering
- ✅ All tests passing

#### Week 8-9: QA
- ⏳ Integration testing
- ⏳ Performance testing
- ⏳ Security audit
- ⏳ Documentation

#### Week 10: Staging
- ⏳ Deploy to staging with flag disabled
- ⏳ Enable feature flag in staging
- ⏳ Validate metrics and leaderboards
- ⏳ Monitor for issues

#### Week 11-12: Production
- ⏳ Deploy to production with flag disabled
- ⏳ Enable flag for 10% of customers
- ⏳ Monitor metrics, error rates
- ⏳ Gradually increase to 50%, 100%

#### Week 13: Cleanup
- ⏳ Remove legacy code paths
- ⏳ Remove feature flag
- ⏳ Update documentation

---

## Performance Impact

### Scenario 1: Root Access with 5000 Agents

**Before**:
```sql
WHERE agent_user_id IN ('user1', 'user2', ..., 'user5000')
-- Huge IN clause, slow SQL parsing
```

**After**:
```sql
-- No WHERE clause
-- Faster query execution
-- Filter in app (no filtering needed for root access)
```

**Net Result**:
- ✅ Saves SQL parsing time
- ✅ No performance penalty (root access returns all users anyway)

### Scenario 2: Limited Access with 100 Managed Users (Out of 5000)

**Before**:
```sql
WHERE agent_user_id IN ('user1', ..., 'user100')  -- Returns 1,000 rows
```

**After**:
- **If using legacy path**: Same as before (WHERE clause used)
- **If using new path**: Same as before (WHERE clause used)

**Why**: Limited access doesn't trigger "query all users" mode.

**Conclusion**: New implementation is always optimal! ✅

### Memory Usage

**Ground Truth Map**:
- Size: ~100-200 bytes per user
- For 5000 users: ~500KB - 1MB
- Negligible compared to query results

**Trade-off**:
- ✅ Upfront cost: One additional API call to fetch ground truth
- ✅ Correctness: Guaranteed no data leakage
- ✅ Simplicity: Single source of truth

---

## Key Takeaways

1. **Fetch ground truth FIRST**: Don't rely on intermediate steps to apply critical filters
2. **Intersect EVERY step**: Any external call result must be intersected with ground truth
3. **UNION semantics**: Users and groups must be unioned, not intersected
4. **Test all paths**: Missing tests for optional parameters led to untested critical bugs
5. **Distinguish root vs limited access**: Use `isRootAccess` flag to handle empty filter cases
6. **Feature flag everything**: Safe rollout requires feature flags
7. **Monitor metrics**: Validate behavior with real data before full rollout

---

**Document Created**: 2026-01-14
**Status**: Implementation completed for 8 APIs, ready for QA and rollout
**Next Steps**: Complete testing phase, deploy to staging, monitor metrics
