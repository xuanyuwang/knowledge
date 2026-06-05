# Submitted Scorecard Edit Permission Fail-and-Freeze Plan

## Summary

Implement the reactive fail-and-freeze approach for submitted scorecards.

When `UpdateScorecard` fails with a permission-denied error for an already-submitted scorecard, the frontend should:

- restore the UI to the last persisted scorecard state
- freeze further editing on that scorecard for the current screen session
- show an inline warning banner at the top of the scorecard
- suppress the handled permission toast for that case

Do not implement a preflight "sniffing" `UpdateScorecard` call. In the current API surface it would still be a real write request, adds duplicate traffic, and still would not remove the need for reactive handling.

## Key Changes

### Shared error handling

- Detect forbidden update failures with `CrestaApiServiceError.isForbiddenError()`
- Only use the new freeze flow for `UpdateScorecard` on already-submitted scorecards
- Preserve existing toast behavior for all other failures

### Conversation scorecard flow

Apply the behavior in:

- `packages/director-app/src/components/scoring/hooks/useSaveScorecardMutation.ts`
- `packages/director-app/src/components/scoring/scorecard-form/ScorecardForm.tsx`

Changes:

- add local frozen state for runtime post-submit permission denial
- on forbidden update failure:
  - reset the form to the last persisted scorecard snapshot
  - freeze the form using the existing read-only/disabled pipeline
  - show a warning banner in the existing alert stack above the performance-score row
- stop future autosave attempts once frozen

Banner copy:

`You no longer have permission to edit this submitted scorecard. Your latest changes were not saved.`

### Process scorecard flow

Apply the behavior in:

- `packages/director-app/src/hooks/coaching/useUpdateScorecard.ts`
- `packages/director-app/src/components/qa/process-scorecard-scoring/ProcessScorecardScoring.tsx`
- `packages/director-app/src/components/qa/process-scorecard-scoring/process-scorecard-criterion/ProcessScorecardCriteria.tsx`

Changes:

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

- `ProcessScorecardCriteria`
  - add optional `readOnly?: boolean`

## Test Plan

### Unit / behavior checks

- forbidden submitted-scorecard update triggers rollback and freeze
- non-forbidden update failures still show the existing toast path
- handled permission denial no longer shows the generic permission toast

### Conversation scorecard checks

- edit an already-submitted scorecard, mock forbidden update:
  - values revert
  - banner appears
  - form becomes read-only
  - autosave stops

### Process scorecard checks

- edit an already-submitted process scorecard, mock forbidden update:
  - values revert
  - banner appears
  - inputs and notes become read-only
  - submit and reset become non-actionable
  - autosave stops

### Manual acceptance

- editable submitted scorecards still save normally
- forbidden submitted scorecards fail gracefully and freeze
- non-permission failures still behave as before

## Assumptions

- backend continues returning `403` / `PERMISSION_DENIED` for this runtime permission denial
- freeze only needs to last for the current loaded scorecard session
- the new banner replaces the handled toast for this case rather than supplementing it
