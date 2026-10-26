# Analytics User Filtering: Comprehensive Investigation

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [User Filter Flow Architecture](#user-filter-flow-architecture)
3. [Key Components Deep Dive](#key-components-deep-dive)
4. [Agent-to-Team Mapping](#agent-to-team-mapping)
5. [Conditional Filter Processing](#conditional-filter-processing)
6. [FilterByAttribute Usage Patterns](#filterbyattribute-usage-patterns)
7. [Root vs Limited Access](#root-vs-limited-access)
8. [Performance Considerations](#performance-considerations)
9. [Testing Strategy](#testing-strategy)

---

## Executive Summary

This document consolidates the investigation of user filtering logic across Analytics Service APIs. It covers:

- **Architecture**: How user and group filters flow through the system
- **Components**: Key functions (`ApplyResourceACL`, `ListUsersMappedToGroups`, `MoveFiltersToUserFilter`)
- **Team Mapping**: How agents are mapped to teams for leaderboard aggregation
- **Optimization**: When filters are processed and when they're skipped
- **Patterns**: Common implementation patterns across 21+ Analytics APIs

### Key Findings

1. **Separation of Concerns**: Two separate user lists serve different purposes
   - `users` parameter → Response construction and metadata enrichment
   - `req.FilterByAttribute.Users` → Query filtering in Clickhouse WHERE clauses

2. **Conditional Processing**: `MoveFiltersToUserFilter` is conditionally called
   - Only when groups need expansion OR deactivated users need filtering
   - Skipped entirely when filters already contain explicit users

3. **Backend Team Attribution**: Team membership determined server-side
   - User Service API provides agent-to-team mappings
   - Clickhouse queries are team-agnostic (group by `agent_user_id` only)
   - Post-query in-memory aggregation combines agent stats into team stats

4. **ACL Bug Discovery**: Empty ACL users case not properly handled
   - Limited access users could see ALL agents' data incorrectly
   - Fixed by using `listAgentOnly` parameter consistently

---

## User Filter Flow Architecture

### High-Level Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. Frontend Request                                              │
│    - FilterByAttribute.Users = [user1, user2]                   │
│    - FilterByAttribute.Groups = [group1, group2]                │
│    - ExcludeDeactivatedUsers = true/false                       │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. ApplyResourceACL                                              │
│    - Filters users/groups based on caller's ACL permissions     │
│    - Output: req.FilterByAttribute.Users = [user1] (filtered)  │
│    - Output: req.FilterByAttribute.Groups = [group1] (filtered)│
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. ListUsersMappedToGroups (OPTIONAL)                           │
│    - Called only if grouping by agent or group                  │
│    - Calls Internal User Service API                            │
│    - Returns:                                                    │
│      * users: [user3, user4] from groups                        │
│      * userNameToGroupNamesMap: {"user3": ["group1"], ...}      │
│      * groupsToAggregate: [group1, ...]                         │
│    - Purpose: Response construction, NOT query filtering        │
│    - Does NOT modify req.FilterByAttribute                      │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. MoveFiltersToUserFilter (CONDITIONAL) ⭐                      │
│                                                                  │
│    IF (len(groups) > 0 OR exclude_deactivated_users):          │
│      - Expands groups to users: [user3, user4]                 │
│      - Filters deactivated users if needed                      │
│      - Merges: req.FilterByAttribute.Users = [user1, user3, user4] │
│      - Clears: req.FilterByAttribute.Groups = nil              │
│    ELSE:                                                         │
│      - SKIP (use existing user filter as-is)                   │
│                                                                  │
│    Purpose: Query filtering for Clickhouse WHERE clause        │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. Execute Clickhouse Query                                      │
│    - WHERE agent_user_id IN ('user1', 'user3', 'user4')        │
│    - GROUP BY agent_user_id, truncated_time                     │
│    - Returns per-agent stats                                     │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. Post-Query Aggregation (if grouping by teams)                │
│    - Uses userNameToGroupNamesMap from step 3                   │
│    - Aggregates per-agent stats into per-team stats             │
│    - Returns response with team attribution                      │
└─────────────────────────────────────────────────────────────────┘
```

### Key Data Structures

#### Request Input
```protobuf
message Attribute {
  repeated User users = 1;              // Direct user filter
  repeated Group groups = 2;            // Group filter (teams)
  bool exclude_deactivated_users = 3;   // Filter inactive users
  GroupMembershipFilter group_membership_filter = 4;  // Direct vs indirect
}
```

#### After Processing
```go
// Two separate user lists with different purposes:

// 1. For response construction (from ListUsersMappedToGroups)
users []*userpb.User                     // Users with full metadata
userNameToGroupNamesMap map[string][]string  // User → Teams mapping
groupsToAggregate []*userpb.Group        // Teams to aggregate

// 2. For query filtering (from MoveFiltersToUserFilter)
req.FilterByAttribute.Users []*userpb.User   // Final user list for WHERE clause
req.FilterByAttribute.Groups = nil           // Cleared after expansion
```

---

## Key Components Deep Dive

### 1. ApplyResourceACL

**File**: `insights-server/internal/shared/common.go`

**Purpose**: Filters users and groups based on caller's ACL permissions.

**Logic**:
```go
func ApplyResourceACL(
    ctx context.Context,
    configClient configpb.ConfigServiceClient,
    customerID string,
    resourceACLHelper *ResourceACLHelper,
    users []*userpb.User,
    groups []*userpb.Group,
    includePeerUserStats bool,
) ([]*userpb.User, []*userpb.Group, error)
```

**Returns**:
- Filtered `users` based on what the caller can view
- Filtered `groups` based on what the caller can view
- Empty lists if caller has no access (limited ACL with no managed resources)

**Key Behavior**:
- Root access users: Returns empty lists (meaning "no filtering")
- Limited access users: Returns only managed users/groups
- Respects `includePeerUserStats` for peer visibility

### 2. ListUsersMappedToGroups

**File**: `insights-server/internal/shared/common.go:779-905`

**Purpose**: Fetches users from groups and builds user-to-team mappings.

**External API Calls**:
- ✅ `ListUsersForAnalytics` (Internal User Service) - Gets users with memberships
- ✅ `ListGroups` (User Service) - Gets all groups for the profile

**Parameters**:
```go
func ListUsersMappedToGroups(
    ctx context.Context,
    userClientRegistry registry.Registry[userpb.UserServiceClient],
    internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
    customerID, profileID string,
    groupFilter []*userpb.Group,              // From ACL-filtered groups
    hasAgentAsGroupByKey bool,                // Grouping by agents?
    includeDirectGroupMembershipsOnly bool,   // Direct vs indirect members
    enableCache bool,
    cache ListUsersCache,
    listAgentOnly bool,                       // ⭐ Filter to agent-only users
) (map[string][]string, []*userpb.Group, []*userpb.User, error)
```

**Returns**:
- `userNameToGroupNamesMap`: Maps userName → list of groupNames
- `groups`: Teams to aggregate
- `users`: Users from groups with full metadata

**User Service API Request**:
```protobuf
message ListUsersForAnalyticsRequest {
  string customer_id = 1;
  string profile_id = 2;
  repeated string group_ids = 3;                    // From groupFilter
  bool agent_only = 4;                              // ⭐ listAgentOnly parameter
  bool include_inactive_users = 5;                  // Always true
  bool include_indirect_group_memberships = 6;      // Based on parameter
}
```

**Response Structure**:
```protobuf
message LiteUser {
  string user_id = 1;
  string full_name = 2;
  repeated UserGroupMembership memberships = 3;  // Contains team info
  string username = 4;
}

message UserGroupMembership {
  LiteGroup group = 1;              // Team/Group details
  bool is_direct_member = 2;        // Direct vs indirect membership
}
```

**Membership Filtering Logic**:
```go
for _, membership := range user.Memberships {
    groupID := membership.Group.GroupId

    // Skip root and default groups for team leaderboard
    if !hasAgentAsGroupByKey && (groupID == consts.RootGroupID || groupID == consts.DefaultGroupID) {
        continue
    }

    // Skip indirect members for agent leaderboard
    if (hasAgentAsGroupByKey || includeDirectGroupMembershipsOnly) && !membership.IsDirectMember {
        continue
    }

    // Only include TEAM type groups (skip virtual groups)
    if membership.Group.GroupType != internaluserpb.LiteGroup_TEAM {
        continue
    }

    groupName := userpb.GroupName{CustomerID: customerID, GroupID: groupID}.String()
    usersToGroups[userName] = append(usersToGroups[userName], groupName)
}
```

**Filtering Rules**:
1. **Skip root and default groups** for team leaderboard queries
2. **Skip indirect members** for agent leaderboard queries
3. **Only include TEAM type groups** (exclude virtual groups)

**Purpose**: This function is used for **response construction**, NOT query filtering. It provides:
- User metadata for enriching response attributes
- User-to-team mappings for post-query aggregation
- Group information for team leaderboards

### 3. MoveFiltersToUserFilter

**File**: `insights-server/internal/shared/common.go:532-617`

**Purpose**: Converts group filters and deactivated user filters into final user list for query filtering.

**External API Calls**:
- ✅ Calls `ListUsersForAnalytics` (via `ParseFiltersToUsers`)

**When It's Called**:
```go
if req.FilterByAttribute != nil &&
   (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
    req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(...)
}
```

**Condition**:
- Groups present: `len(req.FilterByAttribute.Groups) > 0`
- OR Deactivated filter: `req.FilterByAttribute.ExcludeDeactivatedUsers == true`

**When It's SKIPPED**:
- No groups AND no deactivation filtering needed
- Result: Existing `req.FilterByAttribute.Users` used as-is in query

**Logic Flow**:
```go
func MoveFiltersToUserFilter(
    ctx context.Context,
    userClientRegistry registry.Registry[userpb.UserServiceClient],
    internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
    customerID, profileID string,
    filterByAttribute *analyticspb.Attribute,
    includeDirectGroupMembershipsOnly bool,
    listAgentOnly bool,  // ⭐ NEW parameter
) (*analyticspb.Attribute, error) {

    // Case a) Empty user filter → fetch all users matching groups/deactivation
    if len(filterByAttribute.GetUsers()) == 0 {
        users, err := ParseFiltersToUsers(
            ctx, userClientRegistry, internalUserClientRegistry,
            customerID, profileID,
            filterByAttribute.GetGroups(),  // Groups to expand
            filterByAttribute.ExcludeDeactivatedUsers,
            includeDirectGroupMembershipsOnly,
            listAgentOnly,  // ⭐ Pass to ParseFiltersToUsers
        )
        filterByAttribute.Users = users
        filterByAttribute.Groups = nil  // Clear groups after expansion
        return filterByAttribute, nil
    }

    // Case b) Non-empty user filter
    // b1) Groups present → append group users to existing users
    if len(filterByAttribute.Groups) > 0 {
        groupUsers, err := ParseFiltersToUsers(
            ctx, userClientRegistry, internalUserClientRegistry,
            customerID, profileID,
            filterByAttribute.GetGroups(),
            false,  // Don't filter deactivated here
            includeDirectGroupMembershipsOnly,
            listAgentOnly,  // ⭐ Pass to ParseFiltersToUsers
        )
        filterByAttribute.Users = DedupUsers(append(filterByAttribute.Users, groupUsers...))
    }

    // b2) Exclude deactivated → filter existing users
    if filterByAttribute.ExcludeDeactivatedUsers {
        activeUsers, err := ParseFiltersToUsers(
            ctx, userClientRegistry, internalUserClientRegistry,
            customerID, profileID,
            nil,  // No groups
            true,  // Get active users only
            includeDirectGroupMembershipsOnly,
            listAgentOnly,  // ⭐ Pass to ParseFiltersToUsers
        )
        filterByAttribute.Users = FilterToActiveUsers(filterByAttribute.Users, activeUsers)
    }

    filterByAttribute.Groups = nil  // Clear groups after expansion
    return filterByAttribute, nil
}
```

**Why Groups Get Cleared**:
```go
filterByAttribute.Groups = nil
```

Reasons:
1. Groups have been converted to their member users
2. Prevents double-processing (querying groups twice)
3. Clickhouse queries only understand user IDs, not group IDs
4. Avoids confusion about whether group filters have been applied

**Purpose**: This function is used for **query filtering**, providing the final user list for Clickhouse WHERE clauses.

### 4. ParseFiltersToUsers

**File**: `insights-server/internal/shared/common.go:469-531`

**Purpose**: Lists users based on group filter and/or active user filter.

**External API Calls**:
- ✅ `ListUsersForAnalytics` (Internal User Service)

**Parameters**:
```go
func ParseFiltersToUsers(
    ctx context.Context,
    userClientRegistry registry.Registry[userpb.UserServiceClient],
    internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
    customerID, profileID string,
    groupFilter []*userpb.Group,
    activeUserFilter bool,
    includeDirectGroupMembershipsOnly bool,
    listAgentOnly bool,  // ⭐ NEW parameter
) ([]*userpb.User, error)
```

**API Request**:
```protobuf
message ListUsersForAnalyticsRequest {
  string customer_id = 1;
  string profile_id = 2;
  repeated string group_ids = 3;                    // From groupFilter
  bool agent_only = 4;                              // ⭐ listAgentOnly parameter
  bool include_inactive_users = 5;                  // !activeUserFilter
  bool include_indirect_group_memberships = 6;      // Based on parameter
}
```

**Returns**: List of users with `Name`, `Username`, `FullName` populated

### Two Variants for QA APIs

**`MoveGroupFilterToUserFilterForQA`** (common.go:618-723)
- For APIs using `analyticspb.QAAttribute` instead of `analyticspb.Attribute`
- Used by: `RetrieveQAScoreStats`, `RetrieveQAConversations`
- Comment in code: *"Same mechanism as MoveFiltersToUserFilter()"*
- Identical logic, just different protobuf types

---

## Agent-to-Team Mapping

### How Team Attribution Works

**TL;DR**: Team information is **determined in the backend**, not the frontend. The backend queries the User Service API to get agent-to-team mappings, then performs in-memory aggregation.

### Architecture

```
Frontend Request (group by agents/teams)
          ↓
[Insights Server - RetrieveAgentStats]
          ↓
[Call User Service API] → ListUsersForAnalytics RPC
          ↓                       ↓
    [User Service] ← Returns: LiteUser[] with memberships[]
          ↓
[Build usersToGroups map] (userName → groupNames[])
          ↓
[Query ClickHouse] → GROUP BY agent_user_id
          ↓
[ClickHouse] → Returns per-agent stats
          ↓
[Post-processing: Aggregate by Team]
    - Use usersToGroups map
    - Sum stats for all agents in each team
          ↓
[Return Response with Team Attribution]
```

### Clickhouse Query (No Team Information)

**Important**: The ClickHouse database queries do **NOT** include team information directly.

```sql
SELECT
  agent_user_id,
  truncated_time,
  COUNT(DISTINCT agent_user_id) AS total_agent_count,
  COUNT(DISTINCT agent_user_id) AS active_agent_count
FROM conversation_d
WHERE agent_user_id <> ''
  AND agent_user_id IN ('user1', 'user3', 'user4')  -- From FilterByAttribute.Users
GROUP BY agent_user_id, truncated_time
```

Key points:
- Groups by `agent_user_id` only
- **No team/group joins** at database level
- Uses `conversation_d`, `conversation_with_labels_d`, etc. tables
- WHERE clause uses `FilterByAttribute.Users` from `MoveFiltersToUserFilter`

### Post-Query Team Aggregation

**File**: `insights-server/internal/analyticsimpl/retrieve_agent_stats.go:199-255`

**Function**: `convertRowsPerUserToPerGroupAgentStatsResponse`

```go
func convertRowsPerUserToPerGroupAgentStatsResponse(
    perUserResp *analyticspb.RetrieveAgentStatsResponse,
    groups []*userpb.Group,
    userNameToGroupNamesMap map[string][]string,
) (*analyticspb.RetrieveAgentStatsResponse, error) {
    // Initialize results grouped by team
    agentStatsResultsByGroup := make(map[string]*analyticspb.AgentStatsResult)
    for _, group := range groups {
        group.Members = []*userpb.GroupMembership{}
        agentStatsResultsByGroup[group.Name] = &analyticspb.AgentStatsResult{
            Attribute: &analyticspb.Attribute{Groups: []*userpb.Group{group}},
        }
    }

    // Iterate through per-user stats from ClickHouse
    for _, agentStatsResult := range perUserResp.AgentStatsResults {
        if len(agentStatsResult.Attribute.Users) == 0 {
            continue
        }
        userName := agentStatsResult.Attribute.Users[0].GetName()

        // Find which teams this user belongs to
        if groupNames, exists := userNameToGroupNamesMap[userName]; exists {
            for _, groupName := range groupNames {
                if _, exists := agentStatsResultsByGroup[groupName]; exists {
                    // Aggregate stats from this user into their team(s)
                    agentStatsResultsByGroup[groupName].TotalAgentCount += agentStatsResult.TotalAgentCount
                    agentStatsResultsByGroup[groupName].ActiveAgentCount += agentStatsResult.ActiveAgentCount
                    // ... aggregate other metrics
                }
            }
        }
    }

    return response, nil
}
```

### Data Flow Example

#### Request
```
GET /analytics/agent-stats?group_by=teams&team_id=abc123
```

#### Backend Processing

**Step 1: Fetch User-to-Team Mappings**
```
Call: UserService.ListUsersForAnalytics
Result: {
  "alice": ["team-A", "team-B"],
  "bob": ["team-A"],
  "charlie": ["team-C"]
}
```

**Step 2: Query ClickHouse (Team-Agnostic)**
```sql
SELECT agent_user_id, COUNT(*) as conversations
FROM conversation_d
GROUP BY agent_user_id
```
Result:
```
alice: 100 conversations
bob: 50 conversations
charlie: 75 conversations
```

**Step 3: Aggregate by Team (In-Memory)**
```
team-A: 150 conversations (alice + bob)
team-B: 100 conversations (alice)
team-C: 75 conversations (charlie)
```

**Step 4: Return Response**
```json
{
  "team-A": {"conversations": 150},
  "team-B": {"conversations": 100},
  "team-C": {"conversations": 75}
}
```

### Key Findings

#### ✅ What Happens in the Backend

1. **Team information is looked up in the backend**, not passed from the frontend
2. **Agent-to-team mapping comes from the User Service API**
3. **ClickHouse queries are team-agnostic** - they only group by `agent_user_id`
4. **Post-query aggregation** combines agent stats into team stats using in-memory `usersToGroups` map
5. **Membership filtering** respects direct/indirect memberships and team type constraints

#### ❌ What Does NOT Happen

1. **No database joins** for team attribution
2. **No team information in ClickHouse queries**
3. **Frontend does NOT determine team membership**

---

## Conditional Filter Processing

### When MoveFiltersToUserFilter is Called

The function is **conditionally called** based on this pattern found in **21 Analytics API implementations**:

```go
if req.FilterByAttribute != nil &&
   (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
    req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(...)
}
```

### Three Execution Scenarios

#### Scenario 1: Direct User Filter (Function SKIPPED)
```
Request:
  filter_by_attribute.users = [user_1, user_2]
  filter_by_attribute.groups = []
  exclude_deactivated_users = false

→ MoveFiltersToUserFilter: SKIPPED
→ Clickhouse Query uses: [user_1, user_2] directly
```

**Why skip?** The user filter is already explicit. No groups to expand, no deactivation filtering needed.

#### Scenario 2: Group Filter Expansion (Function CALLED)
```
Request:
  filter_by_attribute.users = []
  filter_by_attribute.groups = [group_1, group_2]
  exclude_deactivated_users = false

→ MoveFiltersToUserFilter: CALLED
→ Expands groups to: [user_1, user_2, user_3, user_4]
→ Clickhouse Query uses: [user_1, user_2, user_3, user_4]
```

**Why call?** Clickhouse doesn't understand group membership. Must convert groups → users first.

#### Scenario 3: Combined Filter + Deactivation (Function CALLED)
```
Request:
  filter_by_attribute.users = [user_1, user_2]
  filter_by_attribute.groups = [group_1]
  exclude_deactivated_users = true

→ MoveFiltersToUserFilter: CALLED
→ Expands groups: [user_3, user_4]
→ Combines: [user_1, user_2, user_3, user_4]
→ Filters deactivated: [user_1, user_3] (user_2, user_4 inactive)
→ Clickhouse Query uses: [user_1, user_3]
```

**Why call?** Must expand groups AND filter out deactivated users.

### Why This Design?

#### Performance Optimization
- Avoids unnecessary external API calls (`ListUsersForAnalytics`) when not needed
- If filter already contains explicit users and no groups/deactivation filtering needed, skip the conversion

#### Query Compatibility
- Clickhouse doesn't support GROUP membership queries directly
- Must convert all group filters to explicit user lists before querying

#### Flexibility
- Supports three query modes:
  1. Filter by explicit users only
  2. Filter by groups (expanded to users)
  3. Combined filters with deactivation handling

#### Separation of Concerns
- `ListUsersMappedToGroups`: For response enrichment (metadata, group mappings)
- `MoveFiltersToUserFilter`: For query filtering (WHERE clause users)

---

## FilterByAttribute Usage Patterns

### Complete Data Flow

#### 1. After ApplyResourceACL Call

The `shared.ApplyResourceACL` function modifies `req.FilterByAttribute.Users` and `req.FilterByAttribute.Groups` to filter them based on the caller's ACL permissions:

```go
req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(...)
```

Result:
- `req.FilterByAttribute.Users`: ACL-filtered users
- `req.FilterByAttribute.Groups`: ACL-filtered groups

#### 2. Usage of FilterByAttribute.Groups

**a) Input to ListUsersMappedToGroups:**
```go
userNameToGroupNamesMap, groupsToAggregate, users, err = shared.ListUsersMappedToGroups(
    ...
    req.FilterByAttribute.GetGroups(),  // Used here ⭐
    ...
)
```

**b) Condition check before MoveFiltersToUserFilter:**
```go
if req.FilterByAttribute != nil &&
   (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
    req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(...)
}
```

**c) Inside MoveFiltersToUserFilter:**
- Converts group filters to user filters
- Fetches all users belonging to those groups
- **Clears the groups**: `filterByAttribute.Groups = nil`
- Adds those users to `filterByAttribute.Users`

```go
// From MoveFiltersToUserFilter
users, err := ParseFiltersToUsers(ctx, ..., filterByAttribute.GetGroups(), ...)
filterByAttribute.Users = append(filterByAttribute.Users, users...)
filterByAttribute.Groups = nil  // Groups are cleared! ⭐
```

**d) In per-group aggregation queries:**
Some APIs (like `RetrieveHintStats`, `RetrieveQAScoreStats`) check if groups filter exists before internal calls:

```go
if len(req.FilterByAttribute.Groups) > 0 {
    perUserReqForAll.FilterByAttribute.Users = append(
        perUserReqForAll.FilterByAttribute.Users,
        users...  // Add users from ListUsersMappedToGroups
    )
}
```

#### 3. Usage of FilterByAttribute.Users

**a) Early exit check:**
After `MoveFiltersToUserFilter`, many APIs check if the user list is empty:

```go
if len(req.FilterByAttribute.Users) == 0 {
    return &analyticspb.RetrieveXXXStatsResponse{}, nil
}
```

**b) Passed to Clickhouse query builders:**
The users list is passed to `parseClickhouseFilter` which extracts user IDs and builds SQL WHERE conditions:

```go
// In parseClickhouseFilter
usersCondAndArgs, err := buildUsersConditionAndArgs(attribute.Users, targetTables)

// In buildUsersConditionAndArgs
agentUserIDs := []string{}
for _, user := range users {
    userName, err := userpb.ParseUserName(user.Name)
    agentUserIDs = append(agentUserIDs, userName.UserID)
}
// Builds SQL: WHERE agent_user_id IN (agentUserIDs)
```

**c) Used in PostgreSQL queries:**
For APIs that query PostgreSQL directly (like `RetrieveCommentStats`, `RetrieveCoachingSessionStats`):

```go
filteredAgents, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
if len(filteredAgents) > 0 {
    commentStatsQuery = commentStatsQuery.Where("chat_comments.user_id IN ?",
        fn.MapKeyToSlice(filteredAgents))
}
```

### Visual Flow Diagram

```
1. Original Request
   ├─ FilterByAttribute.Users = [user1, user2]
   └─ FilterByAttribute.Groups = [group1, group2]

2. After ApplyResourceACL
   ├─ FilterByAttribute.Users = [user1] (filtered by ACL)
   └─ FilterByAttribute.Groups = [group1] (filtered by ACL)

3. After ListUsersMappedToGroups
   ├─ Returns: users from group1 = [user3, user4]
   └─ FilterByAttribute unchanged at this point

4. After MoveFiltersToUserFilter
   ├─ FilterByAttribute.Users = [user1, user3, user4] (merged)
   └─ FilterByAttribute.Groups = nil (cleared!)

5. In Clickhouse Query
   └─ WHERE agent_user_id IN ('user1', 'user3', 'user4')
```

### Important Pattern Across All APIs

```
1. ApplyResourceACL
   → Filter users/groups by ACL
   → Modifies req.FilterByAttribute.Users/Groups

2. ListUsersMappedToGroups
   → Get user-to-group mappings for response construction
   → Does NOT modify request

3. MoveFiltersToUserFilter
   → Convert groups to users
   → Clears Groups, updates Users
   → Used for query filtering

4. Query Execution
   → Uses only FilterByAttribute.Users
   → Groups already converted
```

### Why Two Separate User Lists?

| List | Source | Purpose | When Populated |
|------|--------|---------|----------------|
| `users` parameter | `ListUsersMappedToGroups` | Response construction, metadata enrichment | Only when grouping by agent/group |
| `req.FilterByAttribute.Users` | `MoveFiltersToUserFilter` | Query filtering (WHERE clause) | Always (or after ACL if no groups) |

Example from `RetrieveAgentStats`:
```go
// users: For response construction
userNameToGroupNamesMap, groupsToAggregate, users, err = shared.ListUsersMappedToGroups(...)

// req.FilterByAttribute.Users: For query filtering
if postgres.HasEnumValue(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
    return a.retrieveAgentStatsForGroups(
        ctx, req,
        hasAgentAsGroupByKey,
        users,  // ⭐ Response construction
        groupsToAggregate,
        userNameToGroupNamesMap
    )
}
return a.readAgentStatsFromClickhouse(
    ctx, req,  // ⭐ req.FilterByAttribute.Users used in WHERE clause
    users      // ⭐ Response construction
)
```

---

## Root vs Limited Access

### The Bug Discovery

When investigating ACL handling, we discovered a bug in how empty ACL users are handled:

#### Root Access + Empty Filter (CORRECT)
```
ApplyResourceACL: Returns empty aclUsers, empty aclGroups (no filtering)
ListUsersMappedToGroups: Called with empty groups → Lists ALL agents
MoveFiltersToUserFilter:
  - filterByAttribute.Users is empty (case a)
  - Calls ParseFiltersToUsers with empty groups
  - Returns ALL users (agents if listAgentOnly=true)
Result: Query filters by ALL agents ✅
```

#### Limited Access + No Managed Users (BUG - FIXED)
```
ApplyResourceACL: Returns empty aclUsers, empty aclGroups (filtered out)
ListUsersMappedToGroups: Called with empty groups → Lists ALL agents ❌
MoveFiltersToUserFilter:
  - filterByAttribute.Users is empty (case a)
  - Returns ALL users ❌
Problem: Limited access user sees ALL agents' data! 🐛
```

### The Fix: listAgentOnly Parameter

The bug was fixed by consistently using `listAgentOnly` parameter throughout the call chain:

```go
// In ApplyResourceACL
isRootAccess := (len(aclUsers) == 0 && len(aclGroups) == 0)

// In ListUsersMappedToGroups
// Pass listAgentOnly=true for agent activity APIs
userNameToGroupNamesMap, groupsToAggregate, users, err = shared.ListUsersMappedToGroups(
    ...
    listAgentOnly = true,  // ⭐ Filter to agent-only users
)

// In MoveFiltersToUserFilter
// Pass listAgentOnly=true to ParseFiltersToUsers
req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(
    ...
    listAgentOnly = true,  // ⭐ Filter to agent-only users
)

// In ParseFiltersToUsers
// Set AgentOnly flag in API request
request := &internaluserpb.ListUsersForAnalyticsRequest{
    ...
    AgentOnly: listAgentOnly,  // ⭐ Respected by User Service
}
```

### How to Distinguish Root vs Limited Access

```go
// Root access with empty filter:
//   - aclUsers is empty
//   - usersFromGroups contains all agents (or users matching listAgentOnly)
//   - Result: Show all agents' data

// Limited access with no managed users:
//   - aclUsers is empty
//   - usersFromGroups is ALSO empty (because listAgentOnly filters correctly)
//   - Result: Show no data (early return with empty response)
```

---

## Performance Considerations

### 1. Separation of Concerns

**Pro**: Clean architecture
- ClickHouse handles high-volume analytics aggregation
- User Service handles team membership logic
- No complex joins between different data sources

### 2. Caching Layer

**File**: `insights-server/internal/shared/common.go:795-800, 900-902`

The backend includes an **optional caching mechanism** (`ListUsersCache`) to avoid repeated calls to the User Service:

```go
type ListUsersCache struct {
    mu    sync.RWMutex
    cache map[string]*listUsersCacheEntry  // key: customerID:profileID
}

type listUsersCacheEntry struct {
    users      []*internaluserpb.LiteUser
    timestamp  time.Time
    ttl        time.Duration
}
```

**Benefits**:
- User-to-team mappings are cached
- Reduces User Service API calls
- Configurable TTL for cache invalidation

### 3. Trade-offs

**Pros**:
- ✅ Clean separation, easier to maintain
- ✅ ClickHouse queries remain simple and fast
- ✅ No database schema coupling between systems

**Cons**:
- ❌ Additional API call to User Service (mitigated by caching)
- ❌ In-memory aggregation required (potential memory usage for large result sets)
- ❌ Two separate user lists to maintain

### 4. Optimization: Conditional MoveFiltersToUserFilter

**Performance win**: Skip expensive User Service API call when not needed

```go
// BEFORE refactoring (always called):
req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(...)

// AFTER refactoring (conditional):
if req.FilterByAttribute != nil &&
   (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
    req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(...)
}
// If condition false: No API call, use existing users directly
```

**Impact**:
- Saves 1-2 external API calls per analytics query when filters contain explicit users
- Reduces latency for direct user filter queries
- Lower load on User Service

---

## Testing Strategy

### Test Coverage Requirements

When testing Analytics APIs, ensure coverage for these scenarios:

#### 1. ✅ Empty Filter (Root Access)
```go
Request:
  filter_by_attribute.users = []
  filter_by_attribute.groups = []
Expected: Returns all entities (agents, conversations, etc.)
```

#### 2. ✅ User Filter Only (No Groups, No Deactivation)
```go
Request:
  filter_by_attribute.users = [user1, user2]
  filter_by_attribute.groups = []
  exclude_deactivated_users = false
Expected:
  - MoveFiltersToUserFilter SKIPPED
  - Query uses [user1, user2] directly
```

#### 3. ✅ Group Filter Only
```go
Request:
  filter_by_attribute.users = []
  filter_by_attribute.groups = [group1, group2]
Expected:
  - MoveFiltersToUserFilter CALLED
  - Groups expanded to users: [user3, user4, user5]
  - Query uses expanded user list
```

#### 4. ✅ Deactivation Filter Only
```go
Request:
  filter_by_attribute.users = [user1, user2, user3]
  exclude_deactivated_users = true
Expected:
  - MoveFiltersToUserFilter CALLED
  - Deactivated users filtered out: [user1, user3] (user2 inactive)
  - Query uses filtered user list
```

#### 5. ✅ Combined Filters
```go
Request:
  filter_by_attribute.users = [user1]
  filter_by_attribute.groups = [group1]
  exclude_deactivated_users = true
Expected:
  - MoveFiltersToUserFilter CALLED
  - Groups expanded: [user2, user3]
  - Combined: [user1, user2, user3]
  - Deactivated filtered: [user1, user3]
```

#### 6. ✅ ACL Filtering
```go
Test Cases:
  a) Root access + empty filter → All entities
  b) Limited access + managed users → Only managed entities
  c) Limited access + no managed users → Empty response
  d) ACL + group expansion → Both boundaries respected
```

#### 7. ✅ Team Aggregation
```go
Test Cases:
  a) Group by agents → Per-agent stats
  b) Group by teams → Per-team stats (aggregated from agents)
  c) Group by agents + teams → Per-agent-per-team stats
  d) Verify userNameToGroupNamesMap used correctly
