---
name: ParseUserFilterForAnalytics architecture
description: How user filtering flows through ParseUserFilterForAnalytics → ApplyUserFilterFromResult → ClickHouse query in analytics APIs
type: project
---

`ParseUserFilterForAnalytics` in `go-servers/insights-server/internal/analyticsimpl/common_user_filter.go` is the central user filtering function for all analytics APIs.

**Key flow:**
1. Step 0: `listAllUsers()` fetches ground truth users (agents only if `listAgentOnly=true`)
2. Step 1: `applyResourceACL()` applies ACL — returns `finalUsers`, `finalGroups`, `isACLEnabled`, `isRootAccess`
3. Step 2: `shouldUseAllAgents` flag — determines if we should skip user WHERE clause in ClickHouse
4. Step 3: `buildUserGroupMappings()` builds user-to-group maps

**Why:** The `ShouldQueryAllUsers` flag (set from `shouldUseAllAgents`) controls whether `ApplyUserFilterFromResult` passes user IDs to the ClickHouse query or leaves the user list empty (relying on ClickHouse to return "all" data). This optimization avoids large WHERE clauses but assumes ClickHouse data matches the desired user scope — which breaks when `listAgentOnly=true` because ClickHouse contains all users.

**How to apply:** When debugging user filtering bugs, always trace through these 3 checkpoints: (1) what `groundTruthUsers` contains, (2) what `shouldUseAllAgents` evaluates to, (3) what `ApplyUserFilterFromResult` puts into `*users`.

**Old vs New pattern:** Two handlers still use the OLD 3-step pattern (`shared.ApplyResourceACL` + `shared.ListUsersMappedToGroups` + `shared.MoveFiltersToUserFilter`) with hardcoded `listAgentOnly: false`:
- `retrieve_adherences.go` — also missing proto field entirely
- `retrieve_assistance_stats.go` — legacy API, proto field exists but handler ignores it

All other handlers use the new `ParseUserFilterForAnalytics` + `ApplyUserFilterFromResult` pattern and read `req.GetFilterToAgentsOnly()`.
