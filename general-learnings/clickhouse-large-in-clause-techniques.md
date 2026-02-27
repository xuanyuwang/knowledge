# ClickHouse: Handling Large IN Clauses (Thousands of Values)

**Created**: 2026-02-26
**Updated**: 2026-02-26

## Problem

When building queries like `WHERE agent_user_id IN ('user1', 'user2', ..., 'userN')` with thousands of values, you hit ClickHouse's `max_query_size` limit. The default is **262,144 bytes (256 KB)** — defined as `DBMS_DEFAULT_MAX_QUERY_SIZE` in ClickHouse source. With UUIDs (~36 chars each + quotes + commas), you can fit roughly 6,000 values before hitting this limit.

Additional AST-level limits also apply:
- `max_ast_elements` — max number of nodes in the query syntax tree (default: 50,000)
- `max_expanded_ast_elements` — max nodes after macro/alias expansion (default: 500,000)

---

## Approach 1: Increase `max_query_size` Per-Query

### How It Works

`max_query_size` is a session-level setting that can be overridden per-query. You simply set it higher.

### Go Code Example

```go
ctx := clickhouse.Context(context.Background(), clickhouse.WithSettings(clickhouse.Settings{
    "max_query_size":          10 * 1024 * 1024, // 10 MB
    "max_ast_elements":        500000,
    "max_expanded_ast_elements": 500000,
}))

rows, err := conn.Query(ctx, "SELECT ... WHERE agent_user_id IN (?)", userIDs)
```

Or at connection level (applies to all queries on this connection):

```go
conn, err := clickhouse.Open(&clickhouse.Options{
    Addr: []string{"clickhouse:9000"},
    Settings: clickhouse.Settings{
        "max_query_size": 10 * 1024 * 1024,
    },
})
```

### Pros
- **Simplest change** — no query restructuring needed
- Works with existing query builders that inline values
- Can be set per-query or per-connection

### Cons
- **Band-aid, not a cure** — you're just raising the ceiling, not fixing the architecture
- Very large queries consume more memory on the ClickHouse server for parsing/AST
- You also need to raise `max_ast_elements` and `max_expanded_ast_elements` proportionally
- The Go driver still needs to serialize all values into the SQL string, which costs memory on the client side
- **Does not scale** — at 50K+ values, query parsing itself becomes slow

### Changes Required
- **Query builder**: None
- **Parameter passing**: None (if already using slice expansion)
- **Context/settings**: Add `WithSettings` to the context

---

## Approach 2: External Data Tables (RECOMMENDED for Go)

### How It Works

The ClickHouse Go driver's `ext` package lets you send a temporary table alongside your query. The data is transmitted as a separate data block (not as SQL text), so it completely bypasses `max_query_size`. The table exists only for the duration of that single query.

### Go Code Example

```go
import (
    "context"
    "github.com/ClickHouse/clickhouse-go/v2"
    "github.com/ClickHouse/clickhouse-go/v2/ext"
)

func queryWithLargeFilter(ctx context.Context, conn clickhouse.Conn, userIDs []string) error {
    // Create an external table with just the filter values
    filterTable, err := ext.NewTable("user_filter",
        ext.Column("user_id", "String"),
    )
    if err != nil {
        return fmt.Errorf("creating external table: %w", err)
    }

    // Populate it — each Append call adds one row
    for _, uid := range userIDs {
        if err := filterTable.Append(uid); err != nil {
            return fmt.Errorf("appending to external table: %w", err)
        }
    }

    // Attach to context
    queryCtx := clickhouse.Context(ctx,
        clickhouse.WithExternalTable(filterTable),
    )

    // Use IN (SELECT ...) or JOIN — the table name matches what you passed to NewTable
    rows, err := conn.Query(queryCtx, `
        SELECT agent_user_id, score, timestamp
        FROM qa_scores
        WHERE agent_user_id IN (SELECT user_id FROM user_filter)
          AND timestamp >= ?
          AND timestamp < ?
    `, startTime, endTime)
    if err != nil {
        return err
    }
    defer rows.Close()

    for rows.Next() {
        // scan results...
    }
    return rows.Err()
}
```

### Alternative: JOIN instead of IN (SELECT ...)

