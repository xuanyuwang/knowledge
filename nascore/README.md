# N/A Score Support (enableNAScore Feature)

**Created:** 2026-04-12
**Updated:** 2026-04-25

## Overview

Support for allowing N/A (not applicable) options in scorecard templates to have **null scores** instead of requiring numeric scores. This enables proper handling of N/A selections in AutoQA scoring and manual evaluations.

**Feature flag:** `enableNAScore` in director (frontend)

**Key capability:** When N/A is selected for a criterion, the system can now treat it as "no score" (null) rather than assigning it a numeric value. This is critical for AutoQA scenarios where automated scoring may mark criteria as "not applicable" when conditions aren't met.

## Current Status

Two bugs fixed on branch `xuanyu/enable-na-score`:

1. **N/A label display in AutoQA dropdowns** - Fixed to show "N/A (no score)" when score is null instead of "N/A (2)" [Commit: dd96747dc4]
2. **Score auto-population when disabling AutoQA** - Fixed to prevent scores from auto-populating as 0, 1, ... when unchecking "Automated scoring" on new criteria [Commit: 99aaed9705]

Additional work completed:

- **Validation fix** - Updated validation.ts to allow null scores for N/A options while still requiring numeric scores >= 0 for normal options
- **Comprehensive test scenarios** - Created QA test document covering 10 main scenarios, 4 edge cases, 3 regression tests, 2 performance tests
- **Code reviews** - Analyzed TemplateBuilderFormConfigurationStep.tsx, DEFAULT_CRITERION behavior, and AutoQA sync mechanisms

**Critical issue identified (deferred):** DEFAULT_CRITERION provides options but NO scores array, which is incompatible with decoupled scoring after removing legacy migration. User noted this has always been the case with no bug reports, so investigating further before making changes.

## Key Findings

### 1. N/A Option Data Model

- **Option structure:** `{ label: "N/A", value: "not_applicable", isNA: true, score: null }`
- **Migration:** N/A option migration (showNA → isNA) is kept; legacy scores array migration is removed
- **Default behavior:** `showNA` defaults to `true` when copying from default criterion

### 2. AutoQA Integration

- **AutoQA settings:** Default is `detected: 1, not_detected: 0, not_applicable: null`
- **AutoQA checked by default:** Because `!!autoQAFieldValue.triggers` where `triggers = []` (empty array is truthy)
- **Sync mechanism:** useWatch + useMemo provides automatic sync from CriteriaLabeledOptions to AutoQA dropdowns
- **Display fix:** Check for `option.isNA && score == null` to show "no score" instead of falling back to option.value

### 3. Validation Rules

- **For N/A options:** Allow `score: null` OR `score: number >= 0`
- **For normal options:** Require `score: number >= 0`
- **At least one non-zero score rule:** N/A with null doesn't count; N/A with number > 0 does count
- **Implementation:** Pass `options` array to validation functions to check `isNA` flag

### 4. Score Initialization

- **React-hook-form behavior:** `required` rule checks existence, not truthiness (0 passes, null/undefined fail)
- **Legacy migration issue:** Frontend was handling scores array migration (should be backend/one-time script)
- **Fix approach:** Remove legacy scores migration, keep only N/A option migration

## Investigation Documents

All detailed analysis documents are in `/director/.tmp`:

- `AUTOQA_SYNC_ANALYSIS.md` - Validation timing and sync mechanism
- `AUTOQA_REVIEW.md` - Workflows, sync mechanisms, simplification opportunities
- `SCORE_AUTO_POPULATION_FIX.md` - Root cause analysis and solution options
- `score-initialization-investigation.md` - Initial investigation of score defaults
- `QA_TEST_SCENARIOS.md` - Comprehensive test scenarios document
- `CONFIGURATION_STEP_REVIEW.md` - TemplateBuilderFormConfigurationStep.tsx review
- `QUESTIONS_ANSWERED.md` - DEFAULT_CRITERION investigation
- `DEFAULT_CRITERION_ISSUE.md` - Critical issue documentation
- `VALIDATION_FIX_ANALYSIS.md` - Validation fix approach analysis

## Testing Checklist

- [ ] Create new criterion with AutoQA enabled - scores should be empty
- [ ] Uncheck AutoQA - scores should stay empty (not auto-populate)
- [ ] N/A option in AutoQA dropdowns should show "N/A (no score)" when score is null
- [ ] Old templates with showNA=true should still get N/A option migrated
- [ ] Validation should allow null scores for N/A options
- [ ] Validation should require numeric scores >= 0 for normal options

## Next Steps

- Test both fixes in dev environment
- Consider implementing other simplifications identified in AUTOQA_REVIEW.md:
  - Extract duplicate onChange converters (lines 285-305 in TemplateBuilderAutoQA.tsx)
  - Consolidate redundant isDecoupledScoring checks
- Resolve DEFAULT_CRITERION issue if needed
- Update branch status and merge plan

## Log History

| Date | Summary |
|------|---------|
| 2026-04-12 | Project created. Fixed N/A label display bug and score auto-population bug. Updated validation to support null scores for N/A options. Created comprehensive test scenarios. Identified DEFAULT_CRITERION issue (deferred). |
| 2026-04-25 | Full lifecycle analysis of options/scores/auto_qa. Identified 8 pain points. Proposed CriterionOptionsManager refactoring. See `options-scores-lifecycle.md`. |

## Related

- **Frontend:** `~/repos/director` (branch: `xuanyu/enable-na-score`)
- **Investigation docs:** `/director/.tmp` (various analysis documents)
- **Related components:**
  - `CriteriaLabeledOptions.tsx` - Option and score management
  - `TemplateBuilderAutoQA.tsx` - AutoQA dropdown display
  - `TemplateBuilderFormConfigurationStep.tsx` - Main configuration orchestrator
  - `validation.ts` - Score validation logic
