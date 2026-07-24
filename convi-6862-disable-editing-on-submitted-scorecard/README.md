# CONVI-6862 - Disable Editing on Submitted Scorecard

> Migrated navigation: [Scorecard Workflows / Permissions and Visibility](../scorecard-workflows/subdomains/permissions-and-visibility/README.md). This folder remains detailed ticket evidence.

**Created:** 2026-05-19  
**Updated:** 2026-07-22

## Overview

This project captures the merged backend contract, the current frontend implementation, the scorecard-permission evaluation API, and the local validation plan for CONVI-6862.

The active requirement set still comes from the 2026-05-22 product clarification on scope, but the implementation contract has now converged on `submitted_scorecard_editors` / `submittedScorecardEditors` with a `users + teams + groups` shape.

## Current Objective

Keep the knowledge docs aligned with the merged backend behavior and the current frontend behavior, and update PR `director#19100` to use scorecard permission evaluation for proactive submitted-scorecard locking.

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
- `sessions/2026-05-19/codex-requirements-and-design.md`
- `sessions/2026-07-02/codex-convi-7197-dropdown-placement-fix.md`
- `sessions/2026-07-02/codex-fe-be-feature-flag-mismatch-bug.md`
- `sessions/2026-07-08/codex-convi-7206-be-feature-flag-gating.md`
- `sessions/2026-07-22/codex-submitted-editor-update-403.md`
- `sessions/2026-07-22/codex-latest-template-permission-evaluation.md`
- `work-items/CONVI-7350.md`
- `deliverables/convi-7197-scorecard-editors-dropdown-ux-fix.md`
- `decisions/2026-05-19-separate-post-submit-permission.md`
- `decisions/2026-05-22-permitted-users-audience-pivot.md`
- `decisions/2026-05-29-final-fe-submitted-editor-behavior.md`
- `deliverables/plan.md`
- `deliverables/fe-plan.md`
- `deliverables/eng-design-doc.md`
- `deliverables/local-test-plan.md`