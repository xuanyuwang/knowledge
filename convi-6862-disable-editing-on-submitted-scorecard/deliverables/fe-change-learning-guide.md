# CONVI-6862 FE Change Learning Guide

Last reviewed: 2026-06-23

This guide explains the frontend implementation for disabling editing on submitted scorecards when the current user does not have the submitted-scorecard edit permission.

It covers both FE parts:

- template configuration: how admins configure who can edit submitted scorecards
- scorecard runtime: how the UI checks permission, locks submitted scorecards, and handles stale permission failures

## Mental Model

There are two separate responsibilities.

1. Template Builder stores the configured post-submit editors on the template.

The `Scorecard editors` selector writes `permissions.submittedScorecardEditors` with `users`, `teams`, and `groups`. This controls the backend permission rule, but the frontend does not evaluate that rule itself.

2. Scorecard pages ask the backend whether the current user can edit the loaded submitted scorecard.

When a submitted scorecard is displayed, the FE calls `EvaluateScorecardsPermissions` for `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`. If the backend returns `false`, the FE locks the scorecard before any user edit can trigger `UpdateScorecard`.

The backend remains the source of truth. The frontend only renders the configured selector and applies the backend permission result.

## Main Code Paths

### Template Configuration

These files own the template-builder part.

- `packages/director-app/src/features/admin/coaching/template-builder/steps/access/template-builder-advanced/TemplateBuilderAdvanced.tsx`
- `packages/director-app/src/features/admin/coaching/template-builder/TemplateBuilderForm.tsx`
- `packages/director-app/src/features/admin/coaching/template-builder/useSaveScorecardTemplate.ts`
- `packages/director-app/src/features/admin/coaching/template-builder/formTypes.ts`
- `packages/director-api/src/types/models/scoring.ts`
- `packages/director-api/src/services/cresta-api/coaching/transformers.ts`

`TemplateBuilderAdvanced.tsx` renders the user-facing control:

- section: `Advanced`
- label: `Scorecard editors`
- helper text: `Who can edit this scorecard after submission (e.g. change the scorecard evaluation)`
- placeholder: `All users`
- selector type: `UserTeamGroupSelect`
- supported entities: users, teams, groups
- feature flag: `disableEditingOnSubmittedScorecards`

The selector is intentionally not connected to `Who can use this scorecard`.

- Changing `permissions.scorecardGraders` does not filter available submitted editors.
- Changing `permissions.scorecardGraders` does not clear selected submitted editors.
- Empty submitted editors means the tile shows `All users`; the backend decides the effective fallback semantics.

`TemplateBuilderForm.tsx` hydrates API data into form state:

- API/model field: `template.permissions.submittedScorecardEditors`
- form field: `permissions.submittedScorecardEditors`
- form shape: `UserTeamGroupSelection`
- conversion:
  - `users[].name` -> `userNames`
  - `teams[].name` -> `teamNames`
  - `groups[].name` -> `groupNames`

`useSaveScorecardTemplate.ts` saves form state back to the API/model shape:

- `userNames` -> `users: [{ name }]`
- `teamNames` -> `teams: [{ name }]`
- `groupNames` -> `groups: [{ name }]`

`transformers.ts` normalizes missing backend values:

- if `submittedScorecardEditors` is missing, FE model uses empty arrays
- this preserves a stable frontend shape:
  - `users: []`
  - `teams: []`
  - `groups: []`

### Permission Query Bridge

These files own the shared permission-query path.

- `packages/director-app/src/hooks/coaching/useEvaluateScorecardPermissions.ts`
- `packages/director-api/src/services/cresta-api/coaching/coachingApi.ts`
- `packages/director-api/src/services/cresta-api/coaching/apiTypes.ts`
- `packages/director-api/src/hooks/queryUtils.ts`

`useEvaluateScorecardPermissions` accepts:

- `scorecardNames`
- `permissions`
- `enabled`

It normalizes scorecard names and permission enums, includes `requesterUserName` from the authenticated user, and calls:

```ts
CrestaAPI.coaching.evaluateScorecardsPermissions({
  scorecardNames,
  permissions,
  requesterUserName,
});
```

