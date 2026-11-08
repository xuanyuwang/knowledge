# Cross-use-case PI and QM Report — proposed ticket list

Local drafts only. Do not publish these to Linear unless requested. The nine issues created earlier in this session were deleted from Linear at the user's request; their implementation content is preserved here. Linear retains deleted issues for 30 days in Recently deleted.

See the [source inventory](cross-usecase-pi-qm-change-inventory.md) for all 39 paths, evidence, and the shared acceptance fixture. Partly unauthorized request behavior and the server entitlement authority remain unresolved.

| Draft | Title | Depends on |
| --- | --- | --- |
| [DEF](#def) | Preserve and validate the exact reporting use-case selection | None |
| [DEFN](#defn) | Separate current template applicability from historical reporting definitions | DEF |
| [PI-BE](#pi-be) | Add use-case breakdowns and preserve historical facts in PI APIs | DEFN, DEF |
| [TASK](#task) | Discover QM tasks through linked template scope without hiding history | DEFN, DEF |
| [PI-FE](#pi-fe) | Apply local multi-use-case selection across Performance Insights | DEF, PI-BE, DEFN |
| [QM](#qm) | Scope QM Evaluations facts and share selection across report tabs | DEF, TASK, DEFN |
| [APPEAL](#appeal) | Filter Appeals aggregates, details and enrichment by selected use cases | DEFN, QM, DEF |
| [GC](#gc) | Keep Group Calibration report requests and pinned criteria consistent | DEF, DEFN, QM |
| [EXPORT](#export) | Keep PI and QM exports aligned with selected scope and historical revisions | GC, DEFN, QM, PI-BE, DEF, PI-FE, APPEAL |

## DEF

**Proposed title:** [Omni use case] Preserve and validate the exact reporting use-case selection

Reporting hooks and query builders can replace an explicit selection with the global use case, all configured use cases, or an unfiltered query. Define one request contract and enforce it before facts are queried.

Acceptance:

* Preserve every valid authorized explicit selection as a set from UI through API and SQL/ClickHouse. Normalize duplicates/order only; intentional clicked-use-case drilldowns may narrow it.
* Define omitted, explicit empty, ALL, loading, hidden/inactive/removed, malformed and foreign-parent behavior. An empty effective set must never remove a SQL predicate and read all facts.
* Identify the existing server authority for allowed use cases. Frontend profile configuration, user/group ACL helpers and role/feature annotations do not establish that authority by themselves.
* Resolve the open choice for partly unauthorized requests: reject the request, or return an explicitly displayed authorized intersection. Do not silently choose either behavior.
* Validate full resource parent, existence and the agreed authority once; reuse the result across analytics, task/template discovery, scorecard lists and exports. Preserve existing user/group/resource checks and document legacy-caller compatibility.
* Cover A, B, A+B, duplicates, empty/omitted/ALL, foreign-parent names, partial authorization and a permission change. No new permission model is assumed.

### Implementation notes

Relevant paths: Director useScorecardTemplatesLevelSelect, getQAFilterByAttribute, useFilterByAttributes and reporting/export hooks. Backend parseUsercaseNames extracts leaf IDs without checking parent; several QM/manual parsers skip invalid names; optional predicates make empty ambiguous. ListScorecards already has extractAndValidateUsecaseIDs as a parent-validation precedent. Browser observation established endpoint URLs only; no live POST payload or authorization defect was demonstrated.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows PI-02, PI-13; shared contract for all PI/QM/export readers. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* D3: [useScorecardTemplatesLevelSelect.ts:44](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/filters/scorecard-templates-level-select/useScorecardTemplatesLevelSelect.ts#L44>).
* D8: [useQAFilterByAttribute.ts:17](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/useQAFilterByAttribute.ts#L17>); [getQAFilterByAttribute.ts:20](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/getQAFilterByAttribute.ts#L20>); [useQAScoreStatsRequestParams.ts:14](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/useQAScoreStatsRequestParams.ts#L14>).
* D14: [useFilterByAttributes.ts:395](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/insights/useFilterByAttributes.ts#L395>).
* G5: [common_customer_insights.go:434](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/common_customer_insights.go#L434>).
* G10: [manual_qa.go:44](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/manual_qa.go#L44>); [manual_qa.go:72](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/manual_qa.go#L72>).


## DEFN

**Proposed title:** [Omni use case] Separate current template applicability from historical reporting definitions

Current-template readers filter scope before selecting the latest revision and include usecase_ids in DISTINCT identity. A scope edit can therefore surface an older revision. Historical reporting also needs definitions that remain discoverable after a scope or criterion is removed.

Acceptance:

* Select one latest revision by customer/profile/template identity, with deterministic ordering, before applying overlap and the agreed status rules.
* Apply the same behavior to full, metadata-only and shared current-template readers. Return the full canonical body with applicability; do not slice away criteria for other use cases.
* Provide historical identity/definition discovery from eligible facts and pinned revisions for PI, QM hard loads, archived/no-task rows and exports.
* Keep permission-catalog reads distinct from selected-page population and enforce full-definition access.
* Test r1=A+B then r2=A: current B must not return r1 as current, while historical B facts and removed criteria remain readable. Cover deactivated/deleted templates and repeated criterion IDs in different templates.
* Reconcile legacy NULL/empty reads and explicit all-usecases with the nonempty Create/Commit guard already delivered by <issue id="f06e2044-71ba-48ab-b1a6-2721252f6c23" label="CONVI-7981" href="https://linear.app/cresta/issue/CONVI-7981/delta-air-lines-enforce-nonempty-template-use-case-scope-in-backend" />. Do not create another empty-write validation change.

### Implementation notes

Owners: action_list_current_scorecard_templates.go (both list modes); shared/qa/scorecard_template.go ListScorecardTemplates and ListCurrentScorecardTemplateIDs; current-template consumers and action_export_scorecards.go anyCurrentTemplateRequiresPublish. The correctly pinned getLinkedTemplatesForExport and ListScorecards template_revision paths must remain pinned.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows PI-03/04/05/16; QM-02; EX-04. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* G1: [action_list_current_scorecard_templates.go:79](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_list_current_scorecard_templates.go#L79>).
* G2: [scorecard_template.go:104](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/scorecard_template.go#L104>); [scorecard_template.go:143](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/scorecard_template.go#L143>).
* G16: [action_export_scorecards.go:413](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L413>); [action_export_scorecards.go:764](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L764>); [action_export_scorecards.go:1162](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L1162>); [action_list_scorecards.go:333](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_list_scorecards.go#L333>); [transformers.go:411](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/transformers.go#L411>).
* D5: [useGetScorecardTemplatePermissions.ts:109](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/coaching/useGetScorecardTemplatePermissions.ts#L109>).


## PI-BE

**Proposed title:** [Omni use case] Add use-case breakdowns and preserve historical facts in PI APIs

QA stats accepts multiple use cases but lacks use-case grouping. Current-template defaults/config resolution can hide historically scored criteria even when the stored fact predicate is correct.

Acceptance:

* Add a use-case grouping enum, response field and scanner to QA stats; carry stored usecase_id through the latest/deduplicated scorecard CTE and aggregation. Preserve template+criterion identity.
* Implement equivalent filters/grouping for ordinary ClickHouse and metadata/annotation/outcome paths, including value-or-missing and inclusion/exclusion filters.
* Audit scorable-criterion defaults, directionality and respect-template-QA-config behavior in stats and QA examples. Separate current reporting configuration from revision-owned scoring rules; removed historical criteria must remain reportable.
* Carry the agreed selection through paginated examples and existing list-aware conversation stats used for handle time.
* Preserve DISTINCT score rows, ID+scorecard_last_update_time joins, conversation-version handling and time-target behavior.
* Reconcile combined and per-use-case weighted numerators/denominators using r1/r2 historical fixtures. Distinct conversation counts need not add across breakdowns. Benchmark representative queries before release.

### Implementation notes

Paths: retrieve_qa_score_stats.go; retrieve_qa_score_stats_clickhouse.go qaScorecardQuery, qaScoreStatsClickhouseQuery and qaScoreStatsClickhouseQueryWithMetadataView; retrieve_qa_conversations\*.go; common_clickhouse.go; cresta/v1/analytics/qa_stats.proto. ConversationCountChart uses QA stats, not RetrieveConversationStats. AHT uses the latter.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows PI-11, PI-14/15/16; QA stats/examples and conversation stats contract. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* G7: [retrieve_qa_score_stats_clickhouse.go:190](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L190>); [retrieve_qa_score_stats_clickhouse.go:349](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L349>); [retrieve_qa_score_stats_clickhouse.go:410](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go#L410>); [qa_stats.proto:287](<https://github.com/cresta/cresta-proto/blob/52b428ad53b9ffe6cb5329c2739e807eb5fa00df/cresta/v1/analytics/qa_stats.proto#L287>); [common_clickhouse.go:1294](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/common_clickhouse.go#L1294>).
* G8: [retrieve_qa_score_stats.go:966](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go#L966>); [retrieve_qa_score_stats.go:285](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go#L285>).
* G4: [retrieve_qa_conversations.go:116](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_conversations.go#L116>); [retrieve_qa_conversations_clickhouse.go:31](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go#L31>).
* G3: [retrieve_conversation_stats.go:20](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_conversation_stats.go#L20>); [retrieve_conversation_stats_clickhouse.go:32](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_conversation_stats_clickhouse.go#L32>).


## TASK

**Proposed title:** [Omni use case] Discover QM tasks through linked template scope without hiding history

QM tasks persist a copied single use case, and discovery/helpers filter that value. A shared-template task must be usable across the linked template's current scope, while historical task/scorecard associations remain readable.

Acceptance:

* Derive future QM task applicability from the linked latest template before overlap, following the existing task-scope decision. Enforce a valid same-profile template link for relevant create/update paths.
* Update ListDirectorTasks QM branch, getQMTasks, GetTaskByID, ListEvaluationPeriodsForTask and affected readers/jobs/backfill. Audit legacy absent/stale links and define compatible handling before enabling the change.
* Keep ACTIVE/ARCHIVED and historical hard-load identities discoverable after template scope shrinks. Historical scorecards retain concrete use case and pinned revision.
* Preserve task-owned audience, schedule, conversation filters and task-wide quota; do not create a separate task or quota per use case.
* Keep group-calibration task scope independent.
* Cover A+B template → A edit, archived tasks and legacy links; compare query plans/timing before choosing any persisted projection.

### Implementation notes

Paths: action_list_director_task.go buildListQMTaskQuery vs GC branch; shared/qa/manual_qa.go; retrieve_qm_task_stats.go; cron/sync-users/internal/qm-task/backfill_qm_task_config.go (copies template.UsecaseIds\[0\]). Existing knowledge work item qm-task-template-scope.md is the prior design record; this issue owns implementation.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows QM-02/03; future task scope and historical identity. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* G9: [action_list_director_task.go:373](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_list_director_task.go#L373>).
* G10: [manual_qa.go:44](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/manual_qa.go#L44>); [manual_qa.go:72](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/shared/qa/manual_qa.go#L72>).
* G11: [backfill_qm_task_config.go:129](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/cron/sync-users/internal/qm-task/backfill_qm_task_config.go#L129>).
* G12: [retrieve_qm_task_stats.go:279](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L279>); [retrieve_qm_task_stats.go:311](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L311>); [retrieve_qm_task_stats.go:362](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L362>); [retrieve_qm_task_stats.go:442](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L442>); [retrieve_director_task_stats.go:14](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_director_task_stats.go#L14>).


## PI-FE

**Proposed title:** [Omni use case] Apply local multi-use-case selection across Performance Insights

Performance conversations is gated by HasSingleUseCase. Its QA hooks read global selection, while the template picker can replace a supplied array with all available use cases. Add local multi-use-case selection and carry it through every PI view.

Acceptance:

* Initialize local selection from global context; show the effective set and handle loading/empty/ALL using the shared contract. Update route restriction and exact-set template picker.
* Pass the set in actual request params/cache identity for combined/manual/auto cards, chart/delta/peer baselines, criterion/template table, all progression variants, rankings/quintiles and drawers/pages.
* Cover conversation volume via QA score stats and handle time via conversation stats, including unfiltered/single-agent/comparison baselines with each metric's existing template-filter semantics.
* Render combined and use-case breakdowns with template+criterion+use-case identities. Distinguish inapplicable criteria, scored N/A and missing data; preserve weighting.
* Use canonical current definitions for applicability and pinned definitions for historical facts. Exact-ID scorecard enrichment stays tied to returned examples with per-resource checks.
* Selection changes must not display stale rows or cross-contaminate exports; intentional clicked-use-case narrowing must be visible.

### Implementation notes

Inventory PI-01..PI-17 names all callers. Key files: Performance.tsx, HeaderUseCaseSelector.tsx, useScorecardTemplatesLevelSelect.ts, QA request-param builders/getQAFilterByAttribute, useFilterByAttributes.ts, PerformanceConversations.tsx, PerformanceProgression.tsx, LeaderboardByScorecardTemplateItem, LeaderboardPerCriterion, ConversationCountChart and AverageHandleTime\*. Exports are integrated/tested in the export issue.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows PI-01/02/05–12/17. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* D1: [Performance.tsx:8](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/insights/performance/Performance.tsx#L8>).
* D3: [useScorecardTemplatesLevelSelect.ts:44](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/filters/scorecard-templates-level-select/useScorecardTemplatesLevelSelect.ts#L44>).
* D8: [useQAFilterByAttribute.ts:17](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/useQAFilterByAttribute.ts#L17>); [getQAFilterByAttribute.ts:20](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/getQAFilterByAttribute.ts#L20>); [useQAScoreStatsRequestParams.ts:14](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/hooks/util-hooks/useQAScoreStatsRequestParams.ts#L14>).
* D11: [PerformanceProgression.tsx:37](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/insights/performance/performance-progression/PerformanceProgression.tsx#L37>); [PerformanceProgression.tsx:4](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/insights/performance/performance-progression/PerformanceProgression.tsx#L4>).
* D13: [ConversationCountChart.tsx:115](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/conversation-count-chart/ConversationCountChart.tsx#L115>); [AverageHandleTimeInsightsMetric.tsx:15](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/average-handle-time-chart/AverageHandleTimeInsightsMetric.tsx#L15>); [AverageHandleTimeChartGraph.tsx:32](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/insights/qa-insights/average-handle-time-chart/AverageHandleTimeChartGraph.tsx#L32>).


## QM

**Proposed title:** [Omni use case] Scope QM Evaluations facts and share selection across report tabs

QM Report lists and stats use separate global singleton hooks. Two Evaluations scorecard-query branches omit use-case filtering entirely. Establish shared report selection and correct the fact population before adding tab/export consumers.

Acceptance:

* Own one local selected set across Evaluations, Appeals and Group Calibration, including URL/task-cycle hard loads, persisted filters, refreshes and request/cache params.
* Pass it unchanged through useQMTaskStats, useRetrieveDirectorTaskStats, the export fetcher and useRetrieveManualQAStats; stop overwriting explicit request scope.
* Pass validated IDs into fetchScorecardsForPeriod and fetchScorecardsCountGroupedByFrequency and filter scorecards.usecase_id before aggregation. Verify already-filtered no-task/time-range paths and includeExportStats activity fields agree.
* Audit manual stats reviewer/period/agent/group/criterion/overwritten/conversation branches, preserving existing optional fact filters and pinned definitions while fixing parsing/resolution.
* Keep combined template×task×cycle grain and task-wide quotas. Show completion N/A when the selected set covers only part of task scope, including exports; retain full-scope quota and coverage-percentage semantics.
* Test shared and archived tasks, no-task cards, partial cycles, A/B/A+B and scope shrink. Compare counts across overview, charts, drilldowns, pagination and heavier export requests.

### Implementation notes

Director QAReport/useQAReportFilters/useQAReportFiltersToInsightsAttribute, QAReportTabs, useQMTaskStats, useRetrieveDirectorTaskStats and useRetrieveManualQAStats. Backend retrieve_director_task_stats.go routes QM; retrieve_qm_task_stats.go has the two missing predicates; retrieve_manual_qa_stats.go already has optional usecase predicates in count/detail/overwritten queries. Do not describe all manual SQL as missing the filter.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows QM-01/02/04–08; EX-01 sources. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* D17: [QAReport.tsx:19](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/features/qa/report/QAReport.tsx#L19>); [useQAReportFilters.tsx:178](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/hooks/qa-report-filter/useQAReportFilters.tsx#L178>); [useQAReportFiltersToInsightsAttribute.ts:11](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/hooks/useQAReportFiltersToInsightsAttribute.ts#L11>).
* D19: [useRetrieveDirectorTaskStats.ts:14](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/hooks/useRetrieveDirectorTaskStats.ts#L14>); [useQMTaskStats.ts:73](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/hooks/useQMTaskStats.ts#L73>).
* D20: [useRetrieveManualQAStats.ts:23](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/hooks/useRetrieveManualQAStats.ts#L23>).
* G12: [retrieve_qm_task_stats.go:279](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L279>); [retrieve_qm_task_stats.go:311](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L311>); [retrieve_qm_task_stats.go:362](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L362>); [retrieve_qm_task_stats.go:442](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go#L442>); [retrieve_director_task_stats.go:14](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_director_task_stats.go#L14>).
* G13: [retrieve_manual_qa_stats.go:1321](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_manual_qa_stats.go#L1321>); [retrieve_manual_qa_stats.go:1398](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_manual_qa_stats.go#L1398>); [retrieve_manual_qa_stats.go:1505](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_manual_qa_stats.go#L1505>).


## APPEAL

**Proposed title:** [Omni use case] Filter Appeals aggregates, details and enrichment by selected use cases

Appeals frontend replaces request scope with one global use case. Backend validation requires exactly one, but its condition builder drops that scope and the scorecard query has no use-case predicate. Removing singleton validation alone would still produce incorrect results.

Acceptance:

* Consume shared QM report selection and preserve it through useRetrieveAppealStats.
* Accept the agreed valid set, retain it in GetRetrieveAppealStatsConditions and apply fact predicates to requested/original/resolved appeal population and every aggregate/detail branch.
* Verify cards, trends, reviewer leaderboard, table, pagination, adjusted/unadjusted values and CSV source rows use the same population.
* Restrict AppealsTable's conversation-name scorecard expansion to exact returned identities or selected use cases and intended templates. Do not import unrelated scorecards from the same conversation.
* Preserve workflow/user/resource ACLs, pinned template/criterion identities and resolution mapping.
* Test A/B/A+B, multiple scorecards per conversation, changed template scope and malformed/foreign/unauthorized requests using the shared contract.

### Implementation notes

Owners: hooks/qa/useRetrieveAppealStats.ts; components/qa/report/appeals/appeals-table/AppealsTable.tsx; retrieve_appeal_stats.go Validate, GetRetrieveAppealStatsConditions and retrieveAppealScorecards. Export serialization is tracked separately after these sources are corrected.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows QM-09/10; EX-06 source population. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* D21: [useRetrieveAppealStats.ts:24](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/qa/useRetrieveAppealStats.ts#L24>).
* D22: [AppealsTable.tsx:29](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/appeals/appeals-table/AppealsTable.tsx#L29>).
* G14: [retrieve_appeal_stats.go:846](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go#L846>); [retrieve_appeal_stats.go:867](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go#L867>); [retrieve_appeal_stats.go:386](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go#L386>).


## GC

**Proposed title:** [Omni use case] Keep Group Calibration report requests and pinned criteria consistent

Group Calibration stats, recent sessions, drawers and snapshots use different singleton paths. Task SQL always adds usecase_id IN, unlike optional predicates elsewhere. Snapshot labels can use today's template rather than the session's pinned revision.

Acceptance:

* Consume shared QM report selection in stats, task/session lists, recent calibrations, filters, leaderboard drawers, snapshots and response reads. Normalize empty/ALL before query construction.
* Validate resource parents and preserve each GC task's own use case and pinned template revision; do not inherit future QM task scope rules.
* For scorecard facts, prove/enforce and test the task/card use-case invariant, or add a fact predicate. Task-ID filtering alone is insufficient evidence for legacy inconsistent records.
* Resolve session labels/answers from pinned revisions. Keep answer-key/response identity and comparable-criterion denominators; distinguish inapplicable criteria from missing responses.
* Preserve intentional exact-ID child requests under their parent result.
* Test sessions across A/B, r1→r2 edits, mismatched legacy facts, selected-set changes and leaderboard/recent/snapshot/CSV reconciliation.

### Implementation notes

Director useQMReportGroupCalibrationStats, useRecentCalibrationsTableData, group-calibration LeaderboardSideDrawer, useSessionSnapshotFilters and SessionSnapshotChart. Backend retrieve_group_calibration_stats.go buildGroupCalibrationStatsConditions, findTasks, findScores. Existing score query joins pinned template_revision; retain it.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows QM-11–14; EX-07 sources. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* D23: [useQMReportGroupCalibrationStats.ts:10](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/hooks/useQMReportGroupCalibrationStats.ts#L10>).
* D24: [useRecentCalibrationsTableData.ts:51](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/hooks/useRecentCalibrationsTableData.ts#L51>); [LeaderboardSideDrawer.tsx:77](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/group-calibration/group-calibration-leaderboard/components/LeaderboardSideDrawer.tsx#L77>); [useSessionSnapshotFilters.tsx:73](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/hooks/useSessionSnapshotFilters.tsx#L73>).
* D25: [SessionSnapshotChart.tsx:24](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/group-calibration/session-snapshot-chart/SessionSnapshotChart.tsx#L24>).
* G15: [retrieve_group_calibration_stats.go:461](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L461>); [retrieve_group_calibration_stats.go:590](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L590>); [retrieve_group_calibration_stats.go:1056](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go#L1056>).


## EXPORT

**Proposed title:** [Omni use case] Keep PI and QM exports aligned with selected scope and historical revisions

Exports mix loaded rows, lazy child requests and full scorecard fetches. Some hooks replace the caller's use cases, so the downloaded population can differ from the visible report.

Acceptance:

* Capture one selected set when export starts and use it for all child fetches/pages and serialization, even if selection changes during export.
* Cover PI chart/table CSV and progression ZIP; QM cycle/activity CSV with includeExportStats; reviewer/trend CSV; full scorecard export; detailed 1:1 calibration; Appeals and GC CSV.
* Stop useExportScorecard and export-stat hooks overwriting explicit scope. Validate full resource parent/authority in ExportScorecards while preserving its existing list-aware SQL predicate and same-population metadata reads.
* Preserve getLinkedTemplatesForExport's pinned revision join, multi-revision criterion union, historical absence semantics and publish/submitted-column behavior. Correct current config discovery separately.
* Label selected use cases and use stable template+criterion+use-case row identities. Combined/breakdown counts and weighted totals must reconcile; partial task completion remains N/A without duplicated quotas.
* Verify visible report vs paginated details vs every exported dataset using A/B/r1/r2 fixtures. Keep <issue id="4805891a-1d10-47d2-a6ed-2362084998a8" label="CONVI-7795" href="https://linear.app/cresta/issue/CONVI-7795/export-detailed-11-calibration-results" /> baseline 1:1 export and <issue id="843703bd-c4fb-42eb-a5b6-d4bc0d641a33" label="CONVI-7941" href="https://linear.app/cresta/issue/CONVI-7941/download-from-pi-on-of-overwrittens-not-wokring" /> ZIP race work separate; this issue covers cross-use-case compatibility.

### Implementation notes

Inventory EX-01..08 lists callers. Backend action_export_scorecards.go createScorecardQuerySupplier already applies optional usecase_id IN; getLinkedTemplatesForExport is correctly pinned. Director useExportScorecard.tsx, useQMTaskStats export fetcher, QMReportTaskCycles, LeaderboardByReviewerExport, LeaderboardHeader, useOneOnOneCalibrationExport and PI CSV/ZIP children. <issue id="b83afb18-9bff-40b8-95b9-063b1f2cbf6a" label="CONVI-6392" href="https://linear.app/cresta/issue/CONVI-6392/include-qm-scoring-activity-in-report-exports" /> is existing export activity context.

Source inventory: knowledge/scorecard-workflows/deliverables/cross-usecase-pi-qm-change-inventory.md (November 7, 2026), rows EX-01–08; PI-17. Immutable evidence refs: Director a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae, go-servers fc3fa4fa48de29914c8357892f30f23a6c005533, cresta-proto 52b428ad53b9ffe6cb5329c2739e807eb5fa00df. Source review, not deployed validation. Shared acceptance fixture includes authorized A/B, template r1=A+B then r2=A, removed/scoped criteria, manual/auto/N/A/missing facts, archived/no-task/shared tasks and pinned GC sessions. V3/scoped-criterion reader compatibility in the engineering plan remains a release prerequisite.

Source links:

* D26: [QMReportTaskCycles.tsx:105](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/qm-report-task-cycles/QMReportTaskCycles.tsx#L105>).
* D27: [LeaderboardByReviewerExport.tsx:9](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/leaderboard-by-qa-analyst/LeaderboardByReviewerExport.tsx#L9>); [LeaderboardHeader.tsx:89](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/report/leaderboard-per-metric-by-time/leaderboard-header/LeaderboardHeader.tsx#L89>).
* D28: [useExportScorecard.tsx:30](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/hooks/coaching/useExportScorecard.tsx#L30>).
* D29: [useOneOnOneCalibrationExport.ts:104](<https://github.com/cresta/director/blob/a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae/packages/director-app/src/components/qa/one-on-one-calibration-export/useOneOnOneCalibrationExport.ts#L104>).
* G16: [action_export_scorecards.go:413](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L413>); [action_export_scorecards.go:764](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L764>); [action_export_scorecards.go:1162](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_export_scorecards.go#L1162>); [action_list_scorecards.go:333](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/action_list_scorecards.go#L333>); [transformers.go:411](<https://github.com/cresta/go-servers/blob/fc3fa4fa48de29914c8357892f30f23a6c005533/apiserver/internal/coaching/transformers.go#L411>).
