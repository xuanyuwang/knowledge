# CONVI-6151: Deprecate the Usage of Old FetchUsers Tool

**Created:** 2026-02-13
**Updated:** 2026-02-13
**Linear:** https://linear.app/cresta/issue/CONVI-6151/deprecated-the-usage-of-old-fetchuser-tool
**Priority:** High
**Branch:** `convi-6151-deprecated-the-usage-of-old-fetchuser-tool`
**Worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-6151`

## Overview

The `FetchUsers` function in `shared/user/utils.go` has a hard limit of 6000 users (PageSize=6000, errors if NextPageToken is non-empty). This is a problem for customers with more than 6000 users.

**Scope:** Replace usages in `apiserver/internal/coaching` package only.

Replace with either:
- `FetchUsersPaginated` — returns `[]*userpb.User` (collects all pages into a slice)
- `CollectUsersPaginated` — streams pages via a callback, supports `includeUserMemberships`

## Call Sites in `apiserver/internal/coaching` (8 total)

| # | File | Line | Function | What it fetches | Notes |
|---|------|------|----------|-----------------|-------|
| 1 | `action_create_director_task.go` | 57 | `CreateDirectorTask` | 1 user (creator ID) | Result used as `map[string]*userpb.User` for `DBToAPIDirectorTask` |
| 2 | `action_update_director_task.go` | 94 | `UpdateDirectorTask` | 2 users (creator + updater) | Same usage as #1 |
| 3 | `action_get_scorecard.go` | 147 | `getScorecardUsers` | Up to 3 users (creator, updater, submitter) | Result map used by `extractScorecardUsersFromMap` |
| 4 | `action_get_scorecard_template.go` | 120 | `resolveUserAudience` | Users by IDs | Returns `map[string]*userpb.User` directly |
| 5 | `action_list_current_scorecard_templates.go` | 219 | lazy `userToGroupNames` | Agents by IDs | Uses `FetchUsersWithIncludeUserMemberships(true)`, iterates map values for group memberships |
| 6 | `util.go` | 192 | `getAgentUserIDs` | All agents (no specific IDs) | Only uses map keys via `fn.MapKeyToSlice` |
| 7 | `util.go` | 272 | `filterActiveUserIDs` | Agents by IDs | Iterates values to extract user names |
| 8 | `util.go` | 379 | `fetchAgentIDsFromGroupResourceNames` | Agents by group | Only uses map keys via `fn.MapKeyToSlice` |

## Migration Strategy

### Most callers (sites 1-4, 7): need `map[string]*userpb.User`
Use `FetchUsersPaginated` + a small helper to convert `[]*userpb.User` to `map[string]*userpb.User`.

### Site 5: needs `includeUserMemberships=true`
Use `CollectUsersPaginated` with `includeUserMemberships: true`, build the map in the callback.

### Sites 6, 8: only need user IDs (map keys)
Use `FetchUsersPaginated`, extract IDs from the returned slice.

### Helper function to add
```go
// FetchUserMapPaginated fetches users with pagination and returns map[userID]*User.
func FetchUserMapPaginated(
    ctx context.Context,
    client userpb.UserServiceClient,
    customerID string,
    filter *userpb.ListUsersRequest_ListUsersFilter,
    includeUserMemberships bool,
) (map[string]*userpb.User, error) {
    userMap := make(map[string]*userpb.User)
    err := CollectUsersPaginated(ctx, client, customerID, "", filter, 1000, "",
        userpb.ListUsersRequest_SORT_ORDER_UNSPECIFIED, includeUserMemberships,
        func(batch []*userpb.User) error {
            for _, u := range batch {
                userName := fn.Must(userpb.ParseUserName(u.Name))
                userMap[userName.UserID] = u
            }
            return nil
        })
    if err != nil {
        return nil, err
    }
    return userMap, nil
}
```

This makes most call sites a trivial replacement: `FetchUsers(ctx, client, custID, filter)` → `FetchUserMapPaginated(ctx, client, custID, filter, false)`.

## Log History

| Date | Summary |
|------|---------|
| 2026-02-13 | Initial investigation: scoped to coaching package, identified 8 call sites |
