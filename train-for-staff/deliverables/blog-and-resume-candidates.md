# Blog and Resume Candidates

**Created:** 2026-06-05
**Refreshed:** 2026-07-15

This is the active publication pool. Published posts are explicitly excluded so future reviews do not repeatedly recommend the same argument under a slightly different title.

## Published exclusions

Do not return these topics to the active pool unless the proposed post has a clearly different thesis and new evidence:

| Published post | Covered thesis; exclude close variants |
|---|---|
| [Who Sees What: Why Access Semantics Can't Be Left to Convention](https://www.xuanyuwang.com/blog/2026-02-28-staff-perspective-user-filter-consolidation/) | Consolidating divergent user-filter semantics across many APIs; behavioral standard before migration |
| [The Trap of Async Side Effects in Dual-Write Systems](https://www.xuanyuwang.com/blog/2026-03-13-debugging-dual-database-sync/) | PostgreSQL/ClickHouse async races, stale captures, version ordering, and verification |
| [Solving for the Class of Problems, Not the Instance](https://www.xuanyuwang.com/blog/2026-03-13-ext-tables-clickhouse-reference-data/) | ClickHouse external tables for large reference-data filters |
| [Turning Ticket Work into a Domain Reference](https://www.xuanyuwang.com/blog/2026-06-05-turning-ticket-work-into-domain-reference/) | Turning repeated ticket investigations into a durable domain reference and shared engineering leverage |
| [From Scorecard APIs to Business Workflows](https://www.xuanyuwang.com/blog/2026-06-15-from-scorecard-apis-to-business-workflows/) | Moving from artifact-centric scorecard APIs to explicit business workflows |

## Active blog candidates

### 1. The Same Count Can Be Correct Twice

Source projects:

- `analytics/subdomains/conversation-volume/README.md`
- `convi-6842-holiday-inn-pi-vs-closed-conversations/README.md`
- `agent-stats-analytics-behaviors/deliverables/conversation-count-behavior-guide-2026-07.md`

Core argument:

Two product surfaces can display different conversation counts without either being defective because they answer different business questions. Correct analytics work starts by making eligibility, transcript requirements, score applicability, time attribution, and source systems part of the metric contract.

Possible outline:

1. Start with a customer-visible mismatch.
2. Decompose “conversation count” into explicit predicates.
3. Compare Performance Insights, Closed Conversations, Customer Insights, and Leaderboard.
4. Explain why reconciliation is not always equality.
5. Build a metric contract and diagnostic matrix before changing code.

Resume angle:

- Analytics semantics, cross-surface correctness, customer diagnosis, and shared metric contracts.

### 2. When a Metric Changes Its Subject: Agent vs Submitter Semantics

Source projects:

- `convi-6968-schwab-leaderboard-launch/README.md`
- `convi-6968-schwab-leaderboard-launch/deliverables/api-decision-table.md`
- `convi-6968-schwab-leaderboard-launch/deliverables/BE plan.md`

Core argument:

A leaderboard feature looked like a frontend drawer and metric addition, but the real decision was semantic: agent-scoped filters, submitter-scoped filters, reviewer audiences, and time basis all answer different business questions. The useful staff artifact was the API decision table that separated current behavior, MVP behavior, and the future backend contract.

Possible outline:

1. The request: add submitted-scorecard counts and template breakdowns to leaderboard.
2. The trap: existing APIs have similar names but different entity grain, time basis, and attribution semantics.
3. The analysis: compare `RetrieveQAScoreStats`, `RetrieveQAConversations`, `RetrieveScorecardStats`, and `ListScorecards`.
4. The decision: use an MVP provider abstraction while planning the backend path toward explicit agent and submitter axes.
5. The lesson: metric work needs a semantic contract before code reuse.

Resume angle:

- API strategy, analytics correctness, cross-layer product semantics, future-proofing with provider abstraction.

### 3. N/A Is a Data Contract, Not a Display Label

Source projects:

- `scorecard-workflows/subdomains/evaluation-and-scoring/README.md`
- `analytics/subdomains/qa-score/README.md`
- `nascore/README.md`

Core argument:

Null, N/A, zero, excluded, and missing encode different business states. Once an evaluation crosses template JSON, form state, scoring helpers, PostgreSQL, ClickHouse, analytics aggregation, and UI display, collapsing any two states produces plausible but incorrect results. The staff move is to define an end-to-end value contract rather than patch each representation independently.

Possible outline:

1. Distinguish option identity, raw value, mapped score, percentage, weight, and label.
2. Show why N/A and zero require different algebra.
3. Preserve meaning across storage and analytics projections.
4. Test transition and aggregation behavior, not only individual fields.
5. Use a representation matrix as the shared FE/BE/data contract.

Resume angle:

- Semantic correctness across FE/BE/data layers, invariant design, and prevention of plausible-but-wrong analytics.

### 4. Backfills Are Product Operations, Not Database Scripts

Source projects:

- `scorecard-workflows/subdomains/process-scorecards-and-generation/README.md`
- `scorecard-data-sync/README.md`
- `auto-backfill-missing-scorecards/README.md`

Core argument:

A historical repair can execute product logic, create customer-visible annotations, trigger scorecard generation, and alter analytics—not merely copy rows. Safe backfill design therefore needs eligibility rules, dependency ordering, visibility analysis, idempotency, throttling, observability, and an explicit choice between one-time and recurring execution.

Possible outline:

1. Classify generation, projection, and query defects before repairing anything.
2. Model upstream dependencies such as Opera annotations.
3. Treat feature flags and customer visibility as separate controls.
4. Prefer a finite downstream job over a permanent cron for one-time repair.
5. Define idempotency, throttling, monitoring, and rollback as product behavior.

Resume angle:

- Cross-domain architecture, operational safety, customer visibility, and repair strategy.

### 5. Export Parity Is a Product Contract

Source projects:

- `scorecard-workflows/subdomains/group-calibration/README.md`
- `scorecard-template/deliverables/scorecard-export-paths.md`
- `group-calibration/deliverables/convi-7208-numeric-grade-csv-fix-plan.md`

Core argument:

When multiple product surfaces export the same business object through separate frontend and backend implementations, schema and display semantics drift silently. Treating export parity as a product contract—rather than copying formatting code—creates a repeatable way to reason about labels, comments, empty values, ordering, permissions, and historical revisions.

Possible outline:

1. Discover duplicated export paths.
2. Separate transport shape from user-visible representation.
3. Define parity dimensions and intentional differences.
4. Use a canonical value resolver instead of parallel ad hoc mappings.
5. Test exports as stable external contracts.

Resume angle:

- Cross-surface contract design, semantic consistency, and prevention of customer-facing data drift.

## Resume candidates to polish next

These are not final bullets yet. They are high-signal raw material to refine once metrics, PRs, and adoption outcomes are known.

### Scorecard/template domain stewardship

- Built a living scorecard/template domain reference covering lifecycle, business rules, concept map, ticket-pattern log, and system sharp edges across Director, coaching service, Postgres, ClickHouse, and proto boundaries. Reframed repeated scorecard/template bugs from isolated fixes into a domain stewardship problem, creating reusable artifacts that reduce repeated investigation and surface small systemic improvements such as explicit template schema versioning.

### Leaderboard metric semantics and API strategy

- Led API and product-semantics analysis for adding submitted-scorecard leaderboard metrics and template drill-downs. Compared four candidate APIs across entity grain, filter parity, template support, attribution basis, and time semantics; identified agent-vs-submitter gaps in existing QA APIs; and proposed an MVP provider abstraction plus a backend path using explicit submitter filtering and `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER`.

### Submitted scorecard permission drift

- Designed a fail-and-freeze UX contract for submitted scorecards when edit permissions change during an active session. Rejected preflight write checks in favor of authoritative write failure handling: rollback to persisted state, freeze further edits, show an inline warning, and preserve existing behavior for unrelated failures.
