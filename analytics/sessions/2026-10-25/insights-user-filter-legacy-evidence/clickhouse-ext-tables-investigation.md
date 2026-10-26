# ClickHouse `ext` (External Data Tables) Feasibility Investigation

**Created:** 2026-02-26
**Updated:** 2026-02-26

## Summary

The `ext` package from `clickhouse-go/v2` allows sending temporary in-memory tables alongside a query. This is useful for filtering large queries by a set of IDs (e.g., agent user IDs) without embedding them in `IN (...)` clauses. This document investigates whether it can be adopted in go-servers' insights-server.

**Verdict: YES, `ext` can be adopted with minimal refactoring.** The codebase uses the native ClickHouse API (`driver.Conn`) and v2.34.0, which includes the `ext` package. The main change needed is modifying `QueryWithRetry` to accept external tables via context.

---

## 1. Driver Version

- **Package:** `github.com/ClickHouse/clickhouse-go/v2`
- **Version:** `v2.34.0`
- **Source:** `/Users/xuanyu.wang/repos/go-servers/go.mod` (line 12)

This is the v2 driver, which supports both the native protocol API and database/sql. The `ext` package ships with v2 at `github.com/ClickHouse/clickhouse-go/v2/ext`.

## 2. Connection API: Native (Not database/sql)

The codebase uses **exclusively the native API** (`clickhouse.Open` returning `driver.Conn`). There is **no** `sql.Open("clickhouse", ...)` usage anywhere.

### Connection creation chain

1. **Registry layer** (`/Users/xuanyu.wang/repos/go-servers/shared/clickhouse/registry/`)
   - `type.go:49` — `ConnectionResolverFn` returns `driver.Conn`
   - `utils.go:20-22` — Default resolver calls `clickhouse.Open(opts)` returning `driver.Conn`
   - `base_registry.go:22-35` — `CreateNewConnection` stores and returns `driver.Conn`

2. **insights-server connection** (`/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/clickhouse_connect.go`)
   ```go
   func (a AnalyticsServiceImpl) getConversationsClickhouseConn(ctx context.Context, customerID, profileID string) (driver.Conn, error) {
       return a.clickhouseConvoRegistry.GetConnection(ctx, customerID, profileID)
   }
   ```
   Returns `driver.Conn` — the native connection type.

3. **dataplatform layer** (`/Users/xuanyu.wang/repos/go-servers/insights-server/internal/dataplatform/common_clickhouse.go:45-56`)
   ```go
   func queryClickHouseInConversationsCluster(ctx context.Context, clickHouseClient *conversationsclient.ClickHouseClient, customerID, profileID string, chQuery string, chArgs []any) (driver.Rows, error) {
       conn, err := clickHouseClient.ConnectOrGetConn(customerID, profileID)
       // ...
       return clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
   }
   ```

**This is good news.** The `ext` package works with BOTH native API and database/sql. Since we use native API, there are no compatibility concerns.

## 3. Query Execution Pattern

All ClickHouse queries in insights-server follow the same pattern:

```
analyticsimpl handler
  → build SQL string + args (chQuery, chArgs)
  → get connection: a.getConversationsClickhouseConn(ctx, customerID, profileID)
  → execute: clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
  → scan rows
```

### Central execution function

**File:** `/Users/xuanyu.wang/repos/go-servers/shared/clickhouse/shared/common.go`

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

The retry wrapper creates a ClickHouse context with query ID:

```go
// Line 260-262
func createClickhouseContext(ctx context.Context, queryID string) context.Context {
    return clickhouse.Context(ctx, clickhouse.WithQueryID(queryID))
}
```

### All callers in insights-server/internal/analyticsimpl (19 files):

| File | Line |
|------|------|
| `retrieve_qa_score_stats_clickhouse.go` | 633 |
| `retrieve_qa_conversations_clickhouse.go` | 389 |
| `retrieve_conversation_stats_clickhouse.go` | 411 |
| `retrieve_scorecard_stats_clickhouse.go` | 89 |
| `retrieve_agent_stats_clickhouse.go` | 85 |
| `retrieve_manager_stats_clickhouse.go` | 91 |
| `retrieve_adherences_clickhouse.go` | 116 |
| `retrieve_hint_stats_clickhouse.go` | 303 |
| `retrieve_assistance_stats_clickhouse.go` | 229 |
| `retrieve_knowledge_base_stats_clickhouse.go` | 128 |
| `retrieve_knowledge_assist_stats_clickhouse.go` | 261 |
| `retrieve_live_assist_stats_clickhouse.go` | 121 |
| `retrieve_note_taking_stats_clickhouse.go` | 218 |
| `retrieve_guided_workflow_stats_clickhouse.go` | 242 |
| `retrieve_metadata_values_clickhouse.go` | 162 |
| `retrieve_summarization_stats_clickhouse.go` | 254 |
| `retrieve_suggestion_stats_clickhouse.go` | 125 |
| `retrieve_smart_compose_stats_clickhouse.go` | 160 |
| `retrieve_closed_non_empty_conversations_clickhouse.go` | 20 |

All follow the identical pattern: `clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)`.

## 4. Context Passing

Context flows all the way through:
1. gRPC handler receives `ctx`
2. Passed to `getConversationsClickhouseConn(ctx, ...)`
3. Passed to `QueryWithRetry(ctx, conn, query, args...)`
4. Inside retry, transformed via `createClickhouseContext(ctx, queryID)` which calls `clickhouse.Context(ctx, clickhouse.WithQueryID(queryID))`
5. The enriched `clickhouseCtx` is passed to `conn.Query(clickhouseCtx, query, args...)`

**Important detail:** `clickhouse.Context` already merges options. If we attach `WithExternalTable` to the incoming `ctx`, the current `createClickhouseContext` call will **overwrite** it because `clickhouse.Context` creates a new options value.

