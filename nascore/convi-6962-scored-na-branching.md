# CONVI-6962: Scored N/A Does Not Activate Branch Conditions

**Created:** 2026-06-01
**Issue:** [CONVI-6962](https://linear.app/cresta/issue/CONVI-6962/na-in-scorecard-branching-from-scored-na-option-does-not-work-in)
**PR:** [director#19237](https://github.com/cresta/director/pull/19237)

## Investigation

Branch activation in scoring was driven by `isBranchConditionMet` in `packages/director-app/src/components/scoring/utils.ts`.

That logic only recognized N/A when the saved score looked like legacy N/A:

- `score.notApplicable === true`
- `score.numericValue === undefined`

Scored N/A does not look like that. It is stored as a normal numeric selection pointing at the option marked `isNA: true`, with `notApplicable === false`.

The same function also returned early when `branch.condition.not_applicable` was set, which meant a branch configured for `N/A + specific numeric values` effectively ignored the numeric side of the condition.

## Root Cause

This was an independent runtime branching bug:

1. Branch evaluation used only legacy sentinel semantics for N/A and had no access to the parent criterion template needed to recognize scored N/A by `isNA`.
2. The evaluation path short-circuited on `not_applicable` instead of OR-ing `not_applicable` with `numeric_values`.

## Solution

Implemented in `director` on branch `xwang/convi-6962-scored-na-branching`.

- Threaded the parent criterion template into branch evaluation.
- Added helper logic to map scored-N/A numeric selections back to the N/A sentinel when matching branches.
- Changed branch evaluation to return true when either:
  - the selected value matches `not_applicable`, or
  - any selected numeric value matches `numeric_values`
- Preserved legacy unscored-N/A and not-set behavior.

## Verification

Targeted regression coverage was added for:

- scored N/A matching an N/A-only branch
- mixed `not_applicable + numeric_values` conditions using OR semantics

Command run:

```bash
node /Users/xuanyu.wang/repos/director/node_modules/vitest/vitest.mjs run --environment jsdom packages/director-app/src/components/scoring/branchConditionUtils.test.ts
```
