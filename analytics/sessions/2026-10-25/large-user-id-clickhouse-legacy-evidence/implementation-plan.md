# Implementation Plan: ClickHouse `ext` External Tables for Large User ID Lists

**Created:** 2026-03-09
**Updated:** 2026-03-09
**Status:** Merged (PR #26178 + #26250). Staging flag enabled. Pending validation and production rollout.

## Goal

Replace `WHERE agent_user_id IN ('id1', 'id2', ..., 'idN')` with `WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter)` using ClickHouse external data tables (`ext`), so that queries remain small regardless of user count.

## Prerequisites

All confirmed in `ext-tables-feasibility.md`:
- [x] `clickhouse-go/v2` v2.34.0 — includes `ext` package
- [x] Native API (`driver.Conn`) used throughout — `ext` compatible
- [x] `clickhouse.Context` preserves external tables through `QueryWithRetry`
- [x] Zero changes needed to shared `QueryWithRetry` layer

## Architecture

### Current Flow
```
ParseUserFilterForAnalytics -> FinalUsers []*userpb.User
  -> buildUsersConditionAndArgs(users, targetTables)
    -> WHERE agent_user_id IN ('id1', 'id2', ..., 'idN')   <-- PROBLEM: huge SQL
  -> QueryWithRetry(ctx, conn, chQuery, chArgs...)
```

### Target Flow
```
ParseUserFilterForAnalytics -> FinalUsers []*userpb.User
  -> buildUsersExtTable(users) -> *ext.Table
  -> ctx = clickhouse.Context(ctx, clickhouse.WithExternalTable(table))
  -> WHERE agent_user_id IN (SELECT agent_user_id FROM agent_filter)   <-- fixed-size SQL
  -> QueryWithRetry(ctx, conn, chQuery, chArgs...)
```

### Key Design Decision: Where to Apply ext

The user filter flows through `parseClickhouseFilter` -> `buildUsersConditionAndArgs` which builds the SQL condition. There are two options:

**Option A: Modify `buildUsersConditionAndArgs` to return ext table (recommended)**
- Change the function to return an ext table + SQL condition that references it
- Minimal blast radius — contained within the filter-building layer
- All 19 callers automatically benefit

**Option B: Apply ext at each call site**
- Each of the 19 API files builds the ext table and modifies the query
- Maximum code duplication

**Recommendation: Option A** — centralize in the filter-building layer.

---

## Implementation Steps

### Phase 1: Helper Functions (shared layer)

**File:** `insights-server/internal/analyticsimpl/common_clickhouse.go` (or a new `common_ext_table.go`)

#### Step 1.1: Create ext table builder

Takes `[]string` (user IDs) rather than `[]*userpb.User` — both `buildUsersConditionAndArgs` and `parseCommonConditionsForQAAttribute` already extract user IDs before this point, so `[]string` avoids duplicate parsing and keeps the helper decoupled from protobuf types.

```go
import "github.com/ClickHouse/clickhouse-go/v2/ext"

const extTableAgentFilter = "agent_filter"
const extTableAgentFilterColumn = "agent_user_id"

// buildUsersExtTable creates an external data table from a list of user IDs.
// Returns nil if the list is empty (no filtering needed).
func buildUsersExtTable(agentUserIDs []string) (*ext.Table, error) {
    if len(agentUserIDs) == 0 {
        return nil, nil
    }
    table, err := ext.NewTable(extTableAgentFilter, ext.Column(extTableAgentFilterColumn, "String"))
    if err != nil {
        return nil, fmt.Errorf("creating ext table for user filter: %w", err)
    }
    for _, id := range agentUserIDs {
        if err := table.Append(id); err != nil {
            return nil, fmt.Errorf("appending user ID to ext table: %w", err)
        }
    }
    return table, nil
}
```

#### Step 1.2: Create context attachment helper

```go
import "github.com/ClickHouse/clickhouse-go/v2"

// attachExtTablesToContext attaches external tables to a context.
// If no tables are provided, returns the original context unchanged.
func attachExtTablesToContext(ctx context.Context, tables ...*ext.Table) context.Context {
    nonNil := make([]*ext.Table, 0, len(tables))
    for _, t := range tables {
        if t != nil {
            nonNil = append(nonNil, t)
        }
    }
    if len(nonNil) == 0 {
        return ctx
    }
    return clickhouse.Context(ctx, clickhouse.WithExternalTable(nonNil...))
}
```

### Phase 2: Modify Filter Building

**File:** `insights-server/internal/analyticsimpl/common_clickhouse.go`

The current `buildUsersConditionAndArgs` generates:
```go
// Returns: ("agent_user_id IN (?, ?, ?)", [id1, id2, id3])
func buildUsersConditionAndArgs(users []*userpb.User, targetTables []string) (condAndArgs, error)
```

#### Step 2.1: Add ext-table-aware variant

Rather than modifying the existing function (which would affect the old code path), add a new function:

```go
// buildUsersConditionWithExtTable returns:
//   - A SQL condition referencing the external table: "agent_user_id IN (SELECT agent_user_id FROM agent_filter)"
//   - The ext.Table to attach to the context
//   - If users is empty, returns empty condition and nil table
func buildUsersConditionWithExtTable(users []*userpb.User, targetTables []string) (string, *ext.Table, error) {
    if len(users) == 0 {
        return "", nil, nil
    }

    table, err := buildUsersExtTable(users)
    if err != nil {
        return "", nil, err
    }

    // Build condition for each target table
    conditions := make([]string, 0, len(targetTables))
    for _, targetTable := range targetTables {
        col := userIDColumnForTable(targetTable) // e.g., "agent_user_id"
        conditions = append(conditions,
            fmt.Sprintf("%s IN (SELECT %s FROM %s)", col, extTableColumn, extTableName))
    }

    return strings.Join(conditions, " AND "), table, nil
}
```

#### Step 2.2: Integrate into `parseClickhouseFilter`

Modify `parseClickhouseFilter` to use the ext variant and return the ext table:

```go
// Updated signature -- adds *ext.Table to return values
func parseClickhouseFilter(
    attribute *analyticspb.Attribute,
    // ... existing params ...
) (filterCondAndArgs, *ext.Table, error) {
    // ...existing code...

    // Replace:
    //   usersCondAndArgs = buildUsersConditionAndArgs(attribute.Users, targetTables)
    // With:
    usersCondStr, extTable, err := buildUsersConditionWithExtTable(attribute.Users, targetTables)
    if err != nil {
        return filterCondAndArgs{}, nil, err
    }
    // usersCondAndArgs.cond = usersCondStr  (no args needed -- IDs are in ext table)

    // ...rest of existing code...
    return result, extTable, nil
}
```

### Phase 3: Thread ext Table to Query Execution

Each of the 19 ClickHouse query files calls `parseClickhouseFilter` then `QueryWithRetry`. The change is:

```go
// BEFORE (current):
filter, err := parseClickhouseFilter(req.FilterByAttribute, ...)
// ...build chQuery using filter...
rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)

// AFTER:
filter, extTable, err := parseClickhouseFilter(req.FilterByAttribute, ...)
// ...build chQuery using filter (SQL now references ext table)...
ctx = attachExtTablesToContext(ctx, extTable)
rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
```

### Phase 4: Update All 19 Callers

Each file in `insights-server/internal/analyticsimpl/` that calls `parseClickhouseFilter`:

| # | File | Function |
|---|------|----------|
| 1 | `retrieve_qa_score_stats_clickhouse.go` | readQAScoreStatsFromClickhouse |
| 2 | `retrieve_qa_conversations_clickhouse.go` | readQAConversationsFromClickhouse |
| 3 | `retrieve_conversation_stats_clickhouse.go` | readConversationStatsFromClickhouse |
| 4 | `retrieve_scorecard_stats_clickhouse.go` | readScorecardStatsFromClickhouse |
| 5 | `retrieve_agent_stats_clickhouse.go` | readAgentStatsFromClickhouse |
| 6 | `retrieve_manager_stats_clickhouse.go` | readManagerStatsFromClickhouse |
| 7 | `retrieve_adherences_clickhouse.go` | readAdherencesFromClickhouse |
| 8 | `retrieve_hint_stats_clickhouse.go` | readHintStatsFromClickhouse |
| 9 | `retrieve_assistance_stats_clickhouse.go` | readAssistanceStatsFromClickhouse |
| 10 | `retrieve_knowledge_base_stats_clickhouse.go` | readKnowledgeBaseStatsFromClickhouse |
| 11 | `retrieve_knowledge_assist_stats_clickhouse.go` | readKnowledgeAssistStatsFromClickhouse |
| 12 | `retrieve_live_assist_stats_clickhouse.go` | readLiveAssistStatsFromClickhouse |
| 13 | `retrieve_note_taking_stats_clickhouse.go` | readNoteTakingStatsFromClickhouse |
| 14 | `retrieve_guided_workflow_stats_clickhouse.go` | readGuidedWorkflowStatsFromClickhouse |
| 15 | `retrieve_metadata_values_clickhouse.go` | readMetadataValuesFromClickhouse |
| 16 | `retrieve_summarization_stats_clickhouse.go` | readSummarizationStatsFromClickhouse |
| 17 | `retrieve_suggestion_stats_clickhouse.go` | readSuggestionStatsFromClickhouse |
| 18 | `retrieve_smart_compose_stats_clickhouse.go` | readSmartComposeStatsFromClickhouse |
| 19 | `retrieve_closed_non_empty_conversations_clickhouse.go` | readClosedNonEmptyConversationsFromClickhouse |

**Change per file:** ~3 lines (accept ext table from parseClickhouseFilter, attach to context).

---

## Testing Strategy

### Unit Tests

#### 1. Test helper functions
- `buildUsersExtTable` with 0, 1, 100, 5000 users
- `attachExtTablesToContext` with nil tables, one table, multiple tables
- `buildUsersConditionWithExtTable` produces correct SQL condition

#### 2. Test parseClickhouseFilter returns ext table
- Existing tests updated to accept new return value
- Verify ext table contains correct user IDs
- Verify SQL condition references ext table name

#### 3. Test existing query builder tests
- All existing ClickHouse query tests must still pass (the SQL structure changes from `IN (?, ?)` to `IN (SELECT ... FROM agent_filter)`)
- Update golden/expected SQL files (testdata `*_request.sql` files)

### Integration Tests

#### 4. End-to-end with real ClickHouse
- Verify ext tables work through the full query pipeline
- Test with varying user counts (10, 500, 5000)
- Confirm results match the old `IN (...)` approach

### Manual Testing

#### 5. Staging validation
- Run the same 6 test cases from the `ShouldQueryAllUsers` manual testing
- Specifically test `exclude_deactivated_users=true` with a large-customer profile
- Compare query results before and after

---

## Rollout Strategy

### Option A: Feature Flag (Chosen)

Add a boolean field `enableExtTableForUserFilter` to `AnalyticsServiceImpl`, controlled by an environment variable.

**Flag definition:**

```go
// In insights-server/internal/analyticsimpl/analytics_service_impl.go
type AnalyticsServiceImpl struct {
    // ... existing fields ...
    enableExtTableForUserFilter bool  // NEW
}
```

**Environment variable:** `ENABLE_EXT_TABLE_FOR_USER_FILTER`
**Default:** `false`

**Flag wiring (in server startup / dependency injection):**

```go
enableExtTableForUserFilter: env.GetBool("ENABLE_EXT_TABLE_FOR_USER_FILTER", false),
```

**Usage in filter building:**

The flag is passed into `parseClickhouseFilter` (or a wrapper) so the decision is centralized:

```go
func parseClickhouseFilter(
    attribute *analyticspb.Attribute,
    // ... existing params ...
    useExtTable bool,  // NEW: from a.enableExtTableForUserFilter
) (filterCondAndArgs, *ext.Table, error) {
    if useExtTable && len(attribute.GetUsers()) > 0 {
        // Build ext table + SQL referencing it
        usersCondStr, extTable, err := buildUsersConditionWithExtTable(attribute.Users, targetTables)
        // ...
        return result, extTable, nil
    }
    // Legacy: use buildUsersConditionAndArgs with IN (...)
    usersCondAndArgs := buildUsersConditionAndArgs(attribute.Users, targetTables)
    return result, nil, nil
}
```

**At each of the 19 call sites:**

```go
// BEFORE:
filter, err := parseClickhouseFilter(req.FilterByAttribute, ...)

// AFTER:
filter, extTable, err := parseClickhouseFilter(req.FilterByAttribute, ..., a.enableExtTableForUserFilter)
ctx = attachExtTablesToContext(ctx, extTable)  // no-op if extTable is nil (flag disabled)
```

When the flag is disabled, `extTable` is always `nil`, `attachExtTablesToContext` is a no-op, and the SQL uses the existing `IN (...)` clause. Zero behavior change.

**Rollout sequence:**
1. Deploy with flag disabled (old behavior, zero risk)
2. Enable in staging, validate with manual test cases
3. Enable in production globally (or per-cluster if needed)
4. Monitor for 1-2 weeks
5. Remove flag and old `buildUsersConditionAndArgs` code path

### Option B: Threshold-Based

Always use ext when user count exceeds a threshold (e.g., 200). Below that, use the existing `IN (...)` approach.

**Pros:** No feature flag overhead. Smaller queries still use simple `IN (...)`.
**Cons:** Two code paths permanently. Threshold is somewhat arbitrary. Cannot fully disable ext in production if issues arise.

### Recommendation: Option A (Feature Flag)

A feature flag provides safe rollout and easy rollback, consistent with the pattern used for `enableParseUserFilterForAnalytics` in the parent project. The flag controls the entire ext table code path:
- **Flag disabled (default):** All queries use the existing `IN (...)` approach. Zero behavior change.
- **Flag enabled:** Queries with user filters use ext tables instead of `IN (...)`.

This allows staging validation and gradual production rollout with instant rollback if any issues surface (e.g., unexpected ClickHouse version incompatibility, performance regression).

---

## Estimated Effort

| Phase | Work | Estimate |
|-------|------|----------|
| Phase 1 | Helper functions | ~50 lines |
| Phase 2 | Modify filter building | ~80 lines (new function + integration) |
| Phase 3-4 | Thread through 19 callers | ~3 lines x 19 = ~57 lines |
| Testing | Unit + integration + manual | ~200 lines of tests |
| **Total** | | ~400 lines of code |

Most changes are mechanical (Phase 3-4). The core logic is in Phase 1-2.

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| ext tables have performance overhead vs IN for small lists | Low | Low | Benchmarked: ext is ~3x slower at 5 users (~4ms vs ~1ms), breaks even at ~50, and is 3x faster at 10K. See [Performance Benchmark](#performance-benchmark) below. Absolute overhead is <3ms so always-ext is acceptable. |
| ext tables not supported by ClickHouse version | Very Low | High | go-servers uses CH 23.x+ which supports external tables. Verify in staging. |
| SQL golden files need updating | Certain | Low | Mechanical update. Only affects test infra. |
| Context lost through some code path | Low | Medium | Already verified context flows correctly in feasibility study. Integration tests will catch. |

---

## Performance Benchmark

**Measured:** 2026-03-09, ClickHouse 24.2 via testcontainers, 10K seeded conversations, 5 iterations per data point.

| Filter Size | ext table avg | IN clause avg | Ratio (ext/IN) | Winner |
|-------------|-------------|-------------|----------------|--------|
| 5 users | 4.3ms | 1.2ms | 3.48x | IN clause |
| 50 users | 1.4ms | 1.3ms | 1.13x | ~Equal |
| 500 users | 1.5ms | 2.6ms | 0.57x | ext table |
| 2,000 users | 2.5ms | 3.6ms | 0.69x | ext table |
| 5,000 users | 2.4ms | 6.5ms | 0.37x | ext table |
| 10,000 users | 3.5ms | 11.6ms | 0.30x | ext table |

**Analysis:**
- ext tables have a fixed overhead (~1-2ms) for binary protocol setup, making them slower for very small lists
- IN clause cost scales linearly with user count (SQL text parsing), while ext table cost is nearly flat
- Crossover point is around **50 users**
- At 10K users, ext tables are **3.3x faster**

**Decision:** Always use ext tables when the flag is enabled. The worst-case overhead at small user counts (~3ms) is negligible in the context of full analytics query latency (typically 100ms-2s). A threshold-based approach would add code complexity for minimal gain.

---

## Key Behavioral Note (PR #26250)

When the ext table flag is on, `ApplyUserFilterFromResult` always passes `FinalUsers` — even when `ShouldQueryAllUsers=true`. This means:
- **`ShouldQueryAllUsers` is effectively superseded** by ext tables when the flag is on
- All queries filter precisely by the resolved user set from `ParseUserFilterForAnalytics`
- Results may exclude orphaned/unknown user data that was previously included when no user WHERE clause was applied — this is intentional
- For Smart Compose stats (the only API merging ClickHouse + Postgres), both paths receive the same user list — Postgres already handled large user lists in `ShouldQueryAllUsers=false` cases

## Open Questions

1. **Multiple ext tables:** Some queries filter by both `agent_user_id` and `manager_user_id` (e.g., RetrieveLiveAssistStats). Need to support multiple ext tables with different names.
2. **Metrics/logging:** Should we log when ext tables are used vs IN, for observability?
3. **Flag removal timeline:** After successful production rollout, when to remove the flag and legacy `buildUsersConditionAndArgs` code path? Suggest 2-4 weeks of stable production.
