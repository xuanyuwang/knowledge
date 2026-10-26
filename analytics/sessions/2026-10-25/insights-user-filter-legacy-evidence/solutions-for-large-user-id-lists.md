# Solutions for Passing Large User ID Lists in ClickHouse Queries

**Created:** 2026-03-09
**Updated:** 2026-03-09

## Problem

When `ParseUserFilterForAnalytics` resolves user filters, it can produce thousands of user IDs that get embedded in a `WHERE agent_user_id IN (...)` clause. For large customers this exceeds ClickHouse's default max query size (~1MB), causing:

```
code: 62, message: Syntax error: failed at position 1048561
Max query size exceeded
```

## Solution Comparison

| # | Solution | Changes Required | Query Size | Performance | Complexity |
|---|----------|-----------------|------------|-------------|------------|
| 1 | `ShouldQueryAllUsers` flag | Caller-side only | Eliminated | Best (no filter) | Low |
| 2 | `ext` external tables | Caller-side only | Fixed (small SQL) | Good (native protocol) | Low |
| 3 | Increase `max_query_size` | ClickHouse config | Still large | Same | Trivial |
| 4 | Temporary tables | Shared layer + caller | Fixed (small SQL) | OK (extra INSERT round-trip) | Medium |
| 5 | Query batching | Caller-side | Split into N queries | Worse (N round-trips) | Medium |
| 6 | `IN` with subquery from reference table | Schema + write path | Fixed (small SQL) | Good (server-side) | High |
| 7 | User count threshold | Caller-side only | Eliminated above N | Risky (wrong results) | Low |
| 8 | Nil vs empty FinalUsers semantics | Core function | Eliminated | Best (no filter) | Low |
| 9 | `max_query_size` per-query setting | Caller-side (context) | Still large | Same | Trivial |
| 10 | JOIN with inline VALUES | Caller-side | Still large (body) | Similar to IN | Low |

---

## Solution Details

### 1. `ShouldQueryAllUsers` Flag (IMPLEMENTED)

**Status:** Implemented and deployed.

When the filter result means "all users" (root access + empty filter, ACL disabled + empty filter), set a flag so callers skip the WHERE clause entirely.

**Pros:**
- Already working in production
- Zero overhead — no user IDs sent at all
- Correct semantics: "all users" = no filter needed

**Cons:**
- Only solves the "all users" case
- Does NOT help when a large but specific subset is needed (e.g., `exclude_deactivated_users=true` with 4000+ active users out of 5000)

**Coverage:** Root access + empty filter, ACL disabled + empty filter.
**Gap:** Large specific subsets (e.g., exclude deactivated, large group expansion).

---

### 2. ClickHouse `ext` External Tables (CHOSEN - Best General Solution)

**Status:** Investigated and confirmed feasible. See `clickhouse-ext-tables-investigation.md`.

Send user IDs as a temporary in-memory table alongside the query using the `clickhouse-go/v2/ext` package. The SQL references it like a regular table via `IN (SELECT agent_user_id FROM agent_filter)`.

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
- Zero changes to shared `QueryWithRetry` layer — `clickhouse.Context` merges correctly
- Works for ANY number of users (not just "all users")
- Already available in driver v2.34.0, no dependency changes
- Temporary — table exists only for the query lifetime, no cleanup needed

**Cons:**
- First adoption in codebase — no existing patterns to follow (minor)
- Data sent over the wire per query (but binary-encoded, much smaller than SQL text)

**Why best:** Handles all cases (large subset, exclude deactivated, large groups) with minimal code changes and no architectural shifts.

---

### 3. Increase ClickHouse `max_query_size` Server Setting

**Status:** Not pursued.

ClickHouse's `max_query_size` defaults to ~1MB but can be increased in server config or user profiles.

```xml
<profiles>
  <default>
    <max_query_size>10485760</max_query_size> <!-- 10MB -->
  </default>
</profiles>
```

**Pros:**
- Trivial change — no code modifications
- Immediate fix

**Cons:**
- Kicks the can down the road — 10MB limit will also be hit eventually
- Larger queries consume more memory on ClickHouse server for parsing
- Doesn't address the fundamental inefficiency of embedding thousands of string literals in SQL text
- Requires ClickHouse infra team approval and coordination
- Query parsing time grows linearly with query size

**Verdict:** Band-aid, not a real solution.

---

### 4. ClickHouse Temporary Tables (INSERT then JOIN)

**Status:** Not pursued.

Create a temporary table, INSERT user IDs, then JOIN or use `IN (SELECT ... FROM temp_table)`.

```sql
CREATE TEMPORARY TABLE agent_filter (agent_user_id String);
INSERT INTO agent_filter VALUES ('id1'), ('id2'), ...;
SELECT ... FROM score_d WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter);
```

**Pros:**
- Query itself is small
- Standard SQL pattern

**Cons:**
- Requires multiple round-trips (CREATE, INSERT, SELECT, optionally DROP)
- Temporary tables are session-scoped — must ensure same connection is used for all statements
- `QueryWithRetry` abstraction doesn't support multi-statement transactions easily
- Connection pooling complicates session affinity
- The INSERT itself could exceed `max_query_size` if embedding values (though can batch)
- `ext` achieves the same result with zero round-trips

**Verdict:** Strictly worse than `ext` — same concept but with extra round-trips and session management complexity.

---

### 5. Query Batching (Split Users into Chunks)

**Status:** Not pursued.

Split the user ID list into chunks of N (e.g., 500), run N queries, merge results.

```go
const chunkSize = 500
for i := 0; i < len(userIDs); i += chunkSize {
    chunk := userIDs[i:min(i+chunkSize, len(userIDs))]
    // Run query with chunk, accumulate results
}
```

