# Efficient Reference Data Filtering in ClickHouse Analytics Queries

**Notion:** https://www.notion.so/31f4a587b0618196a67fefc7cae0aba9
**Authors:** xuanyu@cresta.ai
**Status:** Under Review
**Last reviewed / updated:** March 9, 2026

## Goal

Establish a general-purpose mechanism for efficiently passing reference data into ClickHouse analytics queries — starting with user ID lists, but applicable to any external dataset that needs to participate in query filtering or joining.

The immediate trigger is that user ID filters exceed ClickHouse's ~1MB query size limit for large customers, but the underlying problem is broader: **our analytics query layer has no way to incorporate data that lives outside ClickHouse without embedding it in SQL text.** This design introduces `ext` external tables as that mechanism.

### Non-goals

- Changing the existing `shared/clickhouse` layer — ext tables flow through context transparently
- Modifying existing user filtering logic — this project only changes how resolved IDs reach ClickHouse
- Replacing the empty-filter optimization — we still skip user condition when query all users on purpose
- Solving the general "ClickHouse can't JOIN with external systems" problem at the infrastructure level (e.g., ClickHouse dictionaries, materialized views) — those are heavier investments for a future iteration

## Background

### The systemic problem: reference data in analytics queries

Cresta's analytics queries run on ClickHouse, which stores **event-oriented data** — conversations, scores, hints, suggestions, etc. But analytics questions almost always require **reference data** that lives outside ClickHouse:

- **User attributes** — who is active, who belongs to which group, who has which role (managed by user service)
- **Scorecard/template metadata** — which scorecards are active, which criteria map to which scores (managed by coaching service)
- **Conversation attributes** — which conversations we are interested in based on sources

This creates a fundamental tension:

1. **We can't preload reference data into ClickHouse flat tables** — user status, group membership, and org structure change frequently. Maintaining synchronized copies would require a real-time sync pipeline, adding complexity and consistency risks.
2. **Reference data must stay separated from event data** — conversations, scores, and logs are append-only event streams suited for ClickHouse. User/group/role data is mutable relational data suited for PostgreSQL. Mixing them violates data ownership boundaries.
3. **We can't always post-process in Go** — many analytics queries aggregate in ClickHouse (COUNT, AVG, percentiles, GROUP BY). Filtering must happen *inside* the query, not after. Only a subset of queries return per-agent rows that could be filtered in application code.

Today, the only way to incorporate reference data is to **embed it as literal values in SQL text** — `WHERE agent_user_id IN ('id1', 'id2', ..., 'idN')`. This works for small lists but breaks down at scale.

### Where this breaks: user ID filters

When `ParseUserFilterForAnalytics` resolves ACL + filters for large customers (thousands of agents), the generated `IN (...)` clause exceeds ClickHouse's `max_query_size` (~1MB):

```
code: 62, message: Syntax error: failed at position 1048561
Max query size exceeded
```

This blocks real-world scenarios:
- `exclude_deactivated_users=true` with 5000+ active agents
- Large group expansions across organizational hierarchies
- Any customer whose agent count pushes SQL past the limit

A prior fix (`ShouldQueryAllUsers` flag) handles the "all users" case by skipping the WHERE clause entirely. But it doesn't help when a **large specific subset** is needed — which is the common case for enterprise customers.

### Why this matters beyond user filters

The same "reference data in SQL text" limitation will surface anywhere we need to filter or join against external datasets in ClickHouse:

- Filtering by a resolved set of scorecard IDs or criteria IDs
- Filtering by conversation IDs from an external search result
- Any future analytics dimension that requires cross-system resolution

Solving this once with a general-purpose mechanism creates **leverage** — every future "pass external data into ClickHouse" problem has a proven, tested pattern to follow.

## Overview

### Solution: ClickHouse `ext` external tables

The `ext` package in `clickhouse-go/v2` sends temporary in-memory tables alongside a query via the native binary protocol. The table exists only for the query's lifetime — no DDL, no cleanup, no session management, no sync pipeline.

```
Analytics API handler
  → ParseUserFilterForAnalytics (resolves user IDs from PG)
  → ShouldQueryAllUsers?
    → Yes: No WHERE clause
    → No:  buildUsersExtTable(userIDs)
           → Attach ext table to Go context
           → SQL: IN (SELECT ... FROM agent_filter) — fixed-size query text
  → QueryWithRetry (ext table sent via binary protocol)
```

The query SQL stays small and constant. User IDs travel via binary protocol — efficient and not subject to `max_query_size`.

### Why `ext` tables over alternatives

10 solutions were evaluated. Key comparison:

| Solution | Solves large lists | Generalizable | Complexity | Verdict |
|----------|-------------------|---------------|------------|---------|
| `ShouldQueryAllUsers` flag | Only "all users" case | No | Low | Already done, partial fix |
| **`ext` external tables** | **Any size** | **Yes — any data type** | **Low** | **Chosen** |
| Increase `max_query_size` | Raises ceiling only | No | Trivial | Band-aid, doesn't fix root cause |
| Temporary tables (CREATE+INSERT) | Yes | Yes | Medium | Extra round-trips, session management |
| Query batching | Splits queries | No | Medium | Breaks aggregation queries |
| Persistent reference table | Yes | Yes | High | Over-engineered: DDL, sync, cleanup |
| Per-query `max_query_size` | Raises ceiling only | No | Trivial | Stopgap, same root cause |

`ext` tables won on four dimensions:

