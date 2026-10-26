# Score Auto-Population Fix

## Problem

**Current behavior:**
1. Create new criterion → AutoQA is CHECKED → score boxes are empty ✅
2. Uncheck "Automated scoring" → scores become 0 and 1 ❌

**Expected behavior:**  
Scores should stay empty when disabling AutoQA

## Root Cause

Line 94-96 in `CriteriaLabeledOptions.tsx`:
```tsx
// Initialize scores for legacy templates that don't have scores yet
if (watchedOptionsField?.length && !watchedScoresField?.length) {
  scoresFieldArray.replace(watchedOptionsField.map((opt) => ({ value: opt.value, score: opt.value })));
}
```

This legacy migration runs when:
- Options exist (Yes, No)
- Scores array is empty or doesn't exist

**When this happens:**
- User unchecks AutoQA
- Component re-renders or re-mounts
- Condition becomes TRUE
- Creates scores using `option.value`: `[{value: 0, score: 0}, {value: 1, score: 1}]`

## Solution Options

### Option 1: Initialize empty scores when options are created

Ensure scores array always exists alongside options, even if empty:
```tsx
const newScore = { value: newValue, score: undefined };  // Empty score
```

Then legacy migration won't trigger (scores array exists).

### Option 2: Make legacy migration smarter

Only migrate if options have labels (real legacy template):
```tsx
if (watchedOptionsField?.length && !watchedScoresField?.length) {
  // Only migrate if this looks like a real legacy template
  const hasNonEmptyLabels = watchedOptionsField.some(opt => opt.label && opt.label !== '');
  if (hasNonEmptyLabels) {
    scoresFieldArray.replace(watchedOptionsField.map((opt) => ({ value: opt.value, score: opt.value })));
  }
}
```

### Option 3: Initialize scores when toggling AutoQA off

In `handleToggleAutoScorable`, when turning AutoQA off, ensure scores exist as empty:
```tsx
// In TemplateBuilderCriterionConfiguration.tsx
if (autoScorable) {
  turnOffAndResetAutoScorable();
  // Ensure scores exist but are empty
  const options = form.getValues(`${itemFieldPath}.settings.options`);
  if (options?.length) {
    form.setValue(`${itemFieldPath}.settings.scores`, 
      options.map((opt, i) => ({ value: i, score: undefined }))
    );
  }
}
```

## Recommended Fix

**Option 2 is safest** - make legacy migration smarter to not run on new criteria.

This way:
- Real legacy templates still get migrated ✅
- New criteria don't auto-populate ✅  
- No changes needed to other parts of the code ✅
