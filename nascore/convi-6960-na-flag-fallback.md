# CONVI-6960: Stored Scored N/A Must Fall Back When `enableNAScore` Is Off

**Created:** 2026-06-01
**Issue:** [CONVI-6960](https://linear.app/cresta/issue/CONVI-6960/scorecard-na-option-inherits-stored-numeric-score-after-enablenascore)
**PR:** [director#19236](https://github.com/cresta/director/pull/19236)

## Investigation

Closed-conversation scoring and scorecard hydration paths were still willing to treat a stored scored-N/A selection as an ordinary numeric option, even after the `enableNAScore` feature flag was disabled.

The result was inconsistent fallback behavior:

- the UI could still surface the stored scored-N/A option
- hydration could reopen scorecards with that numeric option selected
- saving and aggregation could continue to persist and interpret the numeric N/A value instead of converting back to legacy `notApplicable`

## Root Cause

This was an independent feature-flag fallback bug:

1. scoring-side helpers did not consult `enableNAScore`
2. stored scored-N/A values therefore remained numeric through rendering, hydration, submission, and closed-conversation scoring flows even when the flag was off

## Solution

Implemented in `director` on branch `xwang/convi-6960-na-flag-fallback`.

- Added scoring-side helpers for "effective N/A mode" and numeric score fallback behavior.
- When `enableNAScore` is off:
  - real scored-N/A options are filtered out of the rendered choices
  - the legacy synthetic N/A option is shown instead
  - stored scored-N/A values hydrate as legacy N/A
  - submitted N/A values persist as `notApplicable: true` with no numeric value
- Threaded the flag through:
  - option rendering
  - score extraction
  - default form hydration
  - process scorecard scoring
  - score save/update flows

## Verification

Targeted regression coverage was added for:

- filtering scored-N/A options from the UI when the flag is off
- rendering the legacy N/A fallback
- persisting N/A as `notApplicable`
- hydrating stored scored-N/A values back to legacy N/A

Commands run:

```bash
node /Users/xuanyu.wang/repos/director/node_modules/vitest/vitest.mjs run --environment jsdom packages/director-app/src/components/scoring/naScoreUtils.test.ts packages/director-app/src/components/scoring/scorecardNumericValueUtils.test.ts
```
