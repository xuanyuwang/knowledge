# User Filter Behavioral Standard

**Created**: 2026-02-09
**Updated**: 2026-02-13
**Purpose**: Define the expected behaviors of user filtering across all Insights and Coaching APIs. This is implementation-agnostic — it describes *what* the user filter should do, not *how*.

**Context**: User filter logic is currently spread across three places:
1. **`Parse`** (shared/user-filter) — used by coaching APIs. Clean, structured.
2. **`ParseUserFilterForAnalytics`** (insights-server/analyticsimpl) — the newer consolidated function, used by ~12 migrated analytics APIs. It uses old function tools from Analytic service to minimize the compatibility risk, and should be refactored & unified into `Parse` with clean, structured code.
3. **Unorganized inline code** (insights-server/analyticsimpl + shared/common.go) — the old pattern using `ApplyResourceACL` + `ListUsersMappedToGroups` + `MoveFiltersToUserFilter`, still used by ~17 analytics APIs.

The old inline code is being incrementally replaced by `ParseUserFilterForAnalytics`. The end goal is to unify all user filter logic into a single shared package, fully replacing both the old inline code and the current `ParseUserFilterForAnalytics` with a clean, well-tested implementation.

This document captures the union of all behaviors across these implementations, flags known divergences, and proposes the canonical behavior for each case.

**Evidence sources**: Unit tests, implementation code, and caller usage patterns across `retrieve_*_stats.go` and coaching actions.

---

## Table of Contents

