# Large User ID Lists in ClickHouse Queries

**Created:** 2026-03-09
**Updated:** 2026-03-09

## Overview

When Insights analytics APIs resolve user filters (via `ParseUserFilterForAnalytics`), they can produce thousands of user IDs that get embedded in `WHERE agent_user_id IN (...)` clauses. For large customers, this exceeds ClickHouse's default max query size (~1MB).

This project tracks the investigation, solution comparison, and implementation of a robust fix using ClickHouse `ext` (external data tables).

## Background

This work was extracted from the `insights-user-filter` project, which handled the broader user filter consolidation effort. The "too many users" problem was discovered during that work and is now tracked separately since it has its own solution path.

### Problem Trigger Scenarios

| Scenario | Why It Produces Many IDs |
|----------|--------------------------|
| `exclude_deactivated_users=true` | Returns all active users (could be 4000+ out of 5000) |
| Large group expansion | Team with thousands of members |
| Limited-access manager with many reports | ACL returns all managed agent IDs |

### Current State

- **`ShouldQueryAllUsers` flag**: Already implemented. Handles the "all users" case (root access + empty filter) by skipping the WHERE clause entirely.
- **`ext` external tables**: Investigated and confirmed feasible. **Not yet implemented.** This is the next step to handle all remaining large-subset cases.

## Documents

| Document | Description |
|----------|-------------|
| `solutions-comparison.md` | All 10 potential solutions compared side-by-side |
| `ext-tables-feasibility.md` | Deep investigation of ClickHouse `ext` package feasibility in go-servers |
| `problem-and-shouldqueryallusers-fix.md` | Original bug analysis + the `ShouldQueryAllUsers` fix (already shipped) |
| `implementation-plan.md` | Step-by-step plan for implementing `ext` tables |

## Log History

| Date | Summary |
|------|---------|
| 2026-03-09 | Extracted project from `insights-user-filter`. Reorganized docs. Created implementation plan. |

## Related Projects

- `insights-user-filter/` — Parent project (user filter consolidation, ground truth pattern, API migration)
- `user-filter-consolidation/` — Feature flag removal and test fixes