The API bridge sends:

- `parent`
- `scorecardNames`
- `permissions`
- `requesterUserName`

The result shape is:

```ts
Record<string, Partial<Record<ScorecardPermission, boolean>>>
```

For this feature, the only requested permission is:

```ts
ScorecardPermission.SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD
```

## When Permission Requests Are Sent

Permission requests are sent only for submitted scorecards and only when the feature flag is enabled.

### Conversation Scorecard

File:

- `packages/director-app/src/components/scoring/scorecard-form/ScorecardForm.tsx`

The query is enabled when all of these are true:

- `disableEditingOnSubmittedScorecards` is enabled
- `scorecard?.name` exists
- `scorecard.submittedAt` exists
- the scorecard is not in appeal mode

The loaded scorecard name is passed as a one-item list:

```ts
scorecardNames: [scorecard?.name]
```

### Process Scorecard

File:

- `packages/director-app/src/components/qa/process-scorecard-scoring/ProcessScorecardScoring.tsx`

The query is enabled when all of these are true:

- `disableEditingOnSubmittedScorecards` is enabled
- `currentScorecard?.name` exists
- `currentScorecard.submittedAt` exists

The selected process scorecard name is passed as a one-item list:

```ts
scorecardNames: [currentScorecard?.name]
```

### Why Loading State Locks

Both scorecard surfaces treat the permission query loading state as read-only for submitted scorecards.

This is intentional. It prevents the user from editing during the gap between rendering the submitted scorecard and receiving the backend permission result.

The states are:

- `submittedEditPermissionLoading`: query is in flight
- `submittedEditPermissionDenied`: backend returned `false`
- `submittedEditPermissionFrozen`: fallback local lock after a 403 update failure

The effective lock is:

```ts
submittedEditPermissionReadOnly =
  submittedEditPermissionFrozen ||
  submittedEditPermissionDenied ||
  submittedEditPermissionLoading
```

Conversation scorecards also require the feature flag and exclude appeal mode from this lock.

## Conversation Scorecard Runtime Flow

Main files:

- `packages/director-app/src/components/scoring/scorecard-form/ScorecardForm.tsx`
- `packages/director-app/src/components/scoring/hooks/useSaveScorecardMutation.ts`

### Load-Time Lock

`ScorecardForm.tsx` computes:

- `shouldEvaluateSubmittedEditPermission`
- `submittedEditPermissionAllowed`
- `submittedEditPermissionDenied`
- `submittedEditPermissionLoading`
- `submittedEditPermissionReadOnly`

If `submittedEditPermissionReadOnly` is true, it is folded into existing scorecard read-only logic:

- `disableScorecardForm`
- `readOnly` passed into `ScorecardFormItems`
- submit button and related edit controls
- director task selector visibility
- form watcher/autosave subscription

The warning banner appears when permission is explicitly denied or the UI is frozen after a failed update:

```text
You do not have permission to edit this scorecard
```

The banner does not appear only because the query is loading. Loading simply blocks edits temporarily.

### Autosave Path

Conversation scorecards use `useSaveScorecardMutation`.

Typical edit flow:

1. User changes a scorecard field.
2. `form.watch` notices the change.
3. `onFormChange` debounces the save.
4. `saveScorecard` calls `UpdateScorecard` for autosave.
5. On success, cache/query state is invalidated and the scorecard cache is updated.

When `submittedEditPermissionReadOnly` is true:

- the form watcher is not subscribed
- `useSaveScorecardMutation` receives `readOnly: true`
- mutation exits early before creating an update payload
- no `UpdateScorecard` call should be sent from user edits

### Fallback 403 Lock

The load-time permission check can become stale. For example, permission may be revoked after the page loaded.

If `UpdateScorecard` returns a forbidden error for an already-submitted scorecard:

1. `useSaveScorecardMutation` detects:
   - not appeal mode
   - `currentScorecard?.submittedAt`
   - `err instanceof CrestaApiServiceError`
   - `err.isForbiddenError()`
2. It calls `onForbiddenSubmittedEditError`.
3. `ScorecardForm.tsx` handles that callback by:
   - setting `submittedEditPermissionFrozen` to `true`
   - resetting the form to the last persisted scorecard snapshot
   - returning `true` to mark the error handled
