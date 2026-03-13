# Solving the Reference Data Problem in Analytics Queries: ClickHouse External Tables

When your analytics store (a columnar database optimized for aggregation) needs to filter by attributes that live in your application database (a relational store), you have a reference data problem. This post walks through a real case where that problem manifested as a 1MB query size limit, the systematic evaluation of 10 solutions, and why external tables turned out to be the right answer — not just for the immediate problem, but as a generalizable pattern for any reference data filtering.

## The architecture and the gap

The system has two databases with distinct roles:

- **PostgreSQL**: source of truth for entities and their attributes (users, roles, group memberships, activation status)
- **ClickHouse**: columnar analytics store for event data (conversations, scores, activity metrics), optimized for aggregation queries

Analytics APIs follow a common pattern: resolve which users match the filter criteria (by querying PostgreSQL for role, group, status), then query ClickHouse for event data scoped to those users. The user IDs are passed as a `WHERE user_id IN (...)` clause in the ClickHouse SQL.

This works until it doesn't.

## The symptom: 1MB query size limit

A customer with 5,000+ agents triggered this error:

```
Syntax error: failed at position 1048561
Max query size exceeded
```

ClickHouse has a default 1MB limit on query text. When "all active users" means 4,000+ UUIDs (36 characters each), the `IN (...)` clause alone exceeds 144KB. Add the rest of the query, and large orgs blow past the limit.

The initial fix was a `ShouldQueryAllUsers` flag: when the filter resolves to "everyone," skip the `WHERE` clause entirely and let ClickHouse scan all rows. This handled the most common case (admin with no filter = all users) but left three scenarios unaddressed:

| Scenario | Why it still produces thousands of IDs |
|----------|----------------------------------------|
| `exclude_deactivated=true` | Returns all active users (4,000 out of 5,000) |
| Large group expansion | Team with thousands of members |
| Limited-access manager with many reports | ACL returns all managed agent IDs |

These aren't "all users" — they're large subsets that can't use the flag but still exceed query size limits. We needed a solution that scaled for any reference data size.

## Evaluating 10 solutions

Rather than jumping to an implementation, I evaluated 10 approaches systematically. The evaluation criteria: query size independence, performance impact, complexity of changes, compatibility with the existing ClickHouse Go client, and generalizability beyond user IDs.

| # | Solution | Query size independent? | Complexity | Why accepted/rejected |
|---|----------|------------------------|------------|----------------------|
| 1 | Increase `max_query_size` | No (just raises limit) | Low | Rejected: postpones problem, doesn't solve it |
| 2 | `ShouldQueryAllUsers` flag | Only for "all users" | Low | Already shipped; doesn't help subsets |
| 3 | Batch queries + merge | Yes | High | Rejected: complex aggregation merging, changes query semantics |
| 4 | Subquery against PG | Yes | Medium | Rejected: ClickHouse can't query PG natively without external dictionaries |
| 5 | ClickHouse dictionary | Yes | High | Rejected: requires DDL changes, ongoing sync maintenance |
| 6 | Temporary tables | Yes | Medium | Rejected: requires DDL permissions, session management complexity |
| 7 | **External data tables** | **Yes** | **Low** | **Accepted**: upload data alongside query, no DDL, no schema changes |
| 8 | Materialized view sync | Yes | Very high | Rejected: full sync pipeline for a filter parameter |
| 9 | Move filter to app layer | Yes | High | Rejected: loses ClickHouse aggregation efficiency |
| 10 | Hash/compress IDs | Partially | Medium | Rejected: reduces size but doesn't eliminate the limit |

## Why external tables won

ClickHouse's [external data](https://clickhouse.com/docs/en/engines/table-engines/special/external-data) feature lets you upload a temporary in-memory table alongside a query via the binary protocol. The table exists only for that query's lifetime — no DDL, no persistent state, no cleanup.

```sql
-- Instead of:
WHERE user_id IN ('id1', 'id2', ..., 'id5000')

-- External table:
WHERE user_id IN (SELECT user_id FROM ext_user_ids)
```

The user IDs are sent as binary data in the request body, completely bypassing the query text size limit. The table is defined in the query parameters (name, columns, types) and populated from a TSV/CSV payload.

### Why this is better than "just increase the limit"

External tables solve the *class* of problems, not just the immediate instance:

- **Any reference data**: The same mechanism works for user IDs, group IDs, conversation IDs, or any future filter parameter that comes from the application database
- **Better performance**: The data arrives in typed binary format rather than being parsed from SQL text
- **No infrastructure changes**: No new tables, no sync jobs, no DDL permissions needed
- **Deterministic**: No "will this query fit?" uncertainty — it always works regardless of list size

## The implementation: centralize in the filter layer

The analytics codebase had 30+ API endpoints, each constructing ClickHouse queries. The user filter logic had recently been consolidated into a shared function (a separate project). This gave us a natural centralization point.

### Architecture decision: always use external tables

We considered a threshold approach: use `IN (...)` for small lists and external tables for large ones. We rejected this because:

1. **Branching adds complexity** — two code paths to test and maintain
2. **The crossover point is low** — benchmarks showed external tables are faster even at moderate list sizes
3. **Consistency is valuable** — one code path means one set of behaviors to reason about

### Benchmarks

We benchmarked external tables vs. inline `IN (...)` across different user counts on a staging environment:

| User count | Inline `IN (...)` | External table | Ratio |
|------------|-------------------|----------------|-------|
| 10         | 62ms              | 79ms           | 0.8x  |
| 50         | 68ms              | 69ms           | 1.0x  |
| 500        | 91ms              | 73ms           | 1.2x  |
| 1,000      | 142ms             | 76ms           | 1.9x  |
| 5,000      | 397ms             | 82ms           | 4.8x  |
| 10,000     | 250ms             | 76ms           | 3.3x  |

