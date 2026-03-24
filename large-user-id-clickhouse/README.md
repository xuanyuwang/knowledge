# Large User ID Lists in ClickHouse Queries

**Created:** 2026-03-09
**Updated:** 2026-03-24

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
- **`ext` external tables**: Implemented and merged (PR #26178 + #26250). Feature flag `ENABLE_EXT_TABLE_FOR_USER_FILTER` enabled globally via app-level helmrelease (flux-deployments #264076). Awaiting releaser propagation to all prod stages.

## Documents

| Document | Description |
|----------|-------------|
| `solutions-comparison.md` | All 10 potential solutions compared side-by-side |
| `ext-tables-feasibility.md` | Deep investigation of ClickHouse `ext` package feasibility in go-servers |
| `problem-and-shouldqueryallusers-fix.md` | Original bug analysis + the `ShouldQueryAllUsers` fix (already shipped) |
| `implementation-plan.md` | Step-by-step plan for implementing `ext` tables |
| `design-review.md` | Engineering design review doc (local copy of Notion doc) |
| `testing-plan.md` | Staging/production testing plan for ext table rollout |
| `convi-6476-live-assist-broken.md` | CONVI-6476: ext table nil arg regression breaking live assist stats |

## Log History

| Date | Summary |
|------|---------|
| 2026-03-09 | Extracted project from `insights-user-filter`. Reorganized docs. Created implementation plan. |
| 2026-03-10 | Created design review doc on Notion. Addressed CodeRabbit review comments (column rename, defensive fixes). |
| 2026-03-11 | PR #26178 merged. Fixed CI lint errors. Created testing plan for staging rollout. |
| 2026-03-12 | Enabled flag on staging. Found behavior gap: ext table not used when ShouldQueryAllUsers=true. Fixed ApplyUserFilterFromResult to always pass FinalUsers when flag is on. |
| 2026-03-16 | First prod rollout: voice-prod (#264060, merged). Global rollout: moved flag to app-level 00-head (#264076), removed all per-cluster patches. |
| 2026-03-17 | Releaser didn't propagate overnight. Manually added flag to all 3 stage files (#264149). |
| 2026-03-24 | CONVI-6476: ext table nil arg regression in liveAssistStatsClickHouseQuery breaks Leaderboard live assist metrics. Fix: guard arg duplication with nil check. PR #26519 merged. |

## Related Projects

- `insights-user-filter/` — Parent project (user filter consolidation, ground truth pattern, API migration)
- `user-filter-consolidation/` — Feature flag removal and test fixes
