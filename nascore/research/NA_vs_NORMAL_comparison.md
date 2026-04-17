# N/A Option vs Normal Option Comparison

## Expected Differences (Valid)

1. ✅ **Label**: N/A has fixed "N/A" label, normal has user input
2. ✅ **Placeholder**: N/A shows "no score", normal shows "Value"
3. ✅ **isNA flag**: N/A has `isNA: true`, normal doesn't
4. ✅ **Creation trigger**: N/A created by checkbox, normal by "Add Option" button
5. ✅ **Insertion position**: N/A appends to end, normal inserts before N/A if exists
6. ✅ **Initial score**: N/A starts with `null`, normal starts with `0`
7. ✅ **Deletion UI**: N/A removed by unchecking checkbox, normal by trash icon
8. ✅ **Required validation**: Normal score has required rule, N/A doesn't (can be null)
9. ✅ **Min value**: Normal score has `min={0}`, N/A doesn't (can accept null)

## Current Code Comparison

### Creation (onAddLabel)

```tsx
// Line 75-92
const onAddLabel = useCallback(
  (isNA?: boolean): void => {
    const newOption = { 
      label: isNA ? 'N/A' : '',           // ← EXPECTED: different label
      value: newValue, 
      ...(isNA && { isNA: true })         // ← EXPECTED: isNA flag
    };
    const newScore = { 
      value: newValue, 
      score: isNA ? null : 0              // ← EXPECTED: null vs 0
    };
    const existingNAIndex = isNA ? -1 : isNAIndex;
    if (existingNAIndex >= 0) {
      optionsField.insert(existingNAIndex, newOption);  // ← EXPECTED: insert before N/A
    } else {
      optionsField.append(newOption);                   // ← EXPECTED: append
    }
  },
  [watchedOptionsField, optionsField, scoresFieldArray, isNAIndex]
);
```

**✅ Creation logic is correct**

---

### Score Input (Decoupled Mode - enableDuplicateScoreForCriteria)

#### Normal Option Score (Lines 199-215)

```tsx
<Controller
  name={`${fieldName}.settings.scores.${index}.score`}
  rules={{ required: t('criteria-labeled-options.value-required', 'Value is required') }}
  render={({ field, fieldState: { error } }) => (
    <NumberInput
      {...field}                    // ← NO onChange handler
      allowDecimal={false}
      min={0}                       // ← Has min validation
      clampBehavior="none"
      placeholder={t('criteria-labeled-options.value-placeholder', 'Value')}
      error={error?.message}
      hideControls
    />
  )}
/>
```

#### N/A Option Score (Lines 260-277)

```tsx
<Controller
  name={`${fieldName}.settings.scores.${isNAIndex}.score`}
  render={({ field }) => (
    <NumberInput
      {...field}
      allowDecimal={false}
      clampBehavior="none"
      placeholder={t('criteria-labeled-options.na-score-placeholder', 'no score')}  // ← Different placeholder ✓
      hideControls
      onChange={(value) => {        // ← HAS onChange handler ⚠️
        field.onChange(value);
        resetAutoQADetectedFields();
      }}
    />
  )}
/>
```

### ⚠️ INCONSISTENCY FOUND

**Issue:** N/A score has `onChange` handler with `resetAutoQADetectedFields()`, but normal score doesn't.

**Analysis:**
- In **decoupled mode**, autoQA stores **indices** (not score values)
- Changing a score doesn't invalidate autoQA mappings (indices don't change)
- Therefore, `resetAutoQADetectedFields()` is NOT needed when changing scores

**In legacy mode** (lines 217-235), it IS needed:
```tsx
// Legacy mode - autoQA stores VALUES
onChange={(value) => {
  field.onChange(value);
  resetAutoQADetectedFields();  // ← Correct: value changed, clear autoQA
}}
```

---

### Score Input (Legacy Mode - !enableDuplicateScoreForCriteria)

#### Normal Option Value (Lines 217-235)

```tsx
<Controller
  name={`${fieldName}.settings.options.${index}.value`}
  rules={{ required: t('criteria-labeled-options.value-required', 'Value is required') }}
  render={({ field, fieldState: { error } }) => (
    <NumberInput
      {...field}
      allowDecimal={false}
      min={0}
      clampBehavior="none"
      placeholder={t('criteria-labeled-options.value-placeholder', 'Value')}
      error={error?.message}
      hideControls
      onChange={(value) => {
        field.onChange(value);
        resetAutoQADetectedFields();  // ← Correct: in legacy mode, autoQA stores values
      }}
    />
  )}
/>
```

**Note:** Legacy mode doesn't support N/A scores (no decoupled scoring), so there's no N/A comparison.

---

### Deletion

#### Normal Option (Line 238)
```tsx
<ActionIcon onClick={() => handleRemoveOption(index)}>
  <IconTrash size={16} />
</ActionIcon>
```

#### N/A Option (Lines 297-299)
```tsx
// In checkbox onChange
if (!checked && isNAIndex >= 0) {
  handleRemoveOption(isNAIndex);  // ← Same function ✓
}
```

**✅ Deletion uses same function**

---

## Summary of Issues

### 1. ⚠️ INCONSISTENT: onChange handler in decoupled mode

**N/A score (decoupled mode)** has:
```tsx
onChange={(value) => {
  field.onChange(value);
  resetAutoQADetectedFields();  // ← Should NOT be here
}}
```

**Normal score (decoupled mode)** has:
```tsx
{...field}  // ← Correct: no onChange needed
```

**Fix:** Remove the onChange handler from N/A score in decoupled mode. It should match normal scores.

---

### 2. ✅ EXPECTED: Validation differences

**Normal score:**
- `rules={{ required: ... }}`
- `min={0}`
- Shows error message

**N/A score:**
- No required rule (can be null)
- No min (can accept null)
- No error display

**This is correct** - N/A scores are optional, normal scores are required.

---

### 3. ✅ EXPECTED: Placeholder differences

**Normal:** "Value"  
**N/A:** "no score"

This is correct.

---

## Recommended Fix

Remove the onChange handler from N/A score (lines 271-274):

```tsx
// BEFORE
<NumberInput
  {...field}
  onChange={(value) => {
    field.onChange(value);
    resetAutoQADetectedFields();  // ← Remove this
  }}
/>

// AFTER
<NumberInput
  {...field}  // ← Just like normal scores
  allowDecimal={false}
  clampBehavior="none"
  placeholder={t('criteria-labeled-options.na-score-placeholder', 'no score')}
  hideControls
/>
```

This makes N/A score handling truly identical to normal scores (except for the expected differences).
