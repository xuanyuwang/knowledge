# Metadata Enrichment Fix: Always Use FinalUsers for Metadata Enrichment

## Executive Summary

**All refactored APIs have a metadata enrichment bug** where they use `result.UsersFromGroups` instead of `result.FinalUsers` for building the metadata enrichment map.

**Root Cause**: `UsersFromGroups` is a **subset** of `FinalUsers` (only users with group memberships), causing metadata enrichment to fail for user-only filters.

**Solution**: **Always use `result.FinalUsers`** for metadata enrichment. It's simpler, correct, and covers all cases.

---

## Affected APIs

All 12 refactored APIs have this issue:

1. ✅ RetrieveAgentStats
2. ✅ RetrieveConversationStats (CONVI-6005)
3. ✅ RetrieveHintStats (CONVI-6007)
4. ✅ RetrieveKnowledgeAssistStats (CONVI-6020)
5. ✅ RetrieveSuggestionStats (CONVI-6015)
6. ✅ RetrieveSummarizationStats (CONVI-6016)
7. ✅ RetrieveSmartComposeStats (CONVI-6017)
8. ✅ RetrieveNoteTakingStats (CONVI-6018)
9. ✅ RetrieveGuidedWorkflowStats (CONVI-6019)
10. ✅ RetrieveKnowledgeBaseStats (CONVI-6008)
11. ✅ RetrieveQAScoreStats (CONVI-6010)
12. ✅ RetrieveLiveAssistStats (CONVI-6009)

---

## Root Cause Analysis

### Understanding UsersFromGroups vs FinalUsers

From `common_user_filter.go:203-216`:

```go
// Convert groundTruthUsers to []*userpb.User for final result
finalUsers = convertLiteUsersToUsers(groundTruthUsers, customerID)

// usersFromGroups are users that belong to finalGroups (for response construction)
// Build this from userNameToGroupNamesMap
usersFromGroupsMap := make(map[string]*internaluserpb.LiteUser)
for userName, groupNames := range userNameToGroupNamesMap {
    if len(groupNames) > 0 {  // ← Only users with group memberships
        if user, exists := groundTruthUsers[userName]; exists {
            usersFromGroupsMap[userName] = user
        }
    }
}
usersFromGroups := convertLiteUsersToUsers(usersFromGroupsMap, customerID)
```

**Key Facts**:

1. **`FinalUsers`**: ALL users from groundTruthUsers (complete set with full metadata)
2. **`UsersFromGroups`**: SUBSET of users where `len(groupNames) > 0`
3. **Relationship**: `FinalUsers ⊇ UsersFromGroups` (FinalUsers is a superset)
4. **Metadata quality**: Both have identical metadata from groundTruthUsers

### Current Pattern (Buggy)

```go
// Extract values from ParseUserFilterForAnalytics result
users = result.UsersFromGroups  // ❌ BUG: Subset only

if !(shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) ||
    shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP)) {
    users = []*userpb.User{}
    groupsToAggregate = []*userpb.Group{}
    userNameToGroupNamesMap = map[string][]string{}
}
req.FilterByAttribute.Users = result.FinalUsers
req.FilterByAttribute.Groups = result.FinalGroups

// Call clickhouse reading function
return a.readXXXStatsFromClickhouse(ctx, req, users)
```

### The Bug

**Scenario**: User-only filter + Group by AGENT

