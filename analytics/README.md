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

## Inactive legacy consolidation — October 25, 2026

16 legacy project folders with no meaningful content on or after September 25 are now consolidated here. [Canonical summaries and preserved evidence](deliverables/inactive-legacy-project-consolidation-2026-10-25.md) retain historical findings and unfinished work. Population, identity, time and visibility must be compared separately. The July Manager submit-time decision supersedes the June waiver; label evidence and user adoption remain different questions.

## Current State

October 23 CONVI-7777 post-release check: the new backend image is running and Git production config includes chart 0.24.110, but the live ingress still lacks the route and public Assistance Usage POST returns HTTP 405/method-not-registered. Chart reconciliation remains to be verified; Director client code is present on main. [Verification](sessions/2026-10-23/codex-convi-7777-prod-api-verification.md).

The first subdomain migration is complete. The subdomain references and [legacy source index](deliverables/legacy-source-index.md) are canonical navigation; historical ticket folders retain detailed evidence and current uncommitted work.

Work items, sessions, daily logs, and decisions remain at this parent domain. Future work items should record one optional primary subdomain.

Active investigations:

- [CONVI-7874](work-items/CONVI-7874.md) — Compared new Closed Conversations dynamic-group fix #34439 with earlier Oportun PI fix #33026: same user-level ACL principle in separate backend paths, plus split-field/admin and empty-audience handling. [Comparison](deliverables/convi-7874-fix-comparison.html).

