# QM reliability and diagnosability initiative

Status: initial proposal, 2026-09-23. Scope: coaching service and analytics service. Priorities are hypotheses for discussion, not committed delivery scope. No product code or infrastructure changes.

## Outcome

Make customer-impacting failures easier to detect, explain, prevent, and recover from. Organize the initiative around customer questions: why can I not see this data; why is this number missing or different; why is this page slow; did my action actually complete?

## Initial priorities

| Priority | Customer problem | First useful capability | Evidence of success |
| --- | --- | --- | --- |
| P0 | Cannot see expected data | Explain access and selection decisions for one coaching read path and one analytics request | Oncall identifies the exclusion stage without repeatedly reconstructing the request across services |
| P0 | Missing, stale, or inconsistent analytics | Extend existing scorecard sync diagnosis with freshness, missing/stale separation, monitor coverage, and post-repair verification | Detect persistent eligible-data gaps; prove recovery in the read path |
| P0, alongside both | Regressions discovered by customers | A small deterministic end-to-end suite covering visibility and scorecard-to-report flow | Known failures reproduce in tests and fail before release |
| P1 | Intermittently slow reports or failed workflows | Service/RPC baseline plus tracing for the highest-impact customer journeys | Attribute slow requests to access resolution, application work, dependencies, or query execution |
| P1 | Alerts require manual evidence collection | Actionable routing and automated read-only diagnosis | Alerts contain scope, evidence, owner, and next step; shorter time to diagnosis |
| P2 | Repeated known failures and expensive queries | Bounded repairs and evidence-led optimization | Verified recovery, reduced customer tail latency and query cost, unchanged result semantics |

P0/P1/P2 describe investment order, not incident severity. Existing urgent service failures still require immediate handling. Validate ordering against recent support/oncall cases: frequency, customer impact, diagnosis time, recurrence, and effort. The user's report of frequent access questions is the strongest direct prioritization input; local case notes are illustrative rather than a complete incident census.

## Access diagnosis as the first focused investment

Treat “no data” as a symptom with several possible causes: denied access, narrowed user/team scope, request filters, record lifecycle or analytical eligibility, delayed projection, or genuinely absent source records.

Propose a support-facing explanation for a specific requester and request: effective scope, relevant policy/configuration, selection stages, exclusion reasons, and a correlation ID. For example: a selected team expands to a population; ACL narrows it; the date filter narrows eligible records; some records are pending projection. These are explanatory categories, not current instrumentation claims.

The explanation should use the same decision logic as the real request. Refactor only the first selected paths as needed to expose structured reasons; avoid a wholesale permission rewrite. Keep coaching capabilities and analytics eligibility distinct even if they share diagnostic vocabulary. Restrict detailed explanations to authorized operators and avoid disclosing inaccessible records to end users. Keep high-cardinality identity details in controlled traces/diagnostics rather than metric labels.

Relevant foundations: [permission evaluator draft](../../scorecard-permission-policy/README.md), [Insights user-filter semantics](../../analytics/subdomains/insights-user-filter/README.md).

## First dashboard and instrumentation

Start with a compact dashboard answering whether customers can complete key operations and trust the results:

- Service health: request volume, errors, timeouts, latency distribution, and dependency/resource saturation.
- Data health: missing versus stale eligible records, freshness lag, oldest unresolved gap, last successful monitor run, and unchecked profiles.
- Journey health: scorecard submission and subsequent read/report visibility; selected coaching read and analytics report success.
- Diagnosis links: representative traces, query IDs/fingerprints, runbooks, deploy/version context, and relevant configuration.

Use bounded metric dimensions such as service, RPC, region, and outcome; provide controlled customer drill-down without unbounded labels. Trace selected slow/error requests across access resolution, PG/ClickHouse calls, and serialization. Use CPU/heap profiling when evidence points to application resource cost. Check existing instrumentation before adding tools.

Existing [scorecard monitoring](../../scorecard-data-sync/deliverables/monitoring-and-diagnosis.md) checks missing rows and timestamp staleness but does not prove complete field/criterion correctness. Count agreement alone does not establish correctness, and an overdue monitor is unknown coverage rather than healthy data.

## End-to-end testing starts early

Begin with three deterministic scenarios using controlled fixtures and real service/storage boundaries:

1. An allowed requester sees the intended scorecard; a disallowed requester cannot see it through the relevant read/list paths.
2. Create/submit a scorecard, observe its projection within a bounded freshness window, then check its inclusion and value in an analytics response.
3. A representative team/user/date selection returns a known expected population and aggregate, including an expected empty result.

Use service/API tests for broad semantic coverage and a thin browser smoke test for a critical customer journey. Cover meaningful role/team, date-boundary, and lifecycle variants incrementally. Keep production synthetic data isolated if production checks are introduced. Separate deterministic release tests from scheduled synthetic checks and representative-scale latency benchmarks.

## Alerts, investigation, and recovery

Define ownership, expected behavior, and a runbook with each monitor. Add service-level objectives after establishing an initial baseline; do not invent universal latency or freshness limits now.

- PagerDuty: sustained urgent customer impact, a critical journey outage, or rapid exhaustion of an agreed error budget.
- Slack: actionable degradation, persistent scoped mismatch, repair outcomes, or work that can wait for business hours.
- Dashboard only: expected denials, isolated legitimate empty results, and diagnostic signals without an action.

Route by impact and urgency, deduplicate, and include recovery notifications. Do not page on every authorization denial or slow query.

First automate read-only triage: collect affected scope, relevant traces, source/projection comparison, configuration/deploy context, and candidate causes with supporting evidence. The same diagnostic entry point should support a customer ticket even when no alert fired.

Then automate a small set of known repairs with bounded scope/concurrency, repeat-safe behavior, a stop condition, and post-action validation. Use existing scorecard-repair work as a candidate foundation after checking its current state. Workflow completion alone is not proof that the customer's data is correct.

## Suggested first milestone

Pilot one access-sensitive coaching read path and one analytics report, connected to a controlled scorecard lifecycle scenario. Deliver one shared dashboard, request-level explanation, three end-to-end scenarios, one or two actionable alerts, and an attached diagnostic report. Broaden after the pilot demonstrably shortens diagnosis and catches a known regression.

Measure time to diagnosis for recurring access questions, incidents detected before customer reports, actionable-alert fraction, freshness compliance, end-to-end regression detection, and p95/p99 latency for comparable query shapes. Establish baselines before promising improvements. Assign one initiative owner and an owner/runbook for each selected journey.

Query tuning follows the first measured bottleneck: rank by customer impact and frequency, compare fast/slow executions and rows/bytes scanned, and preserve filter/aggregation semantics while validating at representative scale. Urgent, already understood latency defects need not wait for the broader monitoring rollout.

## Evidence and limits

This is inspiration grounded in existing local design and incident notes, not a fresh audit of production, instrumentation, ticket frequency, or deployment state. The historical CONVI-7681 latency investigation is an example of why tail latency and request/query correlation matter; current symptoms and resolution were not checked. Local knowledge documents may also lag deployed behavior.

Additional references: [shared analytics contracts](../../analytics/subdomains/shared-analytics-platform/README.md), [coaching oversized-response precedent](../../scorecard-template/README.md). No credentials or external services were used.
