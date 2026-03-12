# Resume Snippets (Staff-level)

Use these as polished, outcome-focused bullets (with metrics filled in when available).

## Staff Engineer — example bullets

### User Filter Consolidation
- Led a cross-service consolidation of user-filter semantics into a shared library, standardizing ACL + group expansion behavior behind feature flags; reduced correctness/security regressions across analytics endpoints and cut duplicate implementations, improving maintainability and rollout safety.

### ClickHouse External Tables for Reference Data Filtering
- Identified a systemic gap in analytics query infrastructure — no mechanism for passing reference data into ClickHouse without embedding it in SQL text — and designed a general-purpose solution using ClickHouse external tables. Evaluated 10 alternatives, benchmarked performance (3.3x faster at 10K users), and implemented with feature-flagged rollout across 17 call sites. The pattern is now reusable for any external dataset (scorecard IDs, conversation IDs) beyond the original user filter use case.

### Scorecard PG↔ClickHouse Data Consistency
- Investigated and resolved a multi-customer data inconsistency between PostgreSQL and ClickHouse caused by out-of-order async work in scorecard APIs. After the initially proposed fix (CTO-suggested timestamp approach) caused a P2 incident, built custom load testing and verification tools to prove the root cause, then designed and shipped a multi-layered fix (atomic transactions, async re-read pattern, GORM partial updates). Verified with 0 mismatches across 9,155 production scorecards. The PG→CH sync pattern is now reusable for other cross-database consistency problems.