## 5. `ext` Package Availability

The `ext` package is present at:
```
$(go env GOMODCACHE)/github.com/ClickHouse/clickhouse-go/v2@v2.34.0/ext/ext.go
```

It is **not currently imported** anywhere in go-servers (confirmed by grep). No existing code uses `WithExternalTable`.

### ext API

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
ctx := clickhouse.Context(ctx,
    clickhouse.WithExternalTable(table),
)

// Query references it like a regular table
rows, err := conn.Query(ctx, "SELECT * FROM my_table WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter)")
```

## 6. What Needs to Change to Adopt `ext`

### Option A: Modify `QueryWithRetry` to accept external tables (Recommended)

Add a new function or parameter to `QueryWithRetry` that accepts external tables:

```go
// New function in shared/clickhouse/shared/common.go
func QueryWithRetryAndExtTables(ctx context.Context, conn driver.Conn, query string, tables []*ext.Table, args ...any) (driver.Rows, error) {
    ctx = WithQueryContext(ctx, "Query", query)
    var rows driver.Rows
    return rows, runWithRetry(ctx, func(clickhouseCtx context.Context, attempt int, queryID string) error {
        var err error
        rows, err = conn.Query(clickhouseCtx, query, args...)
        return err
    })
}
```

**But there's a problem:** `createClickhouseContext` (line 260-262) currently only sets `WithQueryID`. It would need to also forward `WithExternalTable`:

```go
func createClickhouseContext(ctx context.Context, queryID string, tables ...*ext.Table) context.Context {
    opts := []clickhouse.QueryOption{clickhouse.WithQueryID(queryID)}
    if len(tables) > 0 {
        opts = append(opts, clickhouse.WithExternalTable(tables...))
    }
    return clickhouse.Context(ctx, opts...)
}
```

This requires changes to:
1. `createClickhouseContext` — add tables parameter
2. `wrapFuncWithDeadlineCheckAndAttemptTracking` — thread tables through
3. Add `QueryWithRetryAndExtTables` (or add tables param to existing)

**Estimated effort: ~30-50 lines of code in `common.go`**, plus the caller-side code to build the ext table.

### Option B: Pre-attach to context before calling QueryWithRetry

The caller could do:
```go
ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(table))
rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
```

**Problem:** `createClickhouseContext` inside `runWithRetry` calls `clickhouse.Context(ctx, clickhouse.WithQueryID(queryID))` which **replaces** the options, losing the external table. The `clickhouse.Context` function merges settings maps but creates fresh `QueryOptions` for other fields.

Let me verify this...

Actually, looking at the `clickhouse.Context` implementation more carefully: it stores options in context values and the `Context()` function does merge existing options from the parent context. So if the parent already has `WithExternalTable`, the new `clickhouse.Context(ctx, WithQueryID(...))` call should **preserve** the external tables from the parent context, and only add the query ID.

If that's the case, **Option B works with zero changes to `common.go`**. The caller just needs to:
```go
ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(table))
// QueryWithRetry will call clickhouse.Context(ctx, WithQueryID(...)) which merges, not replaces
```

### Verification: `clickhouse.Context` DOES preserve external tables

Confirmed by reading the source (`context.go`):

```go
func Context(parent context.Context, options ...QueryOption) context.Context {
    var opt QueryOptions
    if ctxOpt, ok := parent.Value(_contextOptionKey).(QueryOptions); ok {
        opt = ctxOpt  // ← copies ALL fields from parent, including .external
    }
    for _, f := range options {
        f(&opt)  // ← applies only the new option (e.g., WithQueryID sets .queryID only)
    }
    // ...
}
```

And `WithQueryID` only touches `o.queryID`:
```go
func WithQueryID(queryID string) QueryOption {
    return func(o *QueryOptions) error { o.queryID = queryID; return nil }
}
```

So when `createClickhouseContext(ctx, queryID)` calls `clickhouse.Context(ctx, clickhouse.WithQueryID(queryID))`, it copies the parent's `QueryOptions` (including any `.external` tables), then sets only the query ID. **External tables are preserved.**

**Option B is confirmed to work with zero changes to `common.go`.**

### Recommended adoption pattern (Option B)

At the call site (e.g., in `retrieve_qa_score_stats_clickhouse.go`):

```go
import "github.com/ClickHouse/clickhouse-go/v2/ext"

// Build external table with agent IDs
agentTable, err := ext.NewTable("agent_filter",
    ext.Column("agent_user_id", "String"),
)
for _, agentID := range agentIDs {
    agentTable.Append(agentID)
}

// Attach to context — this will be preserved through QueryWithRetry
ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(agentTable))

// Use in SQL query
chQuery := `SELECT ... FROM score_d WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter) ...`

// Execute — no changes to QueryWithRetry needed
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
| Refactoring needed | **None to shared layer** — `clickhouse.Context` merges correctly; caller just attaches ext tables to ctx before calling `QueryWithRetry` |

### Conclusion

The go-servers codebase is **fully ready** to adopt `ext` tables:

1. **v2.34.0** includes the `ext` package — no dependency changes needed
2. **Native API** (`driver.Conn`) is used everywhere — `ext` is fully compatible
3. **Centralized query execution** via `clickhouseshared.QueryWithRetry` means changes are localized
4. **Context propagation** is already in place and `clickhouse.Context` correctly merges/preserves external tables through the retry wrapper
5. **Zero changes to `common.go` or the shared layer** — the caller simply attaches external tables to the context before calling `QueryWithRetry`

The only code needed is at the call site: build the `ext.Table`, attach via `clickhouse.Context(ctx, clickhouse.WithExternalTable(...))`, and reference the table name in the SQL query.
