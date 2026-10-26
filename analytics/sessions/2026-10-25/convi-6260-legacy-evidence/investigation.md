# CONVI-6260: Team Leaderboard Not Breaking Out Sub-Teams — Root Cause Analysis

## Issue Summary

**Customer:** Hilton
**Reporter:** Tari Mills | **Assignee:** Eleanore An
**Status:** Triage | **Priority:** Medium
**Linear:** https://linear.app/cresta/issue/CONVI-6260

Hilton reports that when filtering Performance Insights and Assistance Insights by a **parent team** and selecting **"Leaderboard by criteria: Team"**, the leaderboard only shows the parent team as a single row instead of breaking out the sub-teams underneath it.

### Reproduction Steps
1. Filters: Template = "All Key Performance Template"
2. Teams, groups or users = **Team Chad Kestner** (a parent team)
3. Date = This month | Monthly
4. Leaderboard by criteria = **Team**

**Observed:** Only "Team Chad Kestner" appears as one row.
**Expected:** Sub-teams (Beth Coppa, Gustavo Gonzalez, Jackie Wilson, Annie Ramirez, Warren Shum, Tim Maurer) should appear as separate rows with their own metrics.

---

## Architecture Overview

### Team Leaderboard Data Flow

```
Request: FilterByAttribute.Groups = [Parent Team], GroupByAttributeTypes = [GROUP]
    |
    v
ApplyResourceACL → filter by caller permissions
    |
    v
ListUsersMappedToGroups (old path) / ParseUserFilterForAnalytics (new path)
  → returns: userNameToGroupNamesMap, groupsToAggregate, users
    |
    v
retrieveXxxStatsInternalForGroupsInSingleQuery
  → query per-agent stats
  → aggregate per agent into per team using userNameToGroupNamesMap
  → one row per group in groupsToAggregate
```

**Key:** `groupsToAggregate` determines which teams appear as rows in the leaderboard. If only the parent team is in this list, only one row appears.

### Two Code Paths

| | Old Path | New Path |
|---|---|---|
| **Function** | `ListUsersMappedToGroups` | `ParseUserFilterForAnalytics` → `buildUserGroupMappings` |
| **Location** | `shared/common.go:768` | `analyticsimpl/common_user_filter.go:131, 356` |
| **Feature Flag** | Default (flag OFF) | `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS=true` |
| **Production Status** | Only Schwab clusters | **ACTIVE** (enabled everywhere except Schwab) |

### Affected APIs
- **RetrieveAgentStats** (Performance Insights): Uses **new path** (flag ON) → `ParseUserFilterForAnalytics` (`retrieve_agent_stats.go:47`)
- **RetrieveAssistanceStats** (Assistance Insights): **Always uses old path** — no feature flag at all (`retrieve_assistance_stats.go:94`)
- **All 12 migrated APIs** (RetrieveAgentStats, RetrieveConversationStats, RetrieveHintStats, etc.): Use new path when flag is ON

---

## Root Cause Analysis

### Critical Code: The `slices.Contains` Gate

Both the old and new paths share the same pattern for deciding which groups make it into `groupsToAggregate`. The gate is a `slices.Contains` check:

**Old path** (`shared/common.go:865`):
```go
if hasAgentAsGroupByKey || len(filterTeamGroupNames) == 0 || slices.Contains(filterTeamGroupNames, groupName) {
    groupNameToGroup[groupName] = &userpb.Group{...}
}
```

**New path** (`common_user_filter.go:433`):
```go
if hasAgentAsGroupByKey || len(groupNames) == 0 || slices.Contains(groupNames, groupName) {
    groupNameToGroup[groupName] = &userpb.Group{...}
}
```

For team leaderboard (`hasAgentAsGroupByKey=false`) with a group filter applied (`len(groupNames) > 0`), **a group only enters `groupsToAggregate` if it's in the `groupNames`/`filterTeamGroupNames` list.**

The question then becomes: **does this list include sub-teams?**

### Old Path: Child Group Expansion via `ListGroups`

In `ListUsersMappedToGroups`, the group list is built by calling `ListGroups()` (`shared/common.go:698`):

```go
groups, err := ListGroups(ctx, userClientRegistry, customerID, profileID, reqFilterGroups, false)
```

`ListGroups` calls the User Service API with:
- `IncludeGroupMemberships: true`
- `IncludeIndirectGroupMemberships: true`