```

#### 8. ✅ listAgentOnly Parameter
```go
Test Cases:
  a) listAgentOnly=true → Only agent-only users returned
  b) listAgentOnly=false → Users with Agent role (can have other roles)
  c) Verify Agent+Manager users excluded when listAgentOnly=true
```

### Test Implementation Example

```go
func TestRetrieveAgentStats(t *testing.T) {
    tests := []struct {
        name                     string
        users                    []*userpb.User
        groups                   []*userpb.Group
        excludeDeactivatedUsers  bool
        expectMoveFiltersCalled  bool
        expectedUserCount        int
    }{
        {
            name:                    "Direct user filter - skip MoveFilters",
            users:                   []*userpb.User{user1, user2},
            groups:                  []*userpb.Group{},
            excludeDeactivatedUsers: false,
            expectMoveFiltersCalled: false,
            expectedUserCount:       2,
        },
        {
            name:                    "Group filter - call MoveFilters",
            users:                   []*userpb.User{},
            groups:                  []*userpb.Group{group1},
            excludeDeactivatedUsers: false,
            expectMoveFiltersCalled: true,
            expectedUserCount:       3,  // group1 has 3 members
        },
        {
            name:                    "Deactivation filter - call MoveFilters",
            users:                   []*userpb.User{user1, user2, user3},
            groups:                  []*userpb.Group{},
            excludeDeactivatedUsers: true,
            expectMoveFiltersCalled: true,
            expectedUserCount:       2,  // user2 is inactive
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation...
        })
    }
}
```

---

## APIs Using This Pattern

### Using MoveFiltersToUserFilter (19 APIs)
- RetrieveAgentStats
- RetrieveConversationStats
- RetrieveAssistanceStats
- RetrieveHintStats
- RetrieveLiveAssistStats
- RetrieveKnowledgeAssistStats
- RetrieveKnowledgeBaseStats
- RetrieveManagerStats
- RetrieveCoachingSessionStats
- RetrieveCommentStats
- RetrieveScorecardStats
- RetrieveSuggestionStats
- RetrieveSummarizationStats
- RetrieveSmartComposeStats
- RetrieveNoteTakingStats
- RetrieveGuidedWorkflowStats
- RetrieveManualQAStats
- RetrieveManualQAProgress
- RetrieveAdherences

### Using MoveGroupFilterToUserFilterForQA (2 APIs)
- RetrieveQAScoreStats
- RetrieveQAConversations

**Note**: The two variants have identical logic, just different protobuf types (`Attribute` vs `QAAttribute`).

---

## Related Files

### Core Implementation
- `insights-server/internal/shared/common.go`
  - `ApplyResourceACL` - ACL filtering
  - `ListUsersMappedToGroups` (line 724) - User-to-team mapping
  - `MoveFiltersToUserFilter` (line 535) - Group/deactivation filtering
  - `MoveGroupFilterToUserFilterForQA` (line 618) - QA variant
  - `ParseFiltersToUsers` (line 469) - User list fetching

### Example API Implementations
- `insights-server/internal/analyticsimpl/retrieve_agent_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_agent_stats_clickhouse.go`
- `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`

### Proto Definitions
- `cresta-proto/cresta/nonpublic/user/internal_user_service.proto`
  - `ListUsersForAnalytics` RPC
  - `LiteUser` message with memberships
- `cresta-proto/cresta/v1/analytics/analytics_service.proto`
  - `Attribute` message
  - `QAAttribute` message

---

## Key Takeaways

1. **Two Separate User Lists**:
   - `users` from `ListUsersMappedToGroups` → Response construction
   - `req.FilterByAttribute.Users` from `MoveFiltersToUserFilter` → Query filtering

2. **Conditional Processing**:
   - `MoveFiltersToUserFilter` only called when groups or deactivation filtering needed
   - Performance optimization to avoid unnecessary API calls

3. **Backend Team Attribution**:
   - User Service API provides agent-to-team mappings
   - Clickhouse queries are team-agnostic
   - Post-query in-memory aggregation for team stats

4. **ACL Handling**:
   - `ApplyResourceACL` filters users/groups by permissions
   - `listAgentOnly` parameter ensures correct agent filtering
   - Bug fix: Limited access users now correctly see empty results when they manage no users

5. **Groups Get Cleared**:
   - `filterByAttribute.Groups = nil` after `MoveFiltersToUserFilter`
   - Prevents double-processing and query confusion

6. **Consistent Pattern**:
   - All 21+ Analytics APIs follow the same flow
   - Standard refactoring can be applied uniformly

---

**Document Created**: 2026-01-14
**Consolidates**:
- `agent-team-grouping-analysis.md`
- `insights-api-user-filter-parsing.md`
- `move-group-filter-usage-analysis.md`
- `Summary: How FilterByAttribute are used.md`
