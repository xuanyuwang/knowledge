# CONVI-6862 Backend Contract Summary

## Summary

This document is now a summary of the merged backend behavior for CONVI-6862.

The original milestone plan in this project has been implemented. Keep that history in git, but use the behavior below as the active source of truth.

## Merged Backend Behavior

- Backend contract uses `submitted_scorecard_editors` / `submittedScorecardEditors`.
- The field shape is `users`, `teams`, and `groups`, following the existing `UserTeamGroup` runtime model.
- Backend resolves submitted-editor eligibility at runtime.
- Empty or unset submitted editors fall back to the existing edit-permission behavior.
- Submitted-lock enforcement covers:
  - criteria value updates
  - criterion comments
  - general notes edits
  - reset scorecard
- First submit remains allowed for an unsubmitted scorecard.
- In scope:
  - normal Closed Conversations scorecards
  - normal process scorecards
- Out of scope:
  - appeal request scorecards
  - appeal resolve scorecards
  - group calibration answer key scorecards
  - group calibration response scorecards

## Backend Implementation Notes

- Post-submit admins do not bypass an explicit submitted-editor configuration.
- Runtime group and team membership is evaluated dynamically on the backend.
- The merged implementation uses the shipped role/resource contract, not the earlier audience-style design pivot.

## Historical Milestones

The original execution plan was completed in these logical milestones:

1. Establish the submitted-editor contract and permission plumbing.
2. Refactor runtime resolution for user/team/group references.
3. Enforce submitted-editor checks in `UpdateScorecard`.
4. Enforce submitted-editor checks in `ResetScorecard`.
5. Add regression and edge-case coverage.

These milestones remain useful for understanding how the backend was delivered, but they are no longer active implementation instructions.

## Acceptance Semantics

The backend behavior is correct only if all of the following remain true:

- submitted in-scope scorecards are mutable only by configured submitted editors when configured
- empty submitted-editor configuration preserves current edit-permission behavior
- unsubmitted scorecards behave as before
- reset follows the same submitted-lock rule set as update
- team and group membership is resolved dynamically at runtime