```
Request:
  FilterByAttribute.Users = [alice, bob]
  FilterByAttribute.Groups = []  ← No groups
  GroupByAttributeTypes = [AGENT]

After ParseUserFilterForAnalytics:
  result.UsersFromGroups = []        ← Empty (alice/bob have no group memberships in userNameToGroupNamesMap)
  result.FinalUsers = [alice, bob]   ← Complete set

After extracting values:
  users = result.UsersFromGroups = []  ← Empty! ❌

Clickhouse query:
  WHERE agent_user_id IN ('alice', 'bob')  ✅ Correct (uses req.FilterByAttribute.Users)
  Returns results for alice and bob        ✅ Correct

Metadata enrichment (in readXXXStatsFromClickhouse):
  usernameToUserMap := make(map[string]*userpb.User, len(users))
  for _, u := range users {  // ← Empty loop! users = []
      usernameToUserMap[u.Name] = u
  }
  // usernameToUserMap = {}  ← Empty map

  // Later when enriching results:
  if user, exists := usernameToUserMap[attr.Users[0].Name]; exists {
      attr.Users[0].FullName = user.FullName  // ← Never executes ❌
      attr.Users[0].Username = user.Username  // ← Never executes ❌
  }
```

**Result**: Metadata (FullName, Username) is missing in response.

### Why UsersFromGroups Was Used

**Historical context**: Before the refactoring, `UsersFromGroups` was populated by `ListUsersMappedToGroups`, which only ran when there were groups to expand. This made sense because:

1. No groups → No group-to-user mapping needed → `UsersFromGroups` empty
2. With groups → Need mapping → `UsersFromGroups` populated with group members

**After refactoring**: `ParseUserFilterForAnalytics` changed the semantics:
- `FinalUsers` = ALL users being queried (complete set)
- `UsersFromGroups` = Users with group memberships (subset for group aggregation)

The `users` parameter is **only used for metadata enrichment**, so it should use the **complete set** (`FinalUsers`), not a subset.

---

## The Fix

### Proposed Solution

**Simply use `result.FinalUsers` instead of `result.UsersFromGroups`:**

```go
// Extract values from ParseUserFilterForAnalytics result
userNameToGroupNamesMap = result.UserNameToGroupNamesMap
groupsToAggregate = result.GroupsToAggregate
users = result.FinalUsers  // ✅ Always use FinalUsers (complete set)

if !(shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) ||
    shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP)) {
    users = []*userpb.User{}
    groupsToAggregate = []*userpb.Group{}
    userNameToGroupNamesMap = map[string][]string{}
}
req.FilterByAttribute.Users = result.FinalUsers
req.FilterByAttribute.Groups = result.FinalGroups
```

### Why This Fix Is Correct

**Purpose of `users` parameter**:
- Passed to `readXXXStatsFromClickhouse(ctx, req, users)`
- Used to build `usernameToUserMap` for metadata enrichment
- NOT used for filtering (query uses `req.FilterByAttribute.Users`)

**Why FinalUsers is the right choice**:

1. **Complete coverage**: Contains ALL users that could appear in query results
2. **Same metadata quality**: Both FinalUsers and UsersFromGroups come from groundTruthUsers
3. **Superset relationship**: `FinalUsers ⊇ UsersFromGroups`, so using FinalUsers is always safe
4. **Simpler logic**: No complex fallback conditions needed

**Comparison table**:

| Scenario | UsersFromGroups | FinalUsers | Correct Choice |
|----------|----------------|------------|----------------|
| User-only filter + Group by AGENT | [] (empty) ❌ | [alice, bob] ✅ | FinalUsers |
| Group filter + Group by GROUP | [bob, charlie] ✅ | [bob, charlie] ✅ | Both work, FinalUsers simpler |
| User + Group filter + Group by AGENT | [charlie] ⚠️ | [alice, bob, charlie] ✅ | FinalUsers (complete) |
| User-only filter + No grouping | [] (empty) | [alice, bob] | Both work (cleared by next if) |

### Example Flow After Fix

```
Request:
  FilterByAttribute.Users = [alice, bob]
  FilterByAttribute.Groups = []
  GroupByAttributeTypes = [AGENT]

After ParseUserFilterForAnalytics:
  result.UsersFromGroups = []
  result.FinalUsers = [alice, bob]

After extracting with fix:
  users = result.FinalUsers = [alice, bob]  ✅ Complete set!

Metadata enrichment:
  usernameToUserMap = {
    "alice": {FullName: "Alice Agent", Username: "alice"},
    "bob": {FullName: "Bob Agent", Username: "bob"}
  }

  if user, exists := usernameToUserMap[attr.Users[0].Name]; exists {
      attr.Users[0].FullName = user.FullName  ✅ Works!
      attr.Users[0].Username = user.Username  ✅ Works!
  }
```

