# Detailed Comparison: Parse vs ParseUserFilterForAnalytics

## Function Signatures

### Parse (shared/user-filter)
```go
func (p *UserFilterParserImpl) Parse(
    ctx context.Context,
    customerID string,
    userFilterConditions *UserFilterConditions,
    userServiceClient userpb.UserServiceClient,
    configServiceClient config.Client,
    aclHelper auth.ResourceACLHelper,
    logger log.Logger,
) (*FilteredUsersAndGroups, error)
```

### ParseUserFilterForAnalytics (insights-server)
```go
func ParseUserFilterForAnalytics(
    ctx context.Context,
    userClientRegistry registry.Registry[userpb.UserServiceClient],
    internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
    configClient config.Client,
    aclHelper auth.ResourceACLHelper,
    customerID, profileID string,
    reqUsers []*userpb.User,
    reqGroups []*userpb.Group,
    hasAgentAsGroupByKey bool,
    includeDirectGroupMembershipsOnly bool,
    enableListUsersCache bool,  // UNUSED
    listUsersCache shared.ListUsersCache,  // UNUSED
    listAgentOnly bool,
    includePeerUserStats bool,
    shouldMoveFiltersToUserFilter bool,  // UNUSED
    excludeDeactivatedUsers bool,
) (*ParseUserFilterResult, error)
```

## Algorithm Comparison

### Parse Algorithm

```
1. Check if conditions are empty
   - If empty: Fetch ALL users with base filter
   - If not empty: Fetch users from selected users, virtual groups, team groups

2. Apply ACL filtering
   - If ACL disabled: Use all fetched users
   - If ACL enabled:
     - Get resource ACL
     - If root access: Use all fetched users
     - If limited access: Filter to ACL-allowed user IDs
     - Filter group memberships to ACL-allowed groups

3. Build result mappings from user.GroupMemberships
   - Skip root/default groups
   - Build UserNameToAllGroupNames, UserNameToDirectGroupNames
   - Build GroupNameToAllMembers, GroupNameToDirectMembers
```

### ParseUserFilterForAnalytics Algorithm

```
1. Fetch ground truth (ALL users matching criteria)
   - Call ListUsersForAnalytics with agentOnly, excludeDeactivated flags
   - Filter by reqGroups if specified

2. Apply ACL filtering (enhanced)
   - If ACL disabled: Pass through
   - If ACL enabled + root access: Pass through
   - If ACL enabled + limited access:
     - Get managed user IDs and group IDs
     - EXPAND groups to users via ListUsersForAnalytics
     - UNION expanded users with managed users  <-- KEY DIFFERENCE
     - Early return if empty

3. Intersect ACL results with ground truth
   - If root access or ACL disabled: Use ground truth as final users
   - Otherwise: Intersect finalUsers with ground truth

4. Build user-group mappings from ground truth memberships
   - Fetch groups via FetchGroups
   - Build mappings excluding root/default groups
   - Build GroupsToAggregate for response construction
```

## Key Behavioral Differences

### 1. Ground Truth Concept

| Parse | ParseUserFilterForAnalytics |
|-------|----------------------------|
| No ground truth | Fetches ALL users first, then filters |
| Relies on API filters | Uses ground truth for intersection |

**Impact**: Analytics ensures results are always within the "ground truth" set (e.g., only agents, only active users).

### 2. ACL Group Expansion

| Parse | ParseUserFilterForAnalytics |
|-------|----------------------------|
| Filters user's group memberships | Expands ACL groups to member users |
| Groups don't add users | Groups add their members to result |

**Example**:
- User A is managed directly by ACL
- Group G is managed by ACL, contains User B
- Parse returns: User A only
- ParseUserFilterForAnalytics returns: User A + User B

### 3. Profile Scoping

| Parse | ParseUserFilterForAnalytics |
|-------|----------------------------|
| No profile ID | Uses profile ID throughout |
| Customer-level only | Profile-level filtering |

### 4. User Fetching API

| Parse | ParseUserFilterForAnalytics |
|-------|----------------------------|
| `ListUsers` (UserServiceClient) | `ListUsersForAnalytics` (InternalUserServiceClient) |
| Returns `*userpb.User` | Returns `*internaluserpb.LiteUser` |
| Full user object | Lighter object with embedded memberships |

### 5. Result Structure

| Parse | ParseUserFilterForAnalytics |
|-------|----------------------------|
| String-based (UserNames, GroupNames) | Object-based (FinalUsers, FinalGroups) |
| Bidirectional mappings | Unidirectional + aggregation data |

## What Parse Needs to Support Analytics

1. **ProfileID parameter** - for profile-scoped queries
2. **Ground truth mode** - fetch all first, then filter
3. **Group expansion** - expand ACL groups to users
4. **InternalUserServiceClient** - for ListUsersForAnalytics
5. **Additional result fields** - FinalUsers, GroupsToAggregate, etc.
6. **Analytics-specific flags**:
   - `listAgentOnly`
   - `excludeDeactivatedUsers`
   - `includePeerUserStats`
   - `hasAgentAsGroupByKey`
   - `includeDirectGroupMembershipsOnly`

## Recommendation

Use **feature flags/options** to enable analytics behavior:

```go
type ParseOptions struct {
    // Enable analytics mode (ground truth + group expansion)
    AnalyticsMode bool

    // Analytics-specific settings
    ProfileID                         string
    ListAgentOnly                     bool
    ExcludeDeactivatedUsers           bool
    IncludePeerUserStats              bool
    HasAgentAsGroupByKey              bool
    IncludeDirectGroupMembershipsOnly bool
}
```

When `AnalyticsMode=false` (default): Behave exactly like current `Parse`
When `AnalyticsMode=true`: Use ground truth + group expansion logic
