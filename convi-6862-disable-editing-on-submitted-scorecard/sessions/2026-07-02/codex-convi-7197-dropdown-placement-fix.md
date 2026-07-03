# CONVI-7197 — Scorecard editors dropdown placement fix

**Date:** 2026-07-02  
**Merged:** 2026-07-03  
**Ticket:** [CONVI-7197](https://linear.app/cresta/issue/CONVI-7197/fix-ux-for-user-dropdown-for-scorecard-editors)  
**PR:** [director#20375](https://github.com/cresta/director/pull/20375)  
**Branch:** `convi-7197-fix-ux-for-user-dropdown-for-scorecard-editors` in `director`

## Summary

QA reported that the Scorecard editors `UserTeamGroupSelect` in the template builder Access tab was hard to use when the control sits near the bottom of the page. The dropdown initially opened above the input, but jumped below once the user typed in the search box.

## Root cause

The Scorecard editors control uses shared `UserTeamGroupSelect` → `UserTeamGroupPopover` → Mantine `PopoverComponent`.

`UserTeamGroupPopover` defaults to:

- `position: 'bottom-start'`
- `middlewares: POPOVER_MIDDLEWARES_WITH_SIZE` (includes Floating UI **flip**)

Because the control is in the Advanced section at the bottom of the Access tab:

1. On open, flip sees insufficient space below and places the menu **above** the input.
2. When the user types, search mode hides the Teams/Groups/Users tabs and the dropdown height shrinks.
3. Flip recalculates with the smaller height, decides there is now room below, and moves the menu **below** the input — off-screen or hard to scroll.

This affects both template structure v1 and v2 because they share `TemplateBuilderAdvanced`.

## Solution

Pass scoped `popoverProps` to the submitted-scorecard editors `UserTeamGroupSelect` in `TemplateBuilderAdvanced.tsx`:

```tsx
const SUBMITTED_SCORECARD_EDITORS_POPOVER_PROPS = {
  popoverProps: {
    position: 'top-start' as const,
    middlewares: {
      ...POPOVER_MIDDLEWARES_WITH_SIZE,
      flip: false,
    },
  },
};
```

- `position: 'top-start'` pins the preferred placement above the input.
- `flip: false` prevents reposition when search shrinks the dropdown.

**Prop-shape note:** The nested `{ popoverProps: { position, middlewares } }` is correct. `UserTeamGroupSelect` accepts `UserTeamGroupPopoverProps` (`types.ts`), spreads it onto `UserTeamGroupPopover`, which then spreads the inner `popoverProps` onto `PopoverComponent`. Same pattern as `AddReviewerSelect` (`popoverProps={{ popoverProps }}`). Flattening would send `position`/`middlewares` into `...dropdownProps` instead of the Mantine popover.

**File changed:** `packages/director-app/src/features/admin/coaching/template-builder/steps/access/template-builder-advanced/TemplateBuilderAdvanced.tsx`

**Feature flag:** `disableEditingOnSubmittedScorecards` (control only visible when enabled).

## Verification

- Typecheck: no errors related to the change.
- Manual:
  1. Enable `disableEditingOnSubmittedScorecards`.
  2. Admin > Performance > open scorecard template > Scorecard access > Advanced > Scorecard editors.
  3. Open dropdown — appears above input.
  4. Type in search — stays above and remains scrollable.

## Review note

CodeRabbit flagged the nested `popoverProps` shape as incorrect; traced the prop chain and confirmed it is intentional. Replied on [discussion_r3520764466](https://github.com/cresta/director/pull/20375#discussion_r3520764466).