1. **Easy to adopt** — zero changes to shared query layer; caller-side only; already available in driver v2.34.0
2. **Generalizable** — works for any reference data (user IDs today, scorecard IDs or conversation IDs tomorrow), not just the user filter case
3. **Performance** — binary protocol encoding scales nearly flat with data size, while SQL text parsing scales linearly (see benchmarks below)
4. **Minimal blast radius** — temporary (query-lifetime only), no schema changes, no infra coordination, feature-flagged for safe rollout

## Detailed Design

### Architecture

Two helper functions centralize ext table handling in `common_clickhouse.go`:

- `buildUsersExtTable([]string) → *ext.Table` — creates an external table from user IDs; returns nil for empty input
- `attachExtTablesToContext(ctx, ...tables) → ctx` — attaches non-nil tables to context; no-op when all nil

The filter-building layer (`parseClickhouseFilter`, `buildUsersConditionAndArgs`) gains a `useExtTable bool` parameter:
- `false` (default): existing `IN (?, ?, ?)` behavior — zero change
- `true`: returns `IN (SELECT agent_user_id FROM agent_filter)` + `*ext.Table`

All 17 caller files follow a 3-line change:

```go
filter, usersExtTable, err := parseClickhouseFilter(..., a.enableExtTableForUserFilter)
ctx = attachExtTablesToContext(ctx, usersExtTable)
rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery, chArgs...)
```

### Design decisions

**Centralize in filter-building layer, not per call site.** The ext table is built inside `parseClickhouseFilter` / `buildUsersConditionAndArgs`, so all 17 callers benefit automatically. The alternative (each caller builds its own ext table) would mean 17x code duplication and 17 places to get wrong.

**`[]string` input, not `[]*userpb.User`.** The helper takes raw user IDs, decoupled from protobuf types. This keeps it reusable for non-user data in the future.

**Always-ext when flag enabled (no threshold).** Benchmarks show worst-case overhead is ~3ms for tiny lists — negligible vs 100ms-2s full query latency. A threshold-based dual path would add permanent code complexity for minimal gain.

**Feature flag, not permanent dual path.** `ENABLE_EXT_TABLE_FOR_USER_FILTER` (default false) allows staged rollout and instant rollback. After stable production, the flag and legacy code path are removed.

### API

No external API changes. Same gRPC analytics APIs, same responses. This is an internal query optimization.

### Storage

No storage changes. External tables are temporary in-memory, existing only for the query's lifetime.

### Security & Privacy

- No new services or data stores
- Same user IDs, same ClickHouse connection — just binary-encoded instead of SQL text
- Access controls unchanged

### Monitoring

- Feature flag provides instant rollback
- Existing ClickHouse query monitoring (logs, latency) applies unchanged
- Consider logging ext table usage during initial rollout for observability

### SLO

- No change to target SLO
- ClickHouse external tables are a well-established feature (available since early ClickHouse versions)
- Failure mode: disable flag → instant rollback to IN clause

### Performance

Benchmarked on ClickHouse 24.2 via testcontainers, 10K seeded conversations, 5 iterations per data point:

| Filter Size | ext table | IN clause | Ratio | Winner |
|-------------|-----------|-----------|-------|--------|
| 5 users | 4.3ms | 1.2ms | 3.5x slower | IN clause |
| 50 users | 1.4ms | 1.3ms | ~equal | ~Equal |
| 500 users | 1.5ms | 2.6ms | 1.7x faster | ext table |
| 5,000 users | 2.4ms | 6.5ms | 2.7x faster | ext table |
| 10,000 users | 3.5ms | 11.6ms | 3.3x faster | ext table |

ext tables have a fixed ~1-2ms overhead (binary protocol setup). IN clause cost scales linearly with SQL text parsing. Crossover at ~50 users. At 10K users, ext tables are **3.3x faster**.

### Testing Plans

**Unit tests** (`common_clickhouse_test.go`): 6 existing tests updated for new signatures + 4 new flag-enabled tests verifying SQL generation and ext table creation.

**Integration tests** (`ext_table_integration_test.go`) with real ClickHouse via testcontainers:
- Correctness: ext table results match IN clause results across multiple filter sizes
- Scale: 10K users with 5K filtered
- Performance: automated benchmarks at 5, 50, 500, 2K, 5K, 10K users

## Technical Debts

- Dual code path while feature flag exists — removal planned 2-4 weeks after stable production
- `buildUsersConditionAndArgs` (legacy IN-clause builder) becomes dead code after flag removal

## Cost Estimate

No new infra cost. ext tables use the existing ClickHouse connection with marginally less bandwidth (binary encoding vs SQL text for large lists).

## Release Plans & Timelines

**Phase 1: Deploy with flag disabled** (complete, ~0.5 SWE-week)
All code merged, flag defaults to `false`. Zero behavior change.

**Phase 2: Staging validation** (~0.5 SWE-week)
Enable flag in staging. Run manual test matrix: empty filter, user filter, group filter, exclude_deactivated, large customer.

**Phase 3: Production rollout** (~0.5 SWE-week)
Enable globally. Monitor for 1-2 weeks.

**Phase 4: Flag cleanup** (~0.5 SWE-week)
Remove flag, legacy code path, and `useExtTable` parameters.

**Total:** ~2 SWE-weeks including testing and rollout.

**PR:** [go-servers #26178](https://github.com/cresta/go-servers/pull/26178) (draft)

### Future leverage

Once ext tables are proven in production for user IDs, the same pattern can be applied to:
- Scorecard/criteria ID filtering (currently also embedded in SQL)
- Conversation ID sets from external search
- Any reference data that needs to participate in ClickHouse queries

The helper functions (`buildExtTable`, `attachExtTablesToContext`) are intentionally generic and can be reused directly.

## Design Review Notes

*(pending)*