---

## Implementation Plan

### Step 1: Apply Fix to All APIs

**One-line change per API**:

```diff
  // Extract values from result
  userNameToGroupNamesMap = result.UserNameToGroupNamesMap
  groupsToAggregate = result.GroupsToAggregate
- users = result.UsersFromGroups
+ users = result.FinalUsers
```

**Files to update**:
- `retrieve_agent_stats.go:81`
- `retrieve_conversation_stats.go:76`
- `retrieve_hint_stats.go` (find line with `users = result.UsersFromGroups`)
- `retrieve_knowledge_assist_stats.go:74`
- `retrieve_suggestion_stats.go:80`
- `retrieve_summarization_stats.go` (find line)
- `retrieve_smart_compose_stats.go:90`
- `retrieve_note_taking_stats.go:80`
- `retrieve_guided_workflow_stats.go:80`
- `retrieve_knowledge_base_stats.go:80`
- `retrieve_qa_score_stats.go` (find line)
- `retrieve_live_assist_stats.go:75`

### Step 2: Add Test Coverage

Add test case to verify metadata enrichment for user-only filters:

```go
func (s *AnalyticsServiceSuite) TestRetrieveXXXStats_UserOnlyFilter_GroupByAgent_MetadataEnriched() {
    // Setup: User-only filter (no groups)
    reqUsers := []*userpb.User{
        {Name: "customers/test-customer/profiles/test-profile/users/alice"},
        {Name: "customers/test-customer/profiles/test-profile/users/bob"},
    }

    // Mock ground truth with full metadata
    s.mockListAllUsers([]*internaluserpb.LiteUser{
        {UserId: "alice", Username: "alice", FullName: "Alice Agent"},
        {UserId: "bob", Username: "bob", FullName: "Bob Agent"},
    })

    // Request with AGENT grouping
    req := &analyticspb.RetrieveXXXStatsRequest{
        Parent: "customers/test-customer/profiles/test-profile",
        FilterByAttribute: &analyticspb.Attribute{
            Users: reqUsers,
        },
        GroupByAttributeTypes: []analyticspb.AttributeType{
            analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT,
        },
    }

    // Call API
    resp, err := s.service.RetrieveXXXStats(context.Background(), req)

    // Assert: Metadata is enriched
    s.NoError(err)
    s.Len(resp.XXXStatsResults, 2)
    for _, result := range resp.XXXStatsResults {
        s.NotEmpty(result.Attribute.Users[0].FullName, "FullName should be enriched")
        s.NotEmpty(result.Attribute.Users[0].Username, "Username should be enriched")
    }
}
```

### Step 3: Verify All Scenarios Still Work

**Test matrix**:

| Test Case | Groups in Request | Grouping | Expected users value | Expected behavior |
|-----------|------------------|----------|---------------------|-------------------|
| User-only + Group by AGENT | No | AGENT | FinalUsers | ✅ Metadata enriched |
| Group filter + Group by GROUP | Yes | GROUP | FinalUsers | ✅ UsersFromGroups mapping works |
| User + Group + Group by AGENT | Yes | AGENT | FinalUsers | ✅ All users included |
| User-only + No grouping | No | None | FinalUsers → [] | ✅ Cleared by if block |

---

## Benefits of This Approach

### 1. Simplicity
- **One-line change** per API
- **No complex fallback logic**
- **Easy to understand and maintain**

### 2. Correctness
- **Always complete metadata**: FinalUsers contains all queried users
- **No edge cases**: Works for all scenarios
- **Superset relationship**: Safe to use superset for subset's purpose

