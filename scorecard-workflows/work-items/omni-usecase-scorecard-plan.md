# Omni-use-case scorecard engineering plan

**Status:** active — full draft ready for review\
**Primary domain:** scorecard-workflows\
**Primary subdomain:** template-authoring-and-versioning\
**Official ticket:** [CONVI-7928](https://linear.app/cresta/issue/CONVI-7928); reporting implementation proposals are [local drafts](../deliverables/cross-usecase-pi-qm-ticket-drafts.md) only.\
**Last updated:** 2026-11-07

## Objective and Impact

- **Objective:** Turn the existing omni-use-case draft into a complete company-format engineering plan, grounded in settled decisions and current source.
- **Impact:** Make shared-template scoring, multi-use-case reporting, compatibility risks, and implementation/release work reviewable without reconstructing earlier chats.
- **Role:** designed and investigated.

## Scope

Full template/criterion model and P0 template configuration, Performance Insights, QM task, Task Home and QM Report. Include API/storage/security/monitoring/SLO/testing/cost/release analysis and compatibility dependencies. No product code, live database queries, deployment, production backfill, external posting, or new knowledge worktree.

## Source Context

- **Primary repo:** `/Users/xuanyu.wang/repos/go-servers`.
- **Related repos:** `/Users/xuanyu.wang/repos/director`, `/Users/xuanyu.wang/repos/cresta-proto`.
- **Knowledge checkout:** `/Users/xuanyu.wang/repos/knowledge`, main checkout.
- **Source review:** fetched `origin/main` read-only at go-servers `01c0d818b7793656005500f84efb2d1b69aa54bd`, Director `a03e8e6597da5619fdbeb4e2b29120147ca1c2a1`, proto `1938bb241393f0f54e5dd8242f03ba1a6617728e`; source branches unchanged.
- **Artifacts:** [full plan](../deliverables/omni-usecase-scorecard-eng-plan-draft.md), [concise P0 sketch](../deliverables/phase-1-p0-performance-insights-qm-sketch.md), [source ledger/session](../sessions/2026-11-04/codex-omni-usecase-full-engineering-plan.md).

## Current Understanding

The full draft uses the company template, preserves template SQL scope plus versioned JSON/Go criterion authority, same-revision containment/defaults, historical pinning, and no inapplicable score rows. It incorporates five [November 4 decisions](../decisions/2026-11-04-omni-usecase-autoqm-and-reporting.md): manual-only uncovered AutoQM, filtered completion N/A, confirmed ALL reset, reject new overlapping mappings, and live rule scope for new automatic runs.

Proposed engineering choices are V3 required scope, derived future QM task scope, explicit use-case grouping in existing analytics APIs, and serialized template Commit with expected revision recheck. The plan distinguishes these proposals from settled behavior and source facts. The separate [QM task work item](qm-task-template-scope.md) retains the detailed task-specific continuation.

## Findings and Decisions

Latest source still has V1/V2-only parsing/AutoQM gates, singleton trigger authoring, first-use-case task backfill, scope-before-latest template lookup, singular task discovery, and missing selected-use-case filters in two QM report scorecard branches. Current ClickHouse builder contains newer conversation-version handling that must be preserved.

## Blockers and Dependencies

Review V3/API contracts, full-definition edit authorization, direct/background readers, legacy task/multi-trigger inventory, Opera eligibility freshness, lifecycle behavior, and export/Coaching compatibility. No customer data audit or performance/production verification was done. Company 11X API/DB guidance was referenced by the supplied template but not supplied for conformance review.

## Validation and Rollout

Documentation/source review only. Main draft has concrete schema/JSON/Go/protobuf/query examples and end-to-end acceptance cases; effort is a rough 19–29 SWE-week range, not a commitment. Reader-first rollout must retain V3 readers after writes are disabled. Validation results are in the session note.

## Next Actions

1. Review proposed engineering choices and assign API/scoring/analytics/QM/security/export owners.
2. Inventory readers/writers/jobs and audit legacy data in explicitly selected environments.
3. Resolve the exact-selection authorization contract, then establish load baselines and staffed estimates before implementation/rollout.

## Timeline

- 2026-11-04 — Expanded the company-format plan, rechecked three latest source refs, obtained five product answers, and recorded review questions plus release/testing plans. Evidence: [session](../sessions/2026-11-04/codex-omni-usecase-full-engineering-plan.md), [decision](../decisions/2026-11-04-omni-usecase-autoqm-and-reporting.md).

- 2026-11-07 — Expanded PI/QM caller→API→SQL/ClickHouse evidence and observed Walter staging URLs. Preserved nine proposed tickets locally; the issues initially created in Linear were deleted at user request. [Inventory](../deliverables/cross-usecase-pi-qm-change-inventory.md), [draft list](../deliverables/cross-usecase-pi-qm-ticket-drafts.md). Live POST bodies and authorization behavior remain unverified; no product changes.
