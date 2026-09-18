# User Filter Consolidation Project

> Durable domain reference: [Analytics / Insights User Filter](../analytics/subdomains/insights-user-filter/README.md). This initiative folder retains its own execution history.

## Goal

Unify `ParseUserFilterForAnalytics` into `Parse`, using `ListUsersForAnalytics` and `LiteUser` as the underlying implementation.

**Key insight**: Both functions should produce the same final results. The different approaches exist due to historical reasons:
1. `ParseUserFilterForAnalytics` used analytics package functions to minimize risk during refactoring
2. `Parse` was written before `ListUsersForAnalytics` and `LiteUser` existed

## LiteUser Output and Membership Mapping

`LiteUser` contains enough information to construct the unified parser's user metadata and membership outputs:

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

**Conclusion**: `LiteUser` fully supports the output and membership mappings in this table. It does **not** contain the fields needed to evaluate every population filter.

The following existing `Parse` inputs cannot be derived from `LiteUser`:

| Eligibility input | Why `LiteUser` is insufficient |
|-------------------|--------------------------------|
| `Roles` | `LiteUser` does not expose user roles |
| `UserTypes` | `LiteUser` does not expose user type |
| `State` | `LiteUser` does not expose active state or per-profile status |
| `GroupRoles` | `LiteUser` memberships do not expose the role used to qualify a member during group expansion |

These constraints must therefore be enforced before or alongside construction of the `LiteUser` base population. The original plan did not specify how this would work.

## Proposed Changes

### 1. Use ListUsersForAnalytics for canonical analytics identity and membership data

Use `FetchLiteUsers` (`ListUsersForAnalytics`) to produce profile-scoped `LiteUser` metadata and memberships:

```go
// New internal function
func fetchLiteUsers(
    ctx context.Context,
    customerID, profileID string,
    filter *ListLiteUsersFilter,
    client internaluserpb.InternalUserServiceClient,
) (map[string]*internaluserpb.LiteUser, error)
```

This does not, by itself, replace every semantic use of public `ListUsers`. The implementation must also preserve `Roles`, `GroupRoles`, `UserTypes`, and `State` eligibility.

### 1a. Resolve population-filter eligibility

Before implementing `Parser.Parse`, choose and test one of these strategies:

1. **Extend `ListUsersForAnalytics`** to enforce the missing filters server-side. This best matches the original single-fetch architecture, but requires an RPC and auth-service contract change.
2. **Eligibility query plus intersection**: use public `ListUsers` to compute the users eligible under `Roles`, `GroupRoles`, `UserTypes`, and `State`, then intersect that set with the canonical `ListUsersForAnalytics` population. This preserves the current public-filter semantics but adds another query and requires verified identity mapping.
3. **Enrich `LiteUser` and filter locally**: return the missing eligibility fields and evaluate them in the parser. This increases response payloads and duplicates server-side filtering logic, so it should be chosen only deliberately.

Do not silently ignore fields or treat `ListAgentOnly` as equivalent to `Roles=[AGENT]`. Empty `Roles` means no role restriction.

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
1. Decide and implement population-filter eligibility for `Roles`, `GroupRoles`, `UserTypes`, and `State`
2. Port `listAllUsers` logic using `ListUsersForAnalytics` as the canonical metadata/membership population
3. Port `applyResourceACL` logic with group expansion
4. Port `buildUserGroupMappings` logic
5. Add analytics plus coaching/population-filter tests

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

## Future Consideration: Mixed Active-State Semantics

The CONVI-6665 coaching investigation surfaced a useful future requirement that does not fit cleanly into the current coarse-grained `State` / `ExcludeDeactivatedUsers` model:

- Default behavior should continue to exclude deactivated users
- Explicitly searched and selected users may need to bypass state filtering
- Team and group expansion may still need to remain active-only

Short-term product decision:

- Support only the explicit searched-user case for coaching
- Do not broaden team/group expansion semantics yet

Potential future shared-filter strategy:

- For explicitly selected users, ignore user state when requested by the caller
- For users reached through selected teams or groups, continue applying active-only filtering

This is a shared user-filter design concern, not just a coaching concern, because today's `State` flag applies uniformly to all selection sources. If this requirement becomes common, the shared filter API may need to distinguish:

- direct user selection semantics
- group expansion semantics
- default population semantics

## Confirmed Unified Population Contract

As of 2026-09-08, the unified parser must implement all existing population filters represented in `ParseOptions`: `Roles`, `GroupRoles`, `UserTypes`, and `State`, in addition to analytics-oriented `ListAgentOnly` and `ExcludeDeactivatedUsers`. An empty `Roles` slice means no role restriction. `ListAgentOnly` remains semantically distinct from `Roles=[AGENT]`; their combination and the overlap between `State` and `ExcludeDeactivatedUsers` must be validated explicitly rather than resolved by undocumented precedence.

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
| 2026-09-08 | Revalidated Phase 3 against current main and Linear. The unified implementation is still absent; merged API is `Parser.Parse`, analytics caller count and audience use have grown, child expansion is already fixed, and role-sensitive coaching/manual-QA semantics need explicit tests and a population-strategy decision before implementation. Canonical work item: `analytics/work-items/CONVI-6719.md`. |
| 2026-05-07 | Noted future mixed active-state requirement from CONVI-6665: explicitly selected users may need inactive bypass while teams/groups remain active-only. |
| 2026-02-20 | B-SF-3 fix merged to main ([PR #25829](https://github.com/cresta/go-servers/pull/25829)). Linear: CONVI-6284. |
| 2026-02-19 | Fixed B-SF-3 (Divergence 5): `ParseUserFilterForAnalytics` now uses UNION for combined user+group selections. Branch: `xwang/fix-bsf3-union-semantics`. |
| 2026-02-09 | Re-evaluated project against current codebase. Migration is ~41% complete (12/29 APIs). Original unification plan not started; team took incremental migration approach instead. Recommend completing migration first, then reconsidering unification. |

## Related Documents

- [evaluation-2026-02-09.md](./evaluation-2026-02-09.md) - Full re-evaluation against current codebase
- [analysis-unused-params.md](./analysis-unused-params.md) - 3 unused parameters to remove
- [analysis-cache-usage.md](./analysis-cache-usage.md) - Cache investigation
- [comparison-parse-functions.md](./comparison-parse-functions.md) - Detailed comparison (historical reference)
