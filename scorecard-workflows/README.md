# Scorecard Workflows Domain

## Purpose

- 2026-11-05: [Cresta-security/ps oversized scorecard-list investigation](work-items/CONVI-7974.md) traced a one-scorecard request failure to an 18 MB auth user login-status response. Scorecard comments and population are small; remediation is pending.

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

- November 7: [Cross-use-case PI/QM inventory](deliverables/cross-usecase-pi-qm-change-inventory.md) traces 17 PI, 14 QM and eight export paths with source links. The nine proposed tickets, including acceptance criteria and dependencies, are saved in a [local draft list](deliverables/cross-usecase-pi-qm-ticket-drafts.md). The newly created Linear issues were deleted at user request. Partly unauthorized request behavior and server entitlement authority remain unresolved; live evidence covers URLs, not POST bodies. [Session](sessions/2026-11-07/codex-cross-usecase-pi-qm-inventory.md).

- CONVI-7981 client audit found shared-ingestion Opera setup creates templates without use cases; migrate that client before deploying backend PR #35339. Other traced writers set destination scope or preserve existing scope. Proto unchanged by user decision. [Audit](sessions/2026-11-06/codex-convi-7981-client-compatibility.md).

- Created [CONVI-7981](https://linear.app/cresta/issue/CONVI-7981/delta-air-lines-enforce-template-use-case-validation-in-backend-apis) (High priority; Backlog at creation) for Delta target failures and backend template scope validation across all API clients. Includes both empty-scope templates, explicit Commit omission behavior, accurate lookup errors, and direct API tests. Implementation and customer remediation remain pending. [Work item](work-items/CONVI-7981.md).

- Delta inventory now identifies two empty-scope templates: POC Template (13 descriptive criterion IDs) and Customer Experience (5). Their initial writes share a creator and are 695 ms apart. The other two Delta Care templates have populated scope and UUID criteria across their histories; POC IDs overlap none of the other templates' historical IDs. The initial setup/client remains unconfirmed. [Comparison](sessions/2026-11-06/codex-delta-template-comparison.md).

- Live Delta/Walter Chrome comparison: the `0` on POC Template's access tab is the JSX rendering of its empty scope length. Fresh unsaved Delta forms correctly show voice-care and synthetic-care. The current customer setup resolves those use cases; the existing template's original empty-scope authoring request remains unknown. [Evidence](sessions/2026-11-06/codex-delta-template-frontend-path.md).

- Delta frontend inspection found no fresh-create path that drops the selected use case; six local source-function/form-control checks passed. Ordinary copying propagates already-empty scope, while cross-use-case copying sets a destination. The earliest stored Delta revision's descriptive criterion IDs differ from the current builder's generated UUIDs; the original creation client/request remains unconfirmed. [Evidence](sessions/2026-11-06/codex-delta-template-frontend-path.md).

- Delta authoring follow-up: CreateScorecardTemplate and CommitScorecardTemplate permit empty/omitted use-case lists and store them directly. Commit can clear prior scope on omission. New-template UI defaults to the selected use case, while existing empty scope is preserved on save; Delta's original authoring request remains unidentified.

- November 6: [Delta organization-target failure](work-items/CONVI-7981.md) is explained by inconsistent use-case matching: listing accepts empty use-case arrays, but CreateTarget rejects them. Read-only production checks confirm all three revisions exist, contain the reported criteria, have empty scope, and are neither deleted nor deactivated. Remediation and deployed API validation remain pending.

The November 4 [reviewed scorecard race recommendations](../scorecard-data-sync/deliverables/scorecard-races-corrected-recommendations.md), maintained under `scorecard-data-sync` with [CONVI-7784](../scorecard-data-sync/work-items/CONVI-7784.md), incorporate the investigation and Xuanyu's comment corrections. Workflow implications are intended-field writes via field masks, protected parent/criterion recomputation, autosave flushing before Submit, and a preferred shared mutation function with explicit versions and caller-visible optimistic conflicts. The document marks process-save queues and submit flushing done; deployment was not checked here. Audit history is optional. This cross-domain reference does not change the workflow domain's ownership of lifecycle semantics.

The [full omni-use-case engineering plan](deliverables/omni-usecase-scorecard-eng-plan-draft.md) was expanded to the company format on November 4 and checked against freshly fetched go-servers, Director, and cresta-proto source. It includes concrete SQL/JSON/Go examples, all five P0 consumer designs, API/storage/security/reliability/testing/cost/release sections, and a source ledger. The [canonical plan work item](work-items/omni-usecase-scorecard-plan.md) tracks continuing review. The [P0 sketch](deliverables/phase-1-p0-performance-insights-qm-sketch.md) remains a shorter, older companion.

The plan preserves template SQL applicability and criterion none/all/included scope in versioned JSON/typed Go, same-revision containment and historical defaults, template-owned criterion identity, full canonical reads, and no score row for an inapplicable criterion. A full Commit carries the author's final scopes after editor defaults. V3, derived QM task scope, grouped use-case responses, and serialized template Commit are proposed engineering choices, not implemented behavior.

The [November 4 decisions](decisions/2026-11-04-omni-usecase-autoqm-and-reporting.md) confirm manual scoring where an AutoQM rule does not cover a criterion's use case, rejection of overlapping new mappings, current Opera scope for new automatic runs, completion rate N/A for a filtered portion of a task-wide quota, and explicit confirmation when switching a finite template back to ALL resets all criteria. Separate per-use-case Opera rules on one criterion remain the baseline; one shared rule is conditional on `enableGlobalOperaRule`.

The [QM task scope decision](decisions/2026-10-08-qm-task-inherits-template-usecase-scope.md) remains unchanged: future task scope follows the linked current template, while historical scorecards retain their pinned revision and associations. Latest source still copies a first use case in task backfill and filters tasks by a singleton; two QM report scorecard query branches still omit selected-use-case filtering. Reader/job inventory, legacy-data audit, security/API/DB review, and representative performance tests are required before implementation/rollout readiness. No live data, deployment, or flag rollout was checked for the full draft.

October source and staging studies remain dated feasibility evidence. The [PI study](sessions/2026-10-08/codex-performance-insights-clickhouse-feasibility.md) used a sample without a shared template and measured estimated reads, not API latency. The [QM Evaluations trace](sessions/2026-10-08/codex-qm-report-evaluations-query-fit.md) explains report grain and scope-filter gaps; current source confirmations and proposals are distinguished in the full plan.

The first subdomain migration is complete. The subdomain references and [legacy source index](deliverables/legacy-source-index.md) are canonical navigation; historical folders retain detailed evidence and current work.

The [Auto Summarization precedent review](sessions/2026-10-08/codex-autosummarization-usecase-precedent.md) confirms an implemented authoring shape of one multi-use-case template with topic-level subsets. Its empty topic scope means all and cannot encode the scorecard criterion NONE state. Its prior top-level/nested use-case persistence bug is a concrete reason to validate and save scorecard scopes together.

The [AutoQM rule-mapping review](sessions/2026-10-08/codex-multi-usecase-autoqm-rule-mapping.md) preserves the original source trace and October 9 correction about the Opera authoring flag. Its open product questions were answered by the [November 4 decision](decisions/2026-11-04-omni-usecase-autoqm-and-reporting.md); runtime selection and the builder's multiple-rule support remain implementation work.

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

- November 6 post-repair check (16:02:58 UTC): Delta’s 4 templates / 17 revisions now contain no empty/NULL scopes or NULL/blank use-case array elements. Successful target creation and the CONVI-7981 API fix remain pending. [Evidence](sessions/2026-11-06/codex-delta-organization-target-not-found.md).

- November 6 closure: [CONVI-7981](https://linear.app/cresta/issue/CONVI-7981/delta-air-lines-resolve-organization-target-failure-caused-by-empty) is Done at the user’s request after the clean Delta data inventory. Backend/API/UI prevention work was not implemented; successful target-creation replay was not verified.

- November 6 prevention work: CONVI-7981 reopened as In Progress with [backend PR #35339](https://github.com/cresta/go-servers/pull/35339). Shared Create/Commit validation rejects omitted/empty scope; both API suites, Gazelle, and CI-version lint pass. Open PR is not merged/deployed; incident data inventory remains clean. [Implementation evidence](sessions/2026-11-06/codex-convi-7981-backend-validation.md).
