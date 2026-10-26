# CONVI-6862 Frontend Implementation Summary

## Summary

This document captures the current frontend implementation behavior in Director.

Frontend configuration now follows the merged backend contract instead of the earlier user-only / grader-filtered FE plan.

## Active FE Behavior

- Director renders submitted-scorecard editor configuration in `TemplateBuilderAdvanced`.
- FE stores submitted editors in template-builder form state as `UserTeamGroupSelection`.
- FE hydrates and saves all three resource buckets:
  - users
  - teams
  - groups
- FE does not attempt to resolve runtime eligibility locally.
- FE does not filter the submitted-editor selector by `permissions.scorecardGraders`.
- Changing `Who can use this scorecard` does not clear submitted editors.
- Empty-state text is `All users`.
- Runtime submitted-scorecard edit eligibility should be queried through `EvaluateScorecardsPermissions` instead of inferred from template configuration in the browser.

## UX Details

- The control is labeled `Scorecard editors`.
- Helper text is:
  - `Who can edit this scorecard after submission (e.g. change the scorecard evaluation)`
- The control lives in the `Advanced` section for both process and conversation templates.
- The control is disabled when the template is Cresta-only, consistent with the surrounding permission controls.
- When a submitted scorecard is locked because the current user cannot modify submitted-lock-protected scorecards, the inline warning says:
  - `You do not have permission to edit this scorecard`

## FE Data Flow

- On template load, FE maps `submittedScorecardEditors.users`, `.teams`, and `.groups` into `UserTeamGroupSelection`.
- On save, FE serializes `userNames`, `teamNames`, and `groupNames` back into `submittedScorecardEditors.users`, `.teams`, and `.groups`.
- On scorecard load, FE queries `EvaluateScorecardsPermissions` for submitted scorecards and requests `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`.
- FE uses the permission result to lock denied submitted scorecards before any `UpdateScorecard` operation is attempted.
- FE keeps the existing `UpdateScorecard` 403 fail-and-freeze path as a fallback for stale permission data or permission changes after the scorecard was loaded.

## Testing Expectations

High-signal FE validation should confirm:

- the submitted-editor control renders in `TemplateBuilderAdvanced`
- users, teams, and groups are available in the picker
- changing `Who can use this scorecard` does not clear the selection
- saved submitted-editor selections persist after reopening the template
- the empty placeholder reads `All users`
- submitted scorecards denied by `EvaluateScorecardsPermissions` render read-only immediately
- denied submitted scorecards do not fire an initial `UpdateScorecard` attempt
- stale permission denials from `UpdateScorecard` still roll back and freeze the UI
