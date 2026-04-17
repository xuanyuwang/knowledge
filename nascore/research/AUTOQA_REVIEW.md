# TemplateBuilderAutoQA - Code Review

**File:** `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/TemplateBuilderAutoQA/TemplateBuilderAutoQA.tsx`

**Created:** 2026-04-12  
**Purpose:** Configure AutoQA behavior mapping - which Opera rules trigger which criterion scores

---

## Overview

This component handles AutoQA configuration for a criterion. It allows users to:
1. Select an Opera behavior/rule
2. Choose score type (Behavior Done/Not Done vs # of Occurrences)
3. Map outcomes (detected/not detected/not applicable) to score values

---

## Key Workflows

### Workflow 1: Select Behavior

**User Action:** User selects a behavior from the dropdown

**Code Flow:**
```
1. User selects "Greeting detected" from dropdown (line 354-372)
   ↓
2. handleChangeTriggersField() called with "trigger-greeting_detected" (line 187-200)
   ↓
3. Lookup trigger object from triggerOptionsMap (line 191)
   ↓
4. Update form field: auto_qa.triggers = [triggerObject] (line 193)
   ↓
5. Notify parent via onSelectedBehaviorChanged callback (line 196)
```

**Key Code:**
- **Line 187-200:** `handleChangeTriggersField` - converts select value to trigger object
- **Line 354-372:** Behavior dropdown Select component

**Data Flow:**
- **Input:** String like `"trigger-greeting_detected"`
- **Stored:** Array with one trigger object: `[{ type: "trigger", resource_name: "greeting_detected", ... }]`
- **Purpose:** Links this criterion to an Opera rule

---

### Workflow 2: Select Score Type (Behavior D/ND vs # of Occurrences)

**User Action:** User clicks radio button for "Behavior Done/Not Done" or "# of Occurrences"

**Code Flow:**
```
1. User clicks radio button (line 385-400)
   ↓
2. onChangeScoreTypeConfigurationRadio() called (line 310-316)
   ↓
3. setSelectedScoreTypeConfiguration(newValue) - updates parent state
   ↓
4. resetSettings(newValue) - clears outcome mappings for new mode
   ↓
5. UI switches between outcome dropdowns (D/ND) vs numeric bins configurator
```

**Conditional Rendering:**
- **Line 401:** `selectedScoreTypeConfiguration === ScoreTypeConfiguration.BEHAVIOR_DND`
  - **TRUE** → Show 3 dropdowns (detected/not detected/not applicable)
  - **FALSE** → Show NumericBinsAndValuesConfigurator component

**Data Reset:**
When switching modes, `resetSettings` clears incompatible data to prevent corruption.

---

### Workflow 3: Map "If behavior is done" → Score Value

**User Action:** User selects "Yes (5)" from "If behavior is done" dropdown

**Code Flow:**
```
1. User selects "Yes (5)" (line 412-428)
   ↓
2. onChangeDetectedField("0") called - value is INDEX (line 285-294)
   ↓
3. Check isDecoupledScoring (line 287)
   ↓
4. Store NUMBER: detectedField.onChange(Number("0")) = 0
   ↓
5. Form data: auto_qa.detected = 0
```

**Key Insight:**
- Dropdown displays: `"Yes (5)"` (label with score)
- Dropdown value: `"0"` (stringified index)
- Stored in form: `0` (number index)

**Why Convert String → Number?**
- Mantine Select requires **string** values
- Form stores **numbers** (indices or values depending on mode)
- `onChangeDetectedField` bridges the two

---

### Workflow 4: Map "If behavior is not applicable" → N/A Score (NEW)

**User Action:** User selects "N/A (no score)" from "If behavior is not applicable" dropdown

**Code Flow:**
```
1. Check if row should show (line 131)
   showNotApplicableRow = enableNAScore && showNAField && isDND
   ↓
2. If TRUE, render dropdown (line 459-489)
   ↓
3. User selects N/A option (value = stringified isNAIndex)
   ↓
4. onChangeNotApplicableField() converts to number (line 134-139)
   ↓
5. Form data: auto_qa.not_applicable = <index>
```

**Unique Feature:**
- **Clearable** (line 483) - only N/A dropdown can be cleared
- **Optional** - not required validation (detected/not_detected are required)

---

## Data Synchronization Mechanisms

### 1. Options Dropdown → AutoQA Dropdowns (Lines 231-274)

**Source:** `scoreOptions.options` and `scoreOptions.scores`  
**Derived:** `behaviorScoreSelectionOptions` (dropdown options)

**Code:**
```tsx
const behaviorScoreSelectionOptions = useMemo((): ComboboxItem[] => {
  if (isDecoupledScoring) {
    // Use INDEX as value
    scoreOptions.options.forEach((option, index) => {
      const score = scoreOptions.scores?.[index]?.score;
      selectionOptions.push({ 
        label: `${option.label} (${score ?? option.value})`,  // "Yes (5)"
        value: index.toString()                                 // "0"
      });
    });
  } else {
    // Use option.VALUE as value (legacy mode)
    uniqueValueOptions.forEach((option) => {
      selectionOptions.push({ 
        label: option.label,           // "Yes"
        value: option.value.toString() // "5"
      });
    });
  }
}, [scoreOptions.options, scoreOptions.scores, scoreType, isDecoupledScoring]);
```

**Sync Mechanism:**
- `useMemo` with dependencies on `scoreOptions.options` and `scoreOptions.scores`
- Whenever options or scores change, dropdown automatically rebuilds
- **Decoupled mode:** Shows score in label, uses index as value
- **Legacy mode:** Uses option value directly

**Example:**
```
Options: [{ label: "Yes", value: 0, isNA: false }, { label: "N/A", value: 1, isNA: true }]
Scores:  [{ value: 0, score: 5 }, { value: 1, score: null }]

Dropdown options (decoupled):
- { label: "Yes (5)", value: "0" }
- { label: "N/A (no score)", value: "1" }
```

---

### 2. N/A Option Visibility → "Not Applicable" Dropdown (Line 131)

**Condition:**
```tsx
const showNotApplicableRow = enableNAScore && !!showNAField && selectedScoreTypeConfigIsBehaviorDND;
```

**Sync:**
- When user checks "Allow N/A" in CriteriaLabeledOptions:
  - `showNAField` becomes `true`
  - `showNotApplicableRow` becomes `true`
  - Third dropdown appears
- When user unchecks "Allow N/A":
  - `showNAField` becomes `false`
  - `showNotApplicableRow` becomes `false`
  - Third dropdown disappears

**Data cleanup:**
When N/A option is removed in CriteriaLabeledOptions, `handleRemoveOption` remaps `auto_qa.not_applicable` to `null` if it referenced the removed option.

---

### 3. String/Number Conversion (Lines 285-305)

**Problem:** Mantine Select uses strings, form stores numbers

**Solution:** Three similar converters

```tsx
// Detected
const onChangeDetectedField = useCallback((value: string | null) => {
  if (isDecoupledScoring) {
    detectedField.onChange(value != null ? Number(value) : null);
  } else {
    detectedField.onChange(Number(value));
  }
}, [detectedField, isDecoupledScoring]);

// Not Detected
const onChangeNotDetectedField = useCallback((value: string | null) => {
  if (isDecoupledScoring) {
    notDetectedField.onChange(value != null ? Number(value) : null);
  } else {
    notDetectedField.onChange(Number(value));
  }
}, [notDetectedField, isDecoupledScoring]);

// Not Applicable
const onChangeNotApplicableField = useCallback((value: string | null) => {
  notApplicableField.onChange(
    value != null ? (isDecoupledScoring ? parseInt(value) : parseFloat(value)) : null
  );
}, [notApplicableField, isDecoupledScoring]);
```

**Pattern:** All three do the same thing with slight variations

---

## Simplification Opportunities

### 1. **DUPLICATE CODE:** Three onChange converters (Lines 285-305, 134-139)

**Current:**
- `onChangeDetectedField` (10 lines)
- `onChangeNotDetectedField` (10 lines)
- `onChangeNotApplicableField` (6 lines)

**All do the same thing:** Convert string → number for form storage

**Proposed Refactor:**
```tsx
const createNumberFieldOnChange = useCallback(
  (field: { onChange: (value: number | null) => void }) => 
    (value: string | null) => {
      if (isDecoupledScoring) {
        field.onChange(value != null ? Number(value) : null);
      } else {
        field.onChange(Number(value));
      }
    },
  [isDecoupledScoring]
);

// Then use:
onChange={createNumberFieldOnChange(detectedField)}
onChange={createNumberFieldOnChange(notDetectedField)}
onChange={createNumberFieldOnChange(notApplicableField)}
```

**Impact:** -20 lines, eliminates duplication

---

### 2. **DUPLICATE CODE:** Three value converters (Lines 307-308, 133)

**Current:**
```tsx
const detectedFieldValue = detectedField.value != null ? String(detectedField.value) : null;
const notDetectedFieldValue = notDetectedField.value != null ? String(notDetectedField.value) : null;
const notApplicableFieldValue = notApplicableField.value != null ? String(notApplicableField.value) : null;
```

**All do the same thing:** Convert number → string for Select component

**Proposed Refactor:**
```tsx
const toStringValue = (value: number | null | undefined): string | null =>
  value != null ? String(value) : null;

const detectedFieldValue = toStringValue(detectedField.value);
const notDetectedFieldValue = toStringValue(notDetectedField.value);
const notApplicableFieldValue = toStringValue(notApplicableField.value);
```

**OR** inline it directly in JSX:
```tsx
value={detectedField.value != null ? String(detectedField.value) : null}
```

**Impact:** Clearer intent, less duplication

---

### 3. **COMPLEX CONDITION:** showNotApplicableRow (Line 131)

**Current:**
```tsx
const showNotApplicableRow = enableNAScore && !!showNAField && selectedScoreTypeConfigIsBehaviorDND;
```

**Could be clearer:**
```tsx
const showNotApplicableRow = 
  enableNAScore && 
  !!showNAField && 
  selectedScoreTypeConfiguration === ScoreTypeConfiguration.BEHAVIOR_DND;
```

**Better with comment:**
```tsx
// Show "Not Applicable" row when:
// - N/A feature is enabled
// - "Allow N/A" is checked
// - In "Behavior Done/Not Done" mode (not "# of Occurrences")
const showNotApplicableRow = 
  enableNAScore && 
  !!showNAField && 
  selectedScoreTypeConfiguration === ScoreTypeConfiguration.BEHAVIOR_DND;
```

---

### 4. **INCONSISTENCY:** parseInt vs Number (Line 136)

**Current:**
```tsx
value != null ? (isDecoupledScoring ? parseInt(value) : parseFloat(value)) : null
```

**Other two use:**
```tsx
value != null ? Number(value) : null
```

**Question:** Why does `onChangeNotApplicableField` use `parseInt/parseFloat` while others use `Number`?

**Recommendation:** Use `Number()` consistently everywhere unless there's a specific reason.

---

### 5. **UNCLEAR LOGIC:** isDecoupledScoring definition (Line 80-81)

**Current:**
```tsx
const isDecoupledScoring =
  scoreType === CriterionTypes.LabeledRadios || scoreType === CriterionTypes.DropdownNumericValues;
```

**After feature flag cleanup, this is always TRUE** (we removed legacy mode support)

**Question:** Can we simplify or remove this check entirely?

If we're always in decoupled mode, the ternaries in `onChangeDetectedField` etc. are dead code:
```tsx
if (isDecoupledScoring) {
  field.onChange(value != null ? Number(value) : null);  // ← Always this
} else {
  field.onChange(Number(value));                          // ← Never this
}
```

**Proposed:**
```tsx
const onChangeDetectedField = useCallback(
  (value: string | null) => {
    detectedField.onChange(value != null ? Number(value) : null);
  },
  [detectedField]
);
```

---

## Questions for Investigation

1. **Line 80-81:** Is `isDecoupledScoring` always true now after feature flag cleanup?
2. **Line 136:** Why does N/A use `parseInt/parseFloat` instead of `Number()`?
3. **Line 245-255:** Why do we need `uniqBy` for legacy mode but not decoupled?
4. **Line 276-283:** `behaviorScoreDisabled` - complex validation logic, can it be simplified?

---

## Summary

### Key Insights
1. **Data flow:** Options → Dropdown options (useMemo) → User selection → Number conversion → Form storage
2. **Sync mechanism:** `useMemo` dependencies automatically rebuild dropdowns when options change
3. **N/A integration:** Third dropdown appears/disappears based on "Allow N/A" checkbox
4. **String/Number bridge:** Mantine Select requires strings, form stores numbers

### Simplification Targets
1. ✅ **HIGH PRIORITY:** Merge 3 onChange converters into one helper
2. ✅ **HIGH PRIORITY:** Remove `isDecoupledScoring` ternaries if always true
3. ✅ **MEDIUM:** Simplify value string conversion
4. ✅ **LOW:** Add comment to `showNotApplicableRow` condition

### Code Quality
- **Duplication:** ~26 lines of duplicate string/number conversion code
- **Clarity:** Some complex conditions could use comments
- **Dead code risk:** `isDecoupledScoring` ternaries may be obsolete

**Next Steps:** Investigate questions above, then implement simplifications.
