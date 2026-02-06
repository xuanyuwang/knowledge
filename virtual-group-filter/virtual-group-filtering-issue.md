# Virtual Group Filtering Issue in Analytics APIs

## Summary

When filtering by a virtual group (DYNAMIC_GROUP type) in analytics APIs and **grouping by GROUP**, no data is returned for the virtual group. However:
- Filtering by individual users of that virtual group works correctly
- Filtering by virtual group AND grouping by AGENT also works correctly

**The issue is specifically with GROUP aggregation** - user stats are queried correctly, but they cannot be aggregated under the virtual group because the virtual group is explicitly excluded from `groupsToAggregate` and `userNameToGroupNamesMap`.

This affects **both** code paths (`enableParseUserFilterForAnalytics=true` and `false`).

## Root Cause

The issue is in `ListUsersMappedToGroups` function in `insights-server/internal/shared/common.go:767`.

When building `groupsToAggregate` and `userNameToGroupNamesMap`, the code **explicitly skips non-TEAM groups**:

```go
// insights-server/internal/shared/common.go:853-856
if membership.Group.GroupType != internaluserpb.LiteGroup_TEAM {
    // Skip when a group is not team, i.e. when a group is virtual group
    continue
}
```

This means:
1. Virtual groups are never added to `groupsToAggregate`
2. Virtual groups are never added to `userNameToGroupNamesMap`

## Detailed Flow Analysis

### Old Code Path (`enableParseUserFilterForAnalytics=false`)

**File**: `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`

1. **Line 131**: `ApplyResourceACL` is called - passes through the virtual group filter unchanged

2. **Lines 139-165**: `ListUsersMappedToGroups` is called
   - This function iterates through user memberships
   - **Line 853-856**: Skips any group where `GroupType != TEAM`
   - Result: `groupsToAggregate` does NOT contain the virtual group
   - Result: `userNameToGroupNamesMap` does NOT map users to the virtual group

3. **Lines 229-250**: `MoveGroupFilterToUserFilterForQA` is called
   - This correctly expands the virtual group to its member users via `ListUsersForAnalytics`
   - Users ARE correctly identified

4. **Line 181-182**: `retrieveQAScoreStatsInternalForGroupsInSingleQuery` is called (when grouping by GROUP)

5. **Lines 428-493**: `convertRowsPerUserToPerGroupQAScoreStatsResponse` maps user stats to groups
   ```go
   // Line 435-438
   groupMap := make(map[string]*userpb.Group)
   for _, group := range groups {  // groups = groupsToAggregate (missing virtual group!)
       groupMap[group.GetName()] = group
   }

   // Line 441-446
   qaScoreResultByGroup := make(map[string]*analyticspb.QAScoreResult)
   for groupName := range groupMap {
       qaScoreResultByGroup[groupName] = &analyticspb.QAScoreResult{...}
   }

   // Line 449-460: Try to map user scores to groups
   for _, score := range perUserResp.QaScoreResult.Scores {
       if groupNames, exists := userNameToGroupNamesMap[score.GroupedBy.User.Name]; exists {
           for _, groupName := range groupNames {
               if _, exists := qaScoreResultByGroup[groupName]; exists {
                   // Virtual group is NOT in qaScoreResultByGroup!
                   qaScoreResultByGroup[groupName].Scores = append(...)
               }
           }
       }
   }
   ```

6. **Result**: User stats are fetched but cannot be mapped to the virtual group because:
   - The virtual group is not in `groupsToAggregate`
   - The virtual group is not in `userNameToGroupNamesMap`
   - Therefore, `qaScoreResultByGroup` doesn't contain an entry for the virtual group

### Why Individual User Filtering Works

When filtering by individual users instead of a virtual group:
- The user filter is used directly in the ClickHouse query
- No group-to-user mapping is needed
- Stats are returned for each user without needing `groupsToAggregate`

## Affected Code Locations

| File | Line(s) | Description |
|------|---------|-------------|
| `insights-server/internal/shared/common.go` | 853-856 | Skip logic that excludes virtual groups |
| `insights-server/internal/shared/common.go` | 802-808 | Only TEAM groups added to `filterTeamGroupNames` |
| `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go` | 428-493 | Group mapping that fails for virtual groups |

## Impact

This affects all analytics APIs that:
1. Filter by virtual groups (DYNAMIC_GROUP type)
2. **AND** group results by GROUP (not AGENT)

