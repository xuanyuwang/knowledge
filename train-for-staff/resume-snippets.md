# Resume Snippets (Staff-level)

Use these as polished, outcome-focused bullets (with metrics filled in when available).

## Staff Engineer — example bullets

### User Filter Consolidation
- Led cross-service consolidation of user-filter semantics across 30+ analytics APIs and 3 divergent implementations. Wrote an implementation-agnostic behavioral standard covering all input combinations, identified 5 silent behavioral divergences (including a production union-vs-intersection bug), and drove incremental migration from 12/29 to 29/29 endpoints. Treated migration as a tracked product with a dashboard, gated behind feature flags, and validated via shadow-mode testing (10,000+ queries, 0 mismatches).

### ClickHouse External Tables for Reference Data Filtering
- Identified a systemic gap in analytics query infrastructure — no mechanism for passing reference data into ClickHouse without embedding it in SQL text — and designed a general-purpose solution using ClickHouse external tables. Evaluated 10 alternatives, benchmarked performance (4.8x faster at 5K users, flat scaling vs linear degradation), and implemented across 19 caller sites with a 4-phase rollout (dev → shadow mode with 10,000+ query comparison → production canary → global). The "always ext" design traded ~17ms overhead for small lists for zero branching complexity. Pattern is reusable for any reference data type beyond user IDs.

### Scorecard PG↔ClickHouse Data Consistency
- Diagnosed multi-customer scorecard inconsistencies across PostgreSQL and ClickHouse, uncovering async stale-write and ORM lost-update races; designed atomic, post-commit re-read and partial-update fixes, built concurrency/cross-store verification tooling, and led a feature-flagged rollout that produced zero score or submitter mismatches across 2,996 comparable production scorecards over 39 days.

Phone-screen narrative and claims ledger: [`deliverables/phone-screen-scorecard-pg-clickhouse.md`](deliverables/phone-screen-scorecard-pg-clickhouse.md).

## Candidate bullets to refine

These need final metrics, PR links, or adoption evidence before they should be treated as final resume bullets.

### Scorecard/Template Domain Stewardship
- Built a living scorecard/template domain reference spanning Director, coaching service, Postgres, ClickHouse, and proto boundaries. Converted repeated scorecard/template ticket investigations into reusable artifacts: lifecycle docs, concept map, business-rules catalog, ticket-pattern log, and system reference. Used the reference to identify small systemic improvements, including explicit template schema versioning and sequential updater support for evolving template JSON.

### Schwab Leaderboard Metric Semantics and API Strategy
- Led cross-layer API strategy for adding submitted-scorecard leaderboard metrics and template drill-downs. Compared `RetrieveQAScoreStats`, `RetrieveQAConversations`, `RetrieveScorecardStats`, and `ListScorecards` across entity grain, filter parity, template support, attribution axis, and time basis; identified agent-vs-submitter gaps in existing QA APIs; and proposed an MVP data-provider abstraction plus a backend path with explicit submitter filtering and scorecard-submitter grouping.

### Submitted Scorecard Permission Drift
- Designed a fail-and-freeze UX contract for submitted scorecards when edit permissions change during an active session. Rejected preflight write checks in favor of authoritative write-failure handling: rollback to persisted state, freeze further edits, show an inline warning, stop autosave, and preserve existing toast behavior for unrelated failures.