```go
rows, err := conn.Query(queryCtx, `
    SELECT qs.agent_user_id, qs.score, qs.timestamp
    FROM qa_scores qs
    INNER JOIN user_filter uf ON qs.agent_user_id = uf.user_id
    WHERE qs.timestamp >= ?
      AND qs.timestamp < ?
`, startTime, endTime)
```

### Pros
- **Completely bypasses `max_query_size`** — data sent as binary blocks, not SQL text
- **Scales to millions of values** — limited only by memory, not query parsing
- **No server-side config changes needed** — works with default settings
- **Clean separation** — filter data is data, not SQL
- **First-class Go driver support** — `ext` package is well-maintained
- For distributed queries, external tables are automatically sent to all remote servers

### Cons
- **Requires query restructuring** — must change `IN (?)` to `IN (SELECT ... FROM external_table)`
- Slight overhead for very small lists (< 100 values) compared to inline
- External table data is held in memory on both client and server
- **Only works with native protocol** (port 9000), not HTTP interface via Go driver

### Changes Required
- **Query builder**: Change `IN (?)` to `IN (SELECT col FROM ext_table)` or use JOIN
- **Parameter passing**: Replace slice parameter with external table construction
- **Context/settings**: Wrap context with `WithExternalTable`

---

## Approach 3: Temporary Tables (Session-Scoped)

### How It Works

Create a temporary table at the start of a session, INSERT the filter values, then reference it in queries. The table persists for the entire session (connection lifetime).

### Go Code Example

```go
func queryWithTempTable(ctx context.Context, conn clickhouse.Conn, userIDs []string) error {
    // Create temporary table (Memory engine by default)
    err := conn.Exec(ctx, `
        CREATE TEMPORARY TABLE IF NOT EXISTS tmp_user_filter (
            user_id String
        ) ENGINE = Memory
    `)
    if err != nil {
        return err
    }

    // Batch insert the filter values
    batch, err := conn.PrepareBatch(ctx, "INSERT INTO tmp_user_filter (user_id)")
    if err != nil {
        return err
    }
    for _, uid := range userIDs {
        if err := batch.Append(uid); err != nil {
            return err
        }
    }
    if err := batch.Send(); err != nil {
        return err
    }

    // Now query using the temp table
    rows, err := conn.Query(ctx, `
        SELECT agent_user_id, score, timestamp
        FROM qa_scores
        WHERE agent_user_id IN (SELECT user_id FROM tmp_user_filter)
          AND timestamp >= ?
          AND timestamp < ?
    `, startTime, endTime)
    if err != nil {
        return err
    }
    defer rows.Close()

    // Don't forget to clean up if reusing the connection
    defer conn.Exec(ctx, "DROP TABLE IF EXISTS tmp_user_filter")

    for rows.Next() {
        // scan results...
    }
    return rows.Err()
}
```

### Pros
- **Bypasses `max_query_size`** — INSERT uses a separate parser that doesn't count against it
- **Can be reused across multiple queries** in the same session
- **Familiar SQL pattern** — easy to understand
- Batch insert is very efficient

### Cons
- **Requires a dedicated connection** — temp tables are session-scoped, so you need to control which connection you use (no connection pooling randomness)
- **Multiple round-trips** — CREATE + INSERT + SELECT + DROP = 4 round trips minimum
- **Race conditions** — if using connection pools, another goroutine might get a different connection
- **Cleanup responsibility** — you must DROP the table or risk stale data on connection reuse
- **Not safe with `database/sql` interface** — the standard Go sql.DB pool doesn't guarantee same connection

### Changes Required
- **Query builder**: Change `IN (?)` to `IN (SELECT ... FROM tmp_table)`
- **Parameter passing**: Separate batch insert step before the query
- **Connection management**: Must ensure same connection for all steps

---

## Approach 4: Go Driver Slice Expansion (Default Behavior)

### How It Works

The Go ClickHouse driver automatically unfolds Go slices into comma-separated values in the SQL. This is what you're likely already using.

### Go Code Example

```go
// The driver expands []string{"a","b","c"} into 'a','b','c' in the SQL
rows, err := conn.Query(ctx,
    "SELECT * FROM qa_scores WHERE agent_user_id IN (?)",
    userIDs, // []string
)
```