**Pros:**
- Each individual query stays within size limits
- No ClickHouse config or driver changes needed

**Cons:**
- N round-trips to ClickHouse (latency multiplied)
- Must merge/deduplicate results in application code
- Aggregation semantics break: `COUNT(DISTINCT ...)`, `AVG(...)`, percentiles cannot be correctly merged from partial results without complex logic
- Increases ClickHouse load (N queries instead of 1)
- Complex error handling (partial failures)

**Verdict:** Fundamentally broken for aggregation queries. Only viable for simple row-fetch queries.

---

### 6. Reference Table (Persistent User Filter Table)

**Status:** Not pursued.

Maintain a ClickHouse table (e.g., `user_filter_sets`) where the application writes filter sets before querying, then JOINs against it.

```sql
-- Write path (before analytics query)
INSERT INTO user_filter_sets (filter_id, agent_user_id) VALUES ('abc123', 'user1'), ('abc123', 'user2'), ...;

-- Read path (analytics query)
SELECT ... FROM score_d
WHERE agent_user_id IN (SELECT agent_user_id FROM user_filter_sets WHERE filter_id = 'abc123');

-- Cleanup (after query or TTL-based)
-- Table has TTL to auto-expire old filter sets
```

**Pros:**
- Query SQL is tiny and fixed-size
- Could cache filter sets across multiple API calls in the same request
- ClickHouse handles the join server-side efficiently

**Cons:**
- Requires new table schema in ClickHouse (DDL migration, infra coordination)
- Write-before-read pattern adds latency and complexity
- Need cleanup mechanism (TTL, garbage collection)
- Race conditions between write and read if using eventual consistency
- Overkill for a per-request filter that's used once

**Verdict:** Over-engineered for this use case. Makes sense if filter sets were shared across many queries or cached long-term, but our filters are per-request.

---

### 7. User Count Threshold (Skip Filter Above N)

**Status:** Mentioned in `too-many-users-edge-case.md` as Option 2. Not pursued.

If user count exceeds a threshold, skip the WHERE clause entirely.

```go
const maxUsersInWhereClause = 500
if len(users) > maxUsersInWhereClause {
    req.FilterByAttribute.Users = []*userpb.User{} // no filter
} else {
    req.FilterByAttribute.Users = result.FinalUsers
}
```

**Pros:**
- Simple to implement

**Cons:**
- Magic number — what's the right threshold?
- **Incorrect results**: Dropping the filter means querying ALL users, but the intent was a specific subset (e.g., 4000 active users out of 5000 total). The 1000 deactivated users would appear in results.
- Security risk: Could expose data to users who shouldn't see it
- Different behavior for customers just above vs below threshold

**Verdict:** Incorrect by design. Trading correctness for simplicity.

---

### 8. Nil vs Empty FinalUsers Semantics

**Status:** Mentioned in `too-many-users-edge-case.md` as Option 3. Not pursued.

Use different Go values to signal intent:
- `nil` → Query all users (no WHERE clause)
- `[]` (empty) → No users, early return
- Non-empty → Use in WHERE clause

**Pros:**
- Clean API, no extra fields
- No overhead

**Cons:**
- Subtle nil vs empty slice distinction — easy to accidentally break with `make([]User, 0)` or JSON unmarshaling
- Same limitation as `ShouldQueryAllUsers` — only helps when intent is "all users"
- Doesn't solve the large-but-specific-subset case
- Breaking change to existing callers

**Verdict:** Fragile encoding. `ShouldQueryAllUsers` flag (Solution 1) is strictly better for the same use case.

---

### 9. Per-Query `max_query_size` Setting

**Status:** Not pursued.

ClickHouse allows setting `max_query_size` per query via settings in the context:

```go
ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
    "max_query_size": 10485760, // 10MB
}))
```

**Pros:**
- No server-side config changes
- Targeted — only affects analytics queries, not all queries
- Easy to implement (similar pattern to `ext` context attachment)

**Cons:**
- Same fundamental issues as Solution 3 — larger queries still consume more parsing resources
- Doesn't reduce data transfer or parsing overhead
- Still embedding string literals in SQL text

**Verdict:** Quick workaround but doesn't address root inefficiency. Could be a stopgap while implementing `ext`.

---

### 10. JOIN with Inline VALUES

**Status:** Not explored previously.

Use a `VALUES` clause as an inline table expression:

```sql
SELECT ... FROM score_d
WHERE agent_user_id IN (
    SELECT agent_user_id FROM VALUES('agent_user_id String', ('user1'), ('user2'), ...)
)
```

Or using `arrayJoin`:

```sql
SELECT ... FROM score_d
WHERE agent_user_id IN arrayJoin(['user1', 'user2', ...])
```

**Pros:**
- Single query, no extra round-trips
- Slightly more structured than raw `IN (...)`

**Cons:**
- Still embeds all IDs in the SQL text — same size problem
- `arrayJoin` has memory overhead for large arrays
- No real improvement over `IN (...)` for the size limit issue

**Verdict:** Doesn't solve the problem — just a syntactic variation of the same approach.

---

## Recommendation

### Already Implemented
- **Solution 1 (`ShouldQueryAllUsers`)**: Handles the "all users" case. Keep as-is.

### Best Next Step
- **Solution 2 (`ext` external tables)**: Handles ALL remaining cases (large subsets, exclude deactivated, large groups). Minimal code changes, no infra changes, no new dependencies.

### Not Recommended
- Solutions 3, 9: Band-aids that don't address root cause
- Solution 4: Strictly worse than `ext`
- Solution 5: Breaks aggregation semantics
- Solution 6: Over-engineered
- Solution 7: Incorrect results
- Solution 8: Fragile, same limitations as Solution 1
- Solution 10: Doesn't solve the problem

### Implementation Priority

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
