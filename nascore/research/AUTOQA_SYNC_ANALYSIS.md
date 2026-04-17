# AutoQA Dropdown Sync & Validation Analysis

**Created:** 2026-04-12  
**Focus:** How options sync to AutoQA dropdowns + when validation runs

---

## Question 1: When is "Value is required" checked?

### Normal Score Inputs (YES, NO)

**Location:** `CriteriaLabeledOptions.tsx` line 190
```tsx
<Controller
  name={`${fieldName}.settings.scores.${index}.score`}
  rules={{ required: t('criteria-labeled-options.value-required', 'Value is required') }}
  render={({ field, fieldState: { error } }) => (
    <NumberInput {...field} error={error?.message} />
  )}
/>
```

**Validation Rule:** `required: 'Value is required'`

**When Checked:**
1. ✅ **On blur** - when user leaves the input field
2. ✅ **On submit** - when form is submitted
3. ✅ **On change** - after first blur, validates on every change

**What Passes Validation:**
- ✅ `0` - Zero is valid!
- ✅ `5` - Any number
- ❌ `null` - Fails
- ❌ `undefined` - Fails
- ❌ `""` - Empty string fails

**Key Insight:** React-hook-form's `required` rule checks for **existence**, not truthiness!
- `value === 0` → PASSES (value exists)
- `value === null` → FAILS (value doesn't exist)
- `value === undefined` → FAILS (value doesn't exist)

---

### N/A Score Input

**Location:** `CriteriaLabeledOptions.tsx` line 231
```tsx
<Controller
  name={`${fieldName}.settings.scores.${isNAIndex}.score`}
  render={({ field }) => (
    <NumberInput {...field} placeholder="no score" />
  )}
/>
```

**Validation Rule:** NONE - no `rules` prop

**Why No Validation:**
- N/A scores **can be null** (that's the whole point!)
- User can leave it empty or enter a number
- Both are valid states

---

### Why Default Scores (0) Are Not Displayed

**Default Values When Creating Options:**

From `CriteriaLabeledOptions.tsx` line 79:
```tsx
const newScore = { value: newValue, score: isNA ? null : 0 };
```

**Initial State:**
```js
// Creating first option "Yes"
options[0] = { label: "", value: 0 }
scores[0] = { value: 0, score: 0 }  // ← score is 0

// Creating second option "No"
options[1] = { label: "", value: 1 }
scores[1] = { value: 1, score: 0 }  // ← score is 0

// Creating N/A option
options[2] = { label: "N/A", value: 2, isNA: true }
scores[2] = { value: 2, score: null }  // ← score is null
```

**Why Input Shows Empty:**

The NumberInput component shows the actual value:
- When `score = 0`, input displays `"0"` ✅
- But the **label** is `""` (empty string)
- User hasn't filled in "Yes" or "No" yet

**Validation Status:**
- ✅ Validation PASSES because `score = 0` (not null)
- ✅ User can submit form even with empty labels (different validation)
- The `required` rule on line 190 validates the **score**, not the **label**

**There's actually a separate validation for labels!**

From line 186:
```tsx
<Controller
  name={`${fieldName}.settings.options.${index}.label`}
  rules={{ required: t('criteria-labeled-options.label-required', 'Label is required') }}
  ...
/>
```

So both need to pass:
1. Label required → checks `options[i].label` is not empty
2. Value required → checks `scores[i].score` is not null/undefined

---

## Question 2: How are options synced to AutoQA behaviorScoreSelectionOptions?

### Data Flow Diagram

```
CriteriaLabeledOptions.tsx           TemplateBuilderAutoQA.tsx
┌─────────────────────────┐          ┌────────────────────────────┐
│ User edits options:     │          │                            │
│ - Add option            │          │                            │
│ - Edit label            │          │                            │
│ - Edit score            │          │                            │
│ - Remove option         │          │                            │
└────────┬────────────────┘          │                            │
         │                            │                            │
         ▼                            │                            │
┌─────────────────────────┐          │                            │
│ React Hook Form         │          │                            │
│ Form State:             │◄─────────┤ useWatch()                 │
│                         │   line 75│                            │
│ template.items[i]       │          │ const scoreOptions =       │
│  .settings.options[]    ├──────────┤   useWatch({               │
│  .settings.scores[]     │          │     name: `${itemFieldPath}│
└─────────────────────────┘          │           .settings`       │
                                     │   })                       │
                                     │                            │
                                     │ Auto-updates whenever      │
                                     │ options or scores change! │
                                     │                            │
                                     ▼                            │
                            ┌────────────────────────────┐        │
                            │ useMemo (line 231-274)     │        │
                            │                            │        │
                            │ Derives dropdown options   │        │
                            │ from scoreOptions          │        │
                            └────────┬───────────────────┘        │
                                     │                            │
                                     ▼                            │
                            ┌────────────────────────────┐        │
                            │ behaviorScoreSelectionOptions│       │
                            │                            │        │
                            │ [                          │        │
                            │   { label: "Yes (5)",      │        │
                            │     value: "0" },          │        │
                            │   { label: "No (0)",       │        │
                            │     value: "1" },          │        │
                            │   { label: "N/A (no score)",│       │
                            │     value: "2" }           │        │
                            │ ]                          │        │
                            └────────┬───────────────────┘        │
                                     │                            │
                                     ▼                            │
                            ┌────────────────────────────┐        │
                            │ <Select> components        │        │
                            │ - If behavior is done      │        │
                            │ - If behavior is not done  │        │
                            │ - If behavior is N/A       │        │
                            └────────────────────────────┘        │
                                                                  │
```

---

### Sync Mechanism Details

**Step 1: Watch the settings object** (line 75)

```tsx
const scoreOptions = useWatch({ 
  control: form.control, 
  name: `${itemFieldPath}.settings` 
}) as AutoQAScoreDefinitions;
```

**What this does:**
- Subscribes to changes in `template.items[i].settings`
- Includes both `options[]` and `scores[]`
- Auto-updates whenever user modifies options in CriteriaLabeledOptions
- React re-renders AutoQA component with new data

**Step 2: Derive dropdown options** (lines 231-274)

```tsx
const behaviorScoreSelectionOptions = useMemo((): ComboboxItem[] => {
  let selectionOptions: ComboboxItem[] = [];
  
  if (isDecoupledScoring && scoreOptions.options) {
    // Use INDEX as value
    scoreOptions.options.forEach((option: ScoreOption, index: number) => {
      if (option.value !== undefined) {
        const score = scoreOptions.scores?.[index]?.score;
        selectionOptions.push({ 
          label: `${option.label} (${score ?? option.value})`,  // "Yes (5)" or "N/A (no score)"
          value: index.toString()                                // "0", "1", "2"
        });
      }
    });
  }
  
  return selectionOptions;
}, [scoreOptions.options, scoreOptions.scores, scoreType, isDecoupledScoring]);
```

**Dependencies:** `[scoreOptions.options, scoreOptions.scores, ...]`
- When `options` changes → dropdown rebuilds
- When `scores` changes → dropdown rebuilds
- Labels/scores always stay in sync!

---

### Example Trace: User Edits Score

**Initial State:**
```js
options = [{ label: "Yes", value: 0 }]
scores = [{ value: 0, score: 5 }]

behaviorScoreSelectionOptions = [{ label: "Yes (5)", value: "0" }]
```

**User changes score from 5 → 10:**

1. User types "10" in CriteriaLabeledOptions
   ↓
2. Controller updates: `scores[0].score = 10`
   ↓
3. Form state changes → `useWatch` detects change
   ↓
4. `scoreOptions` gets new value with `scores[0].score = 10`
   ↓
5. `useMemo` dependencies changed → recomputes
   ↓
6. `behaviorScoreSelectionOptions` = `[{ label: "Yes (10)", value: "0" }]`
   ↓
7. AutoQA dropdown updates to show "Yes (10)"

**Sync is AUTOMATIC via React's reactivity!**

---

### Example Trace: User Adds N/A Option

**Initial State:**
```js
options = [
  { label: "Yes", value: 0 },
  { label: "No", value: 1 }
]
scores = [
  { value: 0, score: 5 },
  { value: 1, score: 0 }
]

behaviorScoreSelectionOptions = [
  { label: "Yes (5)", value: "0" },
  { label: "No (0)", value: "1" }
]
```

**User checks "Allow N/A":**

1. Checkbox onChange → calls `onAddLabel(true)`
   ↓
2. Appends to arrays:
   ```js
   options[2] = { label: "N/A", value: 2, isNA: true }
   scores[2] = { value: 2, score: null }
   ```
   ↓
3. Form state changes → `useWatch` detects change
   ↓
4. `scoreOptions` gets new arrays with 3 items
   ↓
5. `useMemo` dependencies changed → recomputes
   ↓
6. Loop processes index 2:
   ```tsx
   const score = scoreOptions.scores?.[2]?.score;  // null
   selectionOptions.push({ 
     label: `N/A (${score ?? option.value})`,  // "N/A (2)" - shows value!
     value: "2" 
   });
   ```
   ↓
7. `behaviorScoreSelectionOptions` = 
   ```js
   [
     { label: "Yes (5)", value: "0" },
     { label: "No (0)", value: "1" },
     { label: "N/A (2)", value: "2" }  // ← Shows value, not score
   ]
   ```

**Issue Found!** Line 242 shows `score ?? option.value` when score is null:
```tsx
label: `${option.label} (${score ?? option.value})`
```

For N/A:
- `score = null`
- Falls back to `option.value = 2`
- Shows "N/A (2)" instead of "N/A (no score)"

**This is the bug mentioned in the design doc!**

---

## Summary

### Question 1: When is validation checked?

**Normal Scores:**
- ✅ Has `required` rule
- ✅ Validates on blur, change, submit
- ✅ Passes when `score = 0` (zero is valid!)
- ❌ Fails when `score = null` or `undefined`

**N/A Score:**
- ❌ No validation (can be null)
- User can leave empty or enter number

**Why defaults pass:** New options get `score = 0`, which satisfies `required` validation.

---

### Question 2: How are options synced?

**Mechanism:** `useWatch` + `useMemo` = automatic sync

1. `useWatch` subscribes to `settings` object
2. Any change in options/scores → re-renders AutoQA
3. `useMemo` rebuilds dropdown options
4. Dropdown always shows current state

**Sync is immediate and automatic!**

---

### Issues Found

1. ⚠️ **Bug:** N/A option shows `"N/A (2)"` instead of `"N/A (no score)"` when score is null
   - Root cause: `${score ?? option.value}` at line 242
   - Should be: `${score ?? 'no score'}` or similar

2. ✅ **Good:** Validation correctly allows `score = 0`

3. ✅ **Good:** Sync mechanism is robust via React reactivity
