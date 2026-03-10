# Solutions for Passing Large User ID Lists in ClickHouse Queries

**Created:** 2026-03-09
**Updated:** 2026-03-09
**Origin:** Extracted from `insights-user-filter/solutions-for-large-user-id-lists.md`

## Problem

When `ParseUserFilterForAnalytics` resolves user filters, it can produce thousands of user IDs that get embedded in a `WHERE agent_user_id IN (...)` clause. For large customers this exceeds ClickHouse's default max query size (~1MB), causing:

```
code: 62, message: Syntax error: failed at position 1048561
Max query size exceeded
```

## Solution Comparison

| # | Solution | Changes Required | Query Size | Performance | Complexity | Verdict |
|---|----------|-----------------|------------|-------------|------------|---------|
| 1 | `ShouldQueryAllUsers` flag | Caller-side only | Eliminated | Best | Low | IMPLEMENTED |
| 2 | **`ext` external tables** | Caller-side only | Fixed (small SQL) | Good | Low | **CHOSEN** |
| 3 | Increase `max_query_size` (server) | CH config | Still large | Same | Trivial | Band-aid |
| 4 | Temporary tables (CREATE+INSERT) | Shared layer + caller | Fixed | OK | Medium | Worse than ext |
| 5 | Query batching | Caller-side | Split into N | Worse | Medium | Breaks aggregation |
| 6 | Reference table | Schema + write path | Fixed | Good | High | Over-engineered |
| 7 | User count threshold | Caller-side only | Eliminated above N | Risky | Low | Incorrect results |
| 8 | Nil vs empty FinalUsers | Core function | Eliminated | Best | Low | Fragile |
| 9 | Per-query `max_query_size` | Caller-side (context) | Still large | Same | Trivial | Stopgap only |
| 10 | JOIN with inline VALUES | Caller-side | Still large | Similar | Low | Doesn't help |

---

## Solution Details

### 1. `ShouldQueryAllUsers` Flag (IMPLEMENTED)

When the filter result means "all users" (root access + empty filter, ACL disabled + empty filter), set a flag so callers skip the WHERE clause entirely.

**Pros:** Zero overhead, correct semantics.
**Cons:** Only solves the "all users" case. Does NOT help when a large but specific subset is needed.
**Coverage:** Root access + empty filter, ACL disabled + empty filter.
**Gap:** Large specific subsets (exclude deactivated, large group expansion).

---

### 2. ClickHouse `ext` External Tables (CHOSEN)

Send user IDs as a temporary in-memory table alongside the query using the `clickhouse-go/v2/ext` package. The SQL references it via `IN (SELECT agent_user_id FROM agent_filter)`.

```go
table, _ := ext.NewTable("agent_filter", ext.Column("agent_user_id", "String"))
for _, id := range agentIDs {
    table.Append(id)
}
ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(table))
// SQL: WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter)
```

**Pros:**
- Query SQL stays small and fixed-size regardless of user count
- User IDs sent via native protocol binary encoding (efficient)
- Zero changes to shared `QueryWithRetry` — `clickhouse.Context` merges correctly
- Works for ANY number of users
- Already available in driver v2.34.0, no dependency changes
- Temporary — table exists only for the query lifetime

**Cons:**
- First adoption in codebase (minor)

**Why best:** Handles all cases with minimal code changes and no architectural shifts. See `ext-tables-feasibility.md` for detailed investigation.

---

### 3. Increase `max_query_size` Server Setting

Increase ClickHouse's `max_query_size` from ~1MB to 10MB+ in server config.

**Pros:** Trivial, no code changes.
**Cons:** Kicks the can down the road. Larger queries consume more parsing resources. Requires infra team coordination.
**Verdict:** Band-aid.

---

### 4. Temporary Tables (INSERT then JOIN)

Create a temporary table, INSERT user IDs, then JOIN.

```sql
CREATE TEMPORARY TABLE agent_filter (agent_user_id String);
INSERT INTO agent_filter VALUES ('id1'), ('id2'), ...;
SELECT ... FROM score_d WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter);
```

**Pros:** Query itself is small, standard SQL.
**Cons:** Multiple round-trips (CREATE, INSERT, SELECT). Session-scoped tables require connection affinity. `QueryWithRetry` doesn't support multi-statement transactions. The INSERT could also exceed `max_query_size`.
**Verdict:** Strictly worse than `ext` — same concept but with extra round-trips and session management.

---

### 5. Query Batching (Split Users into Chunks)

Split user IDs into chunks of N, run N queries, merge results.

**Pros:** Each query stays within limits.
**Cons:** **Fundamentally broken for aggregation queries.** `COUNT(DISTINCT ...)`, `AVG(...)`, percentiles cannot be correctly merged from partial results. N round-trips. Complex error handling.
**Verdict:** Only viable for simple row-fetch queries, not analytics aggregations.

---

### 6. Reference Table (Persistent User Filter Table)

Maintain a ClickHouse table where the app writes filter sets before querying, then JOINs against it.

**Pros:** Query SQL is tiny. Could cache filter sets.
**Cons:** Requires new table schema (DDL migration, infra coordination). Write-before-read latency. Cleanup mechanism needed. Race conditions with eventual consistency. Overkill for per-request filters.
**Verdict:** Over-engineered for this use case.

---

### 7. User Count Threshold (Skip Filter Above N)

If user count exceeds a threshold (e.g., 500), skip the WHERE clause entirely.

**Pros:** Simple.
**Cons:** **Incorrect results.** Dropping the filter queries ALL users, but the intent was a specific subset. Security risk. Magic number.
**Verdict:** Incorrect by design.

---

### 8. Nil vs Empty FinalUsers Semantics

Use `nil` = all users, `[]` = no users, non-empty = specific users.

**Pros:** Clean API, no extra fields.
**Cons:** Subtle nil vs empty slice — easy to break. Same limitation as Solution 1 (only helps "all users" case). Breaking change.
**Verdict:** Fragile. `ShouldQueryAllUsers` flag is strictly better.

---

### 9. Per-Query `max_query_size` Setting

Set `max_query_size` per query via ClickHouse context settings.

```go
ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
    "max_query_size": 10485760,
}))
```

**Pros:** No server config changes. Targeted to analytics queries only.
**Cons:** Same fundamental issues as Solution 3 — larger queries still consume more resources. Doesn't reduce data transfer.
**Verdict:** Quick stopgap while implementing `ext`.

---

### 10. JOIN with Inline VALUES / arrayJoin

```sql
WHERE agent_user_id IN arrayJoin(['user1', 'user2', ...])
```

**Pros:** Single query.
**Cons:** Still embeds all IDs in SQL text — same size problem.
**Verdict:** Syntactic variation, doesn't solve the problem.

---

## Recommendation

```
[Already done] Solution 1: ShouldQueryAllUsers flag
      |
      v
[Next]        Solution 2: ext external tables
      |         - Covers: exclude_deactivated_users with large active set
      |         - Covers: large group expansion
      |         - Covers: any future large-subset scenario
      v
[Optional]    Solution 9: Per-query max_query_size bump (stopgap if ext delayed)
```
