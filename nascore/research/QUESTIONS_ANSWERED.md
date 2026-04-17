# Questions Answered - DEFAULT_CRITERION & AutoQA Logic

**Created:** 2026-04-12  
**Related Files:**
- `consts.ts`
- `TemplateBuilderFormConfigurationStep.tsx`
- `TemplateBuilderCriterionConfiguration.tsx`

---

## Q1: What is DEFAULT_CRITERION?

**Location:** `consts.ts` lines 18-28

```tsx
export const DEFAULT_CRITERION = {
  type: CriterionTypes.LabeledRadios,
  weight: 1,
  displayName: 'This is a new criterion',
  required: true,
  settings: {
    options: DEFAULT_CRITERION_SETTINGS_OPTIONS,  // [{ label: 'Yes', value: 1 }, { label: 'No', value: 0 }]
    range: DEFAULT_CRITERION_SETTINGS_RANGE,      // { min: 0, max: 10 }
    showNA: true,
  },
};
```

**Answer:**
- Type: LabeledRadios (dropdown)
- Default options: **Yes = 1, No = 0** (old format, no `score` field!)
- **showNA defaults to TRUE**
- Default range: 0-10 for numeric inputs
- Weight: 1
- Required: true

**IMPORTANT:** `DEFAULT_CRITERION_SETTINGS_OPTIONS` uses old format!
```tsx
export const DEFAULT_CRITERION_SETTINGS_OPTIONS = [
  { label: 'Yes', value: 1 },  // ← No "score" field!
  { label: 'No', value: 0 },
];
```

This is **pre-decoupled scoring** format where `value` is both:
- Wiring value (index for AutoQA mapping)
- Score value (points awarded)

**Why this works:**
- CriteriaLabeledOptions.tsx (line 94-96, REMOVED in our fix) used to migrate this to decoupled format
- Since we removed that migration, these old default options might cause issues!

**Potential Bug:** 
- New criterion gets `options: [{ label: 'Yes', value: 1 }, { label: 'No', value: 0 }]`
- CriteriaLabeledOptions expects `value` to be index (0, 1, 2, ...)
- But DEFAULT_CRITERION has `value: 1, 0` instead of `value: 0, 1`

**Need to verify:** Does this break anything? Should we fix DEFAULT_CRITERION_SETTINGS_OPTIONS?

---

## Q2: Why is AutoQA "checked initially"?

**Location:** `TemplateBuilderCriterionConfiguration.tsx` line 106

```tsx
const [autoScorable, setAutoScorable] = useState<boolean>(
  !!autoQAFieldValue && !!autoQAFieldValue.triggers
);
```

**How it works:**
- Checkbox is checked when: `autoQAFieldValue` exists AND `autoQAFieldValue.triggers` exists (truthy)
- For new criterion: `auto_qa: { triggers: [], detected: 1, not_detected: 0, not_applicable: null }`
- `!!autoQAFieldValue` = `!!{...}` = `true` ✅
- `!!autoQAFieldValue.triggers` = `!![]` = `true` ✅ (empty array is truthy!)

**Answer:** AutoQA checkbox IS checked by default because:
1. New criteria get `auto_qa` object (from TemplateBuilderFormConfigurationStep lines 288-295)
2. `auto_qa.triggers` is empty array `[]`
3. Empty array is truthy in JavaScript: `!![] === true`
4. Checkbox reads `!!autoQAFieldValue.triggers` which is `true`

**This is the root cause of user's observation!**

---

## Q3: How does showNA default for first criterion?

**Answer:** Depends on whether default criterion exists:

### Case 1: First criterion ever (no default)

**Code:** TemplateBuilderFormConfigurationStep.tsx lines 297-300
```tsx
const newCriterion = {
  ...cloneDeep(DEFAULT_CRITERION),  // ← Has showNA: true
  identifier: newCriterionIdentifier,
};
```

**Result:** `showNA: true` (from DEFAULT_CRITERION)

### Case 2: Second criterion (has default)

**Code:** TemplateBuilderFormConfigurationStep.tsx line 312
```tsx
newCriterion.settings.showNA = defaultCriterion.settings.showNA ?? true;
```

**Result:**
- If default has `showNA: false` → use `false`
- If default has `showNA: undefined` → use `true`
- If default has `showNA: true` → use `true`

**Summary:** showNA defaults to `true` in both cases!

---

## Q4: Is `...newAutoQA,` necessary?

**Location:** TemplateBuilderFormConfigurationStep.tsx lines 321-334

```tsx
form.setValue(
  itemsListFieldName,
  [
    ...itemsList,
    {
      ...newCriterion,   // ← From DEFAULT_CRITERION (no auto_qa field)
      ...newAutoQA,      // ← { auto_qa: {...} }
      itemType: 'performance',
    },
  ],
  { shouldDirty: true }
);
```

**Current approach:**
1. Create `newCriterion` from DEFAULT_CRITERION (no `auto_qa`)
2. Create `newAutoQA = { auto_qa: {...} }` separately
3. Spread both: `{ ...newCriterion, ...newAutoQA }`

**Answer:** YES, the spread is necessary with current structure.

**Why?**
- DEFAULT_CRITERION does NOT have `auto_qa` field (see consts.ts lines 18-28)
- `newCriterion` inherits from DEFAULT_CRITERION, so no `auto_qa`
- `newAutoQA` provides the `auto_qa` field
- Spread combines them: `{ type, weight, displayName, settings, auto_qa, itemType }`

