# CONVI-7280: Warn when module criteria reference deactivated Opera behaviors

**Status:** validating
**Primary domain:** `training-simulator`
**Primary subdomain:** `training-content`
**Official ticket:** [CONVI-7280](https://linear.app/cresta/issue/CONVI-7280/add-warning-for-opera-rules-deactivated-in-module-view)
**Last updated:** 2026-08-10

## Objective and Impact

- **Objective:** Surface the existing Performance Config warning pattern when a Training Simulator module references a deactivated Opera behavior, both in the modules table and on the affected evaluation criterion.
- **Customer/system impact:** Prevent supervisors from silently retaining module criteria that cannot produce Opera annotations and may consequently evaluate as pending or N/A.
- **Role:** investigated, designed, and implemented

## Scope

**In scope**

- Add an orange warning icon and tooltip beside affected module names in the modules table.
- Add orange warning text on each affected evaluation-criterion card in the module editor.
- Resolve current behavior activation from Opera behavior data already loaded through `useAutoQATriggers`.
- Preserve the saved criterion label when its behavior is inactive or archived.
- Add frontend regression coverage for active, inactive, archived, unresolved, and not-yet-loaded activation data.

**Non-goals**

- Do not change evaluation, scoring, or N/A semantics.
- Do not block save or automatically remove/replace criteria.
- Do not add proto/backend enrichment for this warning.
- Do not conflate deactivation with target-agent mismatch, missing/late annotations, runtime N/A, or the future Training-Simulator-only applicability proposed by CONVI-7281.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/director` (source); `/Users/xuanyu.wang/repos/cresta-proto` and `/Users/xuanyu.wang/repos/go-servers` (contract verification only)
- **Worktrees:** `/Users/xuanyu.wang/repos/director-convi-7280`
- **Branches:** `convi-7280-add-warning-for-opera-rules-deactivated-in-module-view`, based on `origin/main` at `9bc70a61875e4191446839715294b73c1711f809`
- **PRs/commits:** none yet; implementation is uncommitted in the ticket worktree

## Current Understanding

This is a frontend-only `director` change. `ListTrainingModules` already returns every criterion's stored `behavior_id`, while `useAutoQATriggers` returns the current Opera behaviors with ACTIVE, INACTIVE, and ARCHIVED status. The UI can therefore join module criteria to current behavior state without changing the Training Simulator proto or backend.

The ticket explicitly requires two warning surfaces: the modules table and the affected evaluation-criterion card. The existing Performance Config pattern uses `IconAlertCircle`, `SharedTooltip`, orange design tokens, table copy “This template includes a deactivated behavior,” and criterion copy “This criterion is linked to a deactivated behavior.”

Implementation is complete in `/Users/xuanyu.wang/repos/director-convi-7280`: a shared activation lookup normalizes full behavior resource names and bare IDs; `ModulesTab` aggregates affected criteria into an orange tooltip icon; `EvaluationCriteriaStep` marks affected `CriterionListCard`s and preserves inactive/archived labels. Generated en-US locale entries and targeted regression tests are included.

## Findings and Decisions

- `EvaluationCriterion.behavior_id` is a behavior ID, not a policy/rule ID despite the current proto comment. Policy deactivation propagates to behavior INACTIVE; policy archive/delete propagates to behavior ARCHIVED.
- Module rows have the full `TrainingModule`, including `evaluation_config.criteria`, so the table can derive an aggregate warning after loading Opera behaviors.
- Module edit form values use the full behavior resource name; table payloads normally use the bare behavior ID. Activation matching must normalize both representations.
- `EvaluationCriteriaStep` currently builds its label lookup from active behaviors only. A referenced inactive behavior can therefore render as “Select a behavior” even though the criterion remains linked. The warning change should preserve the stored display name and/or include inactive/archived behaviors in the lookup-only path.
- Unknown behavior IDs should be treated as non-active after behavior data has loaded, matching Performance Config's fail-closed lookup. No warning should render while activation data is unavailable, avoiding false positives during loading/error.
- Deactivation is configuration state only. Runtime N/A, target-agent constraints, and missing annotations must not drive this warning.
- Prefer a small Training Simulator activation resolver over importing the admin feature's Zustand store across feature boundaries. It should build an O(behaviors) lookup once and support both full resource names and bare IDs.

## Blockers and Dependencies

- No product or API blocker.
- CONVI-7281 may later introduce product-area applicability. Keep the helper/status naming specific to activation so “not applicable to Training Simulator” can remain a distinct future state.
- The local `director` checkout was 1,109 commits behind before fetching and contains an unrelated modification to `useMomentGroupFilterFromFilterState.ts`; do not implement directly in that checkout.

## Validation and Rollout

- Targeted frontend tests passed: 4 files, 12 tests covering activation normalization, aggregate state, table icon rendering, criterion warning rendering, and inactive-label lookup.
- `yarn tsc` passed across Director workspaces.
- `yarn workspace @cresta/director-app lint:precommit` and targeted Biome checks passed.
- `yarn i18n:full-extract` generated the two en-US warning keys. It also reported the repository's pre-existing `GlobalAlerts.tsx` dynamic-key extraction warning but completed successfully.
- Verify active criteria have no warning; INACTIVE, ARCHIVED, and unresolved references warn only after trigger data loads.
- Verify targeted-but-active and runtime-N/A criteria do not warn.
- Verify warning disappears after replacing the criterion with an active behavior or after refreshed Opera data reports it active.
- No feature flag or staged backend rollout is required; this is additive warning UI.

## Next Actions

1. Manually verify both warning surfaces in a customer profile containing an inactive/archived Opera behavior.
2. Review the uncommitted diff.
3. Commit, push, and open a PR when requested.

## Timeline

- 2026-08-10 — Investigated Linear requirements, reference screenshots, current `director/origin/main`, Performance Config precedent, Training Simulator frontend data flow, and backend/proto contracts; concluded on a frontend-only two-surface warning plan. Evidence: `log/2026-08-10.md`, `sessions/2026-08-10/codex-convi-7280.md`.
- 2026-08-10 — Created clean implementation worktree `/Users/xuanyu.wang/repos/director-convi-7280` on the ticket branch at `9bc70a6187`; left the modified source checkout untouched.
- 2026-08-10 — Implemented normalized Opera activation lookup, module-table and criterion-card warnings, inactive-label preservation, generated i18n keys, and 12 passing targeted tests; full TypeScript and changed-file lint checks passed.
