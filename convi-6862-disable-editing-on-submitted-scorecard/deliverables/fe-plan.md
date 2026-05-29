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

## UX Details

- The control is labeled `Scorecard editors`.
- Helper text is:
  - `Who can edit this scorecard after submission (e.g. change the scorecard evaluation)`
- The control lives in the `Advanced` section for both process and conversation templates.
- The control is disabled when the template is Cresta-only, consistent with the surrounding permission controls.

## FE Data Flow

- On template load, FE maps `submittedScorecardEditors.users`, `.teams`, and `.groups` into `UserTeamGroupSelection`.
- On save, FE serializes `userNames`, `teamNames`, and `groupNames` back into `submittedScorecardEditors.users`, `.teams`, and `.groups`.
- FE continues to rely on backend enforcement for the true post-submit authorization decision.

## Testing Expectations

High-signal FE validation should confirm:

- the submitted-editor control renders in `TemplateBuilderAdvanced`
- users, teams, and groups are available in the picker
- changing `Who can use this scorecard` does not clear the selection
- saved submitted-editor selections persist after reopening the template
- the empty placeholder reads `All users`
