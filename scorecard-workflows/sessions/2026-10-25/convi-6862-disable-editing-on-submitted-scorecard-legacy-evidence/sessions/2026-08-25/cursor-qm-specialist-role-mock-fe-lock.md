# QM Specialist role-mock FE lock on submitted scorecards

**Date:** 2026-08-25  
**Scope:** Read-only Director FE diagnosis at current `origin/main` (`/Users/xuanyu.wang/repos/director-current`, detached `5398d85a77`)  
**Question:** Why mocking role "QM Specialist" (`QA_SPECIALIST`) disables all submitted-scorecard actions even when `EvaluateScorecardsPermissions` returns `allowed=true`.

## Verdict

The canonical scorecard was acknowledged at `2026-08-03T22:23:07.676647Z`. `ScorecardForm` unconditionally disables the entire form after acknowledgment unless the mocked frontend role includes `QA_ADMIN`:

```text
acknowledgedShouldBlock = acknowledgedAt && !has QA_ADMIN
disableScorecardForm = readOnly || appeal/answerKey || acknowledgedShouldBlock
```

QM Specialist maps to `QA_SPECIALIST`, so role mocking activates this legacy frontend-only lock. It is independent of `EvaluateScorecardsPermissions`; the backend returned `allowed=true`, but that result only contributes to `readOnly` and cannot override `acknowledgedShouldBlock`.

## Canonical UI path

1. `features/conversations/closed/ClosedConversationsPage.tsx` / route
2. `components/conversations/ConversationTranscriptAndSidebar.tsx`
3. `components/conversations/sidebar/scoring/ScorecardTab.tsx`
4. `components/scoring/ScorecardFormLoader.tsx`
5. `components/scoring/scorecard-form/ScorecardForm.tsx`
6. Parallel title-bar: `components/scorecard-reset-button/ResetButton.tsx`

## Permission split (role mock)

- Override: `hooks/user.ts` `useLocalCustomRolesOverride` + `context/CurrentUserContext.tsx` (Cresta Admin only).
- Header warning: "Local roles override only affects the front-end. All data requests/APIs still validate based on the un-altered roles."
- Submitted API hook: `useEvaluateScorecardPermissions` → `requesterUserName = currentUser.name` (identity unchanged).
- Display name mapping: `types/roles.ts` QM Specialist → `AuthProtoRole.QA_SPECIALIST`.

## Disable composition in ScorecardForm

```text
submittedEditPermission.readOnly  ← flag + API MODIFY_SUBMITTED_LOCKED (loading|denied|frozen)
baseReadOnly                      ← isAgentOnly | template.deletedAt | !canGradeScorecard | ownConvo
readOnly = baseReadOnly || submittedEditPermissionReadOnly
acknowledgedShouldBlock           ← acknowledgedAt && !QA_ADMIN
disableScorecardForm = readOnly || appeal/answerKey || acknowledgedShouldBlock
```

When API `allowed=true`, `denied` is false; after load `submittedEditPermissionReadOnly` is false. Remaining full-form lock must come from `baseReadOnly` or `acknowledgedShouldBlock`.

## Canonical response evidence

- The scorecard response contains `acknowledgeTime: 2026-08-03T22:23:07.676647Z`.
- The scorecard agent is Ghia Cuasay (`customers/rcg/users/7ab10935cc0f3b11`, `ghiacuasay@rccl.com`). `AcknowledgeScorecard` verifies that the authenticated caller exactly matches `scorecard.AgentUserID`, so Ghia acknowledged this scorecard.
- Its pinned template response is active, is not deleted, and includes `QA_SPECIALIST` in `scorecardGraders`.
- Therefore the template grader check should pass for QM Specialist; acknowledgment is the definite whole-form lock.
- Reset has an additional independent restriction: only Global Admin, QA Admin, or the scorecard creator can reset. Submitted-editor permission does not override it in the closed-conversation path.

## Production revision verification

Production commit `06cf4817ec3214a6a7b238f217f1bb71e7c9f6dd` contains the same logic at `ScorecardForm.tsx:211-213`:

```text
const acknowledgedShouldBlock = !!scorecard?.acknowledgedAt && !hasAnyProtoRole([AuthProtoRole.QA_ADMIN]);
const disableScorecardForm =
  readOnly || isInAppeal || appealRequested || !!answerKeyScorecard || acknowledgedShouldBlock;
```

The diff from that production commit to current `origin/main` is empty for `ScorecardForm.tsx`, so the diagnosis applies exactly to deployed code.

## Acknowledgment-lock history and intent

- Introduced by `3c4568de03d1bd87063792f3e3014db622178a9f` on 2025-08-25 in [director#13338](https://github.com/cresta/director/pull/13338), CONVI-5098.
- Product context: CSM consensus was that a score should not change after the agent acknowledges it. The proposed exception was effectively Performance Admin (`QA_ADMIN`); managers should be blocked.
- The only later edit to this condition was `fb4a512a3e588f16a34cbdfe6b55264eee4fd22b` on 2025-11-19, mechanically replacing deprecated `hasAnyRole` with `hasAnyProtoRole`. Semantics did not change.
- Submitted-editor integration commit `71cfb178e30dd23afe861127e75aea5db8f3b79b` on 2026-06-25 added the backend permission-derived `readOnly` path but left `acknowledgedShouldBlock` as a separate OR condition.

Conclusion: the QA Admin exception is old and intentional, not a recent regression. The bug is the newer submitted-editor feature failing to reconcile its explicit allow decision with this pre-existing acknowledgment policy.

## Not the primary cause under mock + allowed=true

- `useSubmittedScorecardEditPermission` denied path (would show "You do not have permission to edit this scorecard")
- Feature flag off (then `shouldEvaluate` is false and submitted hook never locks)
- Inactive template alone (permission checker `includeDeactivated=true` by default)

## Fix direction

1. Treat FE role override as invalid for validating submitted-lock API behavior; test with a real QM Specialist user, or make a dev-only evaluate path honor override.
2. If the submitted-editor decision is intended to govern all submitted scorecards, make an explicit backend `allowed=true` override the legacy acknowledgment lock when the feature is enabled. Preserve the legacy rule when the feature is disabled.
3. Expose an explicit `allowed`/resolved state from `useSubmittedScorecardEditPermission`; do not infer an override merely from `readOnly=false`, because query errors currently fail open.
4. Decide separately whether submitted-editor permission should override Reset's creator/admin restriction, then align the closed-conversation and process-scorecard paths.