Then iterates members to find child groups (`shared/common.go:740-757`):
```go
// Parse team and child teams.
for _, g := range listGroups.Groups {
    groupResults = append(groupResults, g)
    for _, m := range g.GetMembers() {
        if m.IsRoot { continue }
        if m.GetGroup() == g.Name && m.MemberType == userpb.GroupMembership_GROUP {
            childGroup := &userpb.Group{
                Name:        userpb.GroupName{CustomerID: customerID, GroupID: m.MemberId}.String(),
                DisplayName: m.MemberDisplayName,
                Type:        userpb.Group_TEAM,
            }
            groupResults = append(groupResults, childGroup)  // <-- child teams added
        }
    }
}
```

So `filterTeamGroupNames` SHOULD include both the parent AND child teams.

**However**, the `ListUsersForAnalytics` call uses `GroupIds` from the ORIGINAL request groups (parent only):
```go
// Line 815-822: Only the original parent group ID, NOT the expanded children
reqFilterGroupIDs := []string{}
for _, group := range reqFilterGroups {  // reqFilterGroups = [parent team only]
    groupName, _ := userpb.ParseGroupName(group.GetName())
    reqFilterGroupIDs = append(reqFilterGroupIDs, groupName.GroupID)
}
```

Per the proto documentation for `ListUsersForAnalyticsRequest.groupIds`:
> "IDs of the groups to list users for. **By default only return users that are direct members of those groups.**"

With `IncludeIndirectGroupMemberships: true`:
> "also include users that are indirect members of the groups specified in group_ids"

**This is where the behavior diverges based on how the User Service resolves memberships.**

### Hypothesis: User Service Membership Scoping

When `ListUsersForAnalytics` is called with `GroupIds = [parent-team-id]`:

1. It returns all users who are members (direct or indirect) of the parent team — agents in sub-teams ARE returned
2. **Each user's `Memberships` field**: this is the critical question
   - **If ALL memberships are returned** (including sub-team): Sub-teams pass the `slices.Contains` check → sub-teams appear in leaderboard ✓
   - **If only memberships related to the queried GroupIds are returned**: Only parent team membership is present → sub-teams DON'T appear ✗

Test mocks suggest ALL memberships are returned (agents have both direct sub-team and indirect parent in their membership list). But production behavior may differ.

### New Path: Confirmed Bug (NOT currently active)

In `buildUserGroupMappings` (`common_user_filter.go:375-398`), the group list is built using `FetchGroups` (from `shared/user-filter/user_filter.go:491`):

```go
filter := &userpb.ListGroupsRequest_ListGroupsFilter{
    GroupIds:   groupIDs,  // [parent-id only]
    GroupTypes: []userpb.Group_Type{userpb.Group_TEAM},
}
groups, err := userfilter.FetchGroups(ctx, customerID, profileID, filter, userServiceClient)
```

Unlike `ListGroups` in the old path, `FetchGroups`:
- Does NOT set `IncludeGroupMemberships: true`
- Does NOT iterate members to find child groups
- Returns ONLY the explicitly requested group(s)

Result: `groupNames = ["customers/.../groups/chad-kestner-id"]` — **no sub-teams**. The `slices.Contains` check at line 433 blocks all sub-teams from `groupsToAggregate`.

**This is a definitive bug, but it's not active in production (feature flag is OFF).**

---

## Summary of Findings

| Path | Active in Prod? | Sub-Team Expansion? | Bug? |
|------|-----------------|---------------------|------|
| Old (`ListUsersMappedToGroups`) | Schwab only + `RetrieveAssistanceStats` everywhere | `ListGroups` expands children ✓ | No known bug |
| New (`ParseUserFilterForAnalytics`) | **YES** (all non-Schwab clusters) | `FetchGroups` does NOT expand children ✗ | **ROOT CAUSE** |

### Root Cause: `buildUserGroupMappings` Missing Child Group Expansion

The bug is in `buildUserGroupMappings` (`common_user_filter.go:375-398`). When the new path is active:

1. `FetchGroups` is called with `GroupIds = [parent-id]` — returns ONLY the parent group, no children
2. `groupNames = [parent-name]` — sub-teams not included
3. For each user's memberships, `slices.Contains(groupNames, groupName)` at line 433 blocks sub-teams from `groupsToAggregate`
4. `groupsToAggregate = [parent team only]` → leaderboard shows one row

