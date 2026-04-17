# enableDuplicateScoreForCriteria Feature Flag Status

## Config Repo (`../config` - latest main)

**Default value:** `false`

```markdown
| enableDuplicateScoreForCriteria | Enables duplicate scores for non-multi select criteria in scorecard template builder | Director, Admin, Scorecards, Template Builder | boolean | false |
```

From `/Users/xuanyu.wang/repos/config/docs/all-feature-flags.md`

## Director Repo - Main Branch Status

**Status:** Feature flag was **CLEANED UP** on April 9, 2026

**Cleanup commit:** `c4e5df3298` - "[CONVI-6471] Clean up enableDuplicateScoreForCriteria feature flag"

### What the cleanup removed:

1. **Removed the feature flag check**
   ```diff
   - const enableDuplicateScoreForCriteria = !!useFeatureFlag('enableDuplicateScoreForCriteria');
   ```

2. **Removed the ternary for normal options** (assumed decoupled mode always on)
   ```diff
   - {showNumericOptions &&
   -   (enableDuplicateScoreForCriteria ? (
   -     <Controller name={`${fieldName}.settings.scores.${index}.score`} ... />
   -   ) : (
   -     <Controller name={`${fieldName}.settings.options.${index}.value`} ... />
   -   ))}
   + {showNumericOptions && (
   +   <Controller name={`${fieldName}.settings.scores.${index}.score`} ... />
   + )}
   ```

3. **Removed legacy mode branches** in `onAddLabel` and `handleRemoveOption`

4. **Removed `resetAutoQADetectedFields` callback**

### Current main branch behavior:

- **Always uses decoupled scoring** (`settings.scores[index].score`)
- **Never uses legacy mode** (the `settings.options[index].value` path is gone)
- No feature flag checks anywhere

## Why My Branch Still Has the Ternary

**Timeline:**
- April 1, 2026: N/A feature added (commit `de6b5a50ae`) - included the ternary because flag still existed
- April 9, 2026: Feature flag cleanup (commit `c4e5df3298`) - removed the ternary from main

My branch (`xuanyu/enable-na-score`) was created from an older base that had the ternary, and I haven't synced with the cleanup yet.

## What Needs to Be Done

### Remove all `enableDuplicateScoreForCriteria` checks:

1. **Line 60:** Remove `useFeatureFlag` call
   ```diff
   - const enableDuplicateScoreForCriteria = useFeatureFlag('enableDuplicateScoreForCriteria');
   ```

2. **Line 73:** Simplify showNAScoreRow
   ```diff
   - const showNAScoreRow = enableNAScore && enableDuplicateScoreForCriteria && !!showNAField.value;
   + const showNAScoreRow = enableNAScore && !!showNAField.value;
   ```

3. **Line 107, 290:** Remove from checkbox onChange
   ```diff
   - if (checked && enableNAScore && enableDuplicateScoreForCriteria && !isNAOption) {
   + if (checked && enableNAScore && !isNAOption) {
   ```

4. **Lines 198-236:** Remove the entire ternary for normal options
   ```diff
   - {showNumericOptions &&
   -   (enableDuplicateScoreForCriteria ? (
   -     <Controller name={`${fieldName}.settings.scores.${index}.score`} ... />
   -   ) : (
   -     <Controller name={`${fieldName}.settings.options.${index}.value`} ... />
   -   ))}
   + {showNumericOptions && (
   +   <Controller
   +     name={`${fieldName}.settings.scores.${index}.score`}
   +     rules={{ required: t('criteria-labeled-options.value-required', 'Value is required') }}
   +     render={({ field, fieldState: { error } }) => (
   +       <NumberInput
   +         {...field}
   +         allowDecimal={false}
   +         min={0}
   +         clampBehavior="none"
   +         placeholder={t('criteria-labeled-options.value-placeholder', 'Value')}
   +         error={error?.message}
   +         hideControls
   +       />
   +     )}
   +   />
   + )}
   ```

### Result:

- **Decoupled mode assumed everywhere** (legacy mode code removed)
- **N/A scores work** (they require decoupled mode anyway)
- **Consistent with main branch**
- **Simpler code** (no feature flag checks, no dead legacy branches)