Both code paths are affected (`enableParseUserFilterForAnalytics=true` and `false`).

**Not affected:**
- Filtering by virtual group + grouping by AGENT (works correctly)
- Filtering by virtual group + no grouping (works correctly)
- Filtering by individual users (works correctly)

## Potential Fix

The skip logic at line 853-856 needs to be revisited to handle virtual groups appropriately when they are explicitly requested in the filter.

## New Code Path Analysis (`enableParseUserFilterForAnalytics=true`)

**The new code path has the SAME issue.**

### Evidence

**File**: `insights-server/internal/analyticsimpl/common_user_filter.go`

1. **Lines 346-349**: `buildUserGroupMappings` only fetches TEAM groups:
   ```go
   filter := &userpb.ListGroupsRequest_ListGroupsFilter{
       GroupIds:   groupIDs,
       GroupTypes: []userpb.Group_Type{userpb.Group_TEAM},  // Only TEAM!
   }
   ```

2. **Lines 382-385**: Same skip logic for non-TEAM groups:
   ```go
   if membership.Group.GroupType != internaluserpb.LiteGroup_TEAM {
       // Skip when a group is not team, i.e. when a group is virtual group
       continue
   }
   ```

### Difference in User Filtering

The new code path does handle virtual groups better for **user filtering** (not group aggregation):

- **Line 138-140**: `filterUsersByGroups` filters users by group membership
  ```go
  if len(reqGroups) > 0 {
      groundTruthUsers = filterUsersByGroups(groundTruthUsers, reqGroups, includeDirectGroupMembershipsOnly)
  }
  ```

- This function (lines 271-305) checks `user.Memberships` which DOES include virtual group memberships
- So users ARE correctly filtered when a virtual group is specified

### The Problem Remains

Even though users are correctly filtered, the `buildUserGroupMappings` function:
1. Only fetches TEAM groups (line 348)
2. Skips virtual groups in user-to-group mapping (line 382-384)

This means when grouping by GROUP:
- `groupsToAggregate` won't contain the virtual group
- `userNameToGroupNamesMap` won't map users to the virtual group
- The same aggregation failure occurs as in the old code path

### Summary Table

| Aspect | Old Code Path | New Code Path |
|--------|---------------|---------------|
| User filtering by virtual group | Works (via `ListUsersForAnalytics`) | Works (via `filterUsersByGroups`) |
| Query returns data for users in virtual group | **YES** | **YES** |
| Virtual groups in `groupsToAggregate` | **NO** (skipped) | **NO** (skipped) |
| Virtual groups in `userNameToGroupNamesMap` | **NO** (skipped) | **NO** (skipped) |
| Group-by-AGENT with virtual group filter | **Works** | **Works** |
| Group-by-GROUP aggregation for virtual groups | **FAILS** | **FAILS** |

### Clarification: What Works vs What Fails

**Works:**
- Filtering users by virtual group membership
- Querying stats for users who are members of a virtual group
- Grouping by AGENT when filtering by a virtual group (returns per-user stats)

**Fails:**
- Grouping by GROUP when filtering by a virtual group
- The virtual group won't appear in the response because:
  1. `groupsToAggregate` doesn't include virtual groups
  2. `userNameToGroupNamesMap` doesn't map users to virtual groups
  3. `convertRowsPerUserToPerGroupQAScoreStatsResponse` can't aggregate user stats under the virtual group

## Recommended Fix

To properly support virtual groups in group-by-GROUP scenarios, the following changes are needed:

### 1. Update `buildUserGroupMappings` (new code path)

**File**: `insights-server/internal/analyticsimpl/common_user_filter.go`

```go
// Line 346-349: Include DYNAMIC_GROUP type
filter := &userpb.ListGroupsRequest_ListGroupsFilter{
    GroupIds:   groupIDs,
    GroupTypes: []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
}

// Lines 382-385: Remove or modify the skip logic
// Option A: Remove the skip entirely
// Option B: Only skip if explicitly filtering by TEAM groups
```

### 2. Update `ListUsersMappedToGroups` (old code path)

**File**: `insights-server/internal/shared/common.go`

Similar changes to include virtual groups in:
- `filterTeamGroupNames` (lines 802-808)
- Membership processing (lines 853-856)

### 3. Alternative: Use `ListUsersFromDynamicGroups`

The existing `ListUsersFromDynamicGroups` function (common.go:1143) already handles virtual groups correctly. Consider:
1. Detecting virtual groups in the filter
2. Calling `ListUsersFromDynamicGroups` for those groups
3. Merging results with team group processing

