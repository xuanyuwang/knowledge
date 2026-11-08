# Cross-use-case scorecards: Performance Insights and QM Report change inventory

Investigated November 7, 2026 (America/Toronto). Primary domain: scorecard-workflows; primary source: go-servers. Related reporting domain: analytics. This is a source-backed implementation inventory and ticket breakdown, not a product change or production validation.

## Result

A multi-select alone cannot provide correct reporting. Current code can widen a supplied template-filter set, overwrite a supplied report/export set with one global use case, ignore malformed IDs until the SQL predicate disappears, and omit the selected set from QM and Appeals fact queries. Template discovery also filters scope before selecting current revisions and includes scope in DISTINCT identity. Historical facts need a separate discovery/read contract so a template scope edit does not erase old results.

Use the [full engineering plan](omni-usecase-scorecard-eng-plan-draft.md) for the overall design and the [November 4 decisions](../decisions/2026-11-04-omni-usecase-autoqm-and-reporting.md) for settled product choices. The older P0 sketch is a lead, not current implementation proof. The present inventory adds request-path evidence and newly identified defensive gaps.

## Source and live evidence

| Repo | Checkout inspected (main) | Fresh origin/main checked |
| --- | --- | --- |
| Director | e520b8985e5a124ead25a69c79cae3903b639f33 | a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae |
| go-servers | 35c364ff698e86e4d8eef77f4504eb85dc76c5cc | fc3fa4fa48de29914c8357892f30f23a6c005533 |
| cresta-proto | a415f145cf366c1f5dc9750fd61969130ed1049d | 52b428ad53b9ffe6cb5329c2739e807eb5fa00df |

Relevant backend files were compared with fetched main. QM/manual-QA/Appeals/shared-template/task-reader query findings below are unchanged there. Newer main changes role-check plumbing in QA comments, agent-only scorecard-status checks, and GC stats; it also adds a metadata-only template list path. Both full and metadata-only current-template queries retain the scope/latest-selection problem. Director reporting paths inspected are unchanged between these two refs. Relevant QA proto grouping fields are unchanged. Do not infer the deployed backend version from these source refs.

Live page: https://walter-dev.voice-staging.cresta.ai/director/insights/performance/conversations . Observed Chat selected, All scorecards tab, Template All, Last 30 days / Weekly, criteria-adherence calculation, real populated charts. Reloaded once and inspected the page's observed resource inventory. Frontend asset path reports `main-5bf21c0`, so live code is not asserted to match either local Director ref.

Observed API resource URLs (under `/v1/customers/cresta/profiles/walter-dev/`):

- `scorecardTemplates:current?includeDeactivated=true` without use-case query; source has a permissions-catalog lookup defaulting to `usecaseNames=[]`.
- `scorecardTemplates:current?includeDeactivated=true&usecaseNames=customers%2Fcresta%2Fprofiles%2Fwalter-dev%2Fusecases%2Fchat`, plus a variant with conversation/process/user-outcome type filters.
- `qaScoreStats:retrieve`, `conversationStats:retrieve`, `filterSets:retrieve`, `conversationDurationBuckets`.
- Metadata/outcome `moments` and `policies` with explicit Chat scope, and separate unfiltered metadata/outcome catalogs. Ancillary catalog reads must be classified by purpose rather than automatically equated with the population of a score query.

The supported browser resource inventory exposes request URLs, not POST bodies, statuses, responses, or multiplicity. Native DevTools interaction could not reliably control its filter and later failed with no available window. Thus actual POST use-case bodies, SQL, ClickHouse query IDs, shared-template results, and unauthorized-user behavior were **not** observed. Payload findings below come from code. No HAR, tokens, response records, or customer data were exported. No new AWS/Okta/Azure/SSH credential was selected or read; browser session and self-authenticating git fetch were used. No customer DB was queried.

## Defensive contract to settle before implementation

Let S be the explicit local selection, A the server-authorized use-case set in the parent customer/profile, and E the effective fact-query set. Explicit valid authorized S must survive all builders unchanged as a set: E=S. De-duplication/canonical ordering is safe; replacing S with the global singleton, all configured use cases, or an unfiltered query is not. A missing selection while configuration loads must not issue an unrestricted query.

