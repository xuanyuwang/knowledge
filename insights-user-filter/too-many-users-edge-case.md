# Edge Case: Too Many Users in ClickHouse Query

## Problem Summary

When `enableParseUserFilterForAnalytics` is enabled, the system may generate ClickHouse queries with extremely long `WHERE agent_user_id IN (...)` clauses, exceeding ClickHouse's query size limit (~1MB).

### Error Message
```
Removing error message from internal/unknown error, original error is: querying clickhouse:
code: 62, message: Syntax error: failed at position 1048561 (''2a2572426cc211a'')
(line 72, col 54993): '2a2572426cc211a', '2a2582426cc22cd', ...
Max query size exceeded: ''2a2572426cc211a''
```

### Root Cause

1. **New Implementation Behavior**: `ParseUserFilterForAnalytics` returns ALL users in `FinalUsers` when:
   - ACL is disabled + empty filter, OR
   - Root access + empty filter

2. **Query Building**: The `FinalUsers` are passed to `buildUsersConditionAndArgs` which creates:
   ```sql
   WHERE agent_user_id IN ('user1', 'user2', ..., 'userN')
   ```

3. **Large Customer Impact**: For customers with many agents (thousands), this creates a query string exceeding ClickHouse's default max query size limit (~1MB).

### Code Flow

```
1. ParseUserFilterForAnalytics (common_user_filter.go)
   │
   │  when shouldUseAllAgents=true:
   │    - groundTruthUsers contains ALL agents
   │    - FinalUsers = convertLiteUsersToUsers(groundTruthUsers) → ALL agents
   │
   └─► Returns FinalUsers with thousands of users

2. RetrieveAgentStats (retrieve_agent_stats.go:89)
   │
   │  req.FilterByAttribute.Users = result.FinalUsers
   │
   └─► Sets ALL users in FilterByAttribute

3. parseClickhouseFilter (common_clickhouse.go:200)
   │
   │  usersCondAndArgs = buildUsersConditionAndArgs(attribute.Users, targetTables)
   │
   └─► Creates WHERE agent_user_id IN (user1, user2, ..., userN)

4. ClickHouse query exceeds size limit → ERROR
```

### Old Implementation Comparison

The **old implementation** (before `enableParseUserFilterForAnalytics`) didn't have this issue because:

1. `shared.ApplyResourceACL` returns empty users/groups when root access or ACL disabled
2. `MoveFiltersToUserFilter` is only called when:
   - There are groups to expand, OR
   - Deactivated users need filtering
3. If neither condition is met, `req.FilterByAttribute.Users` stays empty
4. `buildUsersConditionAndArgs` with empty users returns empty conditions → no WHERE clause for users

```go
// Old implementation (retrieve_agent_stats.go:203-220)
if !a.enableParseUserFilterForAnalytics {
    if req.FilterByAttribute != nil &&
       (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
        // Only call MoveFiltersToUserFilter when needed
        req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(...)
    }
    // Otherwise, req.FilterByAttribute.Users stays empty → no user filter in query
}
```

---

## Proposed Solution

### Option 1: Add `ShouldQueryAllUsers` Flag (Recommended)

Add a new field to `ParseUserFilterResult` to indicate when the query should include all users without explicit filtering.

**Changes to `common_user_filter.go`:**

```go
type ParseUserFilterResult struct {
    // ... existing fields ...

    // ShouldQueryAllUsers indicates that the query should include all users
    // matching the ground truth criteria (role/status filters) without
    // explicit user ID filtering in the WHERE clause.
    //
    // When true:
    //   - FinalUsers contains all matching users (for response construction)
    //   - But callers should NOT add user filter to WHERE clause
    //   - Query will be filtered by time range, profile_id, etc.
    //
    // When false:
    //   - FinalUsers should be used in WHERE clause as usual
    ShouldQueryAllUsers bool
}

func ParseUserFilterForAnalytics(...) (*ParseUserFilterResult, error) {
    // ... existing code ...

    shouldUseAllAgents := (!isACLEnabled && len(finalUsers) == 0) ||
        (isRootAccess && len(finalUsers) == 0)

    // ... existing code ...

    return &ParseUserFilterResult{
        // ... existing fields ...
        ShouldQueryAllUsers: shouldUseAllAgents,  // NEW
    }, nil
}
```

**Changes to API implementations (e.g., `retrieve_agent_stats.go`):**

```go
if a.enableParseUserFilterForAnalytics {
    result, err := ParseUserFilterForAnalytics(...)
    if err != nil {
        return nil, err
    }

    // Only set users in FilterByAttribute when NOT querying all users
    if result.ShouldQueryAllUsers {
        // Clear users filter to avoid giant WHERE clause
        // Query will still be filtered by time range, profile_id, etc.
        req.FilterByAttribute.Users = []*userpb.User{}
    } else {
        req.FilterByAttribute.Users = result.FinalUsers
    }

    // Early return if no users (limited access with no managed users)
    if !result.ShouldQueryAllUsers && len(result.FinalUsers) == 0 {
        return &analyticspb.RetrieveAgentStatsResponse{}, nil
    }
}
```

