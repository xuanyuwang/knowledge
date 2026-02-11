# User Filter Behavioral Standard

**Created**: 2026-02-09
**Updated**: 2026-02-09
**Purpose**: Define the expected behaviors of user filtering across all Insights and Coaching APIs. This is implementation-agnostic — it describes *what* the user filter should do, not *how*.

**Context**: Two implementations exist today — `Parse` (shared/user-filter) and `ParseUserFilterForAnalytics` (insights-server). This document captures the union of all behaviors, flags known divergences, and proposes the canonical behavior for each case.

**Evidence sources**: Unit tests, implementation code, and caller usage patterns across `retrieve_*_stats.go` and coaching actions.

---

## Table of Contents

1. [Terminology](#1-terminology)
2. [Inputs](#2-inputs)
3. [Outputs](#3-outputs)
4. [ACL Behavior](#4-acl-behavior)
5. [Ground Truth (User Population)](#5-ground-truth-user-population)
6. [User Filtering](#6-user-filtering)
7. [Group Filtering](#7-group-filtering)
8. [Combined User + Group Filters](#8-combined-user--group-filters)
9. [Deactivated User Handling](#9-deactivated-user-handling)
10. [Group Membership Tracking](#10-group-membership-tracking)
11. [Group Hierarchy and Child Teams](#11-group-hierarchy-and-child-teams)
12. [Query Optimization (ShouldQueryAllUsers)](#12-query-optimization-shouldqueryallusers)
13. [Caller Contract: How Results Are Used](#13-caller-contract-how-results-are-used)
14. [Metadata Enrichment](#14-metadata-enrichment)
15. [Pagination](#15-pagination)
16. [Error Handling](#16-error-handling)
17. [Sorting Guarantees](#17-sorting-guarantees)
18. [Profile Scoping](#18-profile-scoping)
19. [Behavioral Divergences Between Implementations](#19-behavioral-divergences-between-implementations)
20. [Proposal: Remove hasAgentAsGroupByKey from User Filter](#20-proposal-remove-hasagentasgroupbykey-from-user-filter)
21. [Proposal: Post-Processing Utilities](#21-proposal-post-processing-utilities)

---

## 1. Terminology

| Term | Definition |
|------|-----------|
| **Ground truth** | The complete set of users that could possibly appear in results. Determined by `listAgentOnly` and `excludeDeactivatedUsers` flags. |
| **ACL** | Advanced Data Access Control. When enabled, restricts which users/groups the authenticated user can see. |
| **Root access** | An ACL state where the authenticated user has unrestricted access (e.g., admin). ACL is enabled but does not restrict results. |
| **Limited access** | An ACL state where the authenticated user can only see their managed users/groups. |
| **Managed users/groups** | The specific users and groups an authenticated user with limited access is allowed to see. |
| **Direct membership** | A user belongs to a group directly (not via a parent group). |
| **Indirect membership** | A user belongs to a group via a parent/ancestor group in the team hierarchy. |
| **Virtual group (DYNAMIC)** | A dynamically computed group (e.g., based on rules). |
| **Team group (TEAM)** | A static organizational group (e.g., a team hierarchy). |
| **User filter** | Explicit list of users in the request. |
| **Group filter** | Explicit list of groups in the request. |
| **Group-by-Agent** | Caller is grouping results by agent (`hasAgentAsGroupByKey=true`). |
| **Group-by-Group** | Caller is grouping results by group (e.g., team leaderboard). |

---

## 2. Inputs

Inputs are organized into four categories by their role in the filtering pipeline.

### A. Scoping Context

Determines **where** to look. Set by the system, not by end-user UI selections.

| Input | Type | Description |
|-------|------|-------------|
| `customerID` | `string` | Which customer's data to query |
| `profileID` | `string` | Which customer profile to scope to. Prevents cross-profile data leakage (see B-PS-1). |

Note: Service dependencies like `aclHelper`, `userServiceClient`, `configClient` are **not** inputs — they are injected implementation details and should not appear in the behavioral contract.

### B. User Selection

Represents **what the end user chose** in the UI — explicit picks of users and/or groups.

| Input | Type | Description |
|-------|------|-------------|
| `Users` | `[]*userpb.User` or `[]string` | Explicit user selection (by name or object) |
| `Groups` | `[]*userpb.Group` or `[]string` | Explicit group selection (by name or object), may include both TEAM and DYNAMIC types |

### C. Population Filter

Determines **which users are eligible** to appear in results (the "ground truth"). These constrain the universe of users before any selection or ACL filtering.

| Input | Type | Description | Current duplication |
|-------|------|-------------|---------------------|
| `Roles` | `[]AuthProto_Role` | Filter: user should have **any** of these roles. Used in `ListUsers` fetch queries. | — |
| `UserTypes` | `[]UserType` | Filter by user type (PROD_USER, GUEST_USER, DEV_USER) | — |
| `GroupRoles` | `[]AuthProto_Role` | Filter group memberships by role | — |
| `State` | `User_State` | Filter by user state (active/deactivated). Used in fetch queries. | Overlaps with `excludeDeactivatedUsers` |
| `includePeerUserStats` | `bool` | Include peer users in the accessible user set (affects ACL-managed population) | — |
| `listAgentOnly` | `bool` | Restrict ground truth to users who are **exactly** agents. Maps to `AgentOnly` in `ListUsersForAnalytics`. | — (distinct from `Roles`) |
| `excludeDeactivatedUsers` | `bool` | Exclude deactivated users from ground truth. Maps to `IncludeInactiveUsers=false`. | Overlaps with `State` |

**Clarification on `listAgentOnly` vs `Roles`**: These are NOT the same.
- `listAgentOnly=true` constrains the **ground truth population** — only users who are agents exist in the result universe. This is a server-side filter on `ListUsersForAnalytics` (`AgentOnly=true`).
- `Roles=[AGENT]` constrains **fetch queries** — when fetching users by name/group via `ListUsers`, only return users who have any of the specified roles.

They operate at different levels of the pipeline and are not redundant.

**Redundancy note**: `excludeDeactivatedUsers` and `State` express the same constraint (active-only filtering). In the unified implementation, these should be consolidated. See discussion below.

### D. Membership Resolution

Controls **how group-to-user relationships are resolved**.

| Input | Type | Description | Current duplication |
|-------|------|-------------|---------------------|
| `includeDirectGroupMembershipsOnly` | `bool` | When true, only direct group memberships count for both filtering and mapping | Overlaps with `DirectTeamOnly` |
| `DirectTeamOnly` | `bool` | When true, only consider direct group memberships | **Same as `includeDirectGroupMembershipsOnly`** |

**Redundancy note**: `DirectTeamOnly` (in `UserFilterConditions`) and `includeDirectGroupMembershipsOnly` (a separate parameter) express the same thing. Should be unified into one.

### Proposed: Removed from Inputs

| Input | Reason |
|-------|--------|
| ~~`hasAgentAsGroupByKey`~~ | Caller concern, not a user filter input. See [Section 20](#20-proposal-remove-hasagentasgroupbykey-from-user-filter). |
| ~~`enableListUsersCache`~~ | Unused (dead code). |
| ~~`listUsersCache`~~ | Unused (dead code). |
| ~~`shouldMoveFiltersToUserFilter`~~ | Unused (dead code). |

### Input Redundancy Resolution

The unified implementation should resolve the two duplicated concepts:

#### 1. Deactivated user filtering

`excludeDeactivatedUsers=true` and `State=ACTIVE` express the same constraint. Proposed resolution:

| Option | Pros | Cons |
|--------|------|------|
| **Keep only `State`** | General-purpose, supports future states | Analytics callers must set `State` instead of a bool |
| Keep only `excludeDeactivatedUsers` | Simpler for the common case | Can't express other state filters |

**Recommended**: Keep `State`. It's more general.

#### 2. Direct membership

`DirectTeamOnly` and `includeDirectGroupMembershipsOnly` are the same. Proposed resolution:

**Recommended**: Keep one, call it `DirectMembershipsOnly`. Used in both population filtering (which users belong to a group) and output mapping (which groups appear in user-to-group maps).

---

## 3. Outputs

### Core Outputs (from user filter)

The user filter should return **complete, unfiltered** data. Callers use post-processing utilities (see [Section 21](#21-proposal-post-processing-utilities)) to shape the data for their specific needs.

| Output | Description |
|--------|-------------|
| **FinalUsers** | The filtered set of users to query for. Sorted by Name. |
| **FinalGroups** | The filtered set of groups. |
| **UserNameToDirectGroupNames** | Maps each user to their **direct** team group memberships (including root/default). |
| **UserNameToAllGroupNames** | Maps each user to **all** team group memberships, direct + indirect (including root/default). |
| **GroupNameToDirectMembers** | Maps each group to its direct member users. |
| **GroupNameToAllMembers** | Maps each group to all member users (direct + indirect). |
| **AllGroups** | All team groups encountered across all users' memberships. Unfiltered. |
| **ShouldQueryAllUsers** | Optimization flag — when true, the caller should NOT add a user WHERE clause to the query. |

### Derived Outputs (via post-processing utilities)

Callers derive what they need from the core outputs:

| Derived Output | How to Derive | Used By |
|---------------|---------------|---------|
| UserNameToGroupNamesMap (no root/default) | `StripRootAndDefaultGroups(UserNameToDirectGroupNames)` | Analytics: team leaderboard |
| UserNameToGroupNamesMap (with root/default) | Use `UserNameToDirectGroupNames` directly | Analytics: agent leaderboard |
| GroupsToAggregate (filtered) | `FilterGroups(AllGroups, requestedGroups)` | Analytics: group-by-group aggregation |
| GroupsToAggregate (all) | Use `AllGroups` directly | Analytics: group-by-agent |
| UserNames | `Keys(FinalUsers)` or `Keys(UserNameToAllGroupNames)` | Coaching: user ID extraction |

---

## 4. ACL Behavior

### B-ACL-1: Three ACL States

The user filter must handle exactly three ACL states:

| State | Condition | Behavior |
|-------|-----------|----------|
| **ACL Disabled** | Customer config `EnableAdvancedDataAccessControl = false` | Pass through all input filters unchanged. No restriction. |
| **ACL Enabled + Root Access** | ACL enabled, authenticated user has `IsRootAccess = true` | Pass through all input filters unchanged. Root access bypasses restrictions. |
| **ACL Enabled + Limited Access** | ACL enabled, `IsRootAccess = false` | Return ONLY the authenticated user's managed users and groups. Input filters are overridden. |

> **Test evidence**: `TestACL/acl_enabled_with_root_access`, `TestACL/acl_enabled_without_root_access`

### B-ACL-2: Limited Access Overrides Input Filters

When ACL is enabled with limited access, the managed users/groups **replace** the request's user/group filters, regardless of what was requested.

> **Test evidence**: `TestACL/acl_enabled_without_root_access` — selecting both userA and userB, but ACL only allows userB → result contains only userB.

### B-ACL-3: Empty Managed Set Returns Empty

When ACL is enabled with limited access and the authenticated user has NO managed users, return empty results immediately (early return). Steps 2 and 3 (ground truth intersection, mapping building) are NOT executed.

> **Test evidence**: `EmptyACLReturnsEmpty` — limited access with empty managed set → empty FinalUsers, no further processing.

### B-ACL-4: ACL Users and Groups Use UNION Semantics

When ACL returns both managed users AND managed groups, the final set of accessible users is the **UNION** of:
- Managed user IDs (direct)
- Users expanded from managed group IDs

NOT the intersection.

**Why UNION?** ACL managed users and managed groups are two independent ways to grant access. A manager may be granted direct access to specific users AND access to a group. The correct semantic is: "this user can see anyone they're granted access to, whether directly or via a group."

**What goes wrong without UNION?** If intersection is used instead, a user must appear in BOTH the direct managed list AND a managed group to be visible. This silently drops users:

```
Example:
  ACL managed users: [alice]
  ACL managed groups: [sales-team] containing [bob, charlie]

  UNION (correct):        [alice, bob, charlie]
  INTERSECTION (wrong):   []  ← alice not in sales-team, bob/charlie not in managed users
```

This was discovered as a critical bug. The old code used intersection, which caused managed group members to silently disappear from results. See [insights-user-filter investigation](../insights-user-filter/) for full analysis.

> **Test evidence**: `UnionOfUsersAndGroupsFromACL` — agent1 only in ACL users, agent2 only in group expansion → both returned.
> **Implementation evidence**: `applyResourceACL` in `common_user_filter.go:637` calls `expandGroupsToUsers` then `unionUsers`.

### B-ACL-5: Group Expansion in ACL

When ACL returns managed groups, those groups must be **expanded to their member users** (via `ListUsersForAnalytics` with group filter). The expanded users are then unioned with directly-managed users per B-ACL-4. Group expansion only happens when `filteredGroups` is non-empty and access is limited.

**Why expansion is required:** ClickHouse queries filter by user IDs, not by group membership. If managed groups are not expanded to user lists, the downstream query cannot enforce the ACL restriction. This leads to two failure modes:

1. **Data leakage**: Without expansion, the query has no user filter from the managed groups. A limited-access user could see ALL agents' data instead of only their managed group's data.
2. **Silent data loss**: Alternatively, if the group filter is simply dropped (not expanded), users who are accessible only via managed groups become invisible — the manager can't see their team's data at all.

The expansion step ensures that group-based access grants are converted to concrete user lists that ClickHouse can enforce.

```
Correct flow:
  ACL managed groups: [sales-team]
  → expand: ListUsersForAnalytics(groups=[sales-team]) → [bob, charlie]
  → union with managed users → final accessible set
  → intersect with ground truth → safe, filtered result

Without expansion:
  ACL managed groups: [sales-team]
  → NOT expanded
  → query has no user filter for sales-team members
  → either: all data returned (leakage) or no data returned (loss)
```

> **Implementation evidence**: `applyResourceACL` at line 693-707 — only expands when `len(filteredGroups) > 0`.

---

## 5. Ground Truth (User Population)

### B-GT-1: Ground Truth Is Fetched First

Before any filtering, fetch the complete set of qualifying users as the "ground truth". All subsequent operations (ACL, user filter, group filter) **intersect** with this set — they can only remove users from it, never add.

**Why fetch-all-then-filter instead of gradually-add?** The alternative approach — fetching users from each source (explicit selection, group expansion, ACL) and unioning them into a result — was the old implementation. It caused three classes of bugs:

**Bug 1: Role filtering bypass.** When groups are expanded to users via an external service, the expansion call may not apply the same role filter. The old code expanded managed groups without passing `listAgentOnly`, so non-agent users (managers, visitors) leaked into agent-only results.

```
Old approach (gradually add):
  1. Fetch users from ACL → [alice(agent), bob(agent)]
  2. Expand managed groups → [charlie(agent), diana(manager)]  ← no role filter!
  3. Union → [alice, bob, charlie, diana]  ← diana is a manager in agent-only view!

Ground truth approach (fetch all, then filter):
  1. Fetch ground truth (agents only) → [alice, bob, charlie]  ← diana excluded
  2. ACL users + expand groups → [alice, bob, charlie, diana]
  3. Intersect with ground truth → [alice, bob, charlie]  ← diana safely excluded
```

**Bug 2: Status filtering bypass.** Similarly, group expansion may return deactivated users if the expansion call doesn't apply the active-only filter. With ground truth, deactivated users are never in the ground truth set, so they can't appear regardless of what other steps return.

**Bug 3: Inconsistent metadata.** When users come from different sources (ACL response, group expansion, direct fetch), they may carry different metadata (e.g., stale FullName). Ground truth provides a single canonical source for metadata enrichment, ensuring consistency across all outputs.

The ground truth pattern provides a **safety guarantee**: no matter how many steps or external service calls happen downstream, only users in the ground truth can appear in the final result. Each step can only narrow the set, never widen it.

> **Implementation evidence**: `ParseUserFilterForAnalytics` calls `listAllUsers()` at line 155 before any ACL or filter processing.
> **Historical context**: See [insights-user-filter investigation](../insights-user-filter/) for the full bug analysis.

### B-GT-2: Ground Truth Determined by Flags

| `listAgentOnly` | `excludeDeactivatedUsers` | Ground Truth Contains |
|-----------------|--------------------------|----------------------|
| true | true | Active agents only |
| true | false | All agents (active + deactivated) |
| false | true | All active users |
| false | false | All users |

The `listAgentOnly` flag maps to `AgentOnly` in the `ListUsersForAnalytics` request. The `excludeDeactivatedUsers` flag maps to `IncludeInactiveUsers` (inverted).

> **Implementation evidence**: `listAllUsers` at lines 449-487.

### B-GT-3: All Results Must Be Within Ground Truth

No user should appear in FinalUsers, UsersFromGroups, or UserNameToGroupNamesMap unless they exist in the ground truth. This is the core invariant that prevents the bugs described in B-GT-1.

**What goes wrong without this invariant:**

1. **Limited access + no managed users → data leakage.** In the old implementation, when a limited-access user had no directly managed users, the code fell through to `ListUsersMappedToGroups` which listed ALL agents. The limited-access user saw everyone's data.

    ```
    Old approach (no ground truth):
      ApplyResourceACL → empty aclUsers, empty aclGroups
      ListUsersMappedToGroups(groups=[]) → lists ALL agents  ← no restriction!
      MoveFiltersToUserFilter → returns ALL users
      Result: limited-access user sees ALL agents' data  🐛

    Ground truth approach:
      Fetch ground truth → [all agents]
      ApplyResourceACL → empty managed users
      Intersect with ground truth → empty  ← early return, no data shown  ✓
    ```

2. **Two user lists diverge.** The old code maintained two separate user lists for different purposes: `req.FilterByAttribute.Users` for the ClickHouse WHERE clause, and `users` from `ListUsersMappedToGroups` for response construction. These could return different sets of users, causing the query to filter on one set but the response to show metadata from a different set.

    Ground truth eliminates this: one canonical set used everywhere.

> **Test evidence**: `LimitedAccessFiltersToAgentSubset` — ACL returns [agent1, agent2, manager1], ground truth is agents only → manager1 filtered out.

### B-GT-4: Ground Truth Is the Source for Metadata

User metadata (Username, FullName) should come from the ground truth fetch, not from the ACL response or request input. When `updateGroundTruthUsers` intersects, it keeps the enriched version from ground truth.

**Why this matters:** Without a single metadata source, users fetched from different code paths (ACL, group expansion, direct selection) may carry inconsistent or missing metadata. The old approach had a bug where `UsersFromGroups` was used for metadata enrichment but was a subset of `FinalUsers` — users without group memberships had no metadata enrichment at all.

> **Test evidence**: `EnrichedCorrectly` — ACL returns user with Name only, ground truth has Username/FullName → result enriched from ground truth.
> **Implementation evidence**: `updateGroundTruthUsers` at line 301 — `result[user.Name] = agentUser` uses ground truth version.
> **Historical context**: See [insights-user-filter metadata-enrichment-fix-analysis](../insights-user-filter/metadata-enrichment-fix-analysis.md).

---

## 6. User Filtering

### B-UF-1: Empty User Filter = All Accessible Users

When no users are specified in the request:
- ACL disabled / root access → all users in ground truth
- Limited access → all managed users (intersected with ground truth)

> **Test evidence**: `EmptyRequestReturnsAllAgents`, `RootAccessReturnsAllAgents`

### B-UF-2: Explicit User Filter Restricts Results

When specific users are requested, results are restricted to only those users (intersected with ground truth and ACL).

> **Test evidence**: `WithUserFilterReturnsFilteredAgents` — request [agent1, agent2], ground truth has 3 → returns only [agent1, agent2].

### B-UF-3: Non-Existent Users in Filter Are Silently Dropped

If a requested user doesn't exist in ground truth (e.g., a non-agent when `listAgentOnly=true`), they are silently excluded. No error is raised.

> **Test evidence**: `WithNonAgentFilterReturnsEmpty` — request [manager1], ground truth is agents → empty result.

### B-UF-4: Invalid User Names Return Error

Malformed user resource names (wrong format) should return `InvalidArgument` error with expected format hint: `customers/{customer_id}/users/{user_id}`.

> **Test evidence**: `TestInvalidSelections/Invalid_user_names` — "not/a/valid/username" → InvalidArgument error.

---

## 7. Group Filtering

### B-GF-1: Group Filter Expands to Member Users

When groups are specified in the filter, they are expanded to their member users. The result includes those users (intersected with ground truth).

> **Test evidence**: `WithGroupFilterReturnsOnlyGroupMembers` — group1 filter → returns only agent1/agent2 who are members.
> **Implementation evidence**: `filterUsersByGroups` at line 307 — checks each user's `Memberships` against group IDs.

### B-GF-2: Group Types Are Handled Separately

Groups must be triaged by type before fetching:

| Group Type | Fetch Method |
|-----------|-------------|
| TEAM | Fetch users with team group filter, respects `DirectTeamOnly` |
| DYNAMIC (Virtual) | Fetch users with virtual group filter, GroupRoles cleared |

> **Test evidence**: `TestGroupsByGroupType` — 4 groups correctly categorized into 2 TEAM + 2 DYNAMIC.
> **Implementation evidence**: `Parse` in `user_filter.go` calls `fetchUsersFromSelectedVirtualGroups` and `fetchUsersFromSelectedTeamGroups` separately.

### B-GF-3: Direct Membership Respected for Group Filter

When `includeDirectGroupMembershipsOnly=true`, only direct members of the filtered group are returned. Indirect members (via parent groups) are excluded.

> **Test evidence**: `WithGroupFilterRespectsDirectMembershipOnly` — agent1 direct, agent2 indirect → only agent1 returned.
> **Implementation evidence**: `filterUsersByGroups` at line 334 — `if includeDirectGroupMembershipsOnly && !membership.IsDirectMember { continue }`.

### B-GF-4: Group Filter Works Regardless of ACL State

Group filtering must be applied whether ACL is disabled, root access, or limited access.

> **Test evidence**: `WithGroupFilterReturnsOnlyGroupMembers` — ACL disabled + group filter → correctly filters.

### B-GF-5: Invalid Group Names Return Error

Malformed group resource names should return `InvalidArgument` error with expected format: `customers/{customer_id}/groups/{group_id}`.

> **Test evidence**: `TestInvalidSelections/Invalid_virtual_group_names`, `Invalid_team_group_names`

### B-GF-6: Invalid Group Names in filterUsersByGroups Silently Skipped

When filtering ground truth users by group membership, if a group name fails to parse, it is silently skipped (not an error). This differs from B-GF-5 which is about input validation.

> **Implementation evidence**: `filterUsersByGroups` at line 323 — `if err != nil { continue }`.

---

## 8. Combined User + Group Filters

### B-CF-1: Combined Filters — Divergent Behavior

**This is the most significant behavioral divergence.** The two implementations handle the combination of user filter + group filter differently:

#### New pattern (`ParseUserFilterForAnalytics`): INTERSECTION

When BOTH user filter and group filter are specified, the result is the **intersection**: only users that appear in the user filter AND are members of the specified groups.

> **Test evidence**: `UserAndGroupFiltersTogether` — request users [agent1, manager1] + group [group1], agent1 is member → only agent1 returned.

#### Old pattern (`MoveFiltersToUserFilter`): UNION

When BOTH user filter and group filter are specified, the result is the **union**: all explicitly-listed users PLUS all users from the specified groups, then deduplicated.

> **Implementation evidence**: `MoveFiltersToUserFilter` case b1 at line 586 — `filterByAttribute.Users = DedupUsers(append(filterByAttribute.Users, users...))`.
> **Code comment**: "results should be [A, B, C, D] (append only)" for case b1.

#### Discussion

| Scenario | New (intersection) | Old (union) |
|----------|-------------------|-------------|
| Users=[A,B], Groups=[team2 has B,C,D] | [B] (only B in both) | [A,B,C,D] (union) |

**This divergence needs team decision.** The new pattern is more restrictive; the old pattern is more inclusive. The old pattern's code comments explicitly document the union semantics as intentional.

### B-CF-2: Multi-Source Selection Uses Union (Parse only)

In the shared `Parse` function, when users come from multiple sources (explicit users + virtual groups + team groups), the results are **unioned** (deduplicated). This is different from B-CF-1 — here the union is between different *sources* of users, not between user filter and group filter.

> **Test evidence**: `TestParserWithSelections/all_selections` — userB + virtualGroup (userA, userB) + groupC (userC) → all three returned.

### B-CF-3: Deactivated Filter Combined with Group Filter (Old Pattern)

When user filter is non-empty AND both group filter and deactivated filter are set, the old pattern applies them **sequentially**:
1. First UNION: `existingUsers + groupUsers` (case b1)
2. Then INTERSECTION: `result ∩ activeUsers` (case b2)

Final result: `(explicit users ∪ group users) ∩ active users`

> **Implementation evidence**: `MoveFiltersToUserFilter` case b3 — "both group filter and deactivated user filter set, this case is already covered as above."

---

## 9. Deactivated User Handling

### B-DU-1: Controlled by Ground Truth (New Pattern)

In `ParseUserFilterForAnalytics`, deactivated user exclusion is enforced at the ground truth level. When `excludeDeactivatedUsers=true`, the ground truth fetch requests only active users (`IncludeInactiveUsers=false`). Since all results intersect with ground truth (B-GT-3), deactivated users are excluded from all outputs.

> **Test evidence**: `TrueFiltersDeactivated` — excludeDeactivatedUsers=true → ground truth has only active agents → deactivated agent2 absent from results.

### B-DU-2: Controlled by Post-Filtering (Old Pattern)

In `MoveFiltersToUserFilter`, deactivated user exclusion is a separate step. When `ExcludeDeactivatedUsers=true`, a separate `ParseFiltersToUsers` call fetches all active users, then the current user list is **intersected** with active users (keeping only active).

> **Implementation evidence**: `MoveFiltersToUserFilter` case b2 at line 600 — `filterByAttribute.Users = FilterUsers(filterByAttribute.Users, users)`.

### B-DU-3: Deactivated Users Included When Flag Is False

When `excludeDeactivatedUsers=false`, the ground truth includes inactive users, so they can appear in results.

> **Test evidence**: `FalseIncludesAll` — both active and deactivated agents in results.

### B-DU-4: ListUsersMappedToGroups Always Includes Inactive

The old `ListUsersMappedToGroups` always fetches with `IncludeInactiveUsers=true` regardless of any flag. Deactivated user exclusion is handled separately by `MoveFiltersToUserFilter`.

> **Implementation evidence**: `ListUsersMappedToGroups` at line 835 — `IncludeInactiveUsers: true` (hardcoded).

---

## 10. Group Membership Tracking

### B-GM-1: Track Both Direct and Indirect Memberships

The output must distinguish between direct and indirect group memberships:
- `UserNameToDirectGroupNames` — direct memberships only
- `UserNameToAllGroupNames` — direct + indirect memberships

Both maps should include **all groups** (including root/default). Callers use post-processing utilities to strip unwanted groups.

> **Implementation evidence**: `Parse` in `user_filter.go` maintains separate maps with `addRelation`.

### B-GM-2: Root and Default Groups Included in Raw Output, Stripped by Callers

**Updated**: The user filter should include root and default groups in its output. Callers that don't want them (e.g., team leaderboard) use `StripRootAndDefaultGroups()` to remove them. Callers that do want them (e.g., agent leaderboard) use the output as-is.

This replaces the previous behavior where `hasAgentAsGroupByKey` controlled inclusion/exclusion inside the user filter.

> **Current implementation evidence**: `buildUserGroupMappings` and `ListUsersMappedToGroups` skip root/default unless `hasAgentAsGroupByKey`. **Proposed**: Always include; let callers strip.

### B-GM-3: Only TEAM Groups in Mappings

Virtual/dynamic groups are excluded from user-to-group mappings. Only TEAM type groups appear in the user-to-group maps and `AllGroups`.

> **Implementation evidence**: `buildUserGroupMappings` checks `membership.Group.GroupType == internaluserpb.LiteGroup_TEAM`. `ListUsersMappedToGroups` checks `membership.Group.GroupType != internaluserpb.LiteGroup_TEAM`.

### B-GM-4: Direct-Only Mode for Mappings

When `includeDirectGroupMembershipsOnly=true`, indirect memberships are skipped when **fetching** users from `ListUsersForAnalytics` (the request sets `IncludeIndirectGroupMemberships=false`).

**Updated**: The old behavior where `hasAgentAsGroupByKey=true` also forced direct-only is removed. Since the user filter now returns both `UserNameToDirectGroupNames` and `UserNameToAllGroupNames`, the caller simply picks the appropriate map.

> **Current implementation evidence**: `buildUserGroupMappings` checks `hasAgentAsGroupByKey || includeDirectGroupMembershipsOnly`. **Proposed**: Only check `includeDirectGroupMembershipsOnly` for the fetch; always return both direct and all maps.

### B-GM-5: AllGroups Is Unfiltered; Callers Filter as Needed

**Updated**: The user filter should return **all** encountered TEAM groups in `AllGroups`, regardless of group filter. Callers that need a subset (e.g., group-by-group aggregation with a specific group filter) use `FilterGroups()` utility.

This replaces the previous behavior where `hasAgentAsGroupByKey` and the group filter jointly determined which groups appeared in `GroupsToAggregate`.

> **Current implementation evidence**: `ListUsersMappedToGroups` at line 863 — complex 3-way condition. **Proposed**: Always return all; caller filters.

### B-GM-6: Group Members Cleared in Output

When groups are included in the response (as aggregation buckets), the `Members` field is explicitly set to empty. Response groups contain only Name, DisplayName, and Type — no membership data. This is a **caller concern**, not a user filter concern.

> **Implementation evidence**: `convertRowsPerUserToPerUserPerGroupAgentStatsResponse` — `group.Members = []*userpb.GroupMembership{}`. (Stays in caller code.)

---

## 11. Group Hierarchy and Child Teams

### B-GH-1: Child Teams Expanded from Group Memberships

When fetching groups via `ListGroups`, child teams are discovered from the parent group's membership list. A group membership of type `GROUP` where the parent matches the current group is extracted as a child team.

> **Implementation evidence**: `ListGroups` at lines 745-758 — iterates `g.GetMembers()`, checks `m.MemberType == GROUP` and `m.GetGroup() == g.Name`.

### B-GH-2: Root Group Skipped in Child Expansion

During child team extraction, if a member is the root group (`m.IsRoot`), it is skipped.

> **Implementation evidence**: `ListGroups` at line 748 — `if m.IsRoot { continue }`.

### B-GH-3: Groups Deduplicated After Hierarchy Expansion

After extracting child teams, the full group list is deduplicated by group Name.

> **Implementation evidence**: `ListGroups` return statement — `fnutils.DedupeBy(groupResults, func(g *userpb.Group) string { return g.GetName() })`.

### B-GH-4: Cross-Profile Group Safety

Groups are always fetched scoped to a specific customer profile, even when no group filter is provided. This prevents groups from one profile leaking into another.

> **Implementation evidence**: `ListUsersMappedToGroups` comment at lines 793-798 — "we noticed a group in holidayinn-transfers-voice owning other holidayinn profiles' agents."

---

## 12. Query Optimization (ShouldQueryAllUsers)

### B-QO-1: Flag Definition

`ShouldQueryAllUsers` is an optimization to avoid ClickHouse query size limits when no filtering is needed.

| ACL State | User Filter | Group Filter | ShouldQueryAllUsers |
|-----------|-------------|--------------|---------------------|
| Disabled | Empty | Empty | **true** |
| Disabled | Non-empty | Any | false |
| Disabled | Any | Non-empty | false |
| Root access | Empty | Empty | **true** |
| Root access | Non-empty | Any | false |
| Root access | Any | Non-empty | false |
| Limited access | Any | Any | **always false** |

> **Implementation evidence**: `ParseUserFilterForAnalytics` lines 219-220 — exact boolean expression.

### B-QO-2: Limited Access Is Never "All Users"

A user with limited access should NEVER have `ShouldQueryAllUsers=true`, even if their filter is empty. Limited access always requires a WHERE clause to restrict to managed users.

> **Test evidence**: `Case3_LimitedAccess_EmptyFilter_ReturnsFalse`

### B-QO-3: When True, FinalUsers Still Populated

Even when `ShouldQueryAllUsers=true`, `FinalUsers` should still be populated (for metadata enrichment in the response), but callers should NOT use them in a WHERE clause.

> **Implementation evidence**: `ApplyUserFilterFromResult` at line 581 — when `ShouldQueryAllUsers=true`, sets `*users = []*userpb.User{}` (clears for query), but `result.FinalUsers` still contains data.

### B-QO-4: Early Return When No Access

`ApplyUserFilterFromResult` returns `true` (signaling early return with empty response) when:
- `ShouldQueryAllUsers=false` AND `len(FinalUsers) == 0`

This only happens for limited access with no managed users.

> **Implementation evidence**: `ApplyUserFilterFromResult` at line 591.

---

## 13. Caller Contract: How Results Are Used

This section documents how callers actually use the user filter results. These are not behaviors of the user filter itself, but constraints on what the user filter must produce correctly.

### B-CC-1: Results Discarded When Not Grouping by Agent/Group

Analytics callers discard `UserNameToGroupNamesMap`, `GroupsToAggregate`, and the extracted `users` when the request is NOT grouping by `ATTRIBUTE_TYPE_AGENT` or `ATTRIBUTE_TYPE_GROUP`. In that case (e.g., grouping by TIME_RANGE), only the WHERE clause filtering via `ApplyUserFilterFromResult` matters.

> **Implementation evidence**: `retrieve_agent_stats.go` lines 83-88, `retrieve_conversation_stats.go` lines 76-81 — triple-clear pattern.

### B-CC-2: UserNameToGroupNamesMap Used for Response JOIN

When grouping by agent or group, callers use `UserNameToGroupNamesMap` to join per-user query results with group information. If a user is NOT in the map, their stats are **dropped from the response entirely**.

> **Implementation evidence**: `convertRowsPerUserToPerUserPerGroupAgentStatsResponse` — `if groupNames, exists := userNameToGroupNamesMap[userName]; exists { ... }` — else branch is implicit skip.

### B-CC-3: GroupsToAggregate Used as Aggregation Buckets

When grouping by group (not agent), callers use `GroupsToAggregate` to initialize per-group result buckets. Per-user stats are then summed into the matching group bucket(s) via `UserNameToGroupNamesMap`. A user's stats may contribute to multiple groups if they have multiple memberships.

> **Implementation evidence**: `convertRowsPerUserToPerGroupAgentStatsResponse` at lines 287-312.

### B-CC-4: Coaching Callers Only Use UserNames

All 3 coaching callers (`action_list_coaching_plans.go`, `action_retrieve_coaching_overviews.go`, `action_retrieve_coaching_progresses.go`) only read `UserNames` from the `FilteredUsersAndGroups` result. They convert user names to user IDs and use those for database queries. All other fields (`GroupNames`, `UserNameToDirectGroupNames`, etc.) are computed but never accessed.

> **Implementation evidence**: Coaching files access only `parsedFilter.UserNames` or `parsedUserFilter.UserNames`.

### B-CC-5: Empty FinalUsers Triggers Empty Response

When `ApplyUserFilterFromResult` returns `shouldEarlyReturn=true`, callers return an empty response immediately (not an error).

> **Implementation evidence**: All `retrieve_*_stats.go` files — `if shouldEarlyReturn { return &analyticspb.Retrieve*Response{}, nil }`.

---

## 14. Metadata Enrichment

### B-ME-1: Enrich from Ground Truth

All user objects in the output should have:
- `Name` — resource name (e.g., `customers/{id}/users/{id}`)
- `Username` — login username
- `FullName` — display name

These are enriched from the ground truth fetch, not from the ACL response or request input.

> **Test evidence**: `EnrichedCorrectly`
> **Implementation evidence**: `updateGroundTruthUsers` uses ground truth version; `convertLiteUsersToUsers` maps LiteUser fields.

### B-ME-2: Missing Metadata Is Not an Error

If ground truth has empty Username or FullName, the result should still succeed with empty fields.

> **Test evidence**: `MissingMetadataHandledGracefully`

### B-ME-3: User Name Constructed from CustomerID + UserID

Resource names are constructed as `UserName{CustomerID, UserID}.String()`, not stored directly from the API response. Group names similarly use `GroupName{CustomerID, GroupID}.String()`.

> **Implementation evidence**: `convertLiteUsersToUsers` at line 286.

---

## 15. Pagination

### B-PG-1: Transparent Pagination

All user and group fetching must handle pagination transparently. The caller should not need to know about pagination.

### B-PG-2: Large Result Sets

Pagination must work for large sets (1000+ users). The ground truth fetch uses pages (e.g., 500 per page) and aggregates all pages before returning.

> **Test evidence**: `PaginationHandling` — 1200 agents across 3 pages.

### B-PG-3: Pagination Termination

Pagination loops terminate when `NextPageOffset == 0` (for `ListUsersForAnalytics`) or `NextPageToken == ""` (for `ListGroups`/`ListUsers`).

> **Implementation evidence**: `listAllUsers` at line 481, `ListGroups` at line 762, `FetchUsers` pagination loop.

---

## 16. Error Handling

### B-EH-1: ACL Errors Propagate

If the ACL check fails, the error should be returned to the caller immediately.

> **Test evidence**: `ApplyResourceACLFails`

### B-EH-2: Ground Truth Fetch Errors Propagate

If the user listing fails, the error should be returned before ACL processing.

> **Test evidence**: `ListUsersForAnalyticsFails`

### B-EH-3: Invalid Input Detected Early

Malformed resource names should be caught at parse time with `InvalidArgument` errors.

> **Test evidence**: `TestInvalidSelections/*`

### B-EH-4: Client Creation Errors

If the internal user service client cannot be created for the customer/profile, an `Internal` error is returned.

> **Implementation evidence**: `ParseFiltersToUsers` at line 479.

### B-EH-5: At Least One Filter Required (Old Pattern)

`ParseFiltersToUsers` requires at least one of `groupFilter` or `activeUserFilter` to be set. Calling with neither returns `InvalidArgument`.

> **Implementation evidence**: `ParseFiltersToUsers` at line 476.

---

## 17. Sorting Guarantees

### B-SO-1: Divergent Sort Keys

| Implementation | Sort Key | Evidence |
|---------------|----------|----------|
| `ParseUserFilterForAnalytics` / `buildUserGroupMappings` | Sorted by resource `Name` | `common_user_filter.go` sort |
| `ListUsersMappedToGroups` / `ParseFiltersToUsers` | Sorted by `FullName` | `common.go` at line 877: `users[i].FullName < users[j].FullName` |

**This is a divergence.** The new pattern sorts by Name (resource name), the old pattern sorts by FullName (display name). The team should decide on the canonical sort key.

### B-SO-2: Groups Sorted by Name

`GroupsToAggregate` are sorted by group resource Name.

### B-SO-3: Deterministic Output

Given the same inputs and state, the output must be deterministic (no random ordering).

**Note**: `unionUsers` and `convertLiteUsersToUsers` iterate over maps, which have non-deterministic order in Go. The caller must sort afterward to guarantee determinism.

> **Implementation evidence**: `unionUsers` at line 606 iterates `userMap`; `convertLiteUsersToUsers` at line 285 iterates `liteUsers` map.

---

## 18. Profile Scoping

### B-PS-1: Users Scoped to Customer Profile

User listing should be scoped to a specific customer profile (`profileID`), not just customer-level. This prevents cross-profile data leakage.

> **Implementation evidence**: `ListUsersForAnalytics` request includes `ProfileIds: []string{profileID}`.
> **Implementation evidence**: `ListGroups` uses `CustomerProfiles` filter.

### B-PS-2: Groups Scoped to Customer Profile

Group listing must also be scoped to the customer profile, even when no group filter is provided. Groups from other profiles must not appear.

> **Implementation evidence**: `ListGroups` includes `CustomerProfiles` in filter. Comment in `ListUsersMappedToGroups` documents the cross-profile bug that motivated this.

---

## 19. Behavioral Divergences Between Implementations

These are cases where the two existing implementations (`Parse` and `ParseUserFilterForAnalytics`) or the old/new analytics patterns behave differently. **The team must decide which behavior is canonical.**

### Divergence 1: Ground Truth Concept

| | Parse | ParseUserFilterForAnalytics |
|-|-------|---------------------------|
| **Behavior** | Fetches users matching filter conditions directly. No explicit ground truth. | Fetches ALL qualifying users first as ground truth, then intersects. |
| **Impact** | If a requested user doesn't match role/state filters, they're simply not fetched. | If a requested user is outside ground truth, they're excluded post-fetch. |
| **Proposed canonical** | **ParseUserFilterForAnalytics** — ground truth ensures no leakage of users that shouldn't appear. |

### Divergence 2: Profile Scoping

| | Parse | ParseUserFilterForAnalytics |
|-|-------|---------------------------|
| **Behavior** | Customer-level only (no profile ID). Uses `ListUsers` which doesn't take profileID. | Profile-level scoping (passes profileID to `ListUsersForAnalytics`). |
| **Impact** | Parse may return users across all profiles. Analytics returns users within a specific profile. |
| **Proposed canonical** | **Profile-level scoping** — prevents cross-profile data leakage. Coaching callers would need to pass profileID. |

### Divergence 3: User Fetching API

| | Parse | ParseUserFilterForAnalytics |
|-|-------|---------------------------|
| **Behavior** | Uses `ListUsers` (UserServiceClient) — returns full `User` objects with `GroupMemberships`. | Uses `ListUsersForAnalytics` (InternalUserServiceClient) — returns lightweight `LiteUser` with `Memberships`. |
| **Impact** | `ListUsers` returns heavier objects. `ListUsersForAnalytics` is optimized for analytics workloads. |
| **Proposed canonical** | **ListUsersForAnalytics** — lighter, purpose-built, already validated in production. |

### Divergence 4: Output Format

| | Parse | ParseUserFilterForAnalytics |
|-|-------|---------------------------|
| **Behavior** | Returns string-based `FilteredUsersAndGroups` with name-to-name maps (direct vs. indirect). | Returns object-based `ParseUserFilterResult` with full User/Group objects + ShouldQueryAllUsers. |
| **Impact** | Coaching callers only use `UserNames`. Analytics callers need full objects + optimization flag. |
| **Proposed canonical** | Unified output containing both. |

### Divergence 5: Combined User + Group Filter Semantics

| | ParseUserFilterForAnalytics | MoveFiltersToUserFilter (old pattern) |
|-|---------------------------|--------------------------------------|
| **Behavior** | **INTERSECTION** — user filter ∩ group members | **UNION** — user filter ∪ group members |
| **Example** | Users=[A,B], Group=[team2: B,C,D] → [B] | Users=[A,B], Group=[team2: B,C,D] → [A,B,C,D] |
| **Code evidence** | `filterUsersByGroups` intersects ground truth with group members | `MoveFiltersToUserFilter` case b1: `DedupUsers(append(...))` |
| **Proposed canonical** | **Needs team decision.** Both have valid use cases. |

### Divergence 6: Sorting Key

| | ParseUserFilterForAnalytics / buildUserGroupMappings | ListUsersMappedToGroups / ParseFiltersToUsers |
|-|------------------------------------------------------|-----------------------------------------------|
| **Behavior** | Sorted by resource `Name` | Sorted by `FullName` |
| **Impact** | Different ordering of results depending on which code path is used. |
| **Proposed canonical** | **Needs team decision.** FullName is more human-friendly for UI. Name is more stable (doesn't change when display name changes). |

### Divergence 7: Cache Support

| | Parse | ParseUserFilterForAnalytics | ListUsersMappedToGroups |
|-|-------|---------------------------|------------------------|
| **Behavior** | No caching. | Has cache parameters but they are **unused** (dead code). | Has actual working cache (when `enableListUsersCache=true` and no group filter). |
| **Proposed canonical** | **No cache in the function signature.** Clean up dead code. If caching is needed, implement at a lower level. |

### Divergence 8: Unused Parameters

| Parameter | In ParseUserFilterForAnalytics | Status |
|-----------|-------------------------------|--------|
| `enableListUsersCache` | line 143 | **Unused** — declared but never referenced |
| `listUsersCache` | line 144 | **Unused** — declared but never referenced |
| `shouldMoveFiltersToUserFilter` | line 147 | **Unused** — declared but never referenced |

> **Proposed canonical**: Remove all three. They are migration artifacts.

### Divergence 9: `hasAgentAsGroupByKey` Coupling

| | Current behavior | Proposed behavior |
|-|-----------------|-------------------|
| **Root/default groups** | Included only when `hasAgentAsGroupByKey=true` | Always included in raw output; callers strip with utility |
| **Indirect memberships** | Skipped when `hasAgentAsGroupByKey=true` OR `includeDirectGroupMembershipsOnly=true` | Only controlled by `includeDirectGroupMembershipsOnly`; both direct and all maps always returned |
| **GroupsToAggregate filter** | All groups when `hasAgentAsGroupByKey=true`; filtered when false | Always return all groups; callers filter with utility |

> **Proposed canonical**: Remove `hasAgentAsGroupByKey` from the user filter. See [Section 20](#20-proposal-remove-hasagentasgroupbykey-from-user-filter).

---

## 20. Proposal: Remove `hasAgentAsGroupByKey` from User Filter

### Problem

`hasAgentAsGroupByKey` bundles three unrelated concerns into one boolean:

1. **Include root/default groups in mappings?** — a presentation concern
2. **Force direct-only memberships?** — redundant with `includeDirectGroupMembershipsOnly`
3. **Bypass group filter for GroupsToAggregate?** — an aggregation concern

The flag's name describes a **caller's grouping strategy**, not a user filter behavior. It leaks the "group-by" concept (which is specific to analytics APIs) into a package that should be reusable across coaching, analytics, and future callers.

### What It Controls Today

| Decision | When `true` | When `false` |
|----------|------------|-------------|
| Root/default groups | Included in `UserNameToGroupNamesMap` | Excluded |
| Indirect memberships | Skipped (forces direct-only) | Included (unless `includeDirectGroupMembershipsOnly=true`) |
| GroupsToAggregate | All encountered TEAM groups | Only groups matching filter (if filter present) |

### Proposed Change

**Remove `hasAgentAsGroupByKey` from the user filter function signature.** The user filter returns complete, unfiltered data. Callers apply post-processing.

#### Before (current)

```go
result, err := ParseUserFilterForAnalytics(
    ...,
    hasAgentAsGroupByKey,              // caller concern leaked in
    includeDirectGroupMembershipsOnly,
    ...,
)
// result.UserNameToGroupNamesMap — already filtered by hasAgentAsGroupByKey
// result.GroupsToAggregate — already filtered by hasAgentAsGroupByKey
```

#### After (proposed)

```go
result, err := Parse(
    ...,
    includeDirectGroupMembershipsOnly,
    ...,
)
// result.UserNameToDirectGroupNames — complete (includes root/default)
// result.UserNameToAllGroupNames — complete (includes root/default)
// result.AllGroups — complete (all encountered TEAM groups)

// Caller applies post-processing:
if groupingByAgent {
    // Agent leaderboard: direct groups, keep root/default, all groups
    groupMap = result.UserNameToDirectGroupNames
    groups = result.AllGroups
} else {
    // Team leaderboard: strip root/default, filter to requested groups
    groupMap = userfilter.StripRootAndDefaultGroups(result.UserNameToDirectGroupNames)
    groups = userfilter.FilterGroups(result.AllGroups, requestedGroups)
}
```

### Migration Impact

| Area | Change |
|------|--------|
| `ParseUserFilterForAnalytics` / `buildUserGroupMappings` | Remove `hasAgentAsGroupByKey` parameter. Always build complete maps. Always return all groups. |
| `ListUsersMappedToGroups` | Same removal. Always include root/default. Always return all groups. |
| `retrieve_agent_stats.go` (and 20+ similar files) | Add 2-3 lines of post-processing after the Parse call. |
| `common_user_filter.go` | `buildUserGroupMappings` simplifies — remove 3 conditional branches. |
| New utility functions | ~30 lines of simple map/filter helpers in the user-filter package. |

### Benefits

1. **Separation of concerns**: User filter does filtering. Callers do presentation/aggregation.
2. **Simpler user filter**: 3 fewer conditional branches in `buildUserGroupMappings`.
3. **Reusable**: New callers (coaching, future APIs) don't need to understand analytics-specific grouping concepts.
4. **Testable**: Post-processing utilities are pure functions — trivial to unit test.
5. **Explicit**: Caller code reads like documentation — you see exactly what transformations are applied.

---

## 21. Proposal: Post-Processing Utilities

The user filter package should provide a set of utility functions for callers to transform the raw output into what they need. These are pure functions — no RPC calls, no state.

### Proposed Utilities

#### `StripRootAndDefaultGroups`

Remove root and default groups from user-to-group mappings.

```go
// StripRootAndDefaultGroups removes root group and default group entries
// from a user-to-groups mapping. Used by team leaderboard views where
// root/default groups are not meaningful aggregation keys.
func StripRootAndDefaultGroups(
    userToGroups map[string][]string,
) map[string][]string
```

**Use case**: Team leaderboard — don't want "Root" or "Default" as group buckets.

#### `FilterGroups`

Filter a group list to only include specific groups.

```go
// FilterGroups returns only the groups whose names appear in the allowlist.
// When allowlist is empty, returns all groups unchanged.
func FilterGroups(
    allGroups []*userpb.Group,
    allowlist []*userpb.Group,
) []*userpb.Group
```

**Use case**: Group-by-group aggregation — only aggregate into the groups the user requested.

#### `FilterMappingByGroups`

Filter a user-to-groups mapping to only include entries for specific groups.

```go
// FilterMappingByGroups restricts a user-to-groups mapping to only include
// memberships in the specified groups. Users with no remaining memberships
// are removed from the map.
func FilterMappingByGroups(
    userToGroups map[string][]string,
    allowedGroups []*userpb.Group,
) map[string][]string
```

**Use case**: When aggregating by a specific set of groups, remove irrelevant group memberships from the mapping.

#### `ClearGroupMembers`

Strip membership data from group objects for response construction.

```go
// ClearGroupMembers returns a copy of the groups with Members set to empty.
// Used when including groups in API responses where member lists are not needed.
func ClearGroupMembers(groups []*userpb.Group) []*userpb.Group
```

**Use case**: Response groups should not contain full member lists.

#### `UserNamesFromResult`

Extract user names as string slice (for coaching callers).

```go
// UserNamesFromResult extracts user resource names from FinalUsers.
func UserNamesFromResult(users []*userpb.User) []string
```

**Use case**: Coaching callers that only need string user names.

#### `UserIDsFromResult`

Extract user IDs from user objects.

```go
// UserIDsFromResult extracts user IDs (not full resource names) from users.
func UserIDsFromResult(users []*userpb.User) ([]string, error)
```

**Use case**: Coaching callers that convert user names to IDs for database queries.

#### `ApplyToQuery`

Apply user filter result to a query's WHERE clause (replaces `ApplyUserFilterFromResult`).

```go
// ApplyToQuery applies the user filter result to request filter attributes.
// Returns true if the caller should return an empty response (no access).
//
// When ShouldQueryAllUsers=true: clears the user filter (query all).
// When ShouldQueryAllUsers=false: sets the user filter to FinalUsers.
func ApplyToQuery(
    result *ParseResult,
    users *[]*userpb.User,
    groups *[]*userpb.Group,
) (shouldEarlyReturn bool)
```

**Use case**: All analytics callers — replaces current `ApplyUserFilterFromResult`.

### Typical Caller Patterns

#### Agent Leaderboard (group-by-agent + group-by-group)

```go
result, err := userfilter.Parse(...)

// Agent leaderboard: show all direct groups per agent (including root/default)
groupMap := result.UserNameToDirectGroupNames
groups := userfilter.ClearGroupMembers(result.AllGroups)

if shouldReturn := userfilter.ApplyToQuery(result, &req.Users, &req.Groups); shouldReturn {
    return &Response{}, nil
}
return buildPerUserPerGroupResponse(queryResults, groupMap, groups)
```

#### Team Leaderboard (group-by-group only)

```go
result, err := userfilter.Parse(...)

// Team leaderboard: strip root/default, only aggregate into requested groups
groupMap := userfilter.StripRootAndDefaultGroups(result.UserNameToDirectGroupNames)
groupMap = userfilter.FilterMappingByGroups(groupMap, req.FilterByAttribute.GetGroups())
groups := userfilter.FilterGroups(result.AllGroups, req.FilterByAttribute.GetGroups())
groups = userfilter.ClearGroupMembers(groups)

if shouldReturn := userfilter.ApplyToQuery(result, &req.Users, &req.Groups); shouldReturn {
    return &Response{}, nil
}
return buildPerGroupResponse(queryResults, groupMap, groups)
```

#### Time Range Only (no agent/group grouping)

```go
result, err := userfilter.Parse(...)

// No grouping — only need WHERE clause filtering
if shouldReturn := userfilter.ApplyToQuery(result, &req.Users, &req.Groups); shouldReturn {
    return &Response{}, nil
}
return buildTimeRangeResponse(queryResults)
```

#### Coaching Callers

```go
result, err := userfilter.Parse(...)

// Coaching only needs user IDs
userIDs, err := userfilter.UserIDsFromResult(result.FinalUsers)
return queryCoachingData(userIDs)
```

---

## Appendix A: Test Case to Behavior Mapping

| Test Case | Behaviors Verified |
|-----------|--------------------|
| `RootAccessReturnsAllAgents` | B-ACL-1, B-UF-1, B-SO-1 |
| `LimitedAccessFiltersToAgentSubset` | B-ACL-2, B-GT-3 |
| `EmptyACLReturnsEmpty` | B-ACL-3 |
| `UnionOfUsersAndGroupsFromACL` | B-ACL-4, B-ACL-5 |
| `EmptyRequestReturnsAllAgents` | B-UF-1 |
| `WithUserFilterReturnsFilteredAgents` | B-UF-2 |
| `WithNonAgentFilterReturnsEmpty` | B-UF-3 |
| `WithGroupFilterReturnsOnlyGroupMembers` | B-GF-1, B-GF-4 |
| `WithGroupFilterRespectsDirectMembershipOnly` | B-GF-3 |
| `UserAndGroupFiltersTogether` | B-CF-1 (intersection semantics) |
| `TestParserWithSelections/all_selections` | B-CF-2 |
| `TrueFiltersDeactivated` | B-DU-1, B-GT-2 |
| `FalseIncludesAll` | B-DU-3 |
| `Case1_ACLDisabled_EmptyFilter_ReturnsTrue` | B-QO-1 |
| `Case3_LimitedAccess_EmptyFilter_ReturnsFalse` | B-QO-2 |
| `EnrichedCorrectly` | B-ME-1, B-GT-4 |
| `MissingMetadataHandledGracefully` | B-ME-2 |
| `PaginationHandling` | B-PG-2 |
| `ApplyResourceACLFails` | B-EH-1 |
| `ListUsersForAnalyticsFails` | B-EH-2 |
| `TestInvalidSelections/*` | B-EH-3, B-UF-4, B-GF-5 |
| `TestGroupsByGroupType` | B-GF-2 |
| `TestACL/acl_enabled_without_root_access` | B-ACL-2 |
| `TestACL/acl_enabled_with_root_access` | B-ACL-1 |
| `TestParserEmptySelections/acl_disabled` | B-UF-1 |
| `TestParserEmptySelections/acl_enabled_without_root_access` | B-ACL-2 |

## Appendix B: Behaviors NOT Covered by Tests

These behaviors were discovered from implementation and caller code review only:

| Behavior | Source |
|----------|--------|
| B-GF-6: Invalid group names silently skipped in filterUsersByGroups | `filterUsersByGroups` line 323 |
| B-GM-5: GroupsToAggregate filtered by group filter | `ListUsersMappedToGroups` line 863 |
| B-GM-6: Group Members cleared in output | Caller code in `retrieve_agent_stats.go` |
| B-GH-1–4: Group hierarchy and child team expansion | `ListGroups` lines 745-758 |
| B-CC-1: Results discarded when not grouping by Agent/Group | `retrieve_agent_stats.go` lines 83-88 |
| B-CC-2: Missing map entries cause stats to be dropped | Caller response construction code |
| B-CC-4: Coaching callers only use UserNames | Coaching action files |
| B-DU-4: ListUsersMappedToGroups always includes inactive | `ListUsersMappedToGroups` line 835 |
| B-PS-2: Groups scoped to profile (cross-profile bug) | `ListUsersMappedToGroups` comment |
| B-SO-1: Sort key divergence (Name vs FullName) | `common.go` line 877 vs `common_user_filter.go` |
| B-SO-3: Non-deterministic map iteration in unionUsers | `unionUsers` line 606 |
| B-EH-4: Client creation errors | `ParseFiltersToUsers` line 479 |
| B-EH-5: At least one filter required | `ParseFiltersToUsers` line 476 |
| B-CF-1 divergence: UNION in old pattern | `MoveFiltersToUserFilter` case b1 |
| B-CF-3: Sequential deactivated + group filter | `MoveFiltersToUserFilter` case b3 |