4. The mutation logs the error to console but does not show the old generic permission toast.
5. The scorecard becomes read-only and shows the inline warning.

This is the "reactive fail-and-freeze" fallback.

## Process Scorecard Runtime Flow

Main files:

- `packages/director-app/src/components/qa/process-scorecard-scoring/ProcessScorecardScoring.tsx`
- `packages/director-app/src/components/qa/process-scorecard-scoring/process-scorecard-criterion/ProcessScorecardCriteria.tsx`
- `packages/director-app/src/components/qa/process-scorecard-scoring/process-scorecard-item/ProcessScorecardItem.tsx`
- `packages/director-app/src/hooks/coaching/useUpdateScorecard.ts`

### Load-Time Lock

`ProcessScorecardScoring.tsx` computes the same permission states for `currentScorecard`.

If the current scorecard is submitted and the backend returns denied:

- the warning banner renders above the process scorecard header
- criteria inputs receive `readOnly`
- general notes receive `readOnly`
- the reset path is made non-actionable through the existing reset-button disabled/reference path
- the submit button is disabled
- autosave is blocked

The same warning copy is used:

```text
You do not have permission to edit this scorecard
```

### Autosave Path

Process scorecards have a different autosave path from conversation scorecards.

Typical edit flow:

1. User changes a process scorecard criterion or note.
2. A form watcher calls debounced `onFormChange`.
3. `onFormChange` verifies required criteria/comments are complete.
4. `handleSubmitForm` builds scorecard data from form values.
5. Existing scorecards call `updateScorecard`.
6. `useUpdateScorecard` calls `CrestaAPI.coaching.updateScorecard`.

When `submittedEditPermissionReadOnly` is true:

- `onFormChange` exits before calling `handleSubmitForm`
- `handleSubmitForm` also exits early if called directly
- criterion inputs are read-only/disabled
- note inputs are read-only
- submit/reset controls are disabled or made non-actionable

### Fallback 403 Lock

Process scorecards use `useUpdateScorecard` directly.

`useUpdateScorecard` has an optional `onForbiddenError` callback and `suppressForbiddenToast` option.

`ProcessScorecardScoring.tsx` passes:

- `onForbiddenError`
- `suppressForbiddenToast: true`

If `UpdateScorecard` returns a forbidden error for a submitted process scorecard:

1. `useUpdateScorecard` detects `CrestaApiServiceError.isForbiddenError()`.
2. It calls `onForbiddenError`.
3. `ProcessScorecardScoring.tsx` handles that callback by:
   - checking the feature flag
   - checking `currentScorecard?.submittedAt`
   - setting `submittedEditPermissionFrozen` to `true`
   - resetting form defaults back to `currentScorecard`
   - returning `true`
4. `useUpdateScorecard` suppresses the generic forbidden toast because the error was handled.
5. The process scorecard locks and shows the inline warning.

## How A Scorecard Becomes Locked After Submission

Submission itself does not immediately mean "locked for everyone."

The lock depends on the backend permission result.

### Before Submission

Before submission:

- no submitted-edit permission query is sent
- normal scorecard edit permissions still apply
- autosave and submit behavior are unchanged

### After Submission

After submission:

1. The scorecard has `submittedAt`.
2. The scorecard component enables `EvaluateScorecardsPermissions`.
3. While the query is loading, the scorecard is temporarily read-only.
4. If the backend returns `true`, editing remains available.
5. If the backend returns `false`, the scorecard stays read-only and shows the warning.
6. If the backend initially allows edit but a later `UpdateScorecard` returns 403, the UI reverts and freezes.

### Why The UI Still Handles Update 403

The permission query is a point-in-time result. It can become stale because:

- permissions changed after page load
- template configuration changed elsewhere
- user role/team/group membership changed
- the scorecard/template data was cached

So the frontend needs both:

- proactive load-time permission query
- reactive 403 rollback/freeze fallback

## Feature Flag Boundaries

Feature flag:

```ts
disableEditingOnSubmittedScorecards
```

Gated UI/UX:

- `Scorecard editors` selector in `TemplateBuilderAdvanced`
- conversation submitted-scorecard load-time lock
- conversation fallback freeze on submitted-scorecard 403
- process submitted-scorecard load-time lock
- process fallback freeze on submitted-scorecard 403

Not gated:

- API/model support for `submittedScorecardEditors`
- template load/save round-trip of `submittedScorecardEditors`
- backend bridge for `EvaluateScorecardsPermissions`

This is intentional. Hidden values should still be preserved through template load/save even when the feature UI is off.

## Important Differences Between Conversation And Process Scorecards

Conversation scorecards:

- use `ScorecardForm.tsx`
- use `useSaveScorecardMutation`
- autosave is driven by `form.watch`
- fallback rollback uses `resetScorecardForm(scorecard, appealScorecard)`
- appeals are excluded from the submitted-edit lock

Process scorecards:

- use `ProcessScorecardScoring.tsx`
- use `useUpdateScorecard` directly
- autosave is driven by process form watchers and `handleSubmitForm`
- fallback rollback uses `setDefaultFormValues(currentScorecard)`
- criterion components needed explicit `readOnly` propagation
- comments remain viewable while text areas are read-only

## Practical Debugging Checklist

### If The Permission Query Is Not Sent

Check:

- feature flag `disableEditingOnSubmittedScorecards`
- scorecard has `name`
- scorecard has `submittedAt`
- conversation scorecard is not in appeal mode
- authenticated user has `currentUser.name`

Relevant files:

- `ScorecardForm.tsx`
- `ProcessScorecardScoring.tsx`
- `useEvaluateScorecardPermissions.ts`

### If The Scorecard Does Not Lock After Denied Permission

Check:

- query data shape includes the scorecard name as the first-level key
- permission key is exactly `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`
- returned value is boolean `false`, not missing/undefined
- `submittedEditPermissionReadOnly` is used in the relevant form/edit path

### If The UI Locks But Still Sends UpdateScorecard

Conversation:

- check `form.watch` subscription condition in `ScorecardForm.tsx`
- check `readOnly` passed into `useSaveScorecardMutation`
- check `useSaveScorecardMutation` early return

Process:

- check `onFormChange` early return
- check `handleSubmitForm` early return
- check `readOnly` passed into `ProcessScorecardCriteria`
- check `readOnly` passed into `GeneralNotes`

### If A 403 Shows The Old Toast Instead Of The Inline Banner

Conversation:

- confirm the scorecard is already submitted
- confirm the error is `CrestaApiServiceError`
- confirm `err.isForbiddenError()` is true
- confirm `onForbiddenSubmittedEditError` returns `true`

Process:

- confirm `useUpdateScorecard` receives `onForbiddenError`
- confirm `suppressForbiddenToast: true`
- confirm `onForbiddenError` returns `true`
- confirm the scorecard has `submittedAt`

## Short File Map

Template config:

- `TemplateBuilderAdvanced.tsx`: renders Advanced section and Scorecard editors selector
- `TemplateBuilderForm.tsx`: hydrates submitted editors into form state
- `useSaveScorecardTemplate.ts`: saves submitted editors to template permissions
- `formTypes.ts`: adds form field type
- `scoring.ts`: frontend model shape
- `transformers.ts`: API-to-model normalization

Permission query:

- `useEvaluateScorecardPermissions.ts`: React Query hook for load-time permission checks
- `coachingApi.ts`: API bridge to `EvaluateScorecardsPermissions`
- `apiTypes.ts`: request/result types
- `queryUtils.ts`: query key

Conversation runtime:

- `ScorecardForm.tsx`: permission query, lock state, banner, read-only wiring
- `useSaveScorecardMutation.ts`: autosave/update path and fallback 403 handling

Process runtime:

- `ProcessScorecardScoring.tsx`: permission query, lock state, banner, read-only wiring, process autosave blocking
- `ProcessScorecardCriteria.tsx`: read-only propagation to criterion inputs
- `ProcessScorecardItem.tsx`: comment panel remains openable, comment textarea is read-only
- `useUpdateScorecard.ts`: shared update mutation with optional forbidden handler