## Related Code

- `ListUsersFromDynamicGroups` function (common.go:1143) exists and properly handles virtual groups via `UserService.ListUsers`, but is not used in either code path for group aggregation.

## Testing Recommendations

1. Create a test case that filters by a virtual group and groups by GROUP
2. Verify that:
   - Users in the virtual group are returned
   - The virtual group appears in `groupsToAggregate`
   - Stats are correctly aggregated under the virtual group

---

## Manual Testing Results

**Date**: 2026-02-01
**Environment**: chat-staging (cox/sales)
**Virtual Group**: `019c1a18-b1e7-746c-a7a9-20f9443163d7` (containing users with QA data)

### Test Setup

- Created virtual group containing 3 users with QA score data:
  - `a57aee9c7560b323` (developer_agent)
  - `fbb92c609a26de2c` (api_server_e2e_agent)
  - `8c8b7449f1497e86`

### Test Results

| Test | Filter | Aggregation | Expected | Actual | Result |
|------|--------|-------------|----------|--------|--------|
| 1 | Virtual group | AGENT | Users returned | Users returned (2 with data) | ✅ PASS |
| 2 | Virtual group | GROUP | Virtual group in results | **TEAM groups returned instead** | ❌ FAIL |
| 3 | TEAM group | GROUP | TEAM group in results | TEAM group in results | ✅ PASS |

### Test 1: Virtual Group + AGENT Aggregation (Works)

**Request:**
```json
{
  "parent": "customers/cox/profiles/sales",
  "filter_by_attribute": {
    "groups": [{"name": "customers/cox/groups/019c1a18-b1e7-746c-a7a9-20f9443163d7"}]
  },
  "filter_by_time_range": {
    "start_timestamp": "2025-11-01T00:00:00Z",
    "end_timestamp": "2026-02-01T00:00:00Z"
  },
  "frequency": "DAILY",
  "group_by_attribute_types": ["QA_ATTRIBUTE_TYPE_AGENT"]
}
```

**Result:** ✅ Returns per-user stats for users in the virtual group

### Test 2: Virtual Group + GROUP Aggregation (FAILS)

**Request:**
```json
{
  "parent": "customers/cox/profiles/sales",
  "filter_by_attribute": {
    "groups": [{"name": "customers/cox/groups/019c1a18-b1e7-746c-a7a9-20f9443163d7"}]
  },
  "filter_by_time_range": {
    "start_timestamp": "2025-11-01T00:00:00Z",
    "end_timestamp": "2026-02-01T00:00:00Z"
  },
  "frequency": "DAILY",
  "group_by_attribute_types": ["QA_ATTRIBUTE_TYPE_GROUP"]
}
```

**Result:** ❌ **Virtual group NOT in response.** Instead, returns stats grouped by users' TEAM memberships:
- `temp_manager` (TEAM)
- `Andrew L` (TEAM)
- `API Server E2E Agent` (TEAM)
- `Team Test Coaching Report` (TEAM)

**Key Finding:** The user filtering works correctly (only users from the virtual group are queried), but the GROUP aggregation maps stats to TEAM groups instead of the filtered virtual group.

### Test 3: TEAM Group + GROUP Aggregation (Works)

**Request:**
```json
{
  "parent": "customers/cox/profiles/sales",
  "filter_by_attribute": {
    "groups": [{"name": "customers/cox/groups/9f26c522-f470-49c3-8b6b-947119770ede"}]
  },
  "filter_by_time_range": {
    "start_timestamp": "2026-01-01T00:00:00Z",
    "end_timestamp": "2026-02-01T00:00:00Z"
  },
  "frequency": "DAILY",
  "group_by_attribute_types": ["QA_ATTRIBUTE_TYPE_GROUP"]
}
```

**Result:** ✅ Returns stats aggregated under the TEAM group "Team Test Coaching Report"

### Conclusion

The documented issue is **CONFIRMED**:

1. **Virtual group filter + AGENT aggregation**: ✅ Works
2. **Virtual group filter + GROUP aggregation**: ❌ Fails - virtual group excluded from response
3. **TEAM group filter + GROUP aggregation**: ✅ Works

The root cause is confirmed: `groupsToAggregate` and `userNameToGroupNamesMap` explicitly exclude virtual groups (DYNAMIC_GROUP type), causing GROUP aggregation to fail for virtual groups while still working for TEAM groups.
