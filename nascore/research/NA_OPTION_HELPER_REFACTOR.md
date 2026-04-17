# N/A Option Helper Refactor

**Created:** 2026-04-13

## Summary

Created a shared helper function `checkIsNAOption()` to determine if an option is an N/A option. This centralizes the logic and makes it easy to update once the backend PR merges.

## Implementation

### 1. Created Helper Function

**File:** `packages/director-app/src/features/admin/coaching/template-builder/configuration/optionHelpers.ts`

```typescript
/**
 * Helper to determine if an option is an N/A option.
 *
 * TODO: Once backend PR merges to save isNA field, remove the label check.
 * Backend currently strips isNA field when saving to database, so we need to check label as fallback.
 * After backend fix: only check opt.isNA === true
 *
 * @param opt - The option to check
 * @returns true if the option is an N/A option
 */
export function checkIsNAOption(opt: { label: string; value: number; isNA?: boolean }): boolean {
  return opt.isNA === true || opt.label === 'N/A';
}
```

**Current behavior:**
- Checks both `isNA === true` AND `label === 'N/A'`
- Unblocks development while waiting for backend PR

**After backend PR merges:**
- Remove `|| opt.label === 'N/A'`
- Only check `opt.isNA === true`

### 2. Updated Files

All files now import and use `checkIsNAOption()` instead of checking `opt.isNA` directly:

1. ✅ **CriteriaLabeledOptions.tsx** - 5 instances
   - Line 81: `const isNAOption = watchedOptionsField?.find((opt) => checkIsNAOption(opt));`
   - Line 82: `const isNAIndex = watchedOptionsField?.findIndex((opt) => checkIsNAOption(opt)) ?? -1;`
   - Line 100: `const hasNAOption = currentOptions?.some((opt: OptionWithValue) => checkIsNAOption(opt));`
   - Line 108: `const optionsWithoutNA = currentOptions.filter((opt: OptionWithValue) => !checkIsNAOption(opt));`
   - Line 126: `if (enableNAScore && showNAField.value && !currentOptionsAfter?.some((opt) => checkIsNAOption(opt)))`
   - Line 210: `if (watchedOptionsField?.[index] && checkIsNAOption(watchedOptionsField[index])) return null;`

2. ✅ **NumericBinsAndValuesConfigurator.tsx** - 3 instances
   - Line 62: `const isNAOption = watchedSettingsOptions?.find((opt) => checkIsNAOption(opt));`
   - Line 63: `const isNAIndex = watchedSettingsOptions?.findIndex((opt) => checkIsNAOption(opt)) ?? -1;`
   - Line 88: `if (enableNAScore && showNAField.value && !currentOptions?.some((opt) => checkIsNAOption(opt)))`

3. ✅ **TemplateBuilderAutoQA.tsx** - 1 instance
   - Line 243: `const displayScore = checkIsNAOption(option) && score == null ? 'no score' : (score ?? option.value);`

4. ✅ **TemplateBuilderFormConfigurationStep.tsx** - 1 instance
   - Line 315: `const optionsWithoutNA = defaultCriterion.settings.options.filter((opt) => !checkIsNAOption(opt));`

5. ✅ **validation.ts** - 2 instances
   - Line 288: `const isNAOption = option ? checkIsNAOption(option) : false;`
   - Line 581: `const isNAOption = option ? checkIsNAOption(option) : false;`

6. ✅ **CriterionInputDisplay.tsx** - 3 instances (preview component)
   - Line 136: `const nonNAOptions = labeledRadiosTemplate.settings.options.filter((option) => !checkIsNAOption(option));`
   - Line 180: `const nonNADropdownOptions = dropdownNumericTemplate.settings.options.filter((option) => !checkIsNAOption(option));`
   - Line 244: `const nonNASingleOptions = dropdownNumericTemplate.settings.options.filter((option) => !checkIsNAOption(option));`

7. ✅ **utils.ts** (scoring utils) - 1 instance
   - Line 544: `? criterion.settings.options?.find((opt) => checkIsNAOption(opt))`

**Total:** 16 instances updated across 7 files

## Benefits

1. **Single source of truth** - All N/A checks use the same logic
2. **Easy to update** - Only need to change one file after backend PR merges
3. **Consistent behavior** - No risk of different checks in different files
4. **Self-documenting** - Function name clearly indicates purpose
5. **Unblocks development** - Can work on frontend while waiting for backend

## Next Steps

### After Backend PR Merges

1. Update `checkIsNAOption()` in `optionHelpers.ts`:
   ```typescript
   export function checkIsNAOption(opt: { label: string; value: number; isNA?: boolean }): boolean {
     return opt.isNA === true;  // Remove label check
   }
   ```

2. Remove TODO comment

3. Test that:
   - Creating new criteria with N/A works
   - Saving and reloading preserves N/A option
   - No duplicate N/A options appear

### Backend PR Required

The backend must save the `isNA` field in the options array. Currently it's stripped when saving to database.

**Expected behavior after backend fix:**
```json
{
  "options": [
    {"label": "Yes", "value": 0},
    {"label": "No", "value": 1},
    {"label": "N/A", "value": 2, "isNA": true}  // ← isNA field preserved
  ]
}
```
