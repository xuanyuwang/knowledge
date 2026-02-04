# Cache Usage Analysis

## Finding

The `enableListUsersCache` and `listUsersCache` parameters are **passed to but NOT USED** in `ParseUserFilterForAnalytics`.

### Evidence

Looking at `common_user_filter.go`:
- Parameters are declared at lines 111-112
- They are never referenced in the function body
- The cache is only used by the **old** implementation via `shared.ListUsersMappedToGroups`

### Cache Definition

From `insights-server/internal/shared/list_users_cache.go`:

```go
type ListUsersCache struct {
    UsersToGroupsCache cache.Cache[string, map[string][]string]
    UsersCache         cache.Cache[string, []*userpb.User]
    GroupsCache        cache.Cache[string, []*userpb.Group]
}
```

### Where Cache IS Used

The cache is used in `shared.ListUsersMappedToGroups` (old implementation path):
- When `enableParseUserFilterForAnalytics = false`
- Called from the else branch in retrieve_*_stats.go files

### Recommendation

1. **Remove cache parameters** from `ParseUserFilterForAnalytics` signature
2. If caching is needed for the new implementation, design it differently:
   - Consider caching at a lower level (e.g., `listAllUsers`)
   - Or use request-scoped caching via context

### Impact on Migration

When migrating to the new shared implementation:
- Can safely drop these 2 parameters
- Callers passing cache will need updating (breaking change, but simplifies API)
- Consider if caching should be re-added with a better design later
