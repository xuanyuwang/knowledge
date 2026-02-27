# Bug: Empty User Filter Returns Non-Agent Data ("Unknown" Users on FE)

**Created:** 2026-02-26
**Updated:** 2026-02-26 (Added ClickHouse techniques, always-GROUP-BY analysis, FE usage patterns)
**Status:** Analysis complete, ready for implementation

---

## Problem

When user filters are empty (root access + no explicit user/group filters), ClickHouse queries return data for **all users** including non-agents. The FE displays these non-agent users as `unknown` because they aren't in the metadata enrichment map (which only contains agent-only users).

### Symptoms

- FE shows `unknown` as user names in leaderboards/performance pages
- Only happens when no user or group filters are applied (i.e., "show all" view)

### Reproduction

1. Open Performance/Leaderboard page as root access user
2. Don't apply any user or group filters
3. Observe `unknown` entries in the results

---

## Root Cause

The `ShouldQueryAllUsers=true` path (introduced to fix the [too-many-users query size limit issue](too-many-users-edge-case.md)) omits the `agent_user_id IN (...)` WHERE clause entirely. This means ClickHouse returns data for **all** `agent_user_id` values — including non-agent users (managers, supervisors, etc.).

### Data Flow When Bug Occurs

```
1. Request: empty user/group filters, root access

2. ParseUserFilterForAnalytics (common_user_filter.go:226-227):
   - Ground truth = agent-only users (e.g., 500 agents)
   - shouldUseAllAgents = true (root access + empty filter)
   - ShouldQueryAllUsers = true
   - FinalUsers = [500 agent users]  ← for metadata enrichment

3. ApplyUserFilterFromResult (common_user_filter.go:577-594):
   - ShouldQueryAllUsers=true → req.FilterByAttribute.Users = []  ← empty!

4. ClickHouse query:
   - WHERE agent_user_id <> ''        ← no user ID filter!
   - Returns data for ALL users (agents + managers + supervisors)
   - e.g., 700 user records (500 agents + 200 non-agents)

5. Response construction:
   - Metadata map built from FinalUsers (500 agents only)
   - 500 agent records → enriched with FullName/Username ✅
   - 200 non-agent records → no metadata match → "unknown" ❌
```

### Why Previous Fixes Didn't Catch This

| Fix | What It Solved | Why This Bug Persists |
|-----|---------------|----------------------|
| `listAgentOnly` parameter (Dec 2025) | Ground truth filtered to agents | ClickHouse WHERE clause still omitted when `ShouldQueryAllUsers=true` |
| `ShouldQueryAllUsers` flag (Jan 2026) | Query size limit for large customers | Intentionally removes user WHERE clause — lets non-agent data through |
| Metadata enrichment fix (Jan 2026) | `UsersFromGroups` → `FinalUsers` | Map is correct (agents only), but non-agent ClickHouse rows still exist in response |

---

## Impact: Which FE Views Are Affected

Investigation of the director FE codebase reveals **two categories of impact**:

### Visible Bug: "Unknown" User Names (Per-Agent Views)

When the FE requests per-agent grouping (e.g., `groupBy=[AGENT, GROUP]`), ClickHouse returns one row per `agent_user_id`. Non-agent user rows appear in the response but have no metadata → FE shows "unknown".

**Affected views:**
- Agent Leaderboard table (`groupBy=[AGENT, GROUP]`)
- Per-criterion leaderboard (`groupBy=[AGENT, TIME_RANGE]`)
- LeaderboardByScorecardTemplateItem (`groupBy=[AGENT, CRITERION]`)

### Silent Data Corruption: Inflated Aggregates (Overview/Summary Views)

When the FE requests **no per-agent grouping**, ClickHouse returns aggregate totals across all users. Non-agent data is **silently baked into the numbers** — no "unknown" names are visible, but the metrics are wrong.

**Affected views (Performance page — most impacted):**

| View | groupBy | API(s) Called | What's Wrong |
|------|---------|-------------|-------------|
| Assistance overview cards | `[]` (empty) | ConversationStats, KnowledgeAssistStats, HintStats | Total counts include non-agent activity |
| Score line chart | `[TIME_RANGE]` | QAScoreStats | Score averages include non-agent scores |
| Score metric card | `[TIME_RANGE]` | QAScoreStats | Same — headline number is inflated |
| Conversation count chart | `[TIME_RANGE]` | QAScoreStats, ConversationStats | Conversation volume includes non-agent convos |
| Performance progression heatmap | `[CRITERION, TIME_RANGE]` | QAScoreStats | Per-criterion scores polluted |
| Outcome stats chips | `[CRITERION]` | QAScoreStats | Summary per criterion includes non-agents |

