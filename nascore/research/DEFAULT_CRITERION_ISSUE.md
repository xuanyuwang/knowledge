# Critical Issue: DEFAULT_CRITERION Incompatible with Decoupled Scoring

**Created:** 2026-04-12  
**Severity:** HIGH  
**Status:** Needs immediate fix

---

## Problem

`DEFAULT_CRITERION` in consts.ts provides **options but NO scores**, which is incompatible with the decoupled scoring model after we removed the legacy migration.

---

## Root Cause Analysis

### DEFAULT_CRITERION (consts.ts lines 18-28)

```tsx
export const DEFAULT_CRITERION = {
  type: CriterionTypes.LabeledRadios,
  weight: 1,
  displayName: 'This is a new criterion',
  required: true,
  settings: {
    options: [{ label: 'Yes', value: 1 }, { label: 'No', value: 0 }],  // ← Has options
    range: { min: 0, max: 10 },
    showNA: true,
    // ← NO scores field!
  },
};
```

### Flow When Creating New Criterion

**Step 1:** TemplateBuilderFormConfigurationStep creates criterion (lines 297-300)
```tsx
const newCriterion = {
  ...cloneDeep(DEFAULT_CRITERION),
  identifier: newCriterionIdentifier,
};
```

**Result:**
```tsx
{
  settings: {
    options: [{ label: 'Yes', value: 1 }, { label: 'No', value: 0 }],  // ✅ Has options
    scores: undefined,  // ❌ No scores!
  }
}
```

**Step 2:** CriteriaLabeledOptions.tsx mounts (lines 92-102)
```tsx
useOnMount(() => {
  // Show first option by default
  if (optionsField.fields.length === 0) {  // ← FALSE! Already has 2 options
    onAddLabel();  // ← NOT called
  }
  // ...
});
```

**Problem:**
- `optionsField.fields.length` = 2 (from DEFAULT_CRITERION)
- Condition is FALSE
- `onAddLabel()` is NOT called
- Scores array stays empty/undefined

**Step 3:** Legacy migration (REMOVED in our fix)

**Before our fix:**
```tsx
// Lines 94-96 (REMOVED)
if (watchedOptionsField?.length && !watchedScoresField?.length) {
  scoresFieldArray.replace(watchedOptionsField.map((opt) => ({ value: opt.value, score: opt.value })));
}
```

This would create:
```tsx
scores: [
  { value: 1, score: 1 },
  { value: 0, score: 0 }
]
```

**After our fix:**
- Migration is removed
- Scores stay empty!
- **UI breaks: score inputs have no data**

---

## Current Behavior (BROKEN)

### Creating new criterion:

1. User clicks "Add Criterion"
2. TemplateBuilderFormConfigurationStep creates criterion with DEFAULT_CRITERION
3. Criterion has `options: [Yes, No]` but `scores: undefined`
4. CriteriaLabeledOptions renders
5. Options exist, so doesn't call `onAddLabel()`
6. Scores don't exist, and migration is removed
7. **Score inputs are broken - no data to bind to**

---

## Solution Options

### Option 1: Remove options from DEFAULT_CRITERION

**Change DEFAULT_CRITERION to:**
```tsx
export const DEFAULT_CRITERION = {
  type: CriterionTypes.LabeledRadios,
  weight: 1,
  displayName: 'This is a new criterion',
  required: true,
  settings: {
    options: [],  // ← Empty array
    scores: [],   // ← Empty array
    range: DEFAULT_CRITERION_SETTINGS_RANGE,
    showNA: true,
  },
};
```

**Effect:**
- New criterion starts with empty options and scores
- CriteriaLabeledOptions.tsx useOnMount sees `optionsField.fields.length === 0`
- Calls `onAddLabel()` to create first option with proper score
- ✅ Creates: `options: [{ label: '', value: 0 }]` and `scores: [{ value: 0, score: 0 }]`

