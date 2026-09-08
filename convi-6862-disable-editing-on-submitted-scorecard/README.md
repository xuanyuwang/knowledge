# CONVI-6862 - Disable Editing on Submitted Scorecard

> Migrated navigation: [Scorecard Workflows / Permissions and Visibility](../scorecard-workflows/subdomains/permissions-and-visibility/README.md). This folder remains detailed ticket evidence.

**Created:** 2026-05-19  
**Updated:** 2026-09-02

## Overview

This project captures the merged backend contract, the current frontend implementation, the scorecard-permission evaluation API, and the local validation plan for CONVI-6862.

The active requirement set still comes from the 2026-05-22 product clarification on scope, but the implementation contract has now converged on `submitted_scorecard_editors` / `submittedScorecardEditors` with a `users + teams + groups` shape.

## Current Objective

Complete the staged GA rollout by enabling `disableEditingOnSubmittedScorecards` for remaining production profiles through customer configuration.

## Current Scope

In scope:

- normal Closed Conversations scorecards
- normal process scorecards
- post-submit lock for criteria edits, criterion comments, general notes editing, and reset

Out of scope:

- appeal request
- appeal resolve
- group calibration answer key
- group calibration response

Submit remains a first-submit action for unsubmitted scorecards.

## Key Findings

- Backend is merged and uses `submitted_scorecard_editors` / `submittedScorecardEditors` as a `users + teams + groups` permission object.
- Empty or unset submitted editors fall back to the existing edit-permission behavior on the backend.
- `ResetScorecard` is now explicitly inside the submitted-lock scope for this iteration.
- Frontend exposes submitted-scorecard editors in `TemplateBuilderAdvanced`.
- Frontend now hydrates and saves users, teams, and groups for `submittedScorecardEditors`.
- Frontend does not filter the submitted-editor selector by `permissions.scorecardGraders`, and changing `Who can use this scorecard` does not clear submitted editors.
- The submitted-editor empty state in FE is `All users`.
- `director#19805` added the frontend client wrapper and reusable hook for `EvaluateScorecardsPermissions`.
- Runtime FE should query `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD` when loading submitted scorecards and lock denied scorecards before any update attempt.
- Reactive `UpdateScorecard` 403 fail-and-freeze handling remains as a fallback for stale permissions or permission changes after load.
- The submitted-lock inline warning copy is `You do not have permission to edit this scorecard`.
- The older audience-style permitted-user pivot is now historical context only and should not be treated as the active contract.
- **CONVI-7197 (merged `director#20375`):** Scorecard editors dropdown UX fix — Floating UI flip on `UserTeamGroupPopover` caused the menu to jump below the input when search shrank the dropdown near the bottom of the Access tab; fixed by pinning `top-start` and disabling flip on the submitted-editors selector only.
- **CONVI-7206 (`go-servers#29774`):** Backend submitted-scorecard editor enforcement now reads the same per-customer `disableEditingOnSubmittedScorecards` Director config flag as the frontend. This is a documented compromise to keep FE/BE rollout synchronized under one same-team config knob.
- Proactive permission evaluation currently reads the latest template revision, while `UpdateScorecard` and `ResetScorecard` enforce submitted editors from the scorecard's pinned revision. Allowlist changes after scorecard creation can therefore produce an editable frontend followed by a backend 403.
- **CONVI-7350:** Implemented on `go-servers-convi-7350`. `hasSubmittedScorecardEditPermission` now loads `LatestRevisionName` internally so update/reset enforcement matches proactive evaluation, while pinned revisions remain for scorecard content and scoring.
- Director currently fails open after an `EvaluateScorecardsPermissions` query error because an undefined decision is not treated as denied.
- **GA direction update (2026-08-10):** The earlier unconditional-runtime and flag-removal PRs were closed without merge. GA is proceeding through staged customer configuration rollouts. [config#151187](https://github.com/cresta/config/pull/151187) enables the flag for 44 supplied us-west-2 production profiles.
- **RCG denial investigation (2026-08-25):** Canonical scorecard `019f386e-a934-7271-93ae-838feb4468be` is pinned to `019f1b3c-ee76-724c-9dd5-020f2f41955c@28a6df1c`. That template resource's latest revision is inactive and none of its revisions has submitted editors; similarly named active replacement templates do not affect authorization for this scorecard. Production `EvaluateScorecardsPermissions` returns `allowed: true` for both Quality Assurance 40 (`829fd45d23e3d691`) and Quality Assurance 44 (`83f68adf7584b433`), so their reported block is not reproduced by the backend permission gate.
- **RCG frontend root cause (2026-08-25):** The canonical scorecard was acknowledged on Aug 3. Director's `ScorecardForm` has a legacy independent gate that disables acknowledged scorecards unless the frontend role includes `QA_ADMIN`; QM Specialist (`QA_SPECIALIST`) is therefore locked even when the submitted-scorecard permission API explicitly allows editing. This exact logic is deployed in production commit `06cf4817ec3214a6a7b238f217f1bb71e7c9f6dd`. It was intentionally introduced by CONVI-5098 / [director#13338](https://github.com/cresta/director/pull/13338) in August 2025; the newer submitted-editor integration added another lock path without reconciling this existing policy. Reset also has a separate creator/admin restriction.
- **Lock explanation follow-up:** [CONVI-7598](https://linear.app/cresta/issue/CONVI-7598/explain-scorecard-lock-reasons) introduces typed lock reasons and consistent reason-specific warning/tooltip copy without changing authorization behavior. Implementation is in draft [director#22149](https://github.com/cresta/director/pull/22149).
- **CONVI-7612 review:** [director#22401](https://github.com/cresta/director/pull/22401) overlaps #22149 and adds Cresta-admin lock bypasses backed by [go-servers#31889](https://github.com/cresta/go-servers/pull/31889). Its current head splits visual lock state from the autosave/write guard: `lockReason` disables fields, while `useSaveScorecardMutation` still receives the narrower legacy `readOnly` value. This must be reconciled before merge.

## Status

Active

## Source Context

- **Primary repo:** `director`
- **Repo path:** `/Users/xuanyu.wang/repos/director`
- **Active FE worktree:** `/Users/xuanyu.wang/repos/director-convi-6862`
- **Active FE branch:** `xwang/convi-6862-submitted-scorecard-editors-v2`

Investigation and implementation touch:

- `director`
- `go-servers`

## Log History

| Date | Summary |
|------|---------|
| 2026-05-19 | Created the project and drafted the initial role-based hard-lock design. |
| 2026-05-22 | Refreshed scope from the Linear thread, pivoted proto planning to audience-style permitted users, and prepared the backend branch reset. |
| 2026-05-26 | Verified the landed proto shape is still role-based and documented that current template audience resolution is runtime-based with existing `teams` handling gaps. |
| 2026-05-29 | Documented the merged backend contract, corrected the FE behavior docs, and added a detailed local FE test plan. |
| 2026-06-17 | Documented merged `EvaluateScorecardsPermissions` FE client work and updated the active runtime plan to proactive lock on load with reactive fallback. |
| 2026-07-02 | Triaged FE/BE feature-flag mismatch (CONVI-7206); fixed Scorecard editors dropdown placement bug (CONVI-7197). |
| 2026-07-03 | Merged `director#20375`; documented root cause and solution for CONVI-7197. |
| 2026-07-08 | Prepared `go-servers#29774` for CONVI-7206; fixed the failed coaching coverage job by adding the missing reset-suite config mock and re-triggered CI. |
| 2026-07-22 | Diagnosed submitted-editor FE/BE disagreement as latest-versus-pinned template revision authorization, plus a Director permission-query fail-open edge. |
| 2026-08-06 | Opened coordinated Director, go-servers, and config PRs to make submitted-scorecard restrictions unconditional and remove the retired flag. |
| 2026-08-10 | Recorded closure of the flag-removal PRs and opened the us-west-2 staged config rollout for 44 profiles. |
| 2026-08-25 | Correlated an RCG QA Specialist reset 403 with production permission logs and confirmed the requester was absent from the persisted submitted-editor user IDs; ruled out role and ACL filtering as the root cause. |
| 2026-09-02 | Reviewed overlapping Director PR #22401 against completed draft #22149 and found a merge-blocking visual/write lock-state mismatch. |

## Related Artifacts

- `project.yaml`
- `log/2026-05-19.md`
- `log/2026-05-22.md`
- `log/2026-05-29.md`
- `log/2026-06-17.md`
- `log/2026-07-02.md`
- `log/2026-07-03.md`
- `log/2026-07-08.md`
- `log/2026-07-22.md`
- `log/2026-08-06.md`
- `log/2026-08-10.md`
- `sessions/2026-05-19/codex-requirements-and-design.md`
- `sessions/2026-07-02/codex-convi-7197-dropdown-placement-fix.md`
- `sessions/2026-07-02/codex-fe-be-feature-flag-mismatch-bug.md`
- `sessions/2026-07-08/codex-convi-7206-be-feature-flag-gating.md`
- `sessions/2026-07-22/codex-submitted-editor-update-403.md`
- `sessions/2026-07-22/codex-latest-template-permission-evaluation.md`
- `sessions/2026-08-06/codex-ga-flag-override.md`
- `sessions/2026-08-10/codex-convi-7386-us-west-2-ga.md`
- `work-items/CONVI-6862.md`
- `work-items/CONVI-7350.md`
- `deliverables/convi-7197-scorecard-editors-dropdown-ux-fix.md`
- `decisions/2026-05-19-separate-post-submit-permission.md`
- `decisions/2026-05-22-permitted-users-audience-pivot.md`
- `decisions/2026-05-29-final-fe-submitted-editor-behavior.md`
- `deliverables/plan.md`
- `deliverables/fe-plan.md`
- `deliverables/eng-design-doc.md`
- `deliverables/local-test-plan.md`