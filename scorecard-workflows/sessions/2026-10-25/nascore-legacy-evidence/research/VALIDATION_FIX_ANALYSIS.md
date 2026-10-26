# Validation Fix for N/A Score Support

**Created:** 2026-04-12  
**File:** `packages/director-app/src/features/admin/coaching/template-builder/validation.ts`

---

## Problem

Current validation requires ALL scores to be numbers and >= 0. This fails for N/A options where `score.score` can be `null`.

---

## Locations to Fix

### 1. `validateSettingsScores` function (lines 534-566)

**Current code (line 543):**
```tsx
if (!isNumber(score.score) || !isNumber(score.value) || score.score < 0) {
  return {
    title: t('invalid-options-configured', {
      defaultValue: '{{name}} has invalid options configured.',
      name: displayName,
    }),
    message: t('at-least-one-score-not-configured', 'At least one score is not configured properly.'),
  };
}
```

**Issue:** Fails when `score.score === null` for N/A options

**Called from:**
- Line 90: `validateSettingsScores(item.settings.scores, item.displayName)` in `validateCriterion`
- Line 486: `validateSettingsScores(castedItem.settings.scores, outcomeItem.displayName)` in `validateOutcome`

---

### 2. `validateNumOccurrencesScoreTypeConfigurationItem` function (lines 284-294)

**Current code (line 285):**
```tsx
for (const score of castedItem.settings.scores) {
  if (!isNumber(score.score) || !isNumber(score.value)) {
    return {
      title: t('invalid-options-configured', {
        defaultValue: '{{name}} has invalid options configured.',
        name: item.displayName,
      }),
      message: t('behavior-score-score-not-configured', 'Score for Behavior Score is not configured.'),
    };
  }
}
```

**Issue:** Same - fails when `score.score === null`

---

## Solution Design

### Data Structure Recap

**scores array:**
```tsx
[
  { value: 0, score: 5 },
  { value: 1, score: 0 },
  { value: 2, score: null }  // ← N/A option
]
```

**options array:**
```tsx
[
  { label: 'Yes', value: 0 },
  { label: 'No', value: 1 },
  { label: 'N/A', value: 2, isNA: true }  // ← Has isNA flag
]
```

**Matching:** `score.value === option.value`

---

### Fix 1: Update `validateSettingsScores`

**Change signature to accept options:**
```tsx
function validateSettingsScores(
  scores: SettingsScore[],
  options: Array<{ value: number; isNA?: boolean }> | undefined,
  displayName: string
): { title: string; message: string } | undefined {
  const i18n = getI18n();
  const t = i18n.getFixedT(null, 'director-app-admin', 'coaching.template-builder.validation');

  let nonZeroScoreExists = false;
  for (const score of scores) {
    // Find corresponding option
    const option = options?.find(opt => opt.value === score.value);
    const isNAOption = option?.isNA === true;
    
    // Validate score.value (always required)
    if (!isNumber(score.value)) {
      return {
        title: t('invalid-options-configured', {
          defaultValue: '{{name}} has invalid options configured.',
          name: displayName,
        }),
        message: t('at-least-one-score-not-configured', 'At least one score is not configured properly.'),
      };
    }
    
    // Validate score.score (allow null for N/A options)
    if (isNAOption) {
      // N/A option: score can be null or a valid number >= 0
      if (score.score !== null && (!isNumber(score.score) || score.score < 0)) {
        return {
          title: t('invalid-options-configured', {
            defaultValue: '{{name}} has invalid options configured.',
            name: displayName,
          }),
          message: t('at-least-one-score-not-configured', 'At least one score is not configured properly.'),
        };
      }
      // N/A with numeric score counts as non-zero if > 0
      if (isNumber(score.score) && score.score > 0) {
        nonZeroScoreExists = true;
      }
    } else {
      // Normal option: score must be a valid number >= 0
      if (!isNumber(score.score) || score.score < 0) {
        return {
          title: t('invalid-options-configured', {
            defaultValue: '{{name}} has invalid options configured.',
            name: displayName,
          }),
          message: t('at-least-one-score-not-configured', 'At least one score is not configured properly.'),
        };
      }
      if (score.score > 0) {
        nonZeroScoreExists = true;
      }
    }
  }
  
  if (!nonZeroScoreExists) {
    return {
      title: t('invalid-options-configured', {
        defaultValue: '{{name}} has invalid options configured.',
        name: displayName,
      }),
      message: t('at-least-one-score-non-zero', 'At least one score must be non-zero.'),
    };
  }
  return undefined;
}
```

