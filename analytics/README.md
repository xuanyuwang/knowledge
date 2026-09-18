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

- [INSI-4262](work-items/INSI-4262.md) — Bill West false-negative Active Days are confirmed for Janalie Shene Labeste and Kyle Batobato. Open go-servers PR [#32443](https://github.com/cresta/go-servers/pull/32443) restores true online-session interval overlap and directly fixes their long-session fingerprint; merge/deploy, bounded historical re-label, and verification remain. A separate Director-only green presence state is not qualifying Agent Assist activity.
- [CONVI-7467](work-items/CONVI-7467.md) — A non-default, browser-persisted Conversation Duration filter exactly reproduces Santiago Barreiro's missing Active Days cards, column, and metric option; `All minutes` restores them. Permissions and regular-source activity are present. Santiago-specific before/after confirmation remains pending, and the safe product direction is explanatory UX rather than exposing semantically unreliable duration-filtered Active Days.
- [CONVI-6719](work-items/CONVI-6719.md) — Phase 3 current-behavior validation is complete; implementation has not started. The merged API is `Parser.Parse`, and the design must explicitly preserve distinct analytics `AgentOnly` and coaching/manual-QA role-filter contracts before porting logic.
- [Heartland Behavior Hints adherence mismatch](work-items/heartland-behavior-hints-adherence-mismatch.md) — action-grain `RetrieveHintStats` fix is in review in [go-servers #30782](https://github.com/cresta/go-servers/pull/30782); deployed behavior can produce 160.4%, while the action-deduplicated hint rate is 79.1%.
- [CONVI-7254](work-items/CONVI-7254.md) — HCD customer remediation complete through CONVI-7533; the live monthly cells now render 27% for May and 22% for February. General mixed-revision aggregation semantics remain a broader product question.
- [CONVI-7533](work-items/CONVI-7533.md) — Complete. Deleted all four customer-approved weight-1 outliers after full PostgreSQL backup; PostgreSQL and ClickHouse checks are clean. PI now renders May at 27% (27.45% chart point) and February at 22% (21.73% chart point), and the tracker is Done.
- [CONVI-7238](work-items/CONVI-7238.md) — United Manager Leaderboard undercount from revision-derived N/A exclusion.
- [CONVI-7378](work-items/CONVI-7378.md) — SCAN Consent to Call: PI 0% after manual override under pinned-revision inverted option mapping (diagnosed).
- [CONVI-7384](work-items/CONVI-7384.md) — Coaching Hub weekly/monthly Performance and Scorecards-tab mismatch from different date boundaries and `scorecard_time` versus submit-time membership; instrumented confirmation pending.
- [CONVI-7385](work-items/CONVI-7385.md) — Leaderboard scorecard column/drawer voicemail-filter parity fix in draft Director PR [#21214](https://github.com/cresta/director/pull/21214); manual CNG validation remains.
- [CONVI-7402](work-items/CONVI-7402.md) — PR [#31335](https://github.com/cresta/go-servers/pull/31335) merged and fixes QA score/conversation value-or-missing filters. The Bswift `RetrieveConversationStats` regression is tracked in [CONVI-7706](work-items/CONVI-7706.md); focused go-servers PR [#32440](https://github.com/cresta/go-servers/pull/32440) implements the same OR semantics for Average Handle Time while retaining rejection for other unsupported endpoints. [Semantic review](deliverables/convi-7706-pr-32440-semantic-review.md) found no actionable correctness issue with 176 local SQL checks passing; an existing ES test failure still blocks CI, and staging validation remains.
- [CONVI-7708](work-items/CONVI-7708.md) — Tracks value-or-missing parity for the nine Assistance Insights RPCs that already support value-only/missing-only metadata filters and receive the combined filter from Director. Smart Compose and endpoints without a confirmed metadata-filter contract remain explicitly outside this scope.
- [Value-or-missing shared-caller audit](sessions/2026-09-17/codex-value-or-missing-api-audit.md) — Assistance Insights exposes the same combined metadata filter to nine additional RPCs that already support value-only/missing-only filtering, plus Smart Compose where ordinary metadata groups are currently ignored. Other shared-parser callers have no confirmed Director exposure and require contract/consumer decisions before implementation.
- [CONVI-7667](work-items/CONVI-7667.md) — The Oportun investigation exposed the same process-scorecard request-construction hazard; its draft fix in Director PR [#22772](https://github.com/cresta/director/pull/22772) is now retargeted to the canonical confirmed RCG issue, CONVI-7674.
- [CONVI-7674](work-items/CONVI-7674.md) — Paired RCG HARs pin the cross-user split to hidden persisted state, and Andra confirmed Director PR [#22772](https://github.com/cresta/director/pull/22772)'s preview loads the data. The PR merged after the Monday cutoff, so `release_director_2026-09-17-9c7412f` excludes it; deployed validation requires a later train or explicit hotfix.
- [CONVI-7681](work-items/CONVI-7681.md) — Urgent RCG Director analytics latency blocked live training; a browser capture shows four parallel `RetrieveQAScoreStats` calls spending 47-50 seconds in server wait. Root-cause comparison and mitigation are pending.

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