The crossover point is around 50 users. Below that, external tables have slightly higher overhead from the binary protocol setup. Above it, inline `IN` degrades as the SQL parser processes increasingly large text, while external tables stay flat — the binary upload scales linearly with data size but the query itself stays constant.

For our use case, most production queries involve 100+ users, making "always ext" the clear winner.

### The implementation pattern

The change was concentrated in the shared filter layer:

```go
// Before: returns a SQL fragment with user IDs inline
func BuildUserFilter(ctx context.Context, params FilterParams) (string, error) {
    userIDs := resolveUsers(params)
    if len(userIDs) == 0 {
        return "1=0", nil  // no users = no results
    }
    return fmt.Sprintf("user_id IN ('%s')", strings.Join(userIDs, "','")), nil
}

// After: returns a SQL fragment + external table data
func BuildUserFilter(ctx context.Context, params FilterParams) (string, *ExtTable, error) {
    userIDs := resolveUsers(params)
    if len(userIDs) == 0 {
        return "1=0", nil, nil
    }
    ext := NewExtTable("ext_user_ids", "user_id", "String", userIDs)
    return "user_id IN (SELECT user_id FROM ext_user_ids)", ext, nil
}
```

Each of the 19 caller sites needed a small update: accept the external table from the filter function and pass it through to the ClickHouse query execution layer. The query execution layer was updated once to attach external table data when present.

The total change: ~200 lines of new code in the shared layer, ~50 lines per caller site (mostly mechanical), and ~100 lines of test infrastructure.

## The interaction bug: `ShouldQueryAllUsers` meets external tables

During staging validation, we discovered that the existing `ShouldQueryAllUsers` flag interacted poorly with external tables.

When `ShouldQueryAllUsers` was true, the filter function returned no `WHERE` clause and no external table — correctly, since "all users" means no filtering needed. But one scenario broke this assumption:

An admin with `exclude_deactivated=true` and `ShouldQueryAllUsers=true` should get all *active* users. The flag was set (because the admin has full access and no explicit filter), but the deactivation exclusion meant we actually needed a user list. The flag short-circuited before the exclusion logic ran.

The fix was straightforward — check `ShouldQueryAllUsers` *after* applying exclusion filters, not before. But the bug illustrates a general principle: **when you add a new capability (external tables) to a system with existing optimizations (ShouldQueryAllUsers), test the combinations, not just each feature in isolation.**

## Rollout strategy

The feature rolled out behind a feature flag with four phases:

1. **Dev/staging**: Enable flag, run full API test suite, compare query results with flag on vs. off
2. **Shadow mode on staging**: Run both code paths (inline and ext tables), log when results differ, alert on mismatches — found 0 mismatches across 10,000+ queries
3. **Production canary**: Enable for one customer, monitor query latency and result correctness for one week
4. **Global rollout**: Enable everywhere, monitor for two weeks, then remove the flag and legacy inline code

The shadow-mode phase was the highest-leverage testing investment. It caught the `ShouldQueryAllUsers` interaction bug before any customer was affected.

## What made this staff-level work

### Solving for the class, not the instance

The immediate problem was "user ID lists exceed 1MB." A senior engineer might solve this by increasing the query size limit or adding the `ShouldQueryAllUsers` flag — both valid for the specific trigger. Staff-level work asks: what's the *general* problem? The general problem is that reference data from the application database needs to reach the analytics database, and embedding it in SQL text is architecturally wrong. External tables solve the general problem, making every future reference data need trivially addressable.

### Systematic evaluation over intuition

Evaluating 10 solutions before choosing one feels slow, but it's faster than implementing the wrong solution and migrating away later. The evaluation also became a design document that other engineers could review and challenge — several alternatives I initially favored (temporary tables, ClickHouse dictionaries) were eliminated through the evaluation, not through debate.

### Centralizing the fix

With 30+ API endpoints and 19 distinct caller sites, a per-endpoint fix would have been fragile and incomplete. By routing the fix through the shared filter layer, every endpoint got external table support automatically. This is the leverage of infrastructure work: one change that improves 30 endpoints.

### Designing for zero-threshold simplicity

The "always use ext tables" decision traded a small performance cost at low user counts (~17ms overhead for <50 users) for dramatically simpler code. No threshold constant to tune, no branching logic, no "which path did this query take?" debugging. In production, the p50 query has 200+ users, so the tradeoff is overwhelmingly positive. And if a future use case involves consistently small lists, the external table path still works correctly — it's just slightly slower.

## The general pattern

If your system queries an analytics store with filter parameters resolved from an application database:

1. **Don't embed reference data in SQL text.** It works until it doesn't, and the failure mode (query size exceeded) is a hard wall, not a gradual degradation.
2. **Evaluate the solution space systematically.** Document alternatives, criteria, and tradeoffs. The document itself is a design artifact that de-risks the project.
3. **Centralize the mechanism in the filter/query layer.** Don't fix individual endpoints — fix the abstraction they all use.
4. **Prefer simplicity over optimization at the boundary.** A single code path that's slightly slower for small inputs is better than two code paths that are each optimal for their range.
5. **Test feature interactions, not just features.** New capabilities interact with existing optimizations in ways that unit tests won't catch. Shadow mode (run both paths, compare results) is the most reliable validation.
6. **Solve for the class of problems.** The specific problem (user IDs) is an instance of reference data filtering. A solution that handles any reference data — user IDs, group IDs, conversation IDs — pays compound interest as the system grows.