For tuple-based IN:
```go
// GroupSet adds parentheses: (val1, val2, val3)
rows, err := conn.Query(ctx,
    "SELECT * FROM qa_scores WHERE agent_user_id IN ?",
    clickhouse.GroupSet{Values: []interface{}{"user1", "user2", "user3"}},
)
```

### Pros
- **Simplest possible code** — just pass a slice
- No query restructuring needed
- Works with any query builder that supports parameterized queries

### Cons
- **This IS the problem** — the slice gets expanded into SQL text, hitting `max_query_size`
- With 10K UUIDs: ~36 chars * 10K + quotes + commas ≈ 400 KB > 256 KB default limit
- Client-side memory: the entire expanded query string must fit in memory
- **Not a solution**, just the baseline for comparison

### Changes Required
- None — this is the default behavior

---

## Comparison Matrix

| Approach | Max Values | Round Trips | Query Changes | Config Changes | Connection Pool Safe |
|----------|-----------|-------------|---------------|----------------|---------------------|
| 1. Increase `max_query_size` | ~50K (practical) | 1 | None | Per-query setting | Yes |
| 2. External Data Tables | Millions | 1 | Yes (`IN (SELECT...)`) | None | Yes |
| 3. Temporary Tables | Millions | 4 | Yes (`IN (SELECT...)`) | None | **No** |
| 4. Slice Expansion (default) | ~6K UUIDs | 1 | None | None | Yes |

---

## Recommendation

### For immediate fix (< 1 day of work):

**Increase `max_query_size` to 10MB per-query** (Approach 1). This buys time and requires no query changes:

```go
ctx := clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
    "max_query_size": 10 * 1024 * 1024,
    "max_ast_elements": 500000,
}))
```

### For production-grade solution:

**Use External Data Tables** (Approach 2). This is the cleanest approach because:
1. Data is sent as binary, not SQL text — no size limits from query parsing
2. Single round-trip — no session management complexity
3. Connection-pool safe — external table is attached to the context, not the connection
4. The Go driver has first-class support via the `ext` package
5. ClickHouse itself recommends this pattern for large IN clauses (see [official docs](https://clickhouse.com/docs/en/sql-reference/operators/in): "If a data set is large, put it in a temporary table")

### Implementation pattern for a query builder:

```go
// Helper function to build external table from string slice
func buildStringFilterTable(name, column string, values []string) (*ext.Table, error) {
    table, err := ext.NewTable(name, ext.Column(column, "String"))
    if err != nil {
        return nil, err
    }
    for _, v := range values {
        if err := table.Append(v); err != nil {
            return nil, err
        }
    }
    return table, nil
}

// Usage in query builder
func (qb *QueryBuilder) WithUserFilter(ctx context.Context, userIDs []string) (context.Context, string) {
    const threshold = 1000 // switch strategy above this many values

    if len(userIDs) <= threshold {
        // Small list: inline as usual
        return ctx, fmt.Sprintf("agent_user_id IN (%s)", qb.placeholders(len(userIDs)))
    }

    // Large list: use external table
    table, err := buildStringFilterTable("user_id_filter", "user_id", userIDs)
    if err != nil {
        // fallback to inline with increased max_query_size
        ctx = clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
            "max_query_size": 10 * 1024 * 1024,
        }))
        return ctx, fmt.Sprintf("agent_user_id IN (%s)", qb.placeholders(len(userIDs)))
    }

    ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(table))
    return ctx, "agent_user_id IN (SELECT user_id FROM user_id_filter)"
}
```

This hybrid approach uses inline expansion for small lists (faster, simpler) and external tables for large lists (bypasses size limits).

---

## References

- [ClickHouse IN operator docs](https://clickhouse.com/docs/en/sql-reference/operators/in) — "If a data set is large, put it in a temporary table"
- [ClickHouse external data docs](https://clickhouse.com/docs/en/engines/table-engines/special/external-data)
- [Go driver external data example](https://github.com/ClickHouse/clickhouse-go/blob/main/examples/clickhouse_api/external_data.go)
- [Go driver IN clause examples](https://clickhouse.com/docs/en/integrations/go#using-in-clause)
- ClickHouse source: `DBMS_DEFAULT_MAX_QUERY_SIZE = 262144` in `src/Core/Defines.h`