The user was asked whether explicit partly unauthorized requests should fail or return an openly displayed intersection. That choice remains pending; no implementation should guess it. Also identify the owning server authority for A: the inspected ACL helper describes users/groups, and method proto annotations describe roles/features, neither by itself proves per-use-case authorization. Frontend `getAvailableUsecases` filters config by profile; it is not proof of entitlement. Do not add a new permission model by inference.

Resolve omitted, explicit empty, explicit ALL, hidden/inactive/removed IDs, malformed IDs, and foreign-customer/profile IDs separately. Current proto repeated fields often collapse omitted and empty. A caller-side empty authorized intersection must stop or return empty before existing optional SQL predicates; it must never become the legacy unfiltered sentinel. Preserve compatibility by documenting old callers before changing default request semantics. Template-wide full-definition permissions remain a separate check; a template overlapping S can still contain criteria for other use cases.

Definitions and facts use different predicates:

- Current applicability: select one current revision by customer/profile/template identity **first**, then test overlap with E. Return the canonical full definition and explicit applicability; do not destructively slice it.
- Historical reporting: discover templates/criteria from eligible historical scorecards/revisions and filter facts by their stored use case and pinned revision. Today's template scope or criterion list must not exclude old facts.
- Future QM task discovery: follows its linked current template per the existing task decision. Historical task/scorecard associations remain readable even after the current template removes a use case.
- GC task scope stays its own use-case identity and pinned revision; do not apply the future-QM-task inheritance rule to GC.

## Performance Insights first: caller → API → query

Evidence IDs below link to immutable source lines in the ledger.

