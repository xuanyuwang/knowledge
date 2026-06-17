# Submitted Scorecard Edit Permission Fail-and-Freeze Plan

## Summary

Implement proactive load-time permission locking for submitted scorecards, backed by the existing reactive fail-and-freeze approach.

When a submitted scorecard is loaded, the frontend should call `EvaluateScorecardsPermissions` for `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`.

If the permission result is denied, the frontend should:

- freeze editing before any update operation is attempted
- show an inline warning banner at the top of the scorecard
- prevent autosave/update/reset-style UI actions for that scorecard

If `UpdateScorecard` later fails with a permission-denied error for an already-submitted scorecard, the frontend should still:

- restore the UI to the last persisted scorecard state
- freeze further editing on that scorecard for the current screen session
- show an inline warning banner at the top of the scorecard
- suppress the handled permission toast for that case

Do not implement a preflight "sniffing" `UpdateScorecard` call. In the current API surface it would still be a real write request, adds duplicate traffic, and still would not remove the need for reactive handling.

## Key Changes

### Shared error handling

- Use `useEvaluateScorecardPermissions` to query submitted-scorecard permissions on load.
- Request only `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD` for this feature.
- Treat `allowed=false` as a proactive submitted-scorecard edit lock.
- Detect forbidden update failures with `CrestaApiServiceError.isForbiddenError()`
- Only use the new freeze flow for `UpdateScorecard` on already-submitted scorecards
- Preserve existing toast behavior for all other failures
- Keep reactive handling even after proactive locking because permission data can become stale.

### Conversation scorecard flow

Apply the behavior in:

- `packages/director-app/src/components/scoring/hooks/useSaveScorecardMutation.ts`
- `packages/director-app/src/components/scoring/scorecard-form/ScorecardForm.tsx`

Changes:

- query `EvaluateScorecardsPermissions` for the selected submitted scorecard when the feature flag is enabled
- if denied, set the submitted-edit lock before form changes are possible
- add local frozen state for runtime post-submit permission denial
- on forbidden update failure:
  - reset the form to the last persisted scorecard snapshot
  - freeze the form using the existing read-only/disabled pipeline
  - show a warning banner in the existing alert stack above the performance-score row
- stop future autosave attempts once frozen

Banner copy:

`You do not have permission to edit this scorecard`

### Process scorecard flow

Apply the behavior in:

- `packages/director-app/src/hooks/coaching/useUpdateScorecard.ts`
- `packages/director-app/src/components/qa/process-scorecard-scoring/ProcessScorecardScoring.tsx`
- `packages/director-app/src/components/qa/process-scorecard-scoring/process-scorecard-criterion/ProcessScorecardCriteria.tsx`

Changes:

- query `EvaluateScorecardsPermissions` for the selected submitted process scorecard when the feature flag is enabled
- if denied, set the submitted-edit lock before form changes are possible
- add local frozen state for runtime post-submit permission denial
- on forbidden update failure:
  - reset the form to the last persisted scorecard snapshot
  - freeze the form
  - show a warning banner above the process scorecard header
- stop future autosave attempts once frozen
- make process criterion inputs and notes honor read-only mode

## Public Interface Changes

- `useUpdateScorecard`
  - add optional forbidden-error handling hooks so callers can suppress the default toast and run rollback/freeze behavior

- `useSaveScorecardMutation`
  - add optional forbidden-error handling hooks so `ScorecardForm` can drive rollback/freeze behavior

- `useEvaluateScorecardPermissions`
  - already added by merged `director#19805`
  - used by conversation and process scorecard loading flows to pre-lock denied submitted scorecards

- `ProcessScorecardCriteria`
  - add optional `readOnly?: boolean`

## Test Plan

### Unit / behavior checks

- forbidden submitted-scorecard update triggers rollback and freeze
- denied `EvaluateScorecardsPermissions` result freezes before update
- non-forbidden update failures still show the existing toast path
- handled permission denial no longer shows the generic permission toast

### Conversation scorecard checks

- edit an already-submitted scorecard, mock forbidden update:
  - values revert
  - banner appears
  - form becomes read-only
  - autosave stops
- load an already-submitted scorecard with denied `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`:
  - banner appears immediately
  - form is read-only
  - autosave does not fire

### Process scorecard checks

- edit an already-submitted process scorecard, mock forbidden update:
  - values revert
  - banner appears
  - inputs and notes become read-only
  - submit and reset become non-actionable
  - autosave stops
- load an already-submitted process scorecard with denied `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`:
  - banner appears immediately
  - criteria and notes are read-only
  - reset and submit-style actions are non-actionable

### Manual acceptance

- editable submitted scorecards still save normally
- denied submitted scorecards are locked before an update attempt
- stale forbidden submitted scorecards fail gracefully and freeze
- non-permission failures still behave as before

## Assumptions

- backend exposes `EvaluateScorecardsPermissions` and supports `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`
- backend continues returning `403` / `PERMISSION_DENIED` for stale runtime permission denial
- freeze only needs to last for the current loaded scorecard session
- the new banner replaces the handled toast for this case rather than supplementing it
