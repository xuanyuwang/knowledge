# Scorecard Workflows Domain

## Purpose

The canonical knowledge home for scorecard and template business workflows from authoring and evaluation through submission, permissions, reversal, appeals, group calibration, and related review behavior.

## Scope and Boundaries

**In scope**

- template concepts, authoring, revisioning, schema evolution, and configuration
- scorecard creation, evaluation, scoring, editing, submission, publishing, and reversal
- manual, AutoQM, process, manager, and other scorecard-type behavior
- permissions, visibility, audiences, and capability evaluation
- appeals, appeal comments, group calibration, and review workflows
- generation/backfill behavior when the question is whether a scorecard should exist
- frontend/backend workflow contracts and business-rule invariants

**Out of scope**

- PostgreSQL-to-ClickHouse propagation, ordering, drift, repair, or reindex mechanics: `scorecard-data-sync`
- analytics-page display and aggregation semantics: `analytics`
- delivery mechanics for notifications: `notifications`

## Subdomains

- [Template Authoring and Versioning](subdomains/template-authoring-and-versioning/README.md): template structure, builder behavior, revisions, duplication, and schema compatibility
- [Evaluation and Scoring](subdomains/evaluation-and-scoring/README.md): manual/AutoQM evaluation, option mapping, N/A, weights, outcomes, and score computation
- [Scorecard Lifecycle](subdomains/scorecard-lifecycle/README.md): creation, editing, submission, publishing, reversal, and concurrency/state transitions
- [Permissions and Visibility](subdomains/permissions-and-visibility/README.md): capability policy, audiences, submitted editing, and runtime visibility
- [Appeals](subdomains/appeals/README.md): request/resolve workflows, final-value interpretation, comments, and exports
- [Group Calibration](subdomains/group-calibration/README.md): answer keys, responses, completion, permissions, reporting, and exports
- [Process Scorecards and Generation](subdomains/process-scorecards-and-generation/README.md): process scorecards, existence rules, generation, repair, and backfill orchestration

## Inactive legacy consolidation — October 25, 2026

15 legacy project folders with no meaningful content on or after September 25 are now consolidated here. [Canonical summaries and preserved evidence](deliverables/inactive-legacy-project-consolidation-2026-10-25.md) retain historical findings and unfinished work. Option identity, mapped score and N/A representation are distinct. Submitted-edit permissions, template locks and reset authority require separate checks; scorecard generation is distinct from projection repair.

## Current State

An [expanded P0 engineering sketch](deliverables/phase-1-p0-performance-insights-qm-sketch.md) begins with the template/criterion data model, then maps it to template config, Performance Insights, QM task, QM Task Home, and QM Report. Diagrams and tables show the scope inheritance, per-use-case score rows, and query changes. It includes the proposed derived QM task scope and open AutoQM/reporting decisions. It is a private review aid; the longer engineering plan and focused investigations retain supporting evidence.

The [omni-use-case engineering plan](deliverables/omni-usecase-scorecard-eng-plan-draft.md) now treats a criterion-level field in versioned template JSON/Go as provisionally ready to pursue at roughly 80% confidence in the core model. Criteria stay owned and versioned by one template. Template scope is all or a finite non-empty set; criterion scope is none, all, or a finite included subset. Historical omissions inherit their stored revision's template scope, and canonical Get/list reads normalize them without rewriting history. Full-template Commit may accept an explicit final criterion subset after automatic template-scope transitions, subject to containment and per-use-case coverage. Branch children must stay within their parent's scope as an edge validation rule. The draft maps Coaching, QM, Performance Insights, and export changes at a high level; rollout effort and the exact JSON version remain open. A [2026-10-05 read-only check](sessions/2026-10-05/codex-template-usecase-array-states.md) found every row in the specified `voice-sandbox-2` database has exactly one use case. The proposal is unimplemented.

The PRD calls for both results over selected use cases and per-use-case breakdowns. Performance Insights shows template/criterion applicability and uses N/A in a breakdown where a criterion does not apply; that presentation state corresponds to no stored score row for that use case.

The [QM task scope decision](decisions/2026-10-08-qm-task-inherits-template-usecase-scope.md) establishes that a template-based QM task shares its template's entire use-case scope for future evaluations. The task's audience, quota, schedule, and filters remain task-owned; existing scorecards remain pinned to their evaluated template revision. Task and scorecard backfill review is a pre-implementation gate because current jobs contain single-use-case selection and matching assumptions.
The [QM task/template relationship review](sessions/2026-10-08/codex-qm-task-template-usecase-relationship.md) clarifies that the present task API and database row still carry a singular use case; Director copies the template's first one, and backend create/update does not enforce alignment. The normal QM setup flow and backfill are template-based, but the API and database do not guarantee every QM task has a valid template link. A plural task field or derived read contract, legacy-row audit/migration, and template-change synchronization are implementation decisions still to settle.