**The old path** (`ListUsersMappedToGroups`) avoids this by calling `ListGroups` which sets `IncludeGroupMemberships: true` and explicitly iterates group members to find child groups (lines 740-757). This expansion was lost when the code was migrated to the new path.

### Why Assistance Insights is also affected

`RetrieveAssistanceStats` always uses the old path (no feature flag). If the customer also reports the bug there, it may be a separate issue in the old path or the customer may be primarily experiencing the Performance Insights bug. The old path's `ListGroups` does expand child groups, so sub-teams should appear correctly in the Assistance leaderboard.

---

## Recommended Investigation Steps

### Step 1: Verify User Service Response
Check what `ListUsersForAnalytics` actually returns for Hilton's parent team:
- Call the API with `GroupIds = [chad-kestner-id]`, `IncludeIndirectGroupMemberships = true`
- Inspect returned users' `Memberships` field — do they include sub-team memberships?

### Step 2: Verify ListGroups Expansion
Check what `ListGroups` returns for the parent team:
- Call with `GroupIds = [chad-kestner-id]`, `IncludeGroupMemberships = true`, `IncludeIndirectGroupMemberships = true`
- Inspect the parent group's `Members` field — are sub-teams listed as `GROUP`-type members?

### Step 3: Verify Hilton's Team Hierarchy
Check how sub-teams are configured:
- Are sub-teams `TEAM` type groups?
- Are they direct group-members of the parent team?
- What does the membership structure look like?

### Step 4: Add Logging
Add temporary debug logging in `ListUsersMappedToGroups` to capture:
- What `ListGroups` returns (are child groups expanded?)
- What `filterTeamGroupNames` contains
- What memberships `ListUsersForAnalytics` returns per user
- What ends up in `groupNameToGroup`

---

## Fix Applied

In `buildUserGroupMappings` (`common_user_filter.go:370-398`), replaced `userfilter.FetchGroups` with `shared.ListGroups` which expands child groups:

```diff
-	userServiceClient, err := userClientRegistry.CreateClient(customerID, profileID)
-	...
-	filter := &userpb.ListGroupsRequest_ListGroupsFilter{
-		GroupIds:   groupIDs,
-		GroupTypes: []userpb.Group_Type{userpb.Group_TEAM},
-	}
-	groups, err := userfilter.FetchGroups(ctx, customerID, profileID, filter, userServiceClient)

+	groups, err := shared.ListGroups(ctx, userClientRegistry, customerID, profileID, finalGroups, false)
```

`shared.ListGroups` (defined at `shared/common.go:698`) calls the User Service with `IncludeGroupMemberships: true` and iterates the response to extract child teams as GROUP-type members. This ensures `groupNames` includes both parent and child teams, allowing the `slices.Contains` check at line 433 to pass for sub-teams.

Also removed the now-unused `userfilter` import.

### Reproduction

Verified on `cresta/walter-dev` profile with a `RetrieveQAScoreStats` request:
- `groupByAttributeTypes: ["QA_ATTRIBUTE_TYPE_GROUP"]`
- `groups: [{name: "customers/cresta/groups/c640f1ad-..."}]` (parent team "xuanyu")
- Before fix: response only shows the parent team as one row
- After fix: response should show both parent and sub-teams as separate rows

---

## Key Files Referenced

| File | Lines | Content |
|------|-------|---------|
| `insights-server/internal/shared/common.go` | 698-766 | `ListGroups` — expands child groups |
| `insights-server/internal/shared/common.go` | 768-894 | `ListUsersMappedToGroups` — old path |
| `insights-server/internal/shared/common.go` | 838-873 | Membership iteration with `slices.Contains` gate |
| `insights-server/internal/analyticsimpl/common_user_filter.go` | 131-280 | `ParseUserFilterForAnalytics` — new path |
| `insights-server/internal/analyticsimpl/common_user_filter.go` | 356-445 | `buildUserGroupMappings` — builds groupsToAggregate |
| `insights-server/internal/analyticsimpl/common_user_filter.go` | 433 | `slices.Contains` gate for new path |
| `insights-server/internal/analyticsimpl/retrieve_agent_stats.go` | 17-146 | RetrieveAgentStats handler |
| `insights-server/internal/analyticsimpl/retrieve_assistance_stats.go` | 50-116 | RetrieveAssistanceStats handler |
| `insights-server/internal/analyticsimpl/analyticsimpl.go` | 161-164 | Feature flag initialization |
| `shared/user-filter/user_filter.go` | 491-520 | `FetchGroups` — no child expansion |
