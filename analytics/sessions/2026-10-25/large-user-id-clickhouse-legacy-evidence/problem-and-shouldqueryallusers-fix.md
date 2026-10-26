# Problem: Too Many Users in ClickHouse Query + ShouldQueryAllUsers Fix

**Created:** 2026-01-29
**Updated:** 2026-03-09
**Status:** ShouldQueryAllUsers fix implemented and deployed.
**Origin:** Extracted from `insights-user-filter/too-many-users-edge-case.md`

## Problem Summary

When `ParseUserFilterForAnalytics` is enabled, the system generates ClickHouse queries with extremely long `WHERE agent_user_id IN (...)` clauses, exceeding ClickHouse's query size limit (~1MB).

### Error Message
```
code: 62, message: Syntax error: failed at position 1048561 (''2a2572426cc211a'')
Max query size exceeded
```

### Root Cause

1. `ParseUserFilterForAnalytics` returns ALL users in `FinalUsers` when ACL is disabled + empty filter, or root access + empty filter.
2. `buildUsersConditionAndArgs` creates `WHERE agent_user_id IN ('user1', ..., 'userN')`.
3. For customers with thousands of agents, the query string exceeds ~1MB.

### Old vs New Behavior

The old implementation (before `ParseUserFilterForAnalytics`) didn't have this issue because `ApplyResourceACL` returned empty users/groups for root access, and `MoveFiltersToUserFilter` was only called when groups or deactivation filtering was needed. Empty users = no WHERE clause.

## Fix: ShouldQueryAllUsers Flag (IMPLEMENTED)

Added `ShouldQueryAllUsers` field to `ParseUserFilterResult`. When true, callers skip adding user IDs to the WHERE clause.

### Condition
```go
shouldUseAllAgents := (!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
    (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0)
```

### Usage in API Implementations
```go
if result.ShouldQueryAllUsers {
    req.FilterByAttribute.Users = []*userpb.User{}  // no WHERE clause
} else {
    req.FilterByAttribute.Users = result.FinalUsers
}
```

### Behavior Matrix

| Scenario | ShouldQueryAllUsers | WHERE clause |
|----------|---------------------|--------------|
| Root access + empty filter | true | None |
| ACL disabled + empty filter | true | None |
| Limited access + managed users | false | IN (managed users) |
| Explicit user/group filter | false | IN (filtered users) |
| exclude_deactivated_users=true | false | IN (active users) -- **still large!** |

### Remaining Gap

This fix only handles the "all users" case. When `exclude_deactivated_users=true` or large group expansion produces thousands of specific IDs, the query still exceeds the limit. This is what the `ext` external tables solution addresses.

### Files Modified

1. `common_user_filter.go` — Added `ShouldQueryAllUsers` field
2. 12 API files — Check flag before setting users
3. `common_user_filter_test.go` — 8 test cases

### Manual Testing

Tested on chat-staging (cox/sales) with 6 scenarios covering empty filter, user filter, group filter, virtual group, exclude_deactivated, and include_dev_users. All passed.