### 3. Consistency
- **Same pattern across all APIs**
- **Matches the semantic**: "users for metadata enrichment" = "all queried users"

### 4. Performance
- **Slightly larger map**: FinalUsers may have more users than UsersFromGroups
- **Impact**: Negligible (metadata maps are small, O(n) operations)
- **Benefit**: Guaranteed correctness worth the minimal cost

---

## Alternative Approaches (Rejected)

### Alternative 1: Conditional Fallback

```go
users = result.UsersFromGroups
if len(users) == 0 && len(result.FinalUsers) > 0 &&
    !shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
    users = result.FinalUsers
}
```

**Why rejected**:
- ❌ More complex logic
- ❌ Harder to understand the conditions
- ❌ Still doesn't handle all edge cases (e.g., partial group membership)
- ❌ Doesn't solve the root problem (using wrong field)

### Alternative 2: Modify ParseUserFilterForAnalytics

Change the function to always populate `UsersFromGroups` with all users.

**Why rejected**:
- ❌ Changes semantic meaning of `UsersFromGroups`
- ❌ Breaks the subset relationship
- ❌ Could affect other code that relies on `UsersFromGroups` meaning
- ❌ More invasive change

### Alternative 3: Remove UsersFromGroups Entirely

Remove `UsersFromGroups` from the result struct.

**Why rejected**:
- ❌ `UsersFromGroups` is still used for group-to-user mapping in some scenarios
- ❌ Would require refactoring response construction code
- ❌ Out of scope for this bug fix

---

## Testing Strategy

### Unit Tests

Add tests for each API covering:

1. **User-only filter + Group by AGENT** (the bug scenario)
2. **Group filter + Group by GROUP** (ensure still works)
3. **User + Group filter + Group by AGENT** (mixed scenario)
4. **No grouping** (ensure users cleared correctly)

### Integration Tests

Run existing integration tests to ensure no regressions.

### Manual Testing in Staging

1. Deploy with feature flag enabled
2. Test leaderboards with:
   - User-only filters
   - Group-only filters
   - Mixed filters
3. Verify metadata (FullName, Username) appears correctly

---

## Rollout Plan

1. **Apply fix to all 12 APIs** in a single PR or separate PRs
2. **Add test coverage** for each API
3. **Deploy with feature flag disabled** (uses old implementation)
4. **Enable feature flag in staging**
5. **Verify metadata enrichment** in staging leaderboards
6. **Enable feature flag in production** (gradual rollout: 10% → 50% → 100%)
7. **Monitor metrics** (error rates, response times, data correctness)
8. **Remove legacy code** after successful rollout

---

## Risk Assessment

**Risk**: Very Low

**Rationale**:
- Simple one-line change
- Uses superset instead of subset (always safe)
- Only affects metadata enrichment (cosmetic field)
- Doesn't change query logic or filtering
- Same metadata source (groundTruthUsers)

**Potential Issues**:
- Slightly larger metadata map (negligible performance impact)
- None identified for correctness

**Mitigation**:
- Comprehensive test coverage
- Feature flag for gradual rollout
- Easy rollback if issues detected
- Staging validation before production

---

## Conclusion

**The fix is simple**: Change `users = result.UsersFromGroups` to `users = result.FinalUsers` in all 12 APIs.

**Why this is the right solution**:
1. ✅ FinalUsers is the complete set of queried users
2. ✅ UsersFromGroups is a subset (may be empty)
3. ✅ Metadata enrichment needs complete set, not subset
4. ✅ One-line change is simpler than complex fallback logic
5. ✅ Works correctly for all scenarios

This fix is **critical for completing the user filter refactoring** and ensuring metadata enrichment works correctly in all scenarios.

---

**Document Created**: 2026-01-16
**Document Updated**: 2026-01-16 (Simplified to always use FinalUsers)
**Status**: Analysis complete, ready for implementation
**Next Steps**: Apply one-line fix to all 12 APIs, add test coverage, deploy