- [CONVI-7879](work-items/CONVI-7879.md) — Conversation assistance tracing follow-up to CONVI-7777 created October 22; backend and frontend implemented in separate worktrees from current main. Approved drawer shows eligible conversations, agents and qualifying categories including no-use; tests and synthetic browser verification pass, and frontend typecheck/lint/extraction checks pass. Backend [PR #34417](https://github.com/cresta/go-servers/pull/34417) open October 23; frontend remains local. No deployment.

October 22 CONVI-7777 verification: backend #33993 is merged, but optional per-conversation usage reasons remain unimplemented. The backend explicitly returns `Unimplemented` for details; the open Director PRs consume aggregate percentages only. [Current evidence and remaining work](sessions/2026-10-22/codex-convi-7777-usage-reasons-audit.md) supersede older merge-pending statements below.

- [CONVI-7777](work-items/CONVI-7777.md) — Frontend #24020 merged October 22 at 21:59 Toronto (`27191c3a85`), including review fix `22d6bacda9`; all six review threads resolved, 23 focused tests and all workspace type checks passed. Comment monitoring is paused after merge; deployment and live targeting remain unverified. The revised implementation uses a dedicated conversation assistance usage API and small feature-query functions. The [team decision rationale](deliverables/convi-7777-assistance-usage-api-decision-rationale.md) explains the existing RPC alignment gaps, rejected composition options, chosen query shape, and follow-up cleanup. The protobuf contract merged in [cresta-proto PR #10283](https://github.com/cresta/cresta-proto/pull/10283), with scoped Buf build/lint/compatibility and Bazel generated-Go compilation passing on current main. CI handles committed generated output. The approved optional usage-details contract is now in the same PR: distinct eligible conversation IDs, eligible agents and deduplicated feature-use categories from a dedicated ten-category enum, including zero-use entries, with no team names or start timestamps. Backend details remain pending. Agent rows count distinct conversations per agent; Team rows sum agents; ungrouped overall results deduplicate IDs. Population and trend buckets use conversation start time. The backend Phase I is counts only; usage details remain deferred. A parallel frontend column implementation is now locally available, as detailed below. The [Step 1 contract](deliverables/convi-7777-backend-step-one-contract.md) now follows the user-supplied shared-builder plan: complete eligibility including metadata joins, selected conversation time and aht_sec, and bundled SQL/arguments/external tables. Shared eligibility and feature-query prerequisites are ready. [#34044](https://github.com/cresta/go-servers/pull/34044) is the final refactor, rebased onto fetched main and verified with both legacy suites. [#33993](https://github.com/cresta/go-servers/pull/33993) is detached from stack #34002 and based on #34044. The complete backend Phase I counts implementation is now polished and pushed to #33993 (`9cd953dddc`): seven feature reads, overall/Agent/Team attribution and DAILY/WEEKLY buckets, with the query revision’s full Go93, focused race52 and API Bazel checks passing. Restricted population reads now force external user tables for role/activity restrictions even with the shared flag off; six focused cases and the full API Bazel suite pass. GLOBAL IN cohort filtering is replaced by eligible-agent source predicates and an ordinary join after feature-pair reduction; a two-shard synthetic fixture passed all seven families. The implementation follows the requested two-file API layout. See [distributed-query guidance](subdomains/assistance-insights/clickhouse-distributed-evidence-queries.md) for the historical concern and remaining scan/transfer tradeoff. Details explicitly return Unimplemented. Review the [new code-operation walkthrough](deliverables/convi-7777-api-implementation-review.html); #34044 CI is green; new implementation CI, merge, production benchmarks and deployment remain pending. The standalone [Director client bump #24065](https://github.com/cresta/director/pull/24065) updates current main from @cresta/web-client 2.32.47 to 2.32.59; all workspace type checks, 323 API tests and dependency checks pass locally, CI/merge pending. Current main already includes the usage RPC; the old implementation branches still need a base refresh from 2.30.40. The adjacent Agent/Team column is split into draft [preparation #24019](https://github.com/cresta/director/pull/24019) and stacked [column #24020](https://github.com/cresta/director/pull/24020), with API/query plumbing first and table/visibility/CSV integration second. The rollout flag `enableConversationAssistanceUsageMetric` is registered in merged [flux-deployments #349836](https://github.com/cresta/flux-deployments/pull/349836), default off; frontend integration is pushed to both existing PRs (`505c55f288` / `be6456af12`), with 38 focused tests, app type check, production knip and commit hooks passing; stage enablement remains separate. Preparation CI reports a Receptionist entry-bundle guard failure (+611.3 KB against 200 KB), with remaining CI and browser QA pending. See the [ticket rollout guidance](work-items/CONVI-7777.md#rollout-flag-and-frontend-integration-october-21). Fourteen focused tests, package type checks, production knip and commit hooks pass; remote CI and QA metadata remain pending. See the [October 20 frontend implementation record](sessions/2026-10-20/codex-convi-7777-frontend-column.md). Generated-client replacement and deployed integration remain pending; existing-chart trend is later UI scope. See the [construction record](deliverables/convi-7777-construction-steps.md) and [interactive report](deliverables/convi-7777-assistance-usage-construction-review.html). Prior backend/Director opt-in code is superseded; the Director prototype has been replaced with the dedicated API adapter. Customer data, runtime integration, and performance remain unverified; V1/P2 feature scope is unchanged.

- [CONVI-7715](work-items/CONVI-7715.md) — Oportun's virtual-group PI filter crosses a TEAM-only group ACL boundary for a scoped manager. A backend fix resolves name-only group types and converts dynamic-group membership to user selections. The focused parser suite passes locally. A Walter-dev staging check now returns the same two scored conversations for the fixture TEAM and dynamic group with a non-root manager session. The historical Oportun request/build and production outcome remain unverified.
- [CONVI-7711](work-items/CONVI-7711.md) — Sunbit PG/CH have identical parent populations (131 Digital Support / 99 Resolutions), and the CONVI-7674 ended-time join reproduces 0/0. Sean's failing payload or fixed-build validation remains needed for session attribution. A separate 13-row stale submission projection discrepancy is documented; no repair performed.

- [INSI-4262](work-items/INSI-4262.md) — Bill West false-negative Active Days are confirmed for Janalie Shene Labeste and Kyle Batobato. Open go-servers PR [#32443](https://github.com/cresta/go-servers/pull/32443) restores true online-session interval overlap and directly fixes their long-session fingerprint; merge/deploy, bounded historical re-label, and verification remain. A separate Director-only green presence state is not qualifying Agent Assist activity.
- [CONVI-7747](work-items/CONVI-7747.md) — Comcast Residential Acquisition voice pre-August-20 Active Days false negatives trace to UPN-to-NTID user-ID migration. A full read-only audit found stale labels on 220,472 conversations across 1,006 current agents, with 16,062 agent-days that also had a same-day source-0 conversation. Repair implementation is deferred pending Comcast's desired historical scope and authoritative mapping; the [job specification](deliverables/convi-7747-identity-backfill-job.md) is unsubmitted.
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
- [CONVI-7667](work-items/CONVI-7667.md) — The Oportun process-template case is Done in Linear. A production check on October 6 showed its original July 2025 window populated at 99% and 353 scorecards; Support's renewed blank report still needs an affected-session request if it persists.
- [CONVI-7807](work-items/CONVI-7807.md) — Separate Oportun conversation-linked FUP report is in Triage. The exact visible Support filters produced 2,684 scored cards and 60% in one production session, so the customer/session discrepancy and September 7–30 membership remain to be explained.
- [CONVI-7674](work-items/CONVI-7674.md) — Paired RCG HARs pin the cross-user split to hidden persisted state, and Andra confirmed Director PR [#22772](https://github.com/cresta/director/pull/22772)'s preview loads the data. The PR merged after the Monday cutoff, so `release_director_2026-09-17-9c7412f` excludes it; deployed validation requires a later train or explicit hotfix.
- [CONVI-7681](work-items/CONVI-7681.md) — Urgent RCG Director analytics latency blocked live training; a browser capture shows four parallel `RetrieveQAScoreStats` calls spending 47-50 seconds in server wait. Root-cause comparison and mitigation are pending.

Pattern index: [Mixed-revision QA score semantics](deliverables/mixed-revision-qa-score-semantics.md) unifies the three cases.

PM/manager-ready reports:

- [CONVI-7777 historical scan and time-semantics audit](deliverables/convi-7777-assistance-scan-audit.html) — existing APIs are bounded, all seven new evidence sources are unbounded; eight findings distinguish scan cost, timestamp/copy coverage and bucket attribution. Includes 381 synthetic SQL executions and 107 assertions; production retention and performance remain open.

- [CONVI-7777 frontend column pattern](deliverables/convi-7777-frontend-column-pattern.html) — interactive history and construction walkthrough of data, row mapping, table configuration and CSV; captured source and 14 focused tests.

- [CONVI-7777 feature refactor reviews](deliverables/convi-7777-feature-refactors.html) — index of KB Search, Chat Suggestions, Hints, Notes/Workflow, and Summary/GenAI construction reports with their preserved validation evidence.
- [CONVI-7777 assistance analytics issue register](deliverables/convi-7777-assistance-analytics-issue-register.md) — 15 consolidated defects/contract questions for PM review; AA-01 reproduced with synthetic values on a production ClickHouse connection. Customer-report prevalence remains unverified.
- [CONVI-7378 SCAN investigation](deliverables/convi-7378-scan-consent-qa-score-investigation.md) — initiative options and customer communication
- [CONVI-7254 HCD investigation](deliverables/hcd-mixed-revision-qa-score-investigation.md) — aggregation evidence and open decision

## Consolidated Historical Work

Five inactive ticket projects were consolidated on October 25, 2026. Their original files, including dated logs and scripts, are preserved under the migration session; their old top-level folders have been retired. These summaries preserve historical outcomes and open questions without claiming current ticket or deployment status.

| Work item | Primary subdomain | Last recorded state |
|---|---|---|
| [CONVI-6192](work-items/CONVI-6192.md) | Active Days | Source-exclusion proposal; rollout unverified |
| [CONVI-6242](work-items/CONVI-6242.md) | Active Days | Alaska backfill completed in February; wider rollout unverified |
| [CONVI-6260](work-items/CONVI-6260.md) | Insights User Filter | Child-team fix and review changes recorded in February |
| [CONVI-6494](work-items/CONVI-6494.md) | Leaderboard | Fixture-only partial implementation in May |
| [CONVI-6753](work-items/CONVI-6753.md) | QA Score | Compatibility fix recorded; PR and repair outcomes unverified |

See the [migration session](sessions/2026-10-25/codex-five-legacy-project-consolidation.md) for provenance and verification.

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
