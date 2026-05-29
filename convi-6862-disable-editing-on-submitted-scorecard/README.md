# CONVI-6862 - Disable Editing on Submitted Scorecard

**Created:** 2026-05-19  
**Updated:** 2026-05-29

## Overview

This project captures the merged backend contract, the current frontend implementation, and the local validation plan for CONVI-6862.

The active requirement set still comes from the 2026-05-22 product clarification on scope, but the implementation contract has now converged on `submitted_scorecard_editors` / `submittedScorecardEditors` with a `users + teams + groups` shape.

## Current Objective

Keep the knowledge docs aligned with the merged backend behavior and the current frontend behavior, and provide a concrete local FE validation checklist.

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
- The older audience-style permitted-user pivot is now historical context only and should not be treated as the active contract.

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

## Related Artifacts

- `project.yaml`
- `log/2026-05-19.md`
- `log/2026-05-22.md`
- `log/2026-05-29.md`
- `sessions/2026-05-19/codex-requirements-and-design.md`
- `decisions/2026-05-19-separate-post-submit-permission.md`
- `decisions/2026-05-22-permitted-users-audience-pivot.md`
- `decisions/2026-05-29-final-fe-submitted-editor-behavior.md`
- `deliverables/plan.md`
- `deliverables/fe-plan.md`
- `deliverables/eng-design-doc.md`
- `deliverables/local-test-plan.md`