**Leaderboard page — NOT separately affected:**
The Statistics summary cards (Convo Vol, Active Agents, AHT, etc.) share the **same request** as the Agent Leaderboard table with `groupBy=[AGENT, GROUP]`. They don't make separate overview-only API calls.

### Key Insight

The **silent aggregate inflation on Performance pages** is arguably worse than the visible "unknown" names — users see plausible-looking but incorrect numbers without any indication of a problem.

---

## Why Post-Query Filtering Won't Work

Initial analysis suggested filtering results after ClickHouse returns them. **This approach was rejected** for two reasons:

### 1. Requires per-API changes (13 APIs)

All 13 API files calling `ApplyUserFilterFromResult` would need modification. This is the same scale as the problem we're trying to avoid.

### 2. Some query paths aggregate away the user dimension

Many APIs use `optionalGroupBy` — when no per-agent grouping is requested, the query aggregates across all users:

```sql
-- When GROUP BY is empty (no per-agent breakdown requested):
SELECT
    COUNT(DISTINCT conversation_id) AS total_conversations,
    COUNT(DISTINCT agent_user_id) AS agent_count
FROM conversation_d
WHERE agent_user_id <> ''
-- No GROUP BY agent_user_id!
-- Result: single row with aggregate metrics across ALL users
```

In this case, non-agent data is **already baked into the aggregate counts**. There's no per-user breakdown to filter against. The result is a single row like `{total_conversations: 5000, agent_count: 700}` where 700 includes non-agents.

**APIs with optional GROUP BY** (confirmed from codebase):
- retrieve_agent_stats_clickhouse.go
- retrieve_conversation_stats_clickhouse.go
- retrieve_hint_stats_clickhouse.go
- retrieve_suggestion_stats_clickhouse.go
- retrieve_summarization_stats_clickhouse.go
- retrieve_smart_compose_stats_clickhouse.go
- retrieve_note_taking_stats_clickhouse.go
- retrieve_guided_workflow_stats_clickhouse.go
- retrieve_knowledge_base_stats_clickhouse.go
- retrieve_scorecard_stats_clickhouse.go
- retrieve_qa_score_stats_clickhouse.go

**Conclusion**: The fix **must** be at the query level (WHERE clause), not post-query.

---

## Recommended Fix: Centralized One-Line Change in `ParseUserFilterForAnalytics`

### The Fix

In `common_user_filter.go`, line 226-227, add `&& !listAgentOnly`:

```go
// BEFORE (current):
shouldUseAllAgents := (!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
    (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0)

// AFTER (fixed):
shouldUseAllAgents := ((!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
    (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0)) && !listAgentOnly
```

### Why This Works

**When `listAgentOnly=true`** (agent/team leaderboards, performance pages):
- `shouldUseAllAgents` = `false` (always, regardless of ACL/root state)
- `ShouldQueryAllUsers` = `false`
- `ApplyUserFilterFromResult` → `req.FilterByAttribute.Users = result.FinalUsers` (agent IDs)
- ClickHouse WHERE clause: `agent_user_id IN (agent1, agent2, ...)` → only agent data returned

**When `listAgentOnly=false`** (manager tab, other non-agent-specific views):
- Behaves exactly as before — no regression
- `ShouldQueryAllUsers` can still be `true` for root access + empty filter
- This is correct: when we don't need role filtering, all user data is acceptable
- Metadata enrichment map contains all users (not just agents), so no "unknown" names

### Why This Is Centralized (Zero API Changes)

All 13 APIs go through the same path:

```
ParseUserFilterForAnalytics (common_user_filter.go)
  → sets ShouldQueryAllUsers based on shouldUseAllAgents
  → returns ParseUserFilterResult

ApplyUserFilterFromResult (common_user_filter.go:577)  ← shared by ALL 13 APIs
  → reads result.ShouldQueryAllUsers
  → sets req.FilterByAttribute.Users accordingly
```

The fix is **entirely within `ParseUserFilterForAnalytics`**. No changes to any of the 13 API files.

### Query Size Concern: Not an Issue for Agent-Only

The original `ShouldQueryAllUsers` optimization was needed because ALL users (agents + managers + supervisors) could exceed ClickHouse's ~1MB query size limit for large customers. With `listAgentOnly=true`, we're only including agents:

| Metric | All Users | Agent-Only |
|--------|-----------|------------|
| Typical count | 1,000 - 5,000+ | 200 - 1,000 |
| User ID format | `4556009cdf3d1abf` (16 hex chars) | Same |
| Per-user WHERE overhead | ~20 chars (ID + quotes + comma) | Same |
| WHERE clause size for 1000 users | ~20 KB | ~20 KB |
| ClickHouse limit | ~1 MB | ~1 MB |
| Theoretical max users in WHERE | ~50,000 | ~50,000 |

Even for the largest customers, agent-only counts are well within the 1MB limit. The `ShouldQueryAllUsers` optimization is only needed for the `listAgentOnly=false` case (which still uses it).

---

## Alternative Approaches (Considered and Rejected)

### Option A: Post-Query Filtering (Rejected)

Filter results after ClickHouse returns them.

**Why rejected:**
- Requires changes in all 13 API files (not centralized)
- **Broken for aggregate queries**: When GROUP BY doesn't include `agent_user_id`, non-agent data is already baked into the aggregated metrics — can't filter it out
- More code to maintain

### Option B: Shared Post-Processing Helper (Rejected)

Extract filtering into a shared function called by all APIs.

**Why rejected:**
- Same "broken for aggregate queries" problem as Option A
- Still requires each API to call the helper (13 changes)
- Different result struct types across APIs make a common interface complex

### Option C: ClickHouse-Level Role Filter (Overkill)

Add `is_agent_only` column to ClickHouse analytics tables.

**Why rejected:**
- Requires schema migration across all analytics tables
- ETL pipeline changes needed
- Overkill for this specific issue
- Long lead time

### Option D: Always GROUP BY agent_user_id, Then Re-Aggregate in Go (Rejected)

Force every ClickHouse query to include `agent_user_id` in GROUP BY, filter non-agent rows in Go, then re-aggregate.

**Why rejected** (full analysis: [always-group-by-agent-investigation.md](always-group-by-agent-investigation.md)):
- **~4 weeks of work**: 12 Category B APIs + 1 Category C (LiveAssistStats)
- **COUNT(DISTINCT) correctness bug**: When grouped by agent, a conversation shared by 2 agents appears in both rows. Summing gives 2x the actual count. Requires tracking distinct IDs in Go.
- **LiveAssistStats is Category C**: Uses `manager_user_id AS agent_user_id` swap in a UNION ALL — dual-purpose column makes this very complex.
- **Performance impact**: Instead of 30 rows (one per day), returns 30,000 rows (30 days × 1000 agents). More data over the wire, more Go memory.

### Query Size Safety Net: ClickHouse Techniques for Large IN Clauses

If agent-only counts ever grow large enough to approach the query size limit, there are ClickHouse-level escape hatches (full details: [clickhouse-large-in-clause-techniques.md](../general-learnings/clickhouse-large-in-clause-techniques.md)):

| Approach | Max Values | Effort | Changes Needed |
|----------|-----------|--------|----------------|
| **Increase `max_query_size`** to 10MB per-query | ~50K | Minutes | `clickhouse.WithSettings` on context |
| **External Data Tables** (Go driver `ext` package) | Millions | Zero refactoring | `IN (?)` → `IN (SELECT ... FROM ext_table)` |
| Temporary Tables | Millions | 1-2 days | Same, but not connection-pool safe |

**Quick fix** (if ever needed):
```go
ctx := clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
    "max_query_size": 10 * 1024 * 1024, // 10 MB (default: 256 KB)
}))
```

**Production-grade** (if ever needed): External Data Tables via the `ext` package.

#### Verified: External Data Tables Are a Drop-In for go-servers

Investigation of the go-servers ClickHouse client ([clickhouse-ext-tables-investigation.md](clickhouse-ext-tables-investigation.md)) confirmed that `ext` tables require **zero changes to shared infrastructure**:

