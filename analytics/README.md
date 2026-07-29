# Analytics Domain

## Purpose

The canonical knowledge home for Performance Insights, Assistance Insights, and Leaderboard across frontend and backend. It should make every displayed chart, table, filter, and metric traceable from UI interpretation through request construction, analytics-service behavior, query/aggregation semantics, and source data.

## Scope and Boundaries

**In scope**

- Performance Insights and Leaderboard pages in Director
- Assistance Insights pages, assistance/hint metrics, and engagement semantics
- exact chart, table, score, count, ranking, and grouping semantics
- shared and page-specific filter interpretation
- analytics API request/response contracts
- insights/analytics-service handlers and aggregation/query behavior
- frontend transformations, fallbacks, and display rules
- user/team/group resolution used by these surfaces
- ClickHouse sources and data lineage needed to explain displayed values
- operational diagnosis of mismatches, missing values, and inconsistent counts

**Out of scope**

- scorecard lifecycle and review workflows: `scorecard-workflows`
- PG-to-ClickHouse scorecard projection consistency: `scorecard-data-sync`
- notification delivery: `notifications`

## Knowledge Standard

For each UI element, the domain should eventually record:

1. user-visible meaning and edge cases;
2. applicable filters and their exact interpretation;
3. frontend component and transformation path;
4. API and request fields;
5. backend handler, grouping, calculation, and query path;
6. source tables/events and time/identity attribution;
7. known discrepancies, tests, and operational checks.

## Subdomains

- [Shared Analytics Platform](subdomains/shared-analytics-platform/README.md): API cluster, group-by/request contracts, source data, and cross-page behavior
- [Performance Insights](subdomains/performance-insights/README.md): page charts, tables, filters, FE transformations, and exact display semantics
- [Assistance Insights](subdomains/assistance-insights/README.md): assistance and hint metrics, filters, engagement semantics, and surface-specific API usage
- [Leaderboard](subdomains/leaderboard/README.md): agent/team/manager tabs, ranking, grouping, and leaderboard-specific presentation
- [Active Days](subdomains/active-days/README.md): activity evidence, source filtering, label freshness, and adoption interpretation
- [Quintiles](subdomains/quintiles/README.md): partition/ranking semantics and cross-surface presentation
- [QA Score](subdomains/qa-score/README.md): `RetrieveQAScoreStats`, score/N/A/option semantics, and criterion display behavior
- [Insights User Filter](subdomains/insights-user-filter/README.md): user/team/role resolution, hierarchy, active-state, and scalable filtering
- [Conversation Volume](subdomains/conversation-volume/README.md): surface-specific conversation-count definitions and data sources

Canonical API references:

- [RetrieveHintStats](subdomains/shared-analytics-platform/retrieve-hint-stats.md) — request/response contract, Director usage, ClickHouse query semantics, grouping, invariants, validation, and known defects

## Current State

The first subdomain migration is complete. The subdomain references and [legacy source index](deliverables/legacy-source-index.md) are canonical navigation; historical ticket folders retain detailed evidence and current uncommitted work.

Work items, sessions, daily logs, and decisions remain at this parent domain. Future work items should record one optional primary subdomain.

Active investigations:

- [Heartland Behavior Hints adherence mismatch](work-items/heartland-behavior-hints-adherence-mismatch.md) — diagnosed mixed-unit `RetrieveHintStats` aggregation producing 160.4%; Performance separately uses QA criterion scores, while the action-deduplicated hint rate is 79.1%.
- [CONVI-7254](work-items/CONVI-7254.md) — HCD monthly QA 0% / 100% discrepancies from cross-revision weight domination.
- [CONVI-7238](work-items/CONVI-7238.md) — United Manager Leaderboard undercount from revision-derived N/A exclusion.
- [CONVI-7378](work-items/CONVI-7378.md) — SCAN Consent to Call: PI 0% after manual override under pinned-revision inverted option mapping (diagnosed).
- [CONVI-7384](work-items/CONVI-7384.md) — Coaching Hub weekly/monthly Performance and Scorecards-tab mismatch from different date boundaries and `scorecard_time` versus submit-time membership; instrumented confirmation pending.
- [CONVI-7385](work-items/CONVI-7385.md) — Leaderboard scorecard column/drawer voicemail-filter parity fix in draft Director PR [#21214](https://github.com/cresta/director/pull/21214); manual CNG validation remains.

Pattern index: [Mixed-revision QA score semantics](deliverables/mixed-revision-qa-score-semantics.md) unifies the three cases.

PM/manager-ready reports:

- [CONVI-7378 SCAN investigation](deliverables/convi-7378-scan-consent-qa-score-investigation.md) — initiative options and customer communication
- [CONVI-7254 HCD investigation](deliverables/hcd-mixed-revision-qa-score-investigation.md) — aggregation evidence and open decision

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `director`, `cresta-proto`, `clickhouse-schema`, `config`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `log/2026-07-15.md`
- `deliverables/legacy-source-index.md`
- `deliverables/hcd-mixed-revision-qa-score-investigation.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