---

## Simplification Opportunity: Eliminate `newAutoQA` variable

**Current code** (lines 288-334):
```tsx
// Create separate newAutoQA object
const newAutoQA = {
  auto_qa: {
    triggers: [],
    detected: 1,
    not_detected: 0,
    not_applicable: null,
  } as ScorecardTemplateAutoQA,
};

const newCriterion = {
  ...cloneDeep(DEFAULT_CRITERION),
  identifier: newCriterionIdentifier,
};

// Copy from default criterion
if (defaultCriterion) {
  if ('auto_qa' in defaultCriterion && defaultCriterion.auto_qa) {
    newAutoQA.auto_qa.detected = defaultCriterion.auto_qa.detected;
    newAutoQA.auto_qa.not_detected = defaultCriterion.auto_qa.not_detected;
    newAutoQA.auto_qa.not_applicable = defaultCriterion.auto_qa.not_applicable ?? null;
  }
  // ... other copying
}

form.setValue(itemsListFieldName, [
  ...itemsList,
  {
    ...newCriterion,
    ...newAutoQA,  // ← Extra spread
    itemType: 'performance',
  },
]);
```

**Simplified code:**
```tsx
const newCriterion = {
  ...cloneDeep(DEFAULT_CRITERION),
  identifier: newCriterionIdentifier,
  auto_qa: {
    triggers: [],
    detected: 1,
    not_detected: 0,
    not_applicable: null,
  } as ScorecardTemplateAutoQA,
};

// Copy from default criterion
if (defaultCriterion) {
  if ('auto_qa' in defaultCriterion && defaultCriterion.auto_qa) {
    newCriterion.auto_qa.detected = defaultCriterion.auto_qa.detected;
    newCriterion.auto_qa.not_detected = defaultCriterion.auto_qa.not_detected;
    newCriterion.auto_qa.not_applicable = defaultCriterion.auto_qa.not_applicable ?? null;
  }
  // ... other copying
}

form.setValue(itemsListFieldName, [
  ...itemsList,
  {
    ...newCriterion,  // ← Already has auto_qa
    itemType: 'performance',
  },
]);
```

**Benefits:**
- ✅ Eliminate `newAutoQA` variable
- ✅ Eliminate `...newAutoQA,` spread
- ✅ Simpler, more direct code
- ✅ Fewer object creations

**Same pattern in `handleAddOutcome`:**

Lines 351-391 have identical pattern with separate `newAutoQA` variable. Can be simplified the same way.

---

## Critical Issue: DEFAULT_CRITERION_SETTINGS_OPTIONS

**Problem:** Default options use old format (pre-decoupled scoring)

```tsx
export const DEFAULT_CRITERION_SETTINGS_OPTIONS = [
  { label: 'Yes', value: 1 },  // ← Should be value: 0 (index)
  { label: 'No', value: 0 },   // ← Should be value: 1 (index)
];
```

**Expected format (decoupled scoring):**
```tsx
export const DEFAULT_CRITERION_SETTINGS_OPTIONS = [
  { label: 'Yes', value: 0 },  // ← Index in array
  { label: 'No', value: 1 },
];
```

**Why this matters:**
- AutoQA mapping uses index: "detected: 1" means "map to option at index 1"
- Default has Yes=1, No=0 (backwards!)
- Should have Yes=0, No=1 (sequential indexes)

**Why it didn't break before:**
- Legacy migration at CriteriaLabeledOptions.tsx (lines 94-96, REMOVED in our fix) would renormalize values
- Migration would convert { label: 'Yes', value: 1 } → { label: 'Yes', value: 0 }
- Now that we removed the migration, this might break!

**Test scenario:**
1. Create new criterion
2. DEFAULT_CRITERION gives options: [{ label: 'Yes', value: 1 }, { label: 'No', value: 0 }]
3. CriteriaLabeledOptions.tsx line 76-87 (onAddLabel) expects sequential values starting from 0
4. Mismatch!

**Fix needed:**
```tsx
export const DEFAULT_CRITERION_SETTINGS_OPTIONS = [
  { label: 'Yes', value: 0 },  // ← Fix: index 0
  { label: 'No', value: 1 },   // ← Fix: index 1
];
```

---

## Summary

### Answers:

1. **DEFAULT_CRITERION:** Type=LabeledRadios, options=[Yes=1, No=0], showNA=true, weight=1
   - ⚠️ Options use old format with backwards values!

2. **AutoQA checked by default:** YES, because `!!autoQAFieldValue.triggers` where `triggers = []` (truthy)

3. **showNA defaults:** Always `true` (from DEFAULT_CRITERION or fallback)

4. **`...newAutoQA` necessary:** YES with current structure, but can be simplified

### Issues Found:

1. ⚠️ **Critical:** DEFAULT_CRITERION_SETTINGS_OPTIONS has backwards values (1, 0 instead of 0, 1)
   - Will break now that we removed legacy migration
   - Fix: Change to `[{ label: 'Yes', value: 0 }, { label: 'No', value: 1 }]`

2. **Minor:** `newAutoQA` variable and spread can be eliminated for simpler code

### Recommended Actions:

1. **IMMEDIATE:** Fix DEFAULT_CRITERION_SETTINGS_OPTIONS values
2. **NICE-TO-HAVE:** Simplify by eliminating `newAutoQA` variable
3. **TESTING:** Verify new criteria creation still works after removing legacy migration