1. [Terminology](#1-terminology)
2. [Inputs](#2-inputs)
3. [Outputs](#3-outputs)
4. [ACL Behavior](#4-acl-behavior)
5. [Base Population](#5-base-population)
6. [Selection Filtering](#6-selection-filtering)
7. [Group Selection Mechanics](#7-group-selection-mechanics)
8. [Deactivated User Handling](#8-deactivated-user-handling)
9. [Group Membership Tracking](#9-group-membership-tracking)
10. [Group Hierarchy and Child Teams](#10-group-hierarchy-and-child-teams)
11. [Query Optimization (ShouldQueryAllUsers)](#11-query-optimization-shouldqueryallusers)
12. [Caller Contract: How Results Are Used](#12-caller-contract-how-results-are-used)
13. [Metadata Enrichment](#13-metadata-enrichment)
14. [Pagination](#14-pagination)
15. [Error Handling](#15-error-handling)
16. [Sorting Guarantees](#16-sorting-guarantees)
17. [Profile Scoping](#17-profile-scoping)
18. [Behavioral Divergences Between Implementations](#18-behavioral-divergences-between-implementations)
19. [Proposal: Remove hasAgentAsGroupByKey from User Filter](#19-proposal-remove-hasagentasgroupbykey-from-user-filter)
20. [Proposal: Post-Processing Utilities](#20-proposal-post-processing-utilities)

---

## 1. Terminology

| Term | Definition |
|------|-----------|
| **Base population** | The initial set of users obtained from `ListUsersForAnalytics` as the first step. This API provides a quick, server-side filter that narrows the user universe using simple criteria it natively supports (agent-only, active-only, profile-scoped, group membership). More complex filtering that requires custom logic — ACL evaluation, direct/indirect membership resolution, user+group filter intersection — is performed afterward on this base set. All subsequent operations can only remove users from it, never add. (Previously called "ground truth" in the codebase — variable name `groundTruthUsers`.) |
| **ACL** | Advanced Data Access Control. When enabled, restricts which users/groups the authenticated user can see. |
| **Root access** | An ACL state where the authenticated user has unrestricted access (e.g., admin). ACL is enabled but does not restrict results. |
| **Limited access** | An ACL state where the authenticated user can only see their managed users/groups. |
| **Managed users/groups** | The specific users and groups an authenticated user with limited access is allowed to see. |
| **Direct membership** | A user belongs to a group directly (not via a parent group). |
| **Indirect membership** | A user belongs to a group via a parent/ancestor group in the team hierarchy. |
| **Virtual group (DYNAMIC)** | A dynamically computed group (e.g., based on rules). |
| **Team group (TEAM)** | A static organizational group (e.g., a team hierarchy). |
| **Selected users** | Explicit list of users the end user picked in the request. (Previously called "user filter" in the codebase — `req.FilterByAttribute.Users`, `SelectedUserNames`.) |
| **Selected groups** | Explicit list of groups the end user picked in the request. (Previously called "group filter" in the codebase — `req.FilterByAttribute.Groups`, `SelectedTeamGroupNames`, `SelectedVirtualGroupNames`.) |
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

Determines **which users are eligible** to appear in results (the "base population"). These constrain the universe of users before any selection or ACL filtering.

| Input | Type | Description | Current duplication |
|-------|------|-------------|---------------------|
| `Roles` | `[]AuthProto_Role` | Filter: user should have **any** of these roles. Used in `ListUsers` fetch queries. | — |
| `UserTypes` | `[]UserType` | Filter by user type (PROD_USER, GUEST_USER, DEV_USER) | — |
| `GroupRoles` | `[]AuthProto_Role` | Filter group memberships by role | — |
| `State` | `User_State` | Filter by user state (active/deactivated). Used in fetch queries. | Overlaps with `excludeDeactivatedUsers` |
| `includePeerUserStats` | `bool` | Include peer users in the accessible user set (affects ACL-managed population) | — |
| `listAgentOnly` | `bool` | Restrict base population to users who are **exactly** agents. Maps to `AgentOnly` in `ListUsersForAnalytics`. | — (distinct from `Roles`) |
| `excludeDeactivatedUsers` | `bool` | Exclude deactivated users from base population. Maps to `IncludeInactiveUsers=false`. | Overlaps with `State` |

**Clarification on `listAgentOnly` vs `Roles`**: These are NOT the same.
- `listAgentOnly=true` constrains the **base population** — only users who are agents exist in the result universe. This is a server-side filter on `ListUsersForAnalytics` (`AgentOnly=true`).
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
| ~~`hasAgentAsGroupByKey`~~ | Caller concern, not a user filter input. See [Section 19](#19-proposal-remove-hasagentasgroupbykey-from-user-filter). |
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

**Recommended**: Keep one, call it `DirectMembershipsOnly`. Controls group→user expansion only (which users qualify as members of a selected group). Does NOT affect output mapping — both `UserToDirectGroups` and `UserToAllGroups` are always fully populated (see B-GM-4).

---

## 3. Outputs

### Core Outputs (from user filter)

The user filter should return **complete, unfiltered** data using maps from **resource names to objects**. This design serves both use cases: callers that need resource name sets use the map keys; callers that need metadata (Username, FullName, GroupType) use the map values — no extra RPC calls either way.

Callers use post-processing utilities (see [Section 20](#20-proposal-post-processing-utilities)) to shape the data for their specific needs.

| Output | Type | Description |
|--------|------|-------------|
| **FinalUsers** | `map[string]LiteUser` | The filtered set of users. Key = user resource name, Value = user with metadata. Sorted by key. |
| **FinalGroups** | `map[string]LiteGroup` | The filtered set of groups. Key = group resource name. |
| **UserToDirectGroups** | `map[string][]LiteGroup` | Key = user resource name → **direct** team group memberships (including root/default). |
| **UserToAllGroups** | `map[string][]LiteGroup` | Key = user resource name → **all** team group memberships, direct + indirect (including root/default). |
| **GroupToDirectMembers** | `map[string][]LiteUser` | Key = group resource name → direct member users. |
| **GroupToAllMembers** | `map[string][]LiteUser` | Key = group resource name → all member users (direct + indirect). |
| **AllGroups** | `map[string]LiteGroup` | All team groups encountered across all users' memberships. Key = group resource name. Unfiltered. |
| **ShouldQueryAllUsers** | `bool` | Optimization flag — when true, the caller should NOT add a user WHERE clause to the query. |

**Why maps from resource name to object?** The base population fetch from `ListUsersForAnalytics` already returns `LiteUser`/`LiteGroup` with metadata. Using resource names as keys means:
- `keys(FinalUsers)` → user resource names (for WHERE clauses, coaching callers)
- `values(FinalUsers)` → user objects with metadata (for response construction)
- Natural deduplication by resource name
- Easy lookup: `FinalUsers["customers/x/users/y"]` → user metadata

### Derived Outputs (via post-processing utilities)

Callers derive what they need from the core outputs:

| Derived Output | How to Derive | Used By |
|---------------|---------------|---------|
| User-to-groups map (no root/default) | `StripRootAndDefaultGroups(UserToDirectGroups)` | Analytics: team leaderboard |
| User-to-groups map (with root/default) | Use `UserToDirectGroups` directly | Analytics: agent leaderboard |
| Groups for aggregation (selected) | Use `FinalGroups` directly | Analytics: group-by-group aggregation |
| Groups for aggregation (all) | Use `AllGroups` directly | Analytics: group-by-agent |
| User resource names | `Keys(FinalUsers)` | Coaching, WHERE clause |
| User IDs | `UserIDs(FinalUsers)` | Coaching: database queries |
| Group resource names | `Keys(AllGroups)` | WHERE clause |

---

## 4. ACL Behavior

### B-ACL-1: Three ACL States

The user filter must handle exactly three ACL states:

| State | Condition | Behavior |
|-------|-----------|----------|
| **ACL Disabled** | Customer config `EnableAdvancedDataAccessControl = false` | Pass through all input filters unchanged. No restriction. |
| **ACL Enabled + Root Access** | ACL enabled, authenticated user has `IsRootAccess = true` | Pass through all input filters unchanged. Root access bypasses restrictions. |
| **ACL Enabled + Limited Access** | ACL enabled, `IsRootAccess = false` | Narrow selections to the authenticated user's managed users and groups (see B-ACL-2). |

> **Test evidence**: `TestACL/acl_enabled_with_root_access`, `TestACL/acl_enabled_without_root_access`

### B-ACL-2: Limited Access Narrows Selections to Managed Set

When ACL is enabled with limited access, the result is the **intersection** of the request's selected users/groups with the authenticated user's managed users/groups. The managed set acts as a ceiling — users can only see data for people/groups they manage, so the ACL result is always a **subset** of what was requested.

| Selected Users/Groups | ACL Result |
|----------------------|------------|
| Non-empty | `selected ∩ managed` — only the requested items that the authenticated user can access |
| Empty (no selection) | All managed users/groups — the full set the authenticated user can access |

In both cases, the ACL-returned set then **replaces** the request's selections for all subsequent filtering steps.

> **Test evidence**: `TestACL/acl_enabled_without_root_access` — selecting both userA and userB, but ACL only manages userB → result contains only userB.
> **Implementation evidence**: `applyUserFilters` in `acl_helper.go:456` — iterates `ManagedUserIDs` and keeps only those present in `filterUserIDs` (the selected users). `applyGroupFilters` at line 429 does the same for groups.

### B-ACL-3: Empty Managed Set Returns Empty

When ACL is enabled with limited access and the authenticated user has NO managed users, return empty results immediately (early return). Steps 2 and 3 (base population intersection, mapping building) are NOT executed.

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
  → intersect with base population → safe, filtered result

Without expansion:
  ACL managed groups: [sales-team]
  → NOT expanded
  → query has no user filter for sales-team members
  → either: all data returned (leakage) or no data returned (loss)
```

> **Implementation evidence**: `applyResourceACL` at line 693-707 — only expands when `len(filteredGroups) > 0`.

---

## 5. Base Population

### B-BP-1: Base Population Is Fetched First

Before any filtering, fetch the complete set of qualifying users as the "base population". All subsequent operations (ACL, selected users, selected groups) **intersect** with this set — they can only remove users from it, never add.

**Why fetch-all-then-filter instead of gradually-add?** The alternative approach — fetching users from each source (explicit selection, group expansion, ACL) and unioning them into a result — was the old implementation. It caused three classes of bugs:

**Bug 1: Role filtering bypass.** When groups are expanded to users via an external service, the expansion call may not apply the same role filter. The old code expanded managed groups without passing `listAgentOnly`, so non-agent users (managers, visitors) leaked into agent-only results.

```
Old approach (gradually add):
  1. Fetch users from ACL → [alice(agent), bob(agent)]
  2. Expand managed groups → [charlie(agent), diana(manager)]  ← no role filter!
  3. Union → [alice, bob, charlie, diana]  ← diana is a manager in agent-only view!

Base population approach (fetch all, then filter):
  1. Fetch base population (agents only) → [alice, bob, charlie]  ← diana excluded
  2. ACL users + expand groups → [alice, bob, charlie, diana]
  3. Intersect with base population → [alice, bob, charlie]  ← diana safely excluded
```

**Bug 2: Status filtering bypass.** Similarly, group expansion may return deactivated users if the expansion call doesn't apply the active-only filter. With base population, deactivated users are never in the base population set, so they can't appear regardless of what other steps return.

**Bug 3: Inconsistent metadata.** When users come from different sources (ACL response, group expansion, direct fetch), they may carry different metadata (e.g., stale FullName). Base population provides a single canonical source for metadata enrichment, ensuring consistency across all outputs.

The base population pattern provides a **safety guarantee**: no matter how many steps or external service calls happen downstream, only users in the base population can appear in the final result. Each step can only narrow the set, never widen it.

> **Implementation evidence**: `ParseUserFilterForAnalytics` calls `listAllUsers()` at line 155 before any ACL or filter processing.
> **Historical context**: See [insights-user-filter investigation](../insights-user-filter/) for the full bug analysis.

### B-BP-2: Base Population Determined by `ListUsersForAnalytics` Capabilities

The base population is fetched via `ListUsersForAnalytics`. The flags that shape the base population are those **natively supported by that API** — they are pushed down as server-side filters so the response is already narrowed before any client-side logic runs.

Currently supported flags:

| Flag | Maps to | Effect |
|------|---------|--------|
| `listAgentOnly` | `AgentOnly` in request | Only return users who are agents |
| `excludeDeactivatedUsers` | `IncludeInactiveUsers` (inverted) | Only return active users |

Resulting base population:

| `listAgentOnly` | `excludeDeactivatedUsers` | Base Population Contains |
|-----------------|--------------------------|----------------------|
| true | true | Active agents only |
| true | false | All agents (active + deactivated) |
| false | true | All active users |
| false | false | All users |

**Future expansion**: As `ListUsersForAnalytics` adds support for more filter criteria (e.g., `Roles`, `UserTypes`), the corresponding population filters should be pushed down into this call. Moving filters server-side reduces the base population size early, improving performance and simplifying client-side logic.

> **Implementation evidence**: `listAllUsers` at lines 449-487.

### B-BP-3: All Results Must Be Within Base Population

No user should appear in FinalUsers, UserToDirectGroups, UserToAllGroups, GroupToDirectMembers, or GroupToAllMembers unless they exist in the base population. This is the core invariant that prevents the bugs described in B-BP-1.

**What goes wrong without this invariant:**

1. **Limited access + no managed users → data leakage.** In the old implementation, when a limited-access user had no directly managed users, the code fell through to `ListUsersMappedToGroups` which listed ALL agents. The limited-access user saw everyone's data.

    ```
    Old approach (no base population):
      ApplyResourceACL → empty aclUsers, empty aclGroups
      ListUsersMappedToGroups(groups=[]) → lists ALL agents  ← no restriction!
      MoveFiltersToUserFilter → returns ALL users
      Result: limited-access user sees ALL agents' data  🐛

    Base population approach:
      Fetch base population → [all agents]
      ApplyResourceACL → empty managed users
      Intersect with base population → empty  ← early return, no data shown  ✓
    ```

2. **Two user lists diverge.** The old code maintained two separate user lists for different purposes: `req.FilterByAttribute.Users` for the ClickHouse WHERE clause, and `users` from `ListUsersMappedToGroups` for response construction. These could return different sets of users, causing the query to filter on one set but the response to show metadata from a different set.

    Base population eliminates this: one canonical set used everywhere.

> **Test evidence**: `LimitedAccessFiltersToAgentSubset` — ACL returns [agent1, agent2, manager1], base population is agents only → manager1 filtered out.

### B-BP-4: Base Population Is the Source for Metadata

User metadata (Username, FullName) should come from the base population fetch, not from the ACL response or request input. When `updateGroundTruthUsers` intersects, it keeps the enriched version from base population.

**Why this matters:** Without a single metadata source, users fetched from different code paths (ACL, group expansion, direct selection) may carry inconsistent or missing metadata. The old approach had a bug where users from group expansion (a subset of FinalUsers) were used for metadata enrichment — users without group memberships had no metadata enrichment at all.

> **Test evidence**: `EnrichedCorrectly` — ACL returns user with Name only, base population has Username/FullName → result enriched from base population.
> **Implementation evidence**: `updateGroundTruthUsers` at line 301 — `result[user.Name] = agentUser` uses base population version.
> **Historical context**: See [insights-user-filter metadata-enrichment-fix-analysis](../insights-user-filter/metadata-enrichment-fix-analysis.md).

---

## 6. Selection Filtering

These behaviors apply uniformly to both user selections and group selections. Group selections are first expanded to their member users (see [Section 7](#7-group-selection-mechanics)), then the same rules apply.

### B-SF-1: Empty Selection = All Accessible Users

When no users or groups are specified in the request:
- ACL disabled / root access → all users in base population
- Limited access → all managed users (intersected with base population)

> **Test evidence**: `EmptyRequestReturnsAllAgents`, `RootAccessReturnsAllAgents`

### B-SF-2: Selection Restricts Results

When specific users and/or groups are requested, results are restricted to only those users (from user selection) and members of those groups (from group selection), intersected with base population and ACL.

> **Test evidence**: `WithUserFilterReturnsFilteredAgents` — request [agent1, agent2], base population has 3 → returns only [agent1, agent2].
> **Test evidence**: `WithGroupFilterReturnsOnlyGroupMembers` — group1 selection → returns only agent1/agent2 who are members.

### B-SF-3: Combined User + Group Selections Use UNION

When both users and groups are selected, the result is the **UNION**: all explicitly-selected users PLUS all members of the selected groups, deduplicated. The combined set is then intersected with base population and ACL as usual.

```
Example:
  Selected users: [alice, bob]
  Selected groups: [sales-team] containing [bob, charlie, diana]

  UNION: [alice, bob, charlie, diana]  (deduplicated)
  Intersect with base population → final result
```

> **Test evidence**: `UserAndGroupFiltersTogether`, `TestParserWithSelections/all_selections`
> **Implementation evidence (old pattern)**: `MoveFiltersToUserFilter` case b1 at line 586 — `DedupUsers(append(...))`.

**Fixed (2026-02-19)**: `ParseUserFilterForAnalytics` now correctly uses UNION. The fix is on branch `xwang/fix-bsf3-union-semantics` — two changes in `common_user_filter.go`: (1) skip pre-filtering groundTruthUsers by groups when reqUsers is also present, (2) expand groups and UNION with users for ACL-disabled/root-access cases. Three new tests: `B_SF_3_CombinedUserAndGroupSelectionsUseUnion`, `B_SF_3_DisjointUserAndGroupSelectionsUseUnion`, `B_SF_3_CombinedUserAndGroupUnionWithRootAccess`.

### B-SF-4: Non-Existent Entries Silently Dropped

If a selected user doesn't exist in the base population (e.g., a non-agent when `listAgentOnly=true`), or a selected group has no qualifying members in the base population, they are silently excluded. No error is raised.

> **Test evidence**: `WithNonAgentFilterReturnsEmpty` — request [manager1], base population is agents → empty result.

### B-SF-5: Invalid Resource Names Return Error

Malformed user or group resource names (wrong format) should return `InvalidArgument` error with expected format:
- Users: `customers/{customer_id}/users/{user_id}`
- Groups: `customers/{customer_id}/groups/{group_id}`

> **Test evidence**: `TestInvalidSelections/Invalid_user_names`, `TestInvalidSelections/Invalid_virtual_group_names`, `TestInvalidSelections/Invalid_team_group_names`

### B-SF-6: Selection Filtering Works in All ACL States

Selection filtering is applied regardless of ACL state (disabled, root access, or limited access). The ACL narrows the accessible set (B-ACL-2), then selection filtering further narrows within that set.

> **Test evidence**: `WithGroupFilterReturnsOnlyGroupMembers` — ACL disabled + selected groups → correctly filters.

---

## 7. Group Selection Mechanics

Group selections require additional processing compared to user selections: groups must be expanded to their member users before the general selection rules (Section 6) apply.

### B-GS-1: Groups Expand to Member Users

Selected groups are expanded to their member users. The expansion queries for users who are members of the selected groups. The resulting users are then subject to the same selection rules as explicitly-selected users (intersected with base population per B-SF-2).

> **Test evidence**: `WithGroupFilterReturnsOnlyGroupMembers` — group1 selection → returns only agent1/agent2 who are members.
> **Implementation evidence**: `filterUsersByGroups` at line 307 — checks each user's `Memberships` against group IDs.

### B-GS-2: Group Types Handled Separately

Groups must be triaged by type before expansion:

| Group Type | Fetch Method |
|-----------|-------------|
| TEAM | Fetch users with team group filter, respects `DirectTeamOnly` |
| DYNAMIC (Virtual) | Fetch users with virtual group filter, GroupRoles cleared |

> **Test evidence**: `TestGroupsByGroupType` — 4 groups correctly categorized into 2 TEAM + 2 DYNAMIC.
> **Implementation evidence**: `Parse` in `user_filter.go` calls `fetchUsersFromSelectedVirtualGroups` and `fetchUsersFromSelectedTeamGroups` separately.

### B-GS-3: Direct Membership Controls Group→User Expansion

`includeDirectGroupMembershipsOnly` (and its equivalent `DirectTeamOnly`) controls the **group→user direction**: when expanding a selected group to its member users, only users who are **direct** members of that group are included. Indirect members (who belong via a parent group in the hierarchy) are excluded from the expansion.

This flag does NOT affect the **user→group direction**. Once a user qualifies as a result (via direct membership in the selected group), their full group memberships (both direct and indirect) are still tracked in the output maps. See B-GM-1.

```
Example:
  Selected groups: [sales-team]
  includeDirectGroupMembershipsOnly=true

  sales-team direct members: [alice, bob]
  sales-team indirect members (via parent): [charlie]

  Group→User expansion: [alice, bob]  ← charlie excluded (indirect)

  But for alice (who is direct member of sales-team, indirect member of engineering):
    UserToDirectGroups[alice] = [sales-team]
    UserToAllGroups[alice]    = [sales-team, engineering]  ← indirect groups still tracked
```

> **Test evidence**: `WithGroupFilterRespectsDirectMembershipOnly` — agent1 direct, agent2 indirect → only agent1 returned.
> **Implementation evidence**: `filterUsersByGroups` at line 334 — `if includeDirectGroupMembershipsOnly && !membership.IsDirectMember { continue }`. Both `includeDirectGroupMembershipsOnly` and `DirectTeamOnly` map to `IncludeIndirectGroupMemberships=false` in the `ListUsersForAnalytics`/`ListUsers` request.

### B-GS-4: Unparseable Group Names Silently Skipped During Expansion

When expanding groups to member users, if a group name fails to parse, it is silently skipped (not an error). This differs from B-SF-5, which is about input validation — this is about robustness during the expansion step.

> **Implementation evidence**: `filterUsersByGroups` at line 323 — `if err != nil { continue }`.

---

## 8. Deactivated User Handling

### B-DU-1: Deactivated Users Excluded When Flag Is Set

When `excludeDeactivatedUsers=true` (or equivalently `State=ACTIVE`), deactivated users are excluded from all outputs — FinalUsers, group membership maps, and AllGroups member lists.

> **Test evidence**: `TrueFiltersDeactivated` — excludeDeactivatedUsers=true → deactivated agent2 absent from results.

### B-DU-2: Deactivated Users Included When Flag Is False

When `excludeDeactivatedUsers=false`, deactivated users can appear in results alongside active users.

> **Test evidence**: `FalseIncludesAll` — both active and deactivated agents in results.

---

## 9. Group Membership Tracking

### B-GM-1: Track Both Direct and Indirect Memberships (User→Group Direction)

For each user in the result, the output maps their group memberships in the **user→group direction**. Both direct and indirect memberships are always tracked, regardless of the `includeDirectGroupMembershipsOnly` flag (which only controls the group→user expansion per B-GS-3):

- `UserToDirectGroups` — groups where the user is a **direct** member
- `UserToAllGroups` — groups where the user is a member (direct **+** indirect)

Both maps include **all groups** (including root/default). Callers use post-processing utilities to strip unwanted groups.

This means a user who was included because they're a direct member of a selected group will still have their indirect group memberships visible in `UserToAllGroups`. The flag narrows which users are in the result, not which groups appear per user.

> **Implementation evidence**: `Parse` in `user_filter.go` maintains separate maps with `addRelation`.

### B-GM-2: Root and Default Groups Included in Raw Output, Stripped by Callers

**Updated**: The user filter should include root and default groups in its output. Callers that don't want them (e.g., team leaderboard) use `StripRootAndDefaultGroups()` to remove them. Callers that do want them (e.g., agent leaderboard) use the output as-is.

This replaces the previous behavior where `hasAgentAsGroupByKey` controlled inclusion/exclusion inside the user filter.

> **Current implementation evidence**: `buildUserGroupMappings` and `ListUsersMappedToGroups` skip root/default unless `hasAgentAsGroupByKey`. **Proposed**: Always include; let callers strip.

### B-GM-3: Only TEAM Groups in Mappings

Virtual/dynamic groups are excluded from user-to-group mappings. Only TEAM type groups appear in the user-to-group maps and `AllGroups`.

> **Implementation evidence**: `buildUserGroupMappings` checks `membership.Group.GroupType == internaluserpb.LiteGroup_TEAM`. `ListUsersMappedToGroups` checks `membership.Group.GroupType != internaluserpb.LiteGroup_TEAM`.

### B-GM-4: Callers Choose Which Map to Use

`includeDirectGroupMembershipsOnly` only controls group→user expansion (B-GS-3). The user→group mappings are not controlled by any flag — both `UserToDirectGroups` and `UserToAllGroups` are always fully populated for every user in the result. Callers pick the appropriate map for their use case.

**Note on current implementation**: `buildUserGroupMappings` only builds a single `usersToGroups` map, so the flag (and `hasAgentAsGroupByKey`) controls whether that map contains direct-only or all memberships. This is because there's only one map to populate. In the unified implementation with two separate maps, the flag becomes unnecessary for the mapping step:
- `hasAgentAsGroupByKey` is removed (see [Section 19](#19-proposal-remove-hasagentasgroupbykey-from-user-filter))
- Both maps are always built
- Callers choose `UserToDirectGroups` or `UserToAllGroups` based on their needs

### B-GM-5: AllGroups Is Unfiltered; Callers Filter as Needed

**Updated**: The user filter should return **all** encountered TEAM groups in `AllGroups`, regardless of selected groups. Callers that need a subset (e.g., group-by-group aggregation with specific selected groups) use `FinalGroups` instead of `AllGroups`.

This replaces the previous behavior where `hasAgentAsGroupByKey` and the selected groups jointly determined which groups appeared in `GroupsToAggregate`.

> **Current implementation evidence**: `ListUsersMappedToGroups` at line 863 — complex 3-way condition. **Proposed**: Always return all; caller filters.

### B-GM-6: No Membership Data in Group Output

Response groups should not contain member lists. With `LiteGroup`, this is a non-issue — `LiteGroup` is a lightweight struct that does not carry membership data. (The old pattern used full `Group` proto objects and required callers to explicitly clear `group.Members = []*userpb.GroupMembership{}`.)

> **Implementation evidence (old pattern)**: `convertRowsPerUserToPerUserPerGroupAgentStatsResponse` — `group.Members = []*userpb.GroupMembership{}`. No longer needed with `LiteGroup`.

---

## 10. Group Hierarchy and Child Teams

### B-GH-1: Child Teams Expanded from Group Memberships

When fetching groups via `ListGroups`, child teams are discovered from the parent group's membership list. A group membership of type `GROUP` where the parent matches the current group is extracted as a child team.

> **Implementation evidence**: `ListGroups` at lines 745-758 — iterates `g.GetMembers()`, checks `m.MemberType == GROUP` and `m.GetGroup() == g.Name`.

### B-GH-2: Root Group Skipped in Child Expansion

During child team extraction, if a member is the root group (`m.IsRoot`), it is skipped.

> **Implementation evidence**: `ListGroups` at line 748 — `if m.IsRoot { continue }`.

### B-GH-3: Groups Deduplicated After Hierarchy Expansion

After extracting child teams, the full group list is deduplicated by group Name.

> **Implementation evidence**: `ListGroups` return statement — `fnutils.DedupeBy(groupResults, func(g *userpb.Group) string { return g.GetName() })`.

### B-GH-4: Child Teams Must Appear in GroupsToAggregate

When a parent team is selected as a group filter for a **team leaderboard** (group-by-group), the child teams of that parent must appear in `GroupsToAggregate` (or `FinalGroups` in the proposed output). Otherwise, the leaderboard shows a single row for the parent team instead of breaking out sub-teams.

The old path (`ListUsersMappedToGroups`) achieves this via `ListGroups` which sets `IncludeGroupMemberships: true` and iterates the parent group's members to discover child groups (see B-GH-1). The child groups are appended to the group list, so `filterTeamGroupNames` includes both parent and children. When iterating user memberships, the `slices.Contains(filterTeamGroupNames, groupName)` check passes for child groups, allowing them into `groupsToAggregate`.

**Note**: The current `ParseUserFilterForAnalytics` does NOT expand child groups. Its `buildUserGroupMappings` uses `FetchGroups` which returns only the explicitly-requested group IDs — child groups are not discovered. This is a known bug (see [Divergence 10](#divergence-10-child-group-expansion-in-groupstoaggregate)). The unified implementation must match the old path's behavior.

> **Bug evidence**: CONVI-6260 — Hilton's team leaderboard showed only parent team as a single row instead of sub-teams.
> **Implementation evidence (old path)**: `ListGroups` at lines 745-758 — child group expansion. `ListUsersMappedToGroups` at line 865 — `slices.Contains(filterTeamGroupNames, groupName)` passes for children.
> **Implementation evidence (new path, buggy)**: `buildUserGroupMappings` at line 395 — `FetchGroups` returns only requested groups, no children. Line 433 — `slices.Contains(groupNames, groupName)` fails for child groups.

### B-GH-5: Cross-Profile Group Safety

Groups are always fetched scoped to a specific customer profile, even when no groups are selected. This prevents groups from one profile leaking into another.

> **Implementation evidence**: `ListUsersMappedToGroups` comment at lines 793-798 — "we noticed a group in holidayinn-transfers-voice owning other holidayinn profiles' agents."

**Note**: This was previously numbered B-GH-4 before B-GH-4 (child teams in GroupsToAggregate) was added.

---

## 11. Query Optimization (ShouldQueryAllUsers)

### B-QO-1: Flag Definition

`ShouldQueryAllUsers` is an optimization to avoid ClickHouse query size limits when no filtering is needed.

| ACL State | Selected Users | Selected Groups | ShouldQueryAllUsers |
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

## 12. Caller Contract: How Results Are Used

This section documents how callers actually use the user filter results. These are not behaviors of the user filter itself, but constraints on what the user filter must produce correctly.

### B-CC-1: Results Discarded When Not Grouping by Agent/Group

Analytics callers discard group mapping and group aggregation data when the request is NOT grouping by `ATTRIBUTE_TYPE_AGENT` or `ATTRIBUTE_TYPE_GROUP`. In that case (e.g., grouping by TIME_RANGE), only the WHERE clause filtering via `ApplyToQuery` matters.

> **Implementation evidence**: `retrieve_agent_stats.go` lines 83-88, `retrieve_conversation_stats.go` lines 76-81 — triple-clear pattern.

### B-CC-2: User-to-Groups Mapping Used for Response JOIN

When grouping by agent or group, callers use the user-to-groups mapping (`UserToDirectGroups` or `UserToAllGroups`) to join per-user query results with group information. If a user is NOT in the map, their stats are **dropped from the response entirely**.

> **Implementation evidence**: `convertRowsPerUserToPerUserPerGroupAgentStatsResponse` — `if groupNames, exists := userNameToGroupNamesMap[userName]; exists { ... }` — else branch is implicit skip. (Current code uses string-based map; proposed uses `map[string][]LiteGroup`.)

### B-CC-3: Groups Output Used as Aggregation Buckets

When grouping by group (not agent), callers use the groups output (`FinalGroups` or `AllGroups`, depending on use case) to initialize per-group result buckets. Per-user stats are then summed into the matching group bucket(s) via the user-to-groups mapping. A user's stats may contribute to multiple groups if they have multiple memberships.

> **Implementation evidence**: `convertRowsPerUserToPerGroupAgentStatsResponse` at lines 287-312.

### B-CC-4: Coaching Callers Only Use User IDs

All 3 coaching callers (`action_list_coaching_plans.go`, `action_retrieve_coaching_overviews.go`, `action_retrieve_coaching_progresses.go`) only need user IDs for database queries. They use conversion utilities (`UserIDs(result.FinalUsers)`) to extract IDs from the struct-based output. All other fields (group maps, AllGroups, etc.) are not accessed.

> **Implementation evidence**: Coaching files access only `parsedFilter.UserNames` or `parsedUserFilter.UserNames` (current pattern uses string names; proposed pattern returns structs with conversion utilities).

### B-CC-5: Empty FinalUsers Triggers Empty Response

When `ApplyUserFilterFromResult` returns `shouldEarlyReturn=true`, callers return an empty response immediately (not an error).

> **Implementation evidence**: All `retrieve_*_stats.go` files — `if shouldEarlyReturn { return &analyticspb.Retrieve*Response{}, nil }`.

---

## 13. Metadata Enrichment

### B-ME-1: Enrich from Base Population

All user objects in the output should have:
- `Name` — resource name (e.g., `customers/{id}/users/{id}`)
- `Username` — login username
- `FullName` — display name

These are enriched from the base population fetch, not from the ACL response or request input.

> **Test evidence**: `EnrichedCorrectly`
> **Implementation evidence**: `updateGroundTruthUsers` uses base population version; `convertLiteUsersToUsers` maps LiteUser fields.

### B-ME-2: Missing Metadata Is Not an Error

If base population has empty Username or FullName, the result should still succeed with empty fields.

> **Test evidence**: `MissingMetadataHandledGracefully`

### B-ME-3: User Name Constructed from CustomerID + UserID

Resource names are constructed as `UserName{CustomerID, UserID}.String()`, not stored directly from the API response. Group names similarly use `GroupName{CustomerID, GroupID}.String()`.

> **Implementation evidence**: `convertLiteUsersToUsers` at line 286.

---

## 14. Pagination

### B-PG-1: Transparent Pagination

All user and group fetching must handle pagination transparently. The caller should not need to know about pagination.

### B-PG-2: Large Result Sets

Pagination must work for large sets (1000+ users). The base population fetch uses pages (e.g., 500 per page) and aggregates all pages before returning.

> **Test evidence**: `PaginationHandling` — 1200 agents across 3 pages.

### B-PG-3: Pagination Termination

Pagination loops terminate when `NextPageOffset == 0` (for `ListUsersForAnalytics`) or `NextPageToken == ""` (for `ListGroups`/`ListUsers`).

> **Implementation evidence**: `listAllUsers` at line 481, `ListGroups` at line 762, `FetchUsers` pagination loop.

---

## 15. Error Handling

### B-EH-1: ACL Errors Propagate

If the ACL check fails, the error should be returned to the caller immediately.

> **Test evidence**: `ApplyResourceACLFails`

### B-EH-2: Base Population Fetch Errors Propagate

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

## 16. Sorting Guarantees

### B-SO-1: Users and Groups Sorted by Resource Name

All output lists (`FinalUsers`, `FinalGroups`, `AllGroups`) are sorted by resource name. Resource names are stable identifiers that don't change when display names are updated, ensuring consistent ordering across requests.

**Note on current divergence**: The old pattern (`ListUsersMappedToGroups` / `ParseFiltersToUsers`) sorts by `FullName` instead of resource name (`common.go` line 877). The unified implementation will sort by resource name.

### B-SO-2: Deterministic Output

Given the same inputs and state, the output must be deterministic (no random ordering). The implementation must sort after any map iteration to guarantee this.

> **Implementation note**: `unionUsers` and `convertLiteUsersToUsers` iterate over maps, which have non-deterministic order in Go. Sorting by resource name after map iteration satisfies this requirement.

---

## 17. Profile Scoping

### B-PS-1: Users Scoped to Customer Profile

User listing should be scoped to a specific customer profile (`profileID`), not just customer-level. This prevents cross-profile data leakage.

> **Implementation evidence**: `ListUsersForAnalytics` request includes `ProfileIds: []string{profileID}`.
> **Implementation evidence**: `ListGroups` uses `CustomerProfiles` filter.

### B-PS-2: Groups Scoped to Customer Profile

Group listing must also be scoped to the customer profile, even when no groups are selected. Groups from other profiles must not appear.

> **Implementation evidence**: `ListGroups` includes `CustomerProfiles` in filter. Comment in `ListUsersMappedToGroups` documents the cross-profile bug that motivated this.

---

## 18. Behavioral Divergences Between Implementations

These are cases where the two existing implementations (`Parse` and `ParseUserFilterForAnalytics`) or the old/new analytics patterns behave differently. Each divergence includes the **decided** canonical behavior for the unified implementation.

### Divergence 1: Base Population Concept

| | Parse | ParseUserFilterForAnalytics |
|-|-------|---------------------------|
| **Behavior** | Fetches users matching filter conditions directly. No explicit base population. | Fetches ALL qualifying users first as base population, then intersects. |
| **Impact** | If a requested user doesn't match role/state filters, they're simply not fetched. | If a requested user is outside base population, they're excluded post-fetch. |
| **Decision** | **Base population.** Fetch all qualifying users first, then intersect. Ensures no leakage of users that shouldn't appear. |

### Divergence 2: Profile Scoping

| | Parse | ParseUserFilterForAnalytics |
|-|-------|---------------------------|
| **Behavior** | Customer-level only (no profile ID). Uses `ListUsers` which doesn't take profileID. | Profile-level scoping (passes profileID to `ListUsersForAnalytics`). |
| **Impact** | Parse may return users across all profiles. Analytics returns users within a specific profile. |
| **Decision** | **Profile-level scoping.** Prevents cross-profile data leakage. Coaching callers will need to pass profileID. |

### Divergence 3: User Fetching API

| | Parse | ParseUserFilterForAnalytics |
|-|-------|---------------------------|
| **Behavior** | Uses `ListUsers` (UserServiceClient) — returns full `User` objects with `GroupMemberships`. | Uses `ListUsersForAnalytics` (InternalUserServiceClient) — returns lightweight `LiteUser` with `Memberships`. |
| **Impact** | `ListUsers` returns heavier objects. `ListUsersForAnalytics` is optimized for analytics workloads. |
| **Decision** | **ListUsersForAnalytics.** Lighter, purpose-built, already validated in production. |

### Divergence 4: Output Format

| | Parse | ParseUserFilterForAnalytics |
|-|-------|---------------------------|
| **Behavior** | Returns string-based `FilteredUsersAndGroups` with name-to-name maps (direct vs. indirect). | Returns object-based `ParseUserFilterResult` with full User/Group objects + ShouldQueryAllUsers. |
| **Impact** | Coaching callers only use `UserNames`. Analytics callers need full objects + optimization flag. |
| **Decision** | **Maps from resource name to object.** Keys give callers resource name sets for free; values give metadata for free. No extra RPC calls, no separate conversion for the common cases. See Section 3 (Outputs). |

### Divergence 5: Combined Selected Users + Selected Groups Semantics

| | ParseUserFilterForAnalytics | MoveFiltersToUserFilter (old pattern) |
|-|---------------------------|--------------------------------------|
| **Behavior** | **INTERSECTION** — selected users ∩ group members | **UNION** — selected users ∪ group members |
| **Example** | Users=[A,B], Group=[team2: B,C,D] → [B] | Users=[A,B], Group=[team2: B,C,D] → [A,B,C,D] |
| **Code evidence** | `filterUsersByGroups` intersects base population with group members | `MoveFiltersToUserFilter` case b1: `DedupUsers(append(...))` |
| **Decision** | **UNION** — the canonical behavior is UNION (see B-SF-3). ~~`ParseUserFilterForAnalytics` will be updated to match.~~ **Fixed (2026-02-19)** on branch `xwang/fix-bsf3-union-semantics`. |

### Divergence 6: Sorting Key

| | ParseUserFilterForAnalytics / buildUserGroupMappings | ListUsersMappedToGroups / ParseFiltersToUsers |
|-|------------------------------------------------------|-----------------------------------------------|
| **Behavior** | Sorted by resource `Name` | Sorted by `FullName` |
| **Impact** | Different ordering of results depending on which code path is used. |
| **Decision** | **Resource name.** Stable identifier that doesn't change when display names are updated (see B-SO-1). UI-layer sorting by FullName is a presentation concern. |

### Divergence 7: Cache Support

| | Parse | ParseUserFilterForAnalytics | ListUsersMappedToGroups |
|-|-------|---------------------------|------------------------|
| **Behavior** | No caching. | Has cache parameters but they are **unused** (dead code). | Has actual working cache (when `enableListUsersCache=true` and no groups selected). |
| **Decision** | **No cache in the function signature.** Clean up dead code. If caching is needed, implement at a lower level. |

### Divergence 8: Unused Parameters

| Parameter | In ParseUserFilterForAnalytics | Status |
|-----------|-------------------------------|--------|
| `enableListUsersCache` | line 143 | **Unused** — declared but never referenced |
| `listUsersCache` | line 144 | **Unused** — declared but never referenced |
| `shouldMoveFiltersToUserFilter` | line 147 | **Unused** — declared but never referenced |

> **Decision**: Remove all three. They are migration artifacts.

### Divergence 9: `hasAgentAsGroupByKey` Coupling

| | Current behavior | Proposed behavior |
|-|-----------------|-------------------|
| **Root/default groups** | Included only when `hasAgentAsGroupByKey=true` | Always included in raw output; callers strip with utility |
| **Indirect memberships** | Skipped when `hasAgentAsGroupByKey=true` OR `includeDirectGroupMembershipsOnly=true` | Only controlled by `includeDirectGroupMembershipsOnly`; both direct and all maps always returned |
| **GroupsToAggregate filter** | All groups when `hasAgentAsGroupByKey=true`; filtered when false | Always return all groups in `AllGroups`; callers use `FinalGroups` for selected subset |

> **Decision**: Remove `hasAgentAsGroupByKey` from the user filter. See [Section 19](#19-proposal-remove-hasagentasgroupbykey-from-user-filter).

### Divergence 10: Child Group Expansion in GroupsToAggregate

| | ParseUserFilterForAnalytics / buildUserGroupMappings | ListUsersMappedToGroups (old pattern) |
|-|------------------------------------------------------|--------------------------------------|
| **Behavior** | `FetchGroups` returns only the explicitly-requested group IDs. **No child group expansion.** Sub-teams are missing from `groupsToAggregate`. | `ListGroups` sets `IncludeGroupMemberships: true`, iterates parent group's members to discover child groups. Sub-teams are included in `filterTeamGroupNames` and pass the `slices.Contains` check. |
| **Impact** | Team leaderboard for a parent team shows only one row (the parent) instead of rows for each sub-team. **Active production bug** (CONVI-6260). |
| **Code evidence** | `buildUserGroupMappings` line 395: `FetchGroups(ctx, customerID, profileID, filter, ...)` — filter contains only parent group IDs. Line 433: `slices.Contains(groupNames, groupName)` fails for child group names not in the list. | `ListGroups` lines 745-758: iterates `g.GetMembers()`, appends child groups. Line 865: `slices.Contains(filterTeamGroupNames, groupName)` passes for children. |
| **Decision** | **Expand child groups.** The unified implementation must match the old path's behavior: discover child teams from parent group memberships and include them in the group list used for the `slices.Contains` gate. See B-GH-4. |

---

## 19. Proposal: Remove `hasAgentAsGroupByKey` from User Filter

### Problem

`hasAgentAsGroupByKey` bundles three unrelated concerns into one boolean:

1. **Include root/default groups in mappings?** — a presentation concern
2. **Force direct-only memberships?** — redundant with `includeDirectGroupMembershipsOnly`
3. **Bypass selected groups for GroupsToAggregate?** — an aggregation concern

The flag's name describes a **caller's grouping strategy**, not a user filter behavior. It leaks the "group-by" concept (which is specific to analytics APIs) into a package that should be reusable across coaching, analytics, and future callers.

### What It Controls Today

| Decision | When `true` | When `false` |
|----------|------------|-------------|
| Root/default groups | Included in `UserNameToGroupNamesMap` | Excluded |
| Indirect memberships | Skipped (forces direct-only) | Included (unless `includeDirectGroupMembershipsOnly=true`) |
| GroupsToAggregate | All encountered TEAM groups | Only groups matching selection (if selection present) |

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
// result.UserToDirectGroups — complete (includes root/default)
// result.UserToAllGroups — complete (includes root/default)
// result.AllGroups — complete (all encountered TEAM groups)

// Caller applies post-processing:
if groupingByAgent {
    // Agent leaderboard: direct groups, keep root/default, all groups
    groupMap = result.UserToDirectGroups
    groups = result.AllGroups
} else {
    // Team leaderboard: strip root/default, use FinalGroups (selected groups)
    groupMap = userfilter.StripRootAndDefaultGroups(result.UserToDirectGroups)
    groups = result.FinalGroups
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

## 20. Proposal: Post-Processing Utilities

The user filter package should provide a set of utility functions for callers to transform the raw output into what they need. These are pure functions — no RPC calls, no state.

### Mapping Utilities

#### `StripRootAndDefaultGroups`

```go
// StripRootAndDefaultGroups removes root group and default group entries
// from a user-to-groups mapping.
func StripRootAndDefaultGroups(
    userToGroups map[string][]LiteGroup,
) map[string][]LiteGroup
```

**Use case**: Team leaderboard — don't want "Root" or "Default" as group buckets.

### Conversion Utilities

Since outputs use `map[string]...` keyed by resource name, callers get resource name sets for free via `Keys()`. These utilities handle conversions that require parsing.

#### `UserIDs`

```go
// UserIDs extracts user IDs (not full resource names) from a user map.
// Parses each resource name key to extract the ID component.
func UserIDs(users map[string]LiteUser) ([]string, error)
```

**Use case**: Coaching callers that need IDs for database queries.

#### `GroupNamesToStrings`

```go
// GroupNamesToStrings extracts group resource names from a user-to-groups
// mapping's values, returning a flat string-keyed string-valued map.
func GroupNamesToStrings(
    userToGroups map[string][]LiteGroup,
) map[string][]string
```

**Use case**: Analytics callers that need `map[string][]string` for response JOIN operations.

### Query Utilities

#### `ApplyToQuery`

Apply user filter result to a query's WHERE clause (replaces `ApplyUserFilterFromResult`).

```go
// ApplyToQuery applies the user filter result to request filter attributes.
// Returns true if the caller should return an empty response (no access).
//
// When ShouldQueryAllUsers=true: clears the user list in query (query all).
// When ShouldQueryAllUsers=false: sets the user list in query to Keys(FinalUsers).
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
// LiteGroup has no member lists — safe to include directly in responses
groupMap := result.UserToDirectGroups
groups := result.AllGroups

if shouldReturn := userfilter.ApplyToQuery(result, &req.Users, &req.Groups); shouldReturn {
    return &Response{}, nil
}
return buildPerUserPerGroupResponse(queryResults, groupMap, groups)
```

#### Team Leaderboard (group-by-group only)

```go
result, err := userfilter.Parse(...)

// Team leaderboard: strip root/default, use FinalGroups (already filtered to selected groups)
groupMap := userfilter.StripRootAndDefaultGroups(result.UserToDirectGroups)
groups := result.FinalGroups

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

// Coaching only needs user IDs — convert from structs to strings
userIDs, err := userfilter.UserIDs(result.FinalUsers)
return queryCoachingData(userIDs)
```

---

## Appendix A: Test Case to Behavior Mapping

| Test Case | Behaviors Verified |
|-----------|--------------------|
| `RootAccessReturnsAllAgents` | B-ACL-1, B-SF-1 |
| `LimitedAccessFiltersToAgentSubset` | B-ACL-2, B-BP-3 |
| `EmptyACLReturnsEmpty` | B-ACL-3 |
| `UnionOfUsersAndGroupsFromACL` | B-ACL-4, B-ACL-5 |
| `EmptyRequestReturnsAllAgents` | B-SF-1 |
| `WithUserFilterReturnsFilteredAgents` | B-SF-2 |
| `WithNonAgentFilterReturnsEmpty` | B-SF-4 |
| `WithGroupFilterReturnsOnlyGroupMembers` | B-GS-1, B-SF-6 |
| `WithGroupFilterRespectsDirectMembershipOnly` | B-GS-3 |
| `UserAndGroupFiltersTogether` | B-SF-3 |
| `TestParserWithSelections/all_selections` | B-SF-3 |
| `TrueFiltersDeactivated` | B-DU-1, B-BP-2 |
| `FalseIncludesAll` | B-DU-2 |
| `Case1_ACLDisabled_EmptyFilter_ReturnsTrue` | B-QO-1 |
| `Case3_LimitedAccess_EmptyFilter_ReturnsFalse` | B-QO-2 |
| `EnrichedCorrectly` | B-ME-1, B-BP-4 |
| `MissingMetadataHandledGracefully` | B-ME-2 |
| `PaginationHandling` | B-PG-2 |
| `ApplyResourceACLFails` | B-EH-1 |
| `ListUsersForAnalyticsFails` | B-EH-2 |
| `TestInvalidSelections/*` | B-EH-3, B-SF-5 |
| `TestGroupsByGroupType` | B-GS-2 |
| `TestACL/acl_enabled_without_root_access` | B-ACL-2 |
| `TestACL/acl_enabled_with_root_access` | B-ACL-1 |
| `TestParserEmptySelections/acl_disabled` | B-SF-1 |
| `TestParserEmptySelections/acl_enabled_without_root_access` | B-ACL-2 |

## Appendix B: Behaviors NOT Covered by Tests

These behaviors were discovered from implementation and caller code review only:

| Behavior | Source |
|----------|--------|
| B-GS-4: Unparseable group names silently skipped during expansion | `filterUsersByGroups` line 323 |
| B-GM-5: AllGroups unfiltered; callers use FinalGroups for subset | `ListUsersMappedToGroups` line 863 |
| B-GM-6: No membership data in group output (moot with LiteGroup) | Caller code in `retrieve_agent_stats.go` |
| B-GH-1–3: Group hierarchy and child team expansion | `ListGroups` lines 745-758 |
| B-GH-4: Child teams must appear in GroupsToAggregate (bug in `ParseUserFilterForAnalytics`) | CONVI-6260: `buildUserGroupMappings` line 395, `FetchGroups` missing child expansion |
| B-CC-1: Results discarded when not grouping by Agent/Group | `retrieve_agent_stats.go` lines 83-88 |
| B-CC-2: Missing user-to-groups map entries cause stats to be dropped | Caller response construction code |
| B-CC-4: Coaching callers only use user IDs | Coaching action files |
| B-PS-2: Groups scoped to profile (cross-profile bug) | `ListUsersMappedToGroups` comment |
| B-SO-2: Non-deterministic map iteration in unionUsers | `unionUsers` line 606 |
| B-EH-4: Client creation errors | `ParseFiltersToUsers` line 479 |
| B-EH-5: At least one filter required | `ParseFiltersToUsers` line 476 |
| Divergence 5: `ParseUserFilterForAnalytics` uses INTERSECTION instead of UNION | `filterUsersByGroups` vs `MoveFiltersToUserFilter` case b1 |