The Performance Insights phase-1 source and staging review found that score stats can already intersect criterion IDs with multiple use cases in ClickHouse. Representative one- versus two-use-case queries had the same estimated local-table reads in a seven-day staging sample, including a score breakdown by use case. The page still needs an API/response grouping design. The sample has no shared template and `EXPLAIN` does not establish API latency; see the [focused session note](sessions/2026-10-08/codex-performance-insights-clickhouse-feasibility.md).

A [phase-one cross-page fit review](sessions/2026-10-08/codex-cross-page-usecase-fit.md) found that Coaching Report's session/score filters accept multiple use cases, while QM task discovery/evaluation, Appeals validation, and coaching-plan storage do not yet fulfill the PRD. A [focused QM Report Evaluations trace](sessions/2026-10-08/codex-qm-report-evaluations-query-fit.md) corrected the earlier QM assessment: its manual-QA PostgreSQL queries can filter a list of scorecard use cases, but the overall chart and task-cycle table omit that filter in their scorecard queries. Task/period lookup also assumes a singular task use case, and neither stats RPC currently provides a use-case breakdown. QM task scope and coaching-plan scope are the main separate model decisions; Coaching Hub also assumes one active plan row per agent. Historical reporting follows each scorecard's pinned template revision, as confirmed on 2026-10-08.

The first subdomain migration is complete. The subdomain references and [legacy source index](deliverables/legacy-source-index.md) are canonical navigation; historical folders retain detailed evidence and current work.

The [Auto Summarization precedent review](sessions/2026-10-08/codex-autosummarization-usecase-precedent.md) confirms an implemented authoring shape of one multi-use-case template with topic-level subsets. Its empty topic scope means all and cannot encode the scorecard criterion NONE state. Its prior top-level/nested use-case persistence bug is a concrete reason to validate and save scorecard scopes together.

The [AutoQM rule-mapping review](sessions/2026-10-08/codex-multi-usecase-autoqm-rule-mapping.md) confirms that AutoQM is `auto_qa` configuration on a regular criterion. Multi-use-case Opera rule authoring is gated by `enableGlobalOperaRule`, so the plan treats separate use-case-specific rules attached to one shared criterion as the baseline path. One shared rule remains possible where the flag is enabled. Version-2 JSON already has a trigger array, but Director authors one behavior and backend scoring does not check trigger use-case eligibility. The builder and runtime must support eligible rule selection per conversation. Whether a criterion remains manually scorable without an eligible rule, and how to handle overlapping global and specific rules, remain open product choices.

Work items, sessions, daily logs, and decisions remain at this parent domain. Future work items should record one optional primary subdomain.

Current operational finding:

- [CONVI-7785 conversation template Opera picker](work-items/CONVI-7785.md): Director PR [#23461](https://github.com/cresta/director/pull/23461) is ready for review with inline Before/After videos and smoke-functional QA metadata. It excludes behaviors owned by Training Simulator Opera rules from the Performance Configuration AutoQA picker, including archived rules, while preserving regular-policy and unowned legacy behaviors. A Walter Dev baseline/preview comparison passed. One-set simplification `496873381a` is pushed; new-head CI and codeowner review remain to be verified.

- [CONVI-7744 create-scorecard pagination failure](work-items/CONVI-7744.md): Director draft PR [#23190](https://github.com/cresta/director/pull/23190) makes conversation details follow every `ListScorecards` page, fixing the incomplete read that triggered duplicate creation. Scorecard creation and autosave behavior remain unchanged. The focused regression test, package lint, and repository TypeScript checks pass; preview validation remains pending.

Current product-direction proposal:

- [Walter-dev submitted-edit permission diagnosis](work-items/walter-dev-submitted-edit-permissions.md): live flag is enabled, permission evaluation allows the recorded updater, and GetUser confirms SUPER_ADMIN. This matches the intentional Cresta-admin allowlist exemption in CONVI-7612 / go-servers #31889.

- [Scorecard Configuration Safety](deliverables/scorecard-configuration-safety-initiative-proposals.md) — a two-part lifecycle for preventing template configuration mistakes through preview and AI assistance, then assessing and remediating affected scorecards through an auditable workflow.

Recent completed work:

- [CONVI-7533 pre-deletion scorecard backup](work-items/CONVI-7533.md) — preserved 4 scorecards and 190 score rows as directly readable, checksum-verified CSV in Linear before deletion.

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `director`, `cresta-proto`, `config`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `log/2026-07-15.md`
- `log/2026-07-27.md`
- `log/2026-07-28.md`
- `deliverables/legacy-source-index.md`
- `deliverables/scorecard-configuration-safety-initiative-proposals.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
