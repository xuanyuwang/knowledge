# CONVI-7197 — Scorecard editors dropdown UX fix

**Status:** Merged  
**Ticket:** https://linear.app/cresta/issue/CONVI-7197  
**PR:** https://github.com/cresta/director/pull/20375  
**Merged:** 2026-07-03  
**Parent project:** CONVI-6862 (submitted-scorecard editors control lives in `TemplateBuilderAdvanced`)

## Problem

The Scorecard editors user/team/group dropdown in the template builder was uncomfortable to use in both UI v1 and v2. Near the bottom of the Access tab, the menu opened above the input initially, then flipped below once the user searched — making results hard to reach.

## Root cause

`UserTeamGroupPopover` uses `position: 'bottom-start'` with Floating UI flip enabled. Near the page bottom, flip opens the menu above. When search hides tabs and shrinks the dropdown, flip recalculates and moves it below the input.

## Solution

Override popover placement only for the Scorecard editors `UserTeamGroupSelect`:

- `position: 'top-start'`
- `flip: false` in middlewares

Scoped to `TemplateBuilderAdvanced.tsx`; no change to global `UserTeamGroupSelect` behavior.

## Traceability

- Commit: `[CONVI-7197] Fix Scorecard editors dropdown placement in template builder`
- Session: `sessions/2026-07-02/codex-convi-7197-dropdown-placement-fix.md`