**Benefits:**
- ✅ Compatible with decoupled scoring
- ✅ Clean initialization via `onAddLabel()`
- ✅ No need for legacy migration

**Drawbacks:**
- ❌ User has empty label fields (but they're empty anyway with "This is a new criterion" as display name)

---

### Option 2: Add scores to DEFAULT_CRITERION

**Change DEFAULT_CRITERION to:**
```tsx
export const DEFAULT_CRITERION = {
  type: CriterionTypes.LabeledRadios,
  weight: 1,
  displayName: 'This is a new criterion',
  required: true,
  settings: {
    options: [
      { label: 'Yes', value: 0 },  // ← Fixed: sequential values
      { label: 'No', value: 1 },
    ],
    scores: [
      { value: 0, score: 0 },  // ← Added scores
      { value: 1, score: 0 },
    ],
    range: DEFAULT_CRITERION_SETTINGS_RANGE,
    showNA: true,
  },
};
```

**Benefits:**
- ✅ Complete decoupled scoring setup
- ✅ User gets default "Yes/No" options

**Drawbacks:**
- ❌ Hardcoded default options (not flexible)
- ❌ Still need to fix values from [1,0] to [0,1]

---

### Option 3: Restore minimal migration

**Add back ONLY the scores initialization (not the full migration):**

```tsx
useOnMount(() => {
  // Initialize scores if options exist but scores don't
  if (watchedOptionsField?.length && !watchedScoresField?.length) {
    scoresFieldArray.replace(
      watchedOptionsField.map((opt, index) => ({ 
        value: index,  // ← Use index, not opt.value
        score: 0 
      }))
    );
  }
  
  // Show first option by default if no options
  if (optionsField.fields.length === 0) {
    onAddLabel();
  }
  
  // N/A migration
  const currentOptions = form.getValues(`${fieldName as FieldNameType}.settings.options`);
  if (enableNAScore && showNAField.value && !currentOptions?.some((opt) => opt.isNA)) {
    onAddLabel(true);
  }
});
```

**Benefits:**
- ✅ Handles DEFAULT_CRITERION gracefully
- ✅ Initializes scores with correct values (using index)
- ✅ No breaking changes

**Drawbacks:**
- ❌ Still has migration code (though safer)

---

## Recommended Fix

**Option 1: Remove options from DEFAULT_CRITERION**

This is the cleanest approach:

1. **Change consts.ts:**
   ```tsx
   export const DEFAULT_CRITERION = {
     type: CriterionTypes.LabeledRadios,
     weight: 1,
     displayName: 'This is a new criterion',
     required: true,
     settings: {
       options: [],
       scores: [],
       range: DEFAULT_CRITERION_SETTINGS_RANGE,
       showNA: true,
     },
   };
   ```

2. **CriteriaLabeledOptions.tsx automatically handles it:**
   - `optionsField.fields.length === 0` → TRUE
   - Calls `onAddLabel()` on mount
   - Creates first option with value: 0, score: 0

3. **Default criterion copying still works:**
   - When copying from default criterion, options are copied (lines 313-318 in TemplateBuilderFormConfigurationStep)
   - If user configured first criterion with custom options, those get copied to second criterion

---

## Testing Checklist

After applying fix:

- [ ] Create new criterion → should have one empty option with score 0
- [ ] Click "Add Option" → should create second option with value 1, score 0
- [ ] Check "Allow N/A" → should create N/A option with value 2, score null
- [ ] Create second criterion → should copy options from first (if first was configured)
- [ ] Verify AutoQA dropdowns show correct labels
- [ ] Verify score inputs are not auto-populated when disabling AutoQA
- [ ] Verify no console errors or warnings

---

## Impact Assessment

**Without this fix:**
- ❌ New criteria have broken score inputs
- ❌ Users cannot enter scores
- ❌ Template creation is blocked

**Priority:** CRITICAL - Must fix before merging enableNAScore feature
