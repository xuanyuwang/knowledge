# Bug: `filterToAgentsOnly: true` Not Working With Empty Groups

**Created:** 2026-04-14

## Summary

When `filterToAgentsOnly: true` is set with an empty groups array (`groups: []`), non-agent users (e.g. `9a75a2b2e71cea99`) are **not** filtered out of results. The filter works correctly when a group is selected.

## Root Cause

The bug is in the interaction between `ShouldQueryAllUsers` and the ClickHouse query layer.

### The Problem

In `ParseUserFilterForAnalytics` (file: `insights-server/internal/analyticsimpl/common_user_filter.go`):

**Line 230-231** — The `shouldUseAllAgents` flag:

```go
shouldUseAllAgents := (!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
    (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0)
```

When ACL is disabled (or root access) AND there are no user/group filters, `shouldUseAllAgents = true`. This is propagated as `ShouldQueryAllUsers = true` on line 291.

**Line 594** in `ApplyUserFilterFromResult`:

```go
if result.ShouldQueryAllUsers && !useExtTable {
    *users = []*userpb.User{}   // <-- SETS USERS TO EMPTY
}
```

When `ShouldQueryAllUsers = true` and ext tables are not enabled, the user list passed to ClickHouse is set to **empty**. This was designed as an optimization to avoid sending a huge user list in the WHERE clause when you want "all users."

**But "all users" in the ClickHouse tables includes both agents AND non-agents.** The ground truth filtering (Step 0) correctly fetches only agents via `listAllUsers(..., listAgentOnly=true)`, but that filtered list is **never applied** to the ClickHouse query — it's only used for response metadata enrichment.

### Why It Works With a Group Filter

When `groups` is non-empty (e.g., `groups: [{name: "customers/bswift/groups/01992ac1-..."}]`):

1. `applyResourceACL` returns `finalGroups = [group]` (non-empty)
2. **Line 230-231:** `len(finalGroups) != 0`, so `shouldUseAllAgents = false`
3. `ShouldQueryAllUsers = false`
4. `ApplyUserFilterFromResult` sets `*users = result.FinalUsers` (agents filtered by group)
5. ClickHouse query gets a proper `WHERE agent_user_id IN (...)` clause with only agent IDs

## Code Flow: Broken Case

Request: `filterToAgentsOnly: true`, `groups: []`, `users: []`

| Step | Location | What Happens |
|------|----------|-------------|
| Step 0 | Line 154-165 | `listAllUsers(listAgentOnly=true)` → `groundTruthUsers` = all agents only. **Correct.** |
| Skip | Line 170-172 | `len(reqGroups) == 0`, so group pre-filter skipped |
| Step 1 | Line 183-199 | `applyResourceACL(users=[], groups=[])` → returns `finalUsers=[], finalGroups=[], isACLEnabled=false` |
| Step 2 | Line 230-231 | `shouldUseAllAgents = (!false && 0==0 && 0==0) = true` |
| Step 2b | Line 232-238 | `shouldUseAllAgents=true`, so `updateGroundTruthUsers` is **not called** |
| Step 3 | Line 260 | `finalUsers = convertLiteUsersToUsers(groundTruthUsers)` = all agents |
| Return | Line 291 | `ShouldQueryAllUsers = true` |
| Apply | Line 128 (QA handler) | `ApplyUserFilterFromResult` called |
| **BUG** | Line 594 | `ShouldQueryAllUsers=true && !useExtTable` → `*users = []` (empty user list) |
| Query | ClickHouse | No `agent_user_id IN (...)` clause → returns ALL users including managers |

## Code Flow: Working Case

Request: `filterToAgentsOnly: true`, `groups: [{name: "...group..."}]`, `users: []`

| Step | Location | What Happens |
|------|----------|-------------|
| Step 0 | Line 154-165 | `groundTruthUsers` = all agents |
| Pre-filter | Line 170-172 | `len(reqGroups) > 0 && len(reqUsers) == 0` → `groundTruthUsers = filterUsersByGroups(...)` = agents in group |
| Step 1 | Line 183-199 | `applyResourceACL` → `finalGroups = [group]` (non-empty) |
| Step 2 | Line 230-231 | `len(finalGroups) != 0` → `shouldUseAllAgents = false` |
| Return | Line 291 | `ShouldQueryAllUsers = false` |
| Apply | Line 594 | `ShouldQueryAllUsers=false` → `*users = result.FinalUsers` (agent IDs in group) |
| Query | ClickHouse | `agent_user_id IN (agent1, agent2, ...)` → only agents returned |

## Fix Options

### Option A: Set `ShouldQueryAllUsers = false` when `listAgentOnly = true`

The simplest fix. When `listAgentOnly=true`, we must always pass the agent user list to the query, because ClickHouse data contains all user types.

**Location:** Line 230-231 in `common_user_filter.go`

```go
// Add listAgentOnly guard: when filtering to agents only, we must always
// pass the user list to the query because ClickHouse contains all users.
shouldUseAllAgents := !listAgentOnly && (
    (!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
    (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0))
```

**Risk:** When `listAgentOnly=true` and there are many agents, this could produce a large WHERE clause. However, ext tables (`enableExtTableForUserFilter`) handle this efficiently, and even without ext tables, the agent count per profile is typically manageable.

### Option B: Handle in `ApplyUserFilterFromResult`

Only set users to empty when `ShouldQueryAllUsers=true` AND `listAgentOnly=false`:

```go
if result.ShouldQueryAllUsers && !useExtTable && !listAgentOnly {
    *users = []*userpb.User{}
} else {
    *users = result.FinalUsers
}
```

This would require threading `listAgentOnly` through to `ApplyUserFilterFromResult`, which currently doesn't have that parameter.

### Option C: Handle at the query level

Add a role-based filter to ClickHouse queries when `listAgentOnly=true`. This would require the ClickHouse tables to have role information, which they may not have.

### Recommendation

**Option A** is the cleanest fix. It addresses the root cause at the right level: `ShouldQueryAllUsers` should only be `true` when the ClickHouse data naturally contains only the users we want (i.e., when we're not filtering by role). When `listAgentOnly=true`, we need the agent ID list in the query.

## Affected APIs

All APIs using `ParseUserFilterForAnalytics` with `filterToAgentsOnly: true` and no group/user filter are affected. This includes:

- `RetrieveQAScoreStats`
- `RetrieveAgentStats`
- `RetrieveConversationStats`
- `RetrieveHintStats`
- `RetrieveKnowledgeAssistStats`
- `RetrieveLiveAssistStats`
- `RetrieveNoteTakingStats`
- `RetrieveSmartComposeStats`
- `RetrieveSuggestionStats`
- `RetrieveSummarizationStats`
- `RetrieveKnowledgeBaseStats`
- `RetrieveUserOutcomeStats`
- `RetrieveGuidedWorkflowStats`

## Key File Paths

- **Bug location:** `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/common_user_filter.go` lines 230-231 (shouldUseAllAgents), line 291 (ShouldQueryAllUsers), lines 594-595 (ApplyUserFilterFromResult)
- **QA Score handler:** `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go` lines 75, 93-111, 128
- **Tests:** `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/common_user_filter_test.go`
