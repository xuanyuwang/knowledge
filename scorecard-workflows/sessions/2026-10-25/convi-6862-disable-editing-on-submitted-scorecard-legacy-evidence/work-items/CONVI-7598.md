# CONVI-7598: Explain scorecard lock reasons

**Status:** draft PR  
**Primary domain:** `scorecard-workflows`  
**Primary subdomain:** `permissions-and-visibility`  
**Official ticket:** [CONVI-7598](https://linear.app/cresta/issue/CONVI-7598/explain-scorecard-lock-reasons)  
**Last updated:** 2026-08-26

## Objective

Explain why a scorecard is read-only by calculating a typed primary lock reason and displaying reason-specific warning and tooltip copy.

## Current understanding

The RCG investigation found legitimate acknowledged-scorecard behavior presented without an explanation: QM Specialists are locked after agent acknowledgment while Performance Admins retain edit access. Existing deleted-template, grading-permission, own-conversation, and submitted-permission paths already provide partial warning/tooltip patterns.

## Proposed scope

- Add `ScorecardLockReason`.
- Calculate a deterministic primary reason alongside read-only state.
- Define precedence for overlapping reasons.
- Map each reason to approved user-facing text.
- Reuse the reason in warning alerts and relevant disabled-control tooltips.
- Align closed and process scorecard surfaces without changing authorization semantics.
- Test every reason, precedence, and rendering path.

## Implementation

- **Worktree:** `/Users/xuanyu.wang/repos/director-current`
- **Branch:** `convi-7598-explain-scorecard-lock-reasons`
- **Commits:** `91c3290df0`, `157d826e89`
- **Draft PR:** [director#22149](https://github.com/cresta/director/pull/22149)

## Validation

- Focused lock-reason tests: 14 passing.
- Pre-commit i18n extraction, lint, i18next lint, and formatting hooks: passing.

## Next action

Verify representative lock states manually, especially QM Specialist viewing an acknowledged scorecard, then move the PR out of draft.