| ID / surface | Current request path and query | Required change / ticket group |
| --- | --- | --- |
| PI-01 Route and selection | `Performance.tsx` wraps conversations in `HasSingleUseCase`; header selector uses `selectLimit:1`. Performance filter state has no local selected-use-case set. [D1](#d1)[D2](#d2) | Add local multi-select initialized from global selection, loading/empty behavior, and effective selection display; revise route restriction. PI frontend. |
| PI-02 Template picker | `useScorecardTemplatesLevelSelect`: `usecaseNames || params?.allUsecases ? availableUseCases.map(...) : undefined`. A truthy caller array selects **all available** names, not its contents. Hook fallback uses current global names when undefined. [D3](#d3)[D4](#d4) | Preserve exact supplied set; distinguish ALL from empty/loading. Test [A], [A,B], [] and undefined. Defensive shared frontend + PI frontend. |
| PI-03 Permissions catalog | `useGetScorecardTemplatePermissions` deliberately defaults to an empty list; `useCurrentScorecardTemplates` sends it without a SQL use-case restriction. [D5](#d5) | Keep catalog purpose explicit; enforce definition access server-side. Decide whether lightweight metadata view can serve permission needs; do not silently reuse it as filtered picker population. Definition ticket. |
| PI-04 Current definitions | `ListCurrentScorecardTemplates` filters usecase overlap inside the subquery; DISTINCT includes usecase_ids. Main adds metadata-only path with equivalent selection. Shared `qa.ListCurrentScorecardTemplateIDs` repeats this pattern and collapses multiple scope rows into a map. [G1](#g1)[G2](#g2) | Latest-by-identity before applicability; same contract across full, metadata and shared readers, status handling and deterministic tie break. Separate historical discovery. Definition ticket. |
| PI-05 Canonical detail | `PerformanceConversations` → `useScorecardTemplate`; criteria/trigger display comes from a template read, not score fact grouping. [D6](#d6) | Render current applicability from canonical revision; keep historical revision labels/discovery available when scope changes. Do not filter scored history by current criterion scope. PI frontend + definitions. |
| PI-06 Score cards/chart/delta | `StatsGraphContainer` family / `useGetQAStats` → `useQAScoreStatsRequestParams` → `useQAFilterByAttribute` → `getQAFilterByAttribute`; reads global singleton or undefined for superset, then additional attributes spread last. API wrapper adds parent/timezone and forwards attributes. [D7](#d7)[D8](#d8)[D9](#d9) | Explicit local E on every combined/manual/auto/comparison/prior-range request; prevent overrides from accidentally replacing E. Existing keys contain request params, so propagate E into the params rather than adding an unrelated cache-only field. |
| PI-07 Criteria/template table | `LeaderboardByScorecardTemplateItem` → QA score stats grouped by criterion; CSV uses table rows. [D10](#d10) | Current applicability table plus combined/breakdown scores as required by plan; key criterion by template+criterion+use case where grouped. Scope applies to CSV source rows too. |
| PI-08 Progression table | `PerformanceProgression` has criterion/time, whole-template and per-agent requests; also top-agent, quintile and AI-agent variants. [D11](#d11) | Apply E consistently to every request/row key and baseline; add use-case breakdown; preserve weighting and existing scorecard/criterion modes. |
| PI-09 Per-criterion leaderboard | `LeaderboardPerCriterion` → agent/criterion stats plus quintile request. [D12](#d12) | Propagate E to ranking and details; separate applicability N/A from scored N/A and absent data. |
| PI-10 Volume and handle time | `ConversationCountChart` → `useQAScoreStatsRequestParams` → `RetrieveQAScoreStats` for filtered, unfiltered and delta counts. AHT `AverageHandleTimeInsightsMetric/ChartGraph` → `useInsightsRequestParams` → `useFilterByAttributes` → `RetrieveConversationStats`; its attribute hook falls back to the global singleton unless `insightsAttribute.usecaseNames` is supplied. Both APIs reach their respective ClickHouse builders/common conditions. [D13](#d13)[D14](#d14)[G3](#g3) | Set E explicitly on eligible/filtered volume, AHT, comparison and single-agent baselines; preserve each metric's template-filter semantics. These APIs already accept lists; no new singleton API needed. |
| PI-11 QA examples/pagination | Drawer → `useRetrieveQAConversationsRequestParams` → same QA attribute builder → `RetrieveQAConversations`. API derives scorable IDs from **current** templates when absent. ClickHouse uses common/score/scorecard/conversation/moment conditions. [D15](#d15)[G4](#g4) | Carry E and clicked breakdown use case across all pages; explicit clicked-use-case narrowing must be visible/intentional. Historical criteria must not be lost through current-template defaults. |
| PI-12 Example enrichment | `useScorecardsForQAConversations` lists explicit returned scorecard IDs in chunks; no usecase field. [D16](#d16) | Exact IDs are an intentionally narrower follow-up, not unrestricted fact discovery. Preserve IDs, per-resource access checks and pinned definitions; validate returned facts against selected context, including stale cached results. |
| PI-13 Fact API validation | `parseUsercaseNames` extracts IDs without comparing resource parent; common ClickHouse QA use-case predicate is optional when resulting list empty. Handler user/reviewer ACLs are separate. [G5](#g5)[G6](#g6) | Validate every requested name against parent, existence and defined A before query building. No malformed/foreign names silently reduced to a valid local ID. Resolve explicit empty/ALL before optional predicates. Defensive backend. |
| PI-14 Score query ordinary path | QA stats uses common conditions in scorecard/score query, max scorecard update, projected DISTINCT score rows and join on ID + scorecard_last_update_time. Current filtered_scorecard projection lacks usecase_id. [G7](#g7) | Add QA use-case group enum/response field/scanner; carry stored fact usecase through dedup/latest CTE and GROUP BY. Preserve existing version/time-range/conversation handling and template+criterion identity. |
| PI-15 Metadata query path | `qaScoreStatsClickhouseQueryWithMetadataView` builds separate annotation joins and aggregation; same handler selects it when moment filters apply. [G7](#g7) | Implement same grouping/filter contract in ordinary, metadata/outcome/MV, inclusion/exclusion and value-or-missing paths; test parity. Do not assume changing one SELECT covers both. |
| PI-16 Current criteria/config defaults | `getScoreableCriteria`, lower-is-better resolution, respect-template-QA-config stats and examples read current templates scoped to selected use cases. [G8](#g8)[G4](#g4) | Audit latest config behavior versus historical scoring semantics explicitly. Historical removed criterion must survive default selection; distinguish current reporting configuration (e.g. duration filters) from revision-owned scoring rules. Avoid a fix that merely corrects DISTINCT but still hides old facts. |
| PI-17 Export family | Chart CSV is client-side data; table CSV uses rows; progression ZIP builds multiple child datasets. [D10](#d10)[D11](#d11) | One captured E throughout export, meaningful use-case columns/labels, no cross-use-case row-key collisions, combined/breakdown numerator-denominator reconciliation. Export ticket; avoid duplicating CONVI-7941's async ZIP work. |

## QM Report: all three tabs and drilldowns

| ID / surface | Current caller/API/query | Required change / ticket group |
| --- | --- | --- |
| QM-01 Shared page/filter | `QAReport` → `useQAReportFilters`; `useQAReportFiltersToInsightsAttribute` copies template/task/audience but no selected-use-case field. [D17](#d17) | Add one local E across tabs, URL/task-cycle hard loads, persisted filters, refreshes, cache params and exports. Establish this shared state in QM Evaluations ticket so Appeals/GC/export work consumes it. |
| QM-02 Template/task discovery | `QAReportTabs` loads ACTIVE and ARCHIVED QM tasks and current templates with `[currentUsecase.name]`, even though stats use a separate hook. [D18](#d18) | Same E for lists and stats; current-template scope cannot be sole historical row identity source. Keep archived/no-task/hard-load identities discoverable. |
| QM-03 Task list + period helpers | `ListDirectorTasks` base query filters `tasks.usecase_id`; `getQMTasks`, `qa.GetTaskByID` and `ListEvaluationPeriodsForTask` likewise. First-use-case backfill persists singular task scope. [G9](#g9)[G10](#g10)[G11](#g11) | Derive future QM task scope from linked latest template before overlap; enforce valid same-profile link and audit legacy missing/stale links. Historical identity reads must not disappear after scope shrink. Keep GC distinct. Benchmark query before adding a projection. |
| QM-04 Evaluations overview/chart/cycles | `useQMTaskStats` → `useRetrieveDirectorTaskStats` → public `RetrieveDirectorTaskStats` routes QM to `RetrieveQMTaskStats`. Hook overwrites request set from global. Backend parser silently ignores invalid names. [D19](#d19)[G12](#g12) | Explicit E, strict validation; keep template×task×cycle combined grain. No quota multiplication per selected use case. |
| QM-05 Cycle facts | `fetchScorecardsForPeriod` has no selected-use-case input/predicate. [G12](#g12) | Pass E into every call, filter stored `scorecards.usecase_id` before aggregation; expose per-use-case evaluated breakdown in drilldown. |
| QM-06 Overall chart facts | `fetchScorecardsCountGroupedByFrequency` also has no use-case predicate; no-task/time-range helper already filters it conditionally. [G12](#g12) | Apply E consistently in all branches, including export activity fields and partial cycles. Compare [A], [B], [A,B] with fixture containing shared task. |
| QM-07 Manual stats drilldown | `useRetrieveManualQAStats` overwrites incoming set with global singleton/undefined. Backend `RetrieveManualQAStats` count/detail/overwrite queries already use optional `scorecards.usecase_id IN ?`; template and task-period resolution remain scope-sensitive and silently skip malformed names in several loops. [D20](#d20)[G13](#g13) | Stop overwrite; validate set once; propagate it into reviewer/period/agent/group/criteria/overwritten/conversation branches. Preserve pinned revision lookup. |
| QM-08 Completion | Existing task quotas are task-wide; filtering numerator by E can create misleading completion. [November 4 decision](../decisions/2026-11-04-omni-usecase-autoqm-and-reporting.md). | Show completion N/A when selection covers only part of effective task scope, in UI and exports; never invent per-use-case quota. Full task scope uses existing quota semantics, including coverage-percentage tasks. |
| QM-09 Appeals cards/trends/leaderboard/table | `useRetrieveAppealStats` overwrites supplied set. `Validate` demands exactly one name, but `GetRetrieveAppealStatsConditions` does not retain it and `retrieveAppealScorecards` has no use-case predicate. [D21](#d21)[G14](#g14) | Support valid selected set; carry it into appeal request/original/resolved fact population and all aggregate/detail branches. Removing singleton validation alone would leave a correctness bug. Preserve workflow/user ACLs. |
| QM-10 Appeals enrichment | `AppealsTable` → list scorecards for returned conversation names, optionally current template; no usecase predicate. CSV maps these rows. [D22](#d22) | Guard exact returned scorecard identities or apply E to expanded list; a conversation-name expansion must not import other template/use-case scorecards. Keep row/template revisions and resolution mapping intact. |
| QM-11 GC overview/top reviewers/leaderboard | `useQMReportGroupCalibrationStats` → generic stats hook → `RetrieveDirectorTaskStats` GC branch → `buildGroupCalibrationStatsConditions` → `findTasks`; task SQL always uses `tasks.usecase_id IN E`. [D23](#d23)[G15](#g15) | Supply normalized E consistently (empty currently differs from other APIs); validate resource parent. Retain task's own scope and task/template pinned revision. |
| QM-12 GC score facts | `findScores` filters task IDs and customer/profile then joins scorecards/scores and templates using pinned template_revision; no direct cards.usecase predicate. [G15](#g15) | Either enforce and test task/card use-case invariant at writes and reads, or apply E to facts as well. Task-ID filtering alone is not proof for legacy inconsistent records. Preserve answer-key/response identity and comparable-criterion denominators with scoped criteria. |
| QM-13 GC recent sessions | `useRecentCalibrationsTableData` lists tasks and all scorecards using global singleton. Session filters and leaderboard drawer repeat it. [D24](#d24) | Same E for recent sessions, task lookup, response reads, snapshot and drawers; keep exact-ID child requests scoped to their parent result. |
| QM-14 GC snapshot | `SessionSnapshotChart` → stats grouped by criterion; `useScorecardTemplate` plus `useListAllScorecards({directorTaskNames})` feeds labels/answers. [D25](#d25) | Resolve each session's pinned revision rather than today's template; distinguish inapplicable criterion and missing response. Include context E in reads and derived rows. |

## Exports: separate fetching from serialization

| ID / output | Current source path | Required change |
| --- | --- | --- |
| EX-01 QM task-cycle CSV | `QMReportTaskCycles` calls `useFetchQMTaskStatsForExport({includeExportStats:true})`; fetcher overwrites scope from global. [D19](#d19)[D26](#d26) | Capture selected E at click, carry it into the heavier query and every activity count; shared template×task×cycle row stays combined. Include use-case breakdown without duplicating task-wide quotas; filtered completion N/A. |
| EX-02 Reviewer/trend CSV | `LeaderboardByReviewerExport` and `LeaderboardHeader` serialize loaded rows. [D27](#d27) | Build from responses for same E/time/task/reviewer selection; clear stale rows on selection changes and label selected scope. |
| EX-03 Full scorecard export | Reviewer export → `useExportScorecard` overwrites request with global; `ExportScorecards` parses IDs and builds SQL with optional `usecase_id IN E`. [D28](#d28)[G16](#g16) | Preserve supplied E in hook and validate parent/authority server-side. Use same E in data and linked metadata queries. Do not classify a valid list-aware SQL predicate as missing; the current hook and validation are the gap. |
| EX-04 Revision/config export | `getLinkedTemplatesForExport` joins exact template_revision; `anyCurrentTemplateRequiresPublish` uses current template scope/DISTINCT. [G16](#g16) | Preserve correct historical join/criterion union. Correct current permission/config discovery without replacing historical definition authority; keep existing publish/submitted-column rules. |
| EX-05 Detailed 1:1 calibration CSV | `useOneOnOneCalibrationExport` starts from ManualQA conversation infos then fetches explicit scorecard names with calibration variants and pinned templates. [D29](#d29) | Input infos must come from E. Validate exact-ID expansion and multi-revision header/absence semantics. Existing CONVI-7795 owns baseline export; this ticket only adds cross-use-case compatibility. |
| EX-06 Appeals CSV | Appeals table and leaderboard export client-side rows from appeal stats and scorecard enrichment. [D22](#d22) | Fix source population, then preserve selected use case, criterion/template identity, historical columns and adjusted/unadjusted status. |
| EX-07 GC CSV | GC leaderboard exports rows locally; snapshot/recent-session detail use lists above. [D24](#d24) | E follows input requests and row identity; do not merge criteria across incompatible revisions or count inapplicable criteria as zero. |
| EX-08 PI CSV/ZIP | Chart/criterion/progression exports use chart/table data, including lazy child datasets. [D10](#d10)[D11](#d11) | E must be identical across every child fetch and serialization; test changing selection during export. |

## Tickets and dependencies

The nine proposed tickets are saved only as a [local draft list](cross-usecase-pi-qm-ticket-drafts.md), with complete descriptions, acceptance criteria, source links and dependencies. The Linear issues created earlier were deleted at the user's request. No implementation ticket remains active from this investigation.

| Draft | Proposed work | Depends on | Inventory coverage |
| --- | --- | --- | --- |
| [DEF](cross-usecase-pi-qm-ticket-drafts.md#def) | Preserve and validate the exact reporting use-case selection | Contract decision first | PI-02, PI-13; shared contract for all PI/QM/export readers |
| [DEFN](cross-usecase-pi-qm-ticket-drafts.md#defn) | Separate current template applicability from historical reporting definitions | DEF | PI-03/04/05/16; QM-02; EX-04 |
| [PI-BE](cross-usecase-pi-qm-ticket-drafts.md#pi-be) | Add use-case breakdowns and preserve historical facts in PI APIs | DEF, DEFN | PI-11, PI-14/15/16; QA stats/examples and conversation stats contract |
| [TASK](cross-usecase-pi-qm-ticket-drafts.md#task) | Discover QM tasks through linked template scope without hiding history | DEF, DEFN | QM-02/03; future task scope and historical identity |
| [PI-FE](cross-usecase-pi-qm-ticket-drafts.md#pi-fe) | Apply local multi-use-case selection across Performance Insights | DEF, DEFN, PI-BE | PI-01/02/05–12/17 |
| [QM](cross-usecase-pi-qm-ticket-drafts.md#qm) | Scope QM Evaluations facts and share selection across report tabs | DEF, DEFN, TASK | QM-01/02/04–08; EX-01 sources |
| [APPEAL](cross-usecase-pi-qm-ticket-drafts.md#appeal) | Filter Appeals aggregates, details and enrichment by selected use cases | DEF, DEFN, QM | QM-09/10; EX-06 source population |
| [GC](cross-usecase-pi-qm-ticket-drafts.md#gc) | Keep Group Calibration report requests and pinned criteria consistent | DEF, DEFN, QM | QM-11–14; EX-07 sources |
| [EXPORT](cross-usecase-pi-qm-ticket-drafts.md#export) | Keep PI and QM exports aligned with selected scope and historical revisions | DEF, DEFN, PI-FE, PI-BE, QM, APPEAL, GC | EX-01–08; PI-17 |

Dependencies: DEFN and every fact reader depend on the DEF contract; TASK depends on DEFN; PI-FE depends on DEFN + PI-BE; QM depends on TASK + DEFN; APPEAL/GC consume QM's shared selection and DEF; EXPORT consumes all applicable source fixes. Existing V3/scoped-criterion serialization/read compatibility from the full plan is a release prerequisite, not implemented here. Existing CONVI-7981 (Done in Linear at lookup) adds nonempty scope validation and is not duplicated; fetched main contains it at `transformers.go:411`. Legacy NULL/empty read behavior and explicit `all-usecases` must be reconciled with it, not newly written as empty ALL.

Duplicate search before the subsequently reverted creation: project `Omni-use case support for QM and Coaching` returns CONVI-7791 (Done, plan) and CONVI-7928 (In Progress, plan/kickoff). Team UUID searches for cross, usecase, defensive and QM Report were checked; no matching cross-use-case implementation children found. Similar historical usecase bugs and completed QM/export features are context, not duplicated work. CONVI-7795 baseline 1:1 export, CONVI-6392 export activity and CONVI-7941 PI ZIP behavior remain separate owners.

## Acceptance fixture shared by tickets

Use cases A/B/C with authorization for A/B; shared template T r1 scoped A+B, r2 scoped A; criterion shared X and B-only Y in r1, removed Y/added Z in r2; two templates reusing a criterion ID; manual/auto/all-N/A/missing facts; shared QM task, archived task, no-task scorecard, GC tasks with pinned r1 and distinct task scope. Include malformed/foreign profile names and a permission change during a session.

Check every supported request and SQL path with [A], [B], [A,B], reordered/duplicate names, explicit empty, omitted, ALL, and partly unauthorized selection using the agreed contract. Reconcile weighted numerator and denominator (not averages of percentages), fact counts at their real grain, non-additive distinct conversation counts, combined vs use-case breakdown, metadata path vs ordinary path, UI vs paginated examples vs all exports. Scope edits must not erase historical B scorecards. Empty effective selection must never remove an SQL predicate and query all. Run query plans and timing on representative fixtures before release; the October EXPLAIN estimate is not a latency benchmark.

## Evidence ledger

All source links below are at the freshly fetched origin/main commit shown above. Paths identify owning code, not a production deployment. Line snippets are generated from that ref so links remain reviewable.

### D1

[Performance.tsx:8](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/insights/performance/Performance.tsx#L8).

### D2

[HeaderUseCaseSelector.tsx:78](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/header/HeaderUseCaseSelector.tsx#L78).

### D3

[useScorecardTemplatesLevelSelect.ts:44](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/filters/scorecard-templates-level-select/useScorecardTemplatesLevelSelect.ts#L44).

### D4

[useCurrentScorecardTemplates.ts:19](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/coaching/useCurrentScorecardTemplates.ts#L19).

### D5

[useGetScorecardTemplatePermissions.ts:109](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/coaching/useGetScorecardTemplatePermissions.ts#L109).

### D6

[PerformanceConversations.tsx:85](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/insights/performance/performance-conversations/PerformanceConversations.tsx#L85).

### D7

[useGetQAStats.tsx:10](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/useGetQAStats.tsx#L10); [StatsGraphContainer.tsx:13](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/stats-graph-container/StatsGraphContainer.tsx#L13).

### D8

[useQAFilterByAttribute.ts:17](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/useQAFilterByAttribute.ts#L17); [getQAFilterByAttribute.ts:20](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/getQAFilterByAttribute.ts#L20); [useQAScoreStatsRequestParams.ts:14](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/useQAScoreStatsRequestParams.ts#L14).

### D9

[insightsApi.ts:283](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-api/src/services/cresta-api/insights/insightsApi.ts#L283); [insightsApi.ts:742](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-api/src/services/cresta-api/insights/insightsApi.ts#L742).

### D10

[LeaderboardByScorecardTemplateItem.tsx:11](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/leaderboard-by-scorecard-template-item/LeaderboardByScorecardTemplateItem.tsx#L11).

### D11

[PerformanceProgression.tsx:37](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/insights/performance/performance-progression/PerformanceProgression.tsx#L37); [PerformanceProgression.tsx:4](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/insights/performance/performance-progression/PerformanceProgression.tsx#L4).

### D12

[LeaderboardPerCriterion.tsx:32](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/leaderboard-per-criterion/LeaderboardPerCriterion.tsx#L32).

### D13

[ConversationCountChart.tsx:115](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/conversation-count-chart/ConversationCountChart.tsx#L115); [AverageHandleTimeInsightsMetric.tsx:15](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/average-handle-time-chart/AverageHandleTimeInsightsMetric.tsx#L15); [AverageHandleTimeChartGraph.tsx:32](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/average-handle-time-chart/AverageHandleTimeChartGraph.tsx#L32).

### D14

[useFilterByAttributes.ts:395](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/insights/useFilterByAttributes.ts#L395).

### D15

[useRetrieveQAConversationsRequestParams.ts:13](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/useRetrieveQAConversationsRequestParams.ts#L13); [useRetrieveAllQAConversations.ts:7](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/useRetrieveAllQAConversations.ts#L7).

### D16

[useScorecardsForQAConversations.ts:43](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/useScorecardsForQAConversations.ts#L43).

### D17

[QAReport.tsx:19](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/qa/report/QAReport.tsx#L19); [useQAReportFilters.tsx:178](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/hooks/qa-report-filter/useQAReportFilters.tsx#L178); [useQAReportFiltersToInsightsAttribute.ts:11](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/hooks/useQAReportFiltersToInsightsAttribute.ts#L11).

### D18

[QAReportTabs.tsx:51](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/tabs/QAReportTabs.tsx#L51).

### D19

[useRetrieveDirectorTaskStats.ts:14](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/hooks/useRetrieveDirectorTaskStats.ts#L14); [useQMTaskStats.ts:73](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/hooks/useQMTaskStats.ts#L73).

### D20

[useRetrieveManualQAStats.ts:23](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/hooks/useRetrieveManualQAStats.ts#L23).

### D21

[useRetrieveAppealStats.ts:24](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/qa/useRetrieveAppealStats.ts#L24).

### D22

[AppealsTable.tsx:29](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/appeals/appeals-table/AppealsTable.tsx#L29).

### D23

[useQMReportGroupCalibrationStats.ts:10](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/hooks/useQMReportGroupCalibrationStats.ts#L10).

### D24

[useRecentCalibrationsTableData.ts:51](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/hooks/useRecentCalibrationsTableData.ts#L51); [LeaderboardSideDrawer.tsx:77](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/group-calibration/group-calibration-leaderboard/components/LeaderboardSideDrawer.tsx#L77); [useSessionSnapshotFilters.tsx:73](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/hooks/useSessionSnapshotFilters.tsx#L73).

### D25

[SessionSnapshotChart.tsx:24](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/group-calibration/session-snapshot-chart/SessionSnapshotChart.tsx#L24).

### D26

[QMReportTaskCycles.tsx:105](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/qm-report-task-cycles/QMReportTaskCycles.tsx#L105).

### D27

[LeaderboardByReviewerExport.tsx:9](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/leaderboard-by-qa-analyst/LeaderboardByReviewerExport.tsx#L9); [LeaderboardHeader.tsx:89](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/leaderboard-per-metric-by-time/leaderboard-header/LeaderboardHeader.tsx#L89).

### D28

[useExportScorecard.tsx:30](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/coaching/useExportScorecard.tsx#L30).

### D29

[useOneOnOneCalibrationExport.ts:104](https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/one-on-one-calibration-export/useOneOnOneCalibrationExport.ts#L104).

### G1

[action_list_current_scorecard_templates.go:79](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_list_current_scorecard_templates.go#L79).

### G2

[scorecard_template.go:104](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/scorecard_template.go#L104); [scorecard_template.go:143](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/scorecard_template.go#L143).

### G3

[retrieve_conversation_stats.go:20](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_conversation_stats.go#L20); [retrieve_conversation_stats_clickhouse.go:32](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_conversation_stats_clickhouse.go#L32).

### G4

[retrieve_qa_conversations.go:116](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_conversations.go#L116); [retrieve_qa_conversations_clickhouse.go:31](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go#L31).

### G5

[common_customer_insights.go:434](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/common_customer_insights.go#L434).

### G6

[common_clickhouse.go:817](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/common_clickhouse.go#L817); [common_clickhouse.go:551](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/common_clickhouse.go#L551).

### G7

[retrieve_qa_score_stats_clickhouse.go:190](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L190); [retrieve_qa_score_stats_clickhouse.go:349](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L349); [retrieve_qa_score_stats_clickhouse.go:410](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L410); [qa_stats.proto:287](https://github.com/cresta/cresta-proto/blob/52b428ad53b9ffe6cb5329c2739e807eb5fa00df/cresta/v1/analytics/qa_stats.proto#L287); [common_clickhouse.go:1294](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/common_clickhouse.go#L1294).

### G8

[retrieve_qa_score_stats.go:966](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go#L966); [retrieve_qa_score_stats.go:285](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go#L285).

### G9

[action_list_director_task.go:373](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_list_director_task.go#L373).

### G10

[manual_qa.go:44](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/manual_qa.go#L44); [manual_qa.go:72](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/manual_qa.go#L72).

### G11

[backfill_qm_task_config.go:129](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/cron/sync-users/internal/qm-task/backfill_qm_task_config.go#L129).

### G12

[retrieve_qm_task_stats.go:279](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L279); [retrieve_qm_task_stats.go:311](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L311); [retrieve_qm_task_stats.go:362](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L362); [retrieve_qm_task_stats.go:442](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L442); [retrieve_director_task_stats.go:14](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_director_task_stats.go#L14).

### G13

[retrieve_manual_qa_stats.go:1321](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_manual_qa_stats.go#L1321); [retrieve_manual_qa_stats.go:1398](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_manual_qa_stats.go#L1398); [retrieve_manual_qa_stats.go:1505](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_manual_qa_stats.go#L1505).

### G14

[retrieve_appeal_stats.go:846](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go#L846); [retrieve_appeal_stats.go:867](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go#L867); [retrieve_appeal_stats.go:386](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go#L386).

### G15

[retrieve_group_calibration_stats.go:461](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L461); [retrieve_group_calibration_stats.go:590](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L590); [retrieve_group_calibration_stats.go:1056](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L1056).

### G16

[action_export_scorecards.go:413](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L413); [action_export_scorecards.go:764](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L764); [action_export_scorecards.go:1162](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L1162); [action_list_scorecards.go:333](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_list_scorecards.go#L333); [transformers.go:411](https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/transformers.go#L411).
