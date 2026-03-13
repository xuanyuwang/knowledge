# Consolidating User-Filter Semantics Across 30+ Analytics APIs

When an org grows, "the same logic" quietly gets re-implemented across services. User filtering is a classic example: every team needs it, edge cases accumulate, and correctness failures are hard to detect until customers notice.

This post walks through a real consolidation of user-filter logic that had drifted across three separate implementations, 30+ API endpoints, and two query engines (PostgreSQL and ClickHouse). From a staff engineer's perspective, the interesting parts aren't the code — they're the methodology of defining "correct," the bugs that only surface at scale, and the tradeoff between a clean rewrite and incremental migration.

## The problem: three implementations, subtly different

The system had user-filter logic in three places:

1. **Shared library** (`Parse`) — used by 3 coaching APIs. Clean, well-structured, with explicit options.
2. **Analytics-specific function** (`ParseForAnalytics`) — a newer consolidated function used by ~12 migrated analytics APIs. Built using older utility functions to minimize migration risk.
3. **Inline code** — the original pattern scattered across ~17 analytics API handlers, each composing the same 3-4 utility functions in slightly different ways.

All three aimed to answer the same question: *given a requesting user's access level and the filter parameters, which set of users should this API return data for?*

But they diverged in subtle, consequential ways.

## Where the semantics drifted

### Union vs. intersection

When a request includes both explicit user selections and group selections, should the result be the **union** (users OR group members) or the **intersection** (users who are also in those groups)?

The inline code treated it as intersection. The shared library treated it as union. Neither was documented as a deliberate choice — they just evolved differently.

### Empty filter with full access

When an admin makes a request with no user/group filter, the old inline code returned data for **all users** by simply omitting the `WHERE` clause. The new consolidated function instead fetched all user IDs and put them in a `WHERE user_id IN (...)` clause.

This worked fine until a customer had 5,000+ agents. The generated SQL exceeded ClickHouse's 1MB query size limit:

```
Syntax error: failed at position 1048561
Max query size exceeded
```

Same semantic intent ("all users"), completely different query strategy, invisible until a customer hit the threshold.

### ACL edge cases

Access Control List behavior introduced more divergence:
- What does "empty filter" mean when ACL restricts your view to a subset of users?
- Should a manager with no direct reports see empty results or get an error?
- When expanding a team group, do you respect role constraints (agents only) at every level?

Each implementation answered these differently, and the answers weren't documented — they were embedded in `if/else` chains and test fixtures.

## The approach: behavioral standard first, code second

### Step 1: Define what "correct" means

Before writing any code, I wrote a behavioral standard document — implementation-agnostic, describing *what* the filter should do across every combination of inputs:

- 4 access levels (root, ACL-disabled, limited with reports, limited without)
- 3 filter types (user selection, group selection, combined)
- 2 group types (team groups, virtual/dynamic groups)
- Special flags (exclude deactivated, agents only, include dev users)

For each combination: what's the expected user set? What goes in the WHERE clause? When do we early-return with empty results?

This document became the source of truth. When the three implementations disagreed, we could point to the standard and say "this is the behavior we're targeting" rather than arguing about which legacy behavior was "more correct."

### Step 2: Flag known divergences

Comparing the three implementations against the standard revealed 5 behavioral divergences. Most were silent — they produced wrong results without errors. One (the union vs. intersection mismatch) affected a production customer's analytics.

Each divergence got a severity rating, a fix plan, and a test case that would catch it.

### Step 3: Fix critical bugs immediately, migrate incrementally

Rather than a big-bang rewrite:
1. **Fix the union/intersection bug** in the analytics function — one PR, one behavioral change, gated behind the existing feature flag
2. **Add the `ShouldQueryAllUsers` flag** to prevent the ClickHouse query size explosion — deployed and verified per-customer
3. **Continue migrating** the 17 inline-code APIs to the consolidated function, one at a time

The temptation was to unify all three implementations into one clean library immediately. But the incremental approach was safer: each migration was a small, testable change, and we could verify correctness per-endpoint against the behavioral standard.

## The ClickHouse query size problem deserved its own solution

The `ShouldQueryAllUsers` flag handled the "all users" case (empty filter + full access), but three scenarios still produced large user lists:

| Scenario | Why it produces thousands of IDs |
|----------|----------------------------------|
| `exclude_deactivated_users=true` | Returns all active users (4,000 out of 5,000) |
| Large group expansion | Team with thousands of members |
| Limited-access manager with many reports | ACL returns all managed agent IDs |

For these, we needed a query strategy that scaled. We evaluated 10 options (subqueries, temporary tables, query size increase, batching, etc.) and settled on **ClickHouse external data tables** — uploading the user ID list as a temporary table alongside the query, then joining against it:

```sql
-- Instead of:
WHERE user_id IN ('id1', 'id2', ..., 'id5000')

-- External table:
WHERE user_id IN (SELECT user_id FROM ext_user_ids)
```

This approach has no query size limit, works with the existing ClickHouse Go client, and requires no schema changes. It shipped behind its own feature flag, was validated per-customer on staging, and rolled out after confirming identical query results.

## What made this "staff-level" work

### Defining the standard, not just the code

The behavioral standard document was the highest-leverage artifact. Without it, every PR review became a debate about what the "right" behavior was. With it, reviews could focus on whether the implementation matched the spec. The document also served as onboarding material when other engineers needed to understand filter semantics.

### Treating the migration as a product

The migration had a dashboard: 29 APIs total, tracking which implementation each used, which divergences each was affected by, and the migration status. This turned an amorphous "we should clean this up" into a concrete project with measurable progress (12/29 migrated at the start, tracking toward 29/29).

### Recognizing when a fix creates a new problem

The `ShouldQueryAllUsers` flag was a correct fix for the "empty filter" case, but it introduced a subtle interaction with the external tables feature: when the flag was true, the external table wasn't populated, which meant ClickHouse fell back to scanning all rows without any user filter. This was functionally correct but missed the performance benefit of external tables for customers where "all users" still meant thousands of IDs meeting role/status criteria. Catching this required testing the two features together, not in isolation.

### Choosing incremental migration over clean rewrite

The three-implementation mess was frustrating, and a clean rewrite was tempting. But the incremental approach — fix bugs in the existing consolidated function, migrate endpoints one at a time, verify each against the behavioral standard — was lower risk and produced value sooner. The clean unification can happen later, when all 29 APIs are on the consolidated function and we have comprehensive test coverage.

## The general pattern

If you're consolidating duplicated logic across a codebase:

1. **Write the behavioral standard first.** Implementation-agnostic, covering all input combinations and edge cases. This is your source of truth.
2. **Inventory the divergences.** Compare each implementation against the standard. Rate severity. Some "divergences" are actually features you need to preserve.
3. **Fix critical bugs immediately.** Don't wait for the full migration. Ship targeted fixes behind feature flags.
4. **Migrate incrementally.** One caller at a time, verified against the standard. Resist the big-bang rewrite.
5. **Watch for emergent interactions.** When you add flags and abstractions to handle edge cases, test them in combination, not just in isolation.
6. **Track progress visibly.** A migration without a dashboard is a migration that stalls at 60%.