**Update call sites:**
```tsx
// Line 90
const error = validateSettingsScores(item.settings.scores, item.settings.options, item.displayName);

// Line 486
const error = validateSettingsScores(castedItem.settings.scores, castedItem.settings.options, outcomeItem.displayName);
```

---

### Fix 2: Update `validateNumOccurrencesScoreTypeConfigurationItem`

**Change validation loop (lines 284-294):**
```tsx
if (castedItem.settings?.scores) {
  for (const score of castedItem.settings.scores) {
    // Find corresponding option
    const option = castedItem.settings.options?.find(opt => opt.value === score.value);
    const isNAOption = option?.isNA === true;
    
    // Validate value (always required)
    if (!isNumber(score.value)) {
      return {
        title: t('invalid-options-configured', {
          defaultValue: '{{name}} has invalid options configured.',
          name: item.displayName,
        }),
        message: t('behavior-score-score-not-configured', 'Score for Behavior Score is not configured.'),
      };
    }
    
    // Validate score (allow null for N/A)
    if (isNAOption) {
      // N/A option: score can be null or a valid number
      if (score.score !== null && !isNumber(score.score)) {
        return {
          title: t('invalid-options-configured', {
            defaultValue: '{{name}} has invalid options configured.',
            name: item.displayName,
          }),
          message: t('behavior-score-score-not-configured', 'Score for Behavior Score is not configured.'),
        };
      }
    } else {
      // Normal option: score must be a number
      if (!isNumber(score.score)) {
        return {
          title: t('invalid-options-configured', {
            defaultValue: '{{name}} has invalid options configured.',
            name: item.displayName,
          }),
          message: t('behavior-score-score-not-configured', 'Score for Behavior Score is not configured.'),
        };
      }
    }
  }
}
```

---

## Validation Logic Summary

### For Normal Options
- `score.value`: MUST be number
- `score.score`: MUST be number >= 0

### For N/A Options (isNA: true)
- `score.value`: MUST be number
- `score.score`: CAN be null OR number >= 0

### At Least One Non-Zero Score Rule
- At least one option (including N/A if it has a numeric score) must have score > 0
- N/A with `score: null` does NOT count toward this requirement
- N/A with `score: 5` DOES count

---

## Edge Cases to Consider

### Case 1: All options are 0, N/A is null
```tsx
scores: [
  { value: 0, score: 0 },
  { value: 1, score: 0 },
  { value: 2, score: null }  // N/A
]
```
**Result:** ❌ Validation FAILS - "At least one score must be non-zero"

---

### Case 2: All options are 0, N/A is 5
```tsx
scores: [
  { value: 0, score: 0 },
  { value: 1, score: 0 },
  { value: 2, score: 5 }  // N/A with score
]
```
**Result:** ✅ Validation PASSES - N/A score counts as non-zero

---

### Case 3: One option is positive, N/A is null
```tsx
scores: [
  { value: 0, score: 5 },
  { value: 1, score: 0 },
  { value: 2, score: null }  // N/A
]
```
**Result:** ✅ Validation PASSES

---

### Case 4: N/A only (no other options)
```tsx
options: [{ label: 'N/A', value: 0, isNA: true }]
scores: [{ value: 0, score: null }]
```
**Result:** ❌ Validation FAILS - "At least one score must be non-zero"

**But wait!** This should also fail the minimum options check:
- Line 76-84: `numOfOptions < 2` → validation fails
- So this case is already prevented

---

## Testing Checklist

After applying fix:

- [ ] Normal criterion with Yes=5, No=0 → validates ✅
- [ ] Normal criterion with Yes=0, No=0 → fails "at least one non-zero" ❌
- [ ] Criterion with Yes=5, No=0, N/A=null → validates ✅
- [ ] Criterion with Yes=0, No=0, N/A=null → fails "at least one non-zero" ❌
- [ ] Criterion with Yes=0, No=0, N/A=5 → validates ✅
- [ ] N/A with negative score (N/A=-1) → fails "not configured properly" ❌
- [ ] N/A with non-number string → fails "not configured properly" ❌
- [ ] Save template with N/A null score → succeeds ✅
- [ ] Publish template with N/A null score → succeeds ✅

---

## Implementation Steps

1. Update `validateSettingsScores` function signature to accept options
2. Update validation logic to check for `isNA` flag
3. Update both call sites (lines 90 and 486)
4. Update `validateNumOccurrencesScoreTypeConfigurationItem` validation loop
5. Test all scenarios
6. Update QA test scenarios document if needed
