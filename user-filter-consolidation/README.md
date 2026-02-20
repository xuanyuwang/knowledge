# User Filter Consolidation Project

## Goal

Unify `ParseUserFilterForAnalytics` into `Parse`, using `ListUsersForAnalytics` and `LiteUser` as the underlying implementation.

**Key insight**: Both functions should produce the same final results. The different approaches exist due to historical reasons:
1. `ParseUserFilterForAnalytics` used analytics package functions to minimize risk during refactoring
2. `Parse` was written before `ListUsersForAnalytics` and `LiteUser` existed

## LiteUser Field Mapping

`LiteUser` can fully support `Parse`'s needs:

| Parse needs (from User/GroupMembership) | LiteUser equivalent |
|----------------------------------------|---------------------|
| `user.Name` | Derive: `UserName{CustomerID, UserId}.String()` |
| `user.Username` | `liteUser.Username` |
| `user.FullName` | `liteUser.FullName` |
| `user.GroupMemberships` | `liteUser.Memberships` |
| `group.Group` (name string) | Derive: `GroupName{CustomerID, GroupId}.String()` |
| `group.IsRoot` | Derive: `membership.Group.GroupId == consts.RootGroupID` |
| `group.IsDefault` | Derive: `membership.Group.GroupId == consts.DefaultGroupID` |
| `group.GroupType` | Map: `LiteGroup_Type` → `userpb.Group_Type` |
| `group.IsIndirect` | Derive: `!membership.IsDirectMember` |

**Conclusion**: ✅ `LiteUser` fully supports all `Parse` requirements.

## Proposed Changes

### 1. Switch Parse to use ListUsersForAnalytics

Replace `FetchUsers` (ListUsers) with `FetchLiteUsers` (ListUsersForAnalytics):

```go
// New internal function
func fetchLiteUsers(
    ctx context.Context,
    customerID, profileID string,
    filter *ListLiteUsersFilter,
    client internaluserpb.InternalUserServiceClient,
) (map[string]*internaluserpb.LiteUser, error)
```

### 2. Add ProfileID and Analytics Options

```go
type ParseOptions struct {
    ProfileID                         string  // Required for ListUsersForAnalytics
    ListAgentOnly                     bool    // Filter to agents only
    ExcludeDeactivatedUsers           bool    // Exclude inactive users
    IncludePeerUserStats              bool    // Include peer users in ACL
    IncludeDirectGroupMembershipsOnly bool    // Only direct memberships
}
```

### 3. Update UserFilterConditions

Add fields that were previously separate parameters:

```go
type UserFilterConditions struct {
    // Existing fields
    SelectedUserNames         []string
    SelectedVirtualGroupNames []string
    SelectedTeamGroupNames    []string
    Roles                     []authpb.AuthProto_Role
    GroupRoles                []authpb.AuthProto_Role
    State                     userpb.User_State
    UserTypes                 []enums.UserType
    DirectTeamOnly            bool

    // New: from analytics parameters
    SelectedUsers  []*userpb.User   // Alternative to SelectedUserNames
    SelectedGroups []*userpb.Group  // Alternative to SelectedTeamGroupNames
}
```

### 4. Update Client Container

```go
type UserFilterClients struct {
    InternalUserClient internaluserpb.InternalUserServiceClient  // Primary
    UserClient         userpb.UserServiceClient                  // For FetchGroups
    ConfigClient       config.Client
    ACLHelper          auth.ResourceACLHelper
}
```

### 5. Extend FilteredUsersAndGroups

Add fields needed by analytics callers:

```go
type FilteredUsersAndGroups struct {
    // Existing fields (keep for backward compat)
    UserNames                  []string
    GroupNames                 []string
    UserNameToDirectGroupNames map[string][]string
    UserNameToAllGroupNames    map[string][]string
    GroupNameToDirectMembers   map[string][]string
    GroupNameToAllMembers      map[string][]string

    // New fields for analytics
    Users             []*userpb.User   // Full user objects
    Groups            []*userpb.Group  // Full group objects
    GroupsToAggregate []*userpb.Group  // For response construction
}
```

## Migration Steps

### Phase 1: Prepare shared/user-filter
1. Add `ParseOptions` struct
2. Add new client container supporting `InternalUserServiceClient`
3. Add helper to convert `LiteUser` → result mappings
4. Extend `FilteredUsersAndGroups` with new fields
5. Add new `Parse` signature with options (keep old for compat)

### Phase 2: Implement unified logic
1. Port `listAllUsers` logic (uses ListUsersForAnalytics)
2. Port `applyResourceACL` logic with group expansion
3. Port `buildUserGroupMappings` logic
4. Add comprehensive tests

### Phase 3: Migrate callers
1. Update insights-server `retrieve_*_stats.go` files (12+)
2. Update any other callers of old `Parse`

### Phase 4: Cleanup
1. Remove `ParseUserFilterForAnalytics` from insights-server
2. Remove old `Parse` signature if no longer needed
3. Remove unused helper functions

## Current Callers

### Parse (shared/user-filter) - 3 callers in apiserver/coaching
- `action_list_coaching_plans.go:346`
- `action_retrieve_coaching_overviews.go:152`
- `action_retrieve_coaching_progresses.go:84`

These callers:
- Use `UserFilterConditions` with string-based user/group names
- Pass `userpb.UserServiceClient`
- Only use `UserNames` from the result

### ParseUserFilterForAnalytics - 12+ callers in insights-server
All `retrieve_*_stats.go` files.

## Files to Modify

### shared/user-filter/
- `user_filter.go` - Main implementation changes
- `user_filter_test.go` - Add tests
- `options.go` (new) - ParseOptions and functional options
- `clients.go` (new) - Client container

### apiserver/internal/coaching/ (update to new signature)
- `action_list_coaching_plans.go`
- `action_retrieve_coaching_overviews.go`
- `action_retrieve_coaching_progresses.go`

### insights-server/internal/analyticsimpl/
- Delete `common_user_filter.go` (after migration)
- Delete `common_user_filter_test.go` (after migration)
- Update `retrieve_agent_stats.go`
- Update `retrieve_qa_score_stats.go`
- Update `retrieve_summarization_stats.go`
- Update `retrieve_smart_compose_stats.go`
- Update `retrieve_suggestion_stats.go`
- Update `retrieve_knowledge_base_stats.go`
- Update `retrieve_live_assist_stats.go`
- Update `retrieve_note_taking_stats.go`
- Update `retrieve_guided_workflow_stats.go`
- Update `retrieve_hint_stats.go`
- Update `retrieve_knowledge_assist_stats.go`
- Update `retrieve_conversation_stats.go`

## Log History

| Date | Summary |
|------|---------|
| 2026-02-19 | Fixed B-SF-3 (Divergence 5): `ParseUserFilterForAnalytics` now uses UNION for combined user+group selections. Branch: `xwang/fix-bsf3-union-semantics`. |
| 2026-02-09 | Re-evaluated project against current codebase. Migration is ~41% complete (12/29 APIs). Original unification plan not started; team took incremental migration approach instead. Recommend completing migration first, then reconsidering unification. |

## Related Documents

- [evaluation-2026-02-09.md](./evaluation-2026-02-09.md) - Full re-evaluation against current codebase
- [analysis-unused-params.md](./analysis-unused-params.md) - 3 unused parameters to remove
- [analysis-cache-usage.md](./analysis-cache-usage.md) - Cache investigation
- [comparison-parse-functions.md](./comparison-parse-functions.md) - Detailed comparison (historical reference)