### Option 2: Add User Count Threshold

Add a threshold check to skip the WHERE clause when user count exceeds a limit.

```go
const maxUsersInWhereClause = 500

if len(result.FinalUsers) > maxUsersInWhereClause {
    // Don't add users to WHERE clause - rely on other filters
    req.FilterByAttribute.Users = []*userpb.User{}
} else {
    req.FilterByAttribute.Users = result.FinalUsers
}
```

**Pros**: Simple to implement
**Cons**: Magic number, doesn't distinguish between "all users" and "many specific users"

### Option 3: Change Semantics of FinalUsers

Use different values to distinguish cases:
- `nil` FinalUsers: Query all users (no WHERE clause)
- `[]` (empty slice) FinalUsers: No users to query (early return)
- Non-empty FinalUsers: Use in WHERE clause

**Pros**: Clean API
**Cons**: Breaking change, subtle difference between nil and empty slice

---

## Recommendation

**Option 1 (Add `ShouldQueryAllUsers` flag)** is recommended because:

1. **Explicit**: The flag clearly communicates intent
2. **Safe**: FinalUsers still contains all users for response construction
3. **Backward Compatible**: Doesn't change existing semantics
4. **Correct**: Distinguishes between:
   - "Query all users" (don't add WHERE clause)
   - "Query these specific users" (add WHERE clause)
   - "No users to query" (early return)

---

## Implementation Checklist

### Phase 1: Core Changes

1. [ ] Add `ShouldQueryAllUsers` field to `ParseUserFilterResult`
2. [ ] Set `ShouldQueryAllUsers = true` when `shouldUseAllAgents` is true
3. [ ] Update documentation for `ParseUserFilterResult`

### Phase 2: API Updates

Update all APIs using `ParseUserFilterForAnalytics` to check `ShouldQueryAllUsers`:

1. [ ] RetrieveAgentStats
2. [ ] RetrieveConversationStats
3. [ ] RetrieveHintStats
4. [ ] RetrieveKnowledgeAssistStats
5. [ ] RetrieveSuggestionStats
6. [ ] RetrieveSummarizationStats
7. [ ] RetrieveSmartComposeStats
8. [ ] RetrieveNoteTakingStats
9. [ ] RetrieveGuidedWorkflowStats
10. [ ] RetrieveKnowledgeBaseStats
11. [ ] RetrieveQAScoreStats (uses QA variant)

### Phase 3: Testing

1. [ ] Add unit test for `ShouldQueryAllUsers=true` case
2. [ ] Add integration test with many users
3. [ ] Verify query size stays within limits
4. [ ] Verify correct results for all access levels

---

## Impact Analysis

### Affected Scenarios

| Scenario | Old Behavior | New Behavior (Current Bug) | Fixed Behavior |
|----------|-------------|---------------------------|----------------|
| Root access + empty filter | No user WHERE clause | WHERE IN (all users) → ERROR | No user WHERE clause |
| ACL disabled + empty filter | No user WHERE clause | WHERE IN (all users) → ERROR | No user WHERE clause |
| Limited access + managed users | WHERE IN (managed users) | WHERE IN (managed users) | WHERE IN (managed users) |
| Limited access + no managed | Early return | Early return | Early return |
| Explicit user filter | WHERE IN (filtered users) | WHERE IN (filtered users) | WHERE IN (filtered users) |

### Risk Assessment

- **Low Risk**: The change restores behavior consistent with the old implementation
- **Testing**: Existing tests should pass; add specific tests for large user counts
- **Rollout**: Can be deployed with existing feature flag `enableParseUserFilterForAnalytics`

---

**Document Created**: 2026-01-29
**Status**: ✅ Implementation complete with unit tests

## Implementation Summary

The fix has been implemented by adding `ShouldQueryAllUsers` flag to `ParseUserFilterResult`.

### Files Modified

1. `insights-server/internal/analyticsimpl/common_user_filter.go`
   - Added `ShouldQueryAllUsers` field to `ParseUserFilterResult` struct
   - Updated `shouldUseAllAgents` condition to also check for empty `finalGroups`
   - Updated user filtering logic to only filter when `finalUsers` is non-empty

2. Updated 12 API files to check `ShouldQueryAllUsers`:
   - `retrieve_agent_stats.go`
   - `retrieve_conversation_stats.go`
   - `retrieve_hint_stats.go`
   - `retrieve_live_assist_stats.go`
   - `retrieve_summarization_stats.go`
   - `retrieve_note_taking_stats.go`
   - `retrieve_suggestion_stats.go`
   - `retrieve_knowledge_base_stats.go`
   - `retrieve_guided_workflow_stats.go`
   - `retrieve_knowledge_assist_stats.go`
   - `retrieve_smart_compose_stats.go`
   - `retrieve_qa_score_stats.go`

3. `insights-server/internal/analyticsimpl/common_user_filter_test.go`
   - Added `TestShouldQueryAllUsers` test suite with 8 test cases covering:
     - Case 1: ACL disabled + empty filter → ShouldQueryAllUsers=true
     - Case 1 with user filter → ShouldQueryAllUsers=false
     - Case 2: Root access + empty filter → ShouldQueryAllUsers=true
     - Case 2 with user filter → ShouldQueryAllUsers=false
     - Case 3: Limited access → ShouldQueryAllUsers=false (always)
     - Case 3 with no managed users → ShouldQueryAllUsers=false, FinalUsers=[]
     - Case 4: User filter or group filter (any access level) → ShouldQueryAllUsers=false
     - Large user count (1200+) with empty filter → ShouldQueryAllUsers=true

### Key Logic Changes

**shouldUseAllAgents condition (updated to include group filter check):**
```go
shouldUseAllAgents := (!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
    (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0)
```

**User filtering logic (updated to handle group-only filters):**
```go
if !shouldUseAllAgents && len(finalUsers) > 0 {
    // Filter finalUsers to only include agents from ground truth.
    // Note: We only filter when finalUsers is non-empty. When finalUsers is empty but
    // finalGroups is non-empty (group filter without user filter), the group filtering
    // is handled later in buildUserGroupMappings.
    groundTruthUsers = updateGroundTruthUsers(finalUsers, groundTruthUsers)
}
```

### Key Changes in Each API

```go
// Handle user filtering for the WHERE clause:
// - ShouldQueryAllUsers=true: Don't add user filter (avoids query size limit for large user counts)
//   This happens only for Case 1 (ACL disabled) or Case 2 (root access) with empty filters.
// - ShouldQueryAllUsers=false: Use FinalUsers in WHERE clause
//   This happens for Case 3 (limited access) or Case 4 (group filter) where we MUST filter.
if result.ShouldQueryAllUsers {
    req.FilterByAttribute.Users = []*userpb.User{}
} else {
    req.FilterByAttribute.Users = result.FinalUsers
}

// Early return if no users to query (e.g., limited access with no managed users)
// Note: Only check this when NOT querying all users, since ShouldQueryAllUsers=true
// means we want all users (FinalUsers is populated for metadata enrichment only).
if !result.ShouldQueryAllUsers && len(result.FinalUsers) == 0 {
    return &analyticspb.RetrieveXXXStatsResponse{}, nil
}
```

---

## Manual Testing Results

**Date**: 2026-02-01
**Environment**: chat-staging (cox/sales)
**PR**: https://github.com/cresta/go-servers/pull/25467

### Test Setup

- insights-server running locally with port-forwards to chat-staging ClickHouse
- Used `RetrieveAgentStats` API via grpcurl
- ClickHouse queries logged to `/tmp/insights-server.log`

### Test Cases

| Test | Scenario | ShouldQueryAllUsers | WHERE clause | Result |
|------|----------|---------------------|--------------|--------|
| 1 | Empty filter (root access) | `true` | No user filter | ✅ PASS |
| 2 | With explicit user filter | `false` | User IDs in clause | ✅ PASS |
| 3 | TEAM group filter | `false` | Group member IDs in clause | ✅ PASS |
| 4 | Virtual group filter | `false` | Group member IDs in clause | ✅ PASS |
| 5 | `exclude_deactivated_users: true` | `false` | Active user IDs in clause | ✅ PASS |
| 6 | `include_dev_users: true` | `true` | No user filter | ✅ PASS |

### Test 1: Empty Filter (ShouldQueryAllUsers=true)

**Request:**
```json
{
  "parent": "customers/cox",
  "profile_id": "sales",
  "filter_by_time_range": {
    "start_timestamp": "2026-01-01T00:00:00Z",
    "end_timestamp": "2026-02-01T00:00:00Z"
  },
  "frequency": "DAILY"
}
```

**ClickHouse Query (excerpt):**
```sql
WHERE
  conversation_source in (0, 8)
  AND is_dev_user = 0
  AND agent_user_id <> ''
  AND ((conversation_start_time >= '2026-01-01 00:00:00') AND (conversation_start_time < '2026-02-01 00:00:00'))
```

**Result:** ✅ No `agent_user_id IN (...)` clause - query will not exceed size limit

### Test 2: With User Filter (ShouldQueryAllUsers=false)

**Request:**
```json
{
  "parent": "customers/cox",
  "profile_id": "sales",
  "filter_by_attribute": {
    "users": [{"name": "customers/cox/users/4556009cdf3d1abf"}]
  },
  "filter_by_time_range": {
    "start_timestamp": "2026-01-01T00:00:00Z",
    "end_timestamp": "2026-02-01T00:00:00Z"
  },
  "frequency": "DAILY"
}
```

**ClickHouse Query (excerpt):**
```sql
WHERE
  conversation_source in (0, 8)
  AND is_dev_user = 0
  AND agent_user_id <> ''
  AND ((conversation_start_time >= '2026-01-01 00:00:00') AND (conversation_start_time < '2026-02-01 00:00:00') AND (agent_user_id IN ('4556009cdf3d1abf')))
```

**Result:** ✅ `agent_user_id IN (...)` clause correctly added when filtering by specific users

### Test 3: TEAM Group Filter (ShouldQueryAllUsers=false)

**Request:**
```json
{
  "parent": "customers/cox",
  "profile_id": "sales",
  "filter_by_attribute": {
    "groups": [{"name": "customers/cox/groups/7dfc52d3-ed7b-4fc2-8428-e2f9b5ec3a43"}]
  },
  "filter_by_time_range": {...},
  "frequency": "DAILY"
}
```

**Result:** ✅ `agent_user_id IN (...)` clause with team member IDs

### Test 4: Virtual Group Filter (ShouldQueryAllUsers=false)

**Request:**
```json
{
  "parent": "customers/cox",
  "profile_id": "sales",
  "filter_by_attribute": {
    "groups": [{"name": "customers/cox/groups/0198a2dd-811c-7395-9f8b-bea108e56fc9"}]
  },
  "filter_by_time_range": {...},
  "frequency": "DAILY"
}
```

**Result:** ✅ `agent_user_id IN (...)` clause with virtual group member IDs

### Test 5: exclude_deactivated_users (ShouldQueryAllUsers=false)

**Request:**
```json
{
  "parent": "customers/cox",
  "profile_id": "sales",
  "filter_by_attribute": {
    "exclude_deactivated_users": true
  },
  "filter_by_time_range": {...},
  "frequency": "DAILY"
}
```

**Result:** ✅ `agent_user_id IN (...)` clause with ALL active user IDs

**Note:** This generates a large WHERE clause with all active users. For large customers, this could still approach the query size limit. However, this is by design - when explicitly filtering deactivated users, we must include the active user IDs.

### Test 6: include_dev_users (ShouldQueryAllUsers=true)

**Request:**
```json
{
  "parent": "customers/cox",
  "profile_id": "sales",
  "filter_by_attribute": {
    "include_dev_users": true
  },
  "filter_by_time_range": {...},
  "frequency": "DAILY"
}
```

**Result:** ✅ No `agent_user_id IN (...)` clause - only time range filter

### Conclusion

The `ShouldQueryAllUsers` flag is working as expected:
- When empty filter with root access → flag is `true` → no user IDs in WHERE clause → prevents query size limit issues
- When explicit user filter → flag is `false` → user IDs correctly added to WHERE clause
- When group filter → flag is `false` → group member IDs in WHERE clause
- When `exclude_deactivated_users=true` → flag is `false` → active user IDs in WHERE clause (by design)
- When `include_dev_users=true` (no other filters) → flag is `true` → no user filter

### Remaining Test Cases (Covered by Unit Tests)

| Test | Scenario | Unit Test Coverage |
|------|----------|-------------------|
| 7 | Limited access (non-root user) + empty filter | ✅ `TestShouldQueryAllUsers/Case3_LimitedAccess_EmptyFilter_ReturnsFalse` |
| 8 | Limited access + no managed users | ✅ `TestShouldQueryAllUsers/Case3_LimitedAccess_NoManagedUsers_ReturnsFalseWithEmptyUsers` |

**Unit Test Details:**

**Test 7: Limited Access + Empty Filter**
- **File:** `common_user_filter_test.go:1303-1342`
- **Setup:** ACL enabled, `isRootAccess=false`, manager has access to [agent1, agent2] only
- **Expected:** `ShouldQueryAllUsers=false`, `FinalUsers=[agent1, agent2]`
- **Reason:** Limited access users must NEVER have `ShouldQueryAllUsers=true` - they should only see their managed users

**Test 8: Limited Access + No Managed Users**
- **File:** `common_user_filter_test.go:1344-1365`
- **Setup:** ACL enabled, `isRootAccess=false`, manager has no managed users
- **Expected:** `ShouldQueryAllUsers=false`, `FinalUsers=[]` (triggers early return in API)
- **Reason:** User with no access should get empty results, not all users

**Note:** Manual testing of these scenarios requires a limited access token (non-admin user), which is non-trivial to obtain in staging environments. The unit tests provide comprehensive coverage of these scenarios.
