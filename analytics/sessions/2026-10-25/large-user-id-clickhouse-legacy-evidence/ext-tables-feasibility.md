# ClickHouse `ext` (External Data Tables) Feasibility Investigation

**Created:** 2026-02-26
**Updated:** 2026-03-09
**Origin:** Extracted from `insights-user-filter/clickhouse-ext-tables-investigation.md`

## Summary

The `ext` package from `clickhouse-go/v2` allows sending temporary in-memory tables alongside a query. This is useful for filtering large queries by a set of IDs without embedding them in `IN (...)` clauses.

**Verdict: YES, `ext` can be adopted with minimal refactoring.** The codebase uses the native ClickHouse API (`driver.Conn`) and v2.34.0, which includes the `ext` package. The caller just attaches external tables to the context before calling `QueryWithRetry` — zero changes to the shared layer.

---

## 1. Driver Version

- **Package:** `github.com/ClickHouse/clickhouse-go/v2`
- **Version:** `v2.34.0`
- **Source:** `/Users/xuanyu.wang/repos/go-servers/go.mod` (line 12)

The `ext` package ships with v2 at `github.com/ClickHouse/clickhouse-go/v2/ext`.

## 2. Connection API: Native (Not database/sql)

The codebase uses **exclusively the native API** (`clickhouse.Open` returning `driver.Conn`). No `sql.Open("clickhouse", ...)` usage anywhere.

### Connection creation chain

1. **Registry layer** (`shared/clickhouse/registry/`)
   - `type.go:49` — `ConnectionResolverFn` returns `driver.Conn`
   - `utils.go:20-22` — Default resolver calls `clickhouse.Open(opts)` returning `driver.Conn`
   - `base_registry.go:22-35` — `CreateNewConnection` stores and returns `driver.Conn`

2. **insights-server** (`insights-server/internal/analyticsimpl/clickhouse_connect.go`)
   - `getConversationsClickhouseConn` returns `driver.Conn`

3. **dataplatform layer** (`insights-server/internal/dataplatform/common_clickhouse.go:45-56`)
   - Calls `clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)`

The `ext` package works with both native API and database/sql. No compatibility concerns.

## 3. Query Execution Pattern

All ClickHouse queries follow:

```
analyticsimpl handler
  -> build SQL string + args (chQuery, chArgs)
  -> get connection: a.getConversationsClickhouseConn(ctx, customerID, profileID)
  -> execute: clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
  -> scan rows
```

### Central execution function

**File:** `shared/clickhouse/shared/common.go`

```go
// Line 216-224
func QueryWithRetry(ctx context.Context, conn driver.Conn, query string, args ...any) (driver.Rows, error) {
    ctx = WithQueryContext(ctx, "Query", query)
    var rows driver.Rows
    return rows, runWithRetry(ctx, func(clickhouseCtx context.Context, attempt int, queryID string) error {
        var err error
        rows, err = conn.Query(clickhouseCtx, query, args...)
        return err
    })
}
```

### All callers in insights-server/internal/analyticsimpl (19 files)

All follow the identical pattern: `clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)`.

## 4. Context Passing and Preservation

Context flows: gRPC handler -> getConversationsClickhouseConn -> QueryWithRetry -> createClickhouseContext -> conn.Query.

Inside the retry wrapper, `createClickhouseContext` calls `clickhouse.Context(ctx, clickhouse.WithQueryID(queryID))`.

### Key Question: Does `clickhouse.Context` preserve external tables?

**YES.** Confirmed by reading the `clickhouse-go` source (`context.go`):

```go
func Context(parent context.Context, options ...QueryOption) context.Context {
    var opt QueryOptions
    if ctxOpt, ok := parent.Value(_contextOptionKey).(QueryOptions); ok {
        opt = ctxOpt  // copies ALL fields from parent, including .external
    }
    for _, f := range options {
        f(&opt)  // applies only the new option (e.g., WithQueryID sets .queryID only)
    }
}
```

`WithQueryID` only touches `o.queryID`. External tables attached to the parent context are preserved.

## 5. `ext` Package API

```go
import "github.com/ClickHouse/clickhouse-go/v2/ext"

// Create a table with typed columns
table, _ := ext.NewTable("agent_filter",
    ext.Column("agent_user_id", "String"),
)

// Append rows
for _, agentID := range agentIDs {
    table.Append(agentID)
}

// Attach to context
ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(table))

// Query references it like a regular table
rows, err := conn.Query(ctx,
    "SELECT * FROM my_table WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter)")
```

The `ext` package is present in the module cache but **not currently imported** anywhere in go-servers.

## 6. Adoption Pattern (Zero Shared Layer Changes)

At the call site:

```go
import "github.com/ClickHouse/clickhouse-go/v2/ext"

// Build external table with agent IDs
agentTable, err := ext.NewTable("agent_filter",
    ext.Column("agent_user_id", "String"),
)
for _, agentID := range agentIDs {
    agentTable.Append(agentID)
}

// Attach to context -- preserved through QueryWithRetry
ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(agentTable))

// Use in SQL query
chQuery := `SELECT ... FROM score_d WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter) ...`

// Execute -- no changes to QueryWithRetry needed
rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
```

## 7. Assessment

| Criterion | Status |
|-----------|--------|
| Driver version (v2 required) | v2.34.0 |
| Native API (not database/sql) | Native (`driver.Conn`) throughout |
| `ext` package available | Yes, ships with v2.34.0 |
| Context flows through to query | Yes, all the way |
| Current code uses `ext` | No, first adoption |
| Refactoring needed | **None to shared layer** |

### Conclusion

The go-servers codebase is **fully ready** to adopt `ext` tables. The only code needed is at the call site: build the `ext.Table`, attach via context, and reference the table name in SQL.