- **Driver**: `clickhouse-go/v2` v2.34.0 — `ext` package already available at `github.com/ClickHouse/clickhouse-go/v2/ext`
- **Connection type**: Native API (`clickhouse.Open` → `driver.Conn`), which is what `ext` requires (NOT `database/sql`)
- **Context preservation**: All 13 APIs execute queries via `clickhouseshared.QueryWithRetry(ctx, conn, query, args...)`. The retry wrapper calls `clickhouse.Context(ctx, clickhouse.WithQueryID(...))` internally. Verified from driver source (`context.go`) that this **copies all parent context options including `.external` tables** before applying new options. External tables attached to the parent context flow through unchanged.

**Adoption pattern** (at the call site — no shared layer changes):
```go
import "github.com/ClickHouse/clickhouse-go/v2/ext"

agentTable, _ := ext.NewTable("agent_filter", ext.Column("agent_user_id", "String"))
for _, id := range agentIDs {
    agentTable.Append(id)
}
ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(agentTable))

// Existing call — works unchanged, ext table flows through QueryWithRetry
rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
```

SQL change: `agent_user_id IN (?)` → `agent_user_id IN (SELECT agent_user_id FROM agent_filter)`

**Current assessment**: Agent-only counts (typically 200-1,000) generate ~20 KB WHERE clauses, far below the 256 KB default limit. These escape hatches are documented for future safety but not needed today.

---

## Implementation Plan

### Change

Single file: `insights-server/internal/analyticsimpl/common_user_filter.go`

```diff
- shouldUseAllAgents := (!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
-     (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0)
+ shouldUseAllAgents := ((!isACLEnabled && len(finalUsers) == 0 && len(finalGroups) == 0) ||
+     (isRootAccess && len(finalUsers) == 0 && len(finalGroups) == 0)) && !listAgentOnly
```

### Testing

1. [ ] Update existing `TestShouldQueryAllUsers` cases:
   - New case: `listAgentOnly=true` + root access + empty filter → `ShouldQueryAllUsers=false`
   - New case: `listAgentOnly=true` + ACL disabled + empty filter → `ShouldQueryAllUsers=false`
   - Existing case: `listAgentOnly=false` + root access + empty filter → `ShouldQueryAllUsers=true` (unchanged)
2. [ ] Run full test suite: `bazel test //insights-server/internal/analyticsimpl:common_user_filter_test`
3. [ ] Manual testing in staging: verify no "unknown" users in leaderboard with empty filters
4. [ ] Verify query size stays reasonable for large customers with `listAgentOnly=true`

### Rollout

- Feature flag: `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` (already exists)
- Deploy → enable in staging → verify → enable in production
- No additional feature flag needed since this is a bugfix within the existing flag scope

---

## Connection to Previous Issues

This is the **third manifestation** of the tension between "query all users" semantics and "agent-only" filtering:

1. **Dec 2025**: `listAgentOnly` not passed to all APIs → non-agent data in results
2. **Jan 2026**: `listAgentOnly` users too many for WHERE clause → query size limit exceeded
   → Fix: `ShouldQueryAllUsers` flag to skip WHERE clause
3. **Feb 2026 (THIS BUG)**: Skipping WHERE clause lets non-agent data through again
   → Fix: Don't skip WHERE clause when `listAgentOnly=true` (agent counts are safe for query size)

The fundamental insight: **`ShouldQueryAllUsers` was designed for the "no role filter" case**. When `listAgentOnly=true`, we always need the WHERE clause because ClickHouse has no concept of user roles. The agent-only user count is small enough that the query size limit is not a concern.

---

## Related Documents

- [too-many-users-edge-case.md](too-many-users-edge-case.md) — The `ShouldQueryAllUsers` fix that introduced this bug
- [consolidated-investigation-analysis.md](consolidated-investigation-analysis.md) — Full architecture of user filter flow
- [always-group-by-agent-investigation.md](always-group-by-agent-investigation.md) — Why always-GROUP-BY-agent was rejected (~4 weeks, correctness issues)
- [fe-group-by-usage-patterns.md](fe-group-by-usage-patterns.md) — FE views with/without per-agent grouping
- [clickhouse-large-in-clause-techniques.md](../general-learnings/clickhouse-large-in-clause-techniques.md) — ClickHouse query size escape hatches
- [clickhouse-ext-tables-investigation.md](clickhouse-ext-tables-investigation.md) — Verified ext tables are drop-in for go-servers (v2.34.0, native API, context preserved through QueryWithRetry)
