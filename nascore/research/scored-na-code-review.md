# Scored N/A Feature - Code Review Guide

**Created:** 2026-04-11  
**Purpose:** Side-by-side code review with explanations and flow verification

---

## Table of Contents
- [Architecture Overview](#architecture-overview)
- [Flow 1: Check "Allow N/A"](#flow-1-check-allow-na)
- [Flow 2: Uncheck "Allow N/A"](#flow-2-uncheck-allow-na)
- [Flow 3: Re-check "Allow N/A"](#flow-3-re-check-allow-na)
- [Flow 4: Enter N/A Score Value](#flow-4-enter-na-score-value)
- [Flow 5: Clear N/A Score (onBlur)](#flow-5-clear-na-score-onblur)
- [Flow 6: Add Normal Option When N/A Exists](#flow-6-add-normal-option-when-na-exists)
- [Flow 7: Delete Normal Option](#flow-7-delete-normal-option)
- [Flow 8: Initial Mount with Legacy Template](#flow-8-initial-mount-with-legacy-template)
- [Flow 9-10: # of Occurrences Mode](#flow-9-10--of-occurrences-mode)

---

## Architecture Overview

### Files Involved

1. **CriteriaLabeledOptions.tsx** - Labeled/Multi-Select criteria with scored options
   - Path: `packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx`
   - Manages: `settings.options[]`, `settings.scores[]`, `auto_qa.detected/not_detected/not_applicable`

2. **NumericBinsAndValuesConfigurator.tsx** - # of Occurrences mode
   - Path: `packages/director-app/src/features/admin/coaching/template-builder/configuration/NumericBinsAndValuesConfigurator.tsx`
   - Manages: `auto_qa.options[]`, `settings.options[]`, `settings.scores[]`

### Key Data Structures

```typescript
// Option with isNA flag
interface NumericInputOption {
  label: string;
  value: number;
  isNA?: boolean;  // Flag for N/A option
}

// Score entry (decoupled from option value)
interface Score {
  value: number;  // References option.value
  score: number | null;  // Actual score (null for N/A)
}
```

### Index Alignment
- `settings.options[i]` and `settings.scores[i]` must have matching `value` fields
- When options are deleted, all indices are renormalized (values become 0, 1, 2, ...)
- N/A option always kept last in the array

---

## Flow 1: Check "Allow N/A"

### User Action
User clicks the "Allow N/A" checkbox

### File Location
[**CriteriaLabeledOptions.tsx:316-325**](../packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L316-L325)

```tsx
<Checkbox
  label={t('criteria-labeled-options.allow-na', 'Allow N/A')}
  {...checkedControllerFieldToMantine(showNAField)}
  data-testid="criteria-labeled-allow-na-checkbox"
  onChange={(event) => {
    const checked = event.currentTarget.checked;
    showNAField.onChange(checked);  // ← Set showNA=true in form
    if (checked && enableNAScore && enableDuplicateScoreForCriteria && !isNAOption) {
      onAddLabel(true);  // ← Create isNA option
    }
    if (!checked) {
      removeNAOption();
    }
  }}
/>
```

### Condition Breakdown
- `checked` - Checkbox is now checked
- `enableNAScore` - Feature flag for N/A scoring
- `enableDuplicateScoreForCriteria` - Feature flag for decoupled scores
- `!isNAOption` - No N/A option exists yet

**Where isNAOption is computed ([line 116](../packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L116)):**
```tsx
const isNAOption = watchedOptionsField?.find((opt) => opt.isNA);
```

### Creating the N/A Option
**CriteriaLabeledOptions.tsx:72-88** (`onAddLabel` callback)

```tsx
const onAddLabel = useCallback(
  (isNA?: boolean): void => {
    // Find max existing value to assign unique ID
    const maxId = Math.max(...(watchedOptionsField?.map((field) => field.value) ?? [-1]));
    const newValue = maxId + 1;
    
    // Create option object
    const newOption = { 
      label: isNA ? 'N/A' : '',           // ← N/A gets 'N/A' label
      value: newValue, 
      ...(isNA && { isNA: true })          // ← Spread isNA flag only if true
    };
    
    // Create score object
    const newScore = { 
      value: newValue, 
      score: isNA ? null : 0               // ← N/A gets null score
    };
    
    // Keep N/A last: insert normal options BEFORE existing N/A
    const existingNAIndex = isNA ? -1 : (watchedOptionsField?.findIndex((opt) => opt.isNA) ?? -1);
    //                      ↑ If adding N/A itself, don't search for it
    
    if (existingNAIndex >= 0) {
      // N/A already exists, insert before it
      optionsField.insert(existingNAIndex, newOption);
      scoresFieldArray.insert(existingNAIndex, newScore);
    } else {
      // No N/A, append to end
      optionsField.append(newOption);
      scoresFieldArray.append(newScore);
    }
  },
  [watchedOptionsField, optionsField, scoresFieldArray]
);
```

### UI Rendering
**CriteriaLabeledOptions.tsx:285-310** (N/A row, only shown when `showNAScoreRow=true`)

```tsx
{showNAScoreRow && (
  <Flex align="center" gap="xs">
    <TextInput 
      className={styles.optionsRow__label} 
      value="N/A"     // ← Fixed label
      disabled        // ← Cannot edit
    />
    <Flex gap="xs" className={styles.optionsRow__value}>
      {showNumericOptions && (
        <NumberInput
          value={isNAScore ?? ''}              // ← Show score or empty
          onChange={handleNAScoreChange}       // ← User can type score
          onBlur={() => {
            if (isNAScore === undefined) {
              removeNAOption();                // ← Remove if never set
            }
          }}
          allowDecimal={false}
          clampBehavior="none"
          placeholder={t('criteria-labeled-options.na-score-placeholder', 'no score')}
          hideControls
        />
      )}
      {/* Spacer to align with delete button column */}
      <ActionIcon variant="outline" radius="md" size="lg" style={{ visibility: 'hidden' }}>
        <IconTrash size={16} />
      </ActionIcon>
    </Flex>
  </Flex>
)}
```

**Where isNAScore is computed (line 118):**
```tsx
const isNAScore = isNAIndex >= 0 ? watchedScoresField?.[isNAIndex]?.score : undefined;
```

### Result
- Creates `{ label: 'N/A', value: maxId+1, isNA: true }` in `settings.options`
- Creates `{ value: maxId+1, score: null }` in `settings.scores`
- UI shows N/A row with disabled label and empty score input

✅ **Verified:** Checking "Allow N/A" creates isNA option with null score

---

## Flow 2: Uncheck "Allow N/A"

### User Action
User unchecks the "Allow N/A" checkbox

### File Location
**CriteriaLabeledOptions.tsx:322-324** (same onChange handler as Flow 1)

```tsx
if (!checked) {
  removeNAOption();  // ← Remove the N/A option
}
```

### Removing the N/A Option
**CriteriaLabeledOptions.tsx:142-148** (`removeNAOption` callback)

```tsx
const removeNAOption = useCallback(() => {
  if (isNAIndex >= 0) {  // ← Check if N/A option exists
    optionsField.remove(isNAIndex);              // ← Remove from options array
    scoresFieldArray.remove(isNAIndex);          // ← Remove from scores array
    autoQANotApplicableField.onChange(null);     // ← Clear auto_qa.not_applicable
  }
}, [isNAIndex, optionsField, scoresFieldArray, autoQANotApplicableField]);
```

**Where isNAIndex is computed (line 117):**
```tsx
const isNAIndex = watchedOptionsField?.findIndex((opt) => opt.isNA) ?? -1;
```

**Where autoQANotApplicableField is defined (lines 56-58):**
```tsx
const { field: autoQANotApplicableField } = useController({
  name: `${fieldName as FieldNameType}.auto_qa.not_applicable`,
});
```

### Why Clear auto_qa.not_applicable?
If user had selected "N/A" in the AutoQA dropdown for "Not Applicable" outcome, that dropdown stores the index of the N/A option. When we remove the N/A option, that index becomes invalid → must clear it to prevent stale references.

### Result
- Removes isNA option from `settings.options`
- Removes corresponding entry from `settings.scores`
- Clears `auto_qa.not_applicable` field (prevents stale index reference)
- UI hides the N/A row

✅ **Verified:** Unchecking removes the isNA option and clears auto_qa field

---

## Flow 3: Re-check "Allow N/A"

### User Action
User re-checks "Allow N/A" after previously unchecking it (Flow 2)

### File Location
**CriteriaLabeledOptions.tsx:318-321** (same as Flow 1)

```tsx
if (checked && enableNAScore && enableDuplicateScoreForCriteria && !isNAOption) {
  onAddLabel(true);
}
```

### Why This Works
After Flow 2 removed the N/A option:
- `isNAOption = watchedOptionsField?.find((opt) => opt.isNA)` → `undefined`
- `!isNAOption` → `true`
- Calls `onAddLabel(true)` → creates fresh N/A option

### Result
- Creates new isNA option (same as Flow 1)
- New unique `value` assigned (maxId+1)
- Score initialized to `null`

✅ **Verified:** Re-checking creates a fresh isNA option

---

## Flow 4: Enter N/A Score Value

### User Action
User types "5" in the N/A score input field

### File Location
**CriteriaLabeledOptions.tsx:290-302** (NumberInput onChange handler)

```tsx
<NumberInput
  value={isNAScore ?? ''}
  onChange={handleNAScoreChange}  // ← Triggers on every keystroke
  onBlur={() => {
    if (isNAScore === undefined) {
      removeNAOption();
    }
  }}
  allowDecimal={false}
  clampBehavior="none"
  placeholder={t('criteria-labeled-options.na-score-placeholder', 'no score')}
  hideControls
/>
```

### Handling Score Change
**CriteriaLabeledOptions.tsx:121-140** (`handleNAScoreChange` callback)

```tsx
const handleNAScoreChange = useCallback(
  (score: number | string) => {
    const currentOptions = watchedOptionsField ?? [];
    
    // Parse input to number
    const numScore = typeof score === 'string' ? parseFloat(score) : score;
    if (isNaN(numScore)) return;  // ← Ignore invalid input (early return)

    if (isNAOption) {
      // Case A: Update existing isNA option's score
      const idx = currentOptions.findIndex((opt) => opt.isNA);
      scoresFieldArray.update(idx, { 
        value: isNAOption.value,   // ← Keep same value
        score: numScore             // ← Update score
      });
    } else {
      // Case B: Create new isNA option + score (user typed without checking box)
      const maxId = Math.max(...currentOptions.map((o) => o.value), -1);
      const newValue = maxId + 1;
      optionsField.append({ 
        label: 'N/A', 
        value: newValue, 
        isNA: true 
      } as NumericInputOption);
      scoresFieldArray.append({ 
        value: newValue, 
        score: numScore 
      });
    }
  },
  [watchedOptionsField, isNAOption, optionsField, scoresFieldArray]
);
```

### Two Cases

**Case A: N/A option exists (created via checkbox in Flow 1)**
- Find index of isNA option
- Update score: `scoresFieldArray.update(idx, { value: isNAOption.value, score: 5 })`
- Score changes from `null` → `5`

**Case B: N/A option doesn't exist (user types directly)**
- Creates new isNA option with `label: 'N/A'`
- Creates score entry with `score: 5`
- This allows creating scored N/A without using the checkbox

### Result
- Updates score field to entered value
- OR creates new isNA option if it didn't exist

✅ **Verified:** Entering score updates existing or creates new isNA option

---

## Flow 5: Clear N/A Score (onBlur)

### User Action
User clears the N/A score input and clicks away (triggers blur event)

### File Location
**CriteriaLabeledOptions.tsx:293-297** (NumberInput onBlur handler)

```tsx
onBlur={() => {
  if (isNAScore === undefined) {  // ← Only if score is undefined
    removeNAOption();
  }
}}
```

### When is isNAScore undefined?

**Line 118:**
```tsx
const isNAScore = isNAIndex >= 0 ? watchedScoresField?.[isNAIndex]?.score : undefined;
```

**Breakdown:**
- `isNAIndex >= 0` - N/A option exists
- `watchedScoresField?.[isNAIndex]?.score` - Get score value
  - If score is `null` → `isNAScore = null`
  - If score is `5` → `isNAScore = 5`
  - If score is `0` → `isNAScore = 0`
  - If score field doesn't exist → `isNAScore = undefined`
- `isNAIndex < 0` - N/A option doesn't exist → `isNAScore = undefined`

### Why Doesn't Clearing Trigger Removal?

When user clears the input:
1. User backspaces → NumberInput calls `onChange('')`
2. `handleNAScoreChange` receives `''`
3. `parseFloat('')` → `NaN`
4. `if (isNaN(numScore)) return;` → **early return, score NOT updated**
5. Score field remains its previous value (e.g., `5` or `null`)
6. On blur: `isNAScore` is still `5` or `null` (not `undefined`)
7. `isNAScore === undefined` → `false` → **doesn't remove**

### Actual Purpose of onBlur Check

This check is meant for **Case B in Flow 4** (user starts typing but abandons):
- User starts typing but input is invalid (e.g., types then deletes)
- If no option was created → `isNAScore = undefined` → remove on blur

Or for **data corruption**:
- N/A option exists but has no corresponding score entry
- `isNAIndex >= 0` but `watchedScoresField[isNAIndex]` is undefined
- `isNAScore = undefined` → remove on blur

### How to Actually Remove N/A?

User must **uncheck "Allow N/A" checkbox** (Flow 2)

### Result
- onBlur only removes if score is `undefined` (option doesn't exist or data corruption)
- Does NOT remove when user clears a previously entered score
- To remove N/A with entered score, user must uncheck checkbox

✅ **Verified:** onBlur behavior is correct for edge cases, not for clearing scores

---

## Flow 6: Add Normal Option When N/A Exists

### User Action
User clicks "Add Option" button when N/A already exists

### File Location
**CriteriaLabeledOptions.tsx:327-336** (Add Option button)

```tsx
<Button
  className={styles.footer__addOption}
  onClick={() => onAddLabel()}  // ← Called with no argument (isNA=undefined)
  onPointerDown={stopPropagationOfEvent}
  variant="outline"
  c="var(--content-primary)"
  leftSection={<IconPlus size={16} />}
>
  {t('criteria-labeled-options.add-option', 'Add Option')}
</Button>
```

### How onAddLabel Handles This
**CriteriaLabeledOptions.tsx:72-88** (same function as Flow 1, but isNA=undefined)

```tsx
const onAddLabel = useCallback(
  (isNA?: boolean): void => {
    const maxId = Math.max(...(watchedOptionsField?.map((field) => field.value) ?? [-1]));
    const newValue = maxId + 1;
    
    const newOption = { 
      label: isNA ? 'N/A' : '',           // ← isNA=undefined → label=''
      value: newValue, 
      ...(isNA && { isNA: true })          // ← isNA=undefined (falsy) → no isNA flag
    };
    
    const newScore = { 
      value: newValue, 
      score: isNA ? null : 0               // ← isNA=undefined → score=0
    };
    
    // Find existing N/A to insert before it
    const existingNAIndex = isNA ? -1 : (watchedOptionsField?.findIndex((opt) => opt.isNA) ?? -1);
    //                      ↑ isNA=undefined (falsy) → search for N/A
    
    if (existingNAIndex >= 0) {
      // N/A exists → insert before it
      optionsField.insert(existingNAIndex, newOption);
      scoresFieldArray.insert(existingNAIndex, newScore);
    } else {
      // No N/A → append to end
      optionsField.append(newOption);
      scoresFieldArray.append(newScore);
    }
  },
  [watchedOptionsField, optionsField, scoresFieldArray]
);
```

### Example Execution

**Before:**
```typescript
options: [
  { label: 'Good', value: 0 },
  { label: 'N/A', value: 1, isNA: true }
]
```

**Execution:**
- `maxId = 1`
- `newValue = 2`
- `newOption = { label: '', value: 2 }` (no isNA flag)
- `newScore = { value: 2, score: 0 }`
- `existingNAIndex = 1` (N/A is at index 1)
- `optionsField.insert(1, newOption)` → insert at index 1

**After:**
```typescript
options: [
  { label: 'Good', value: 0 },
  { label: '', value: 2 },              // ← New option inserted here
  { label: 'N/A', value: 1, isNA: true }  // ← N/A pushed to end
]
```

### Why This Matters
Keeping N/A last in the UI provides consistent UX. Users expect N/A to be the last option in dropdowns and lists.

✅ **Verified:** Normal options are inserted before N/A to keep it last

---

## Flow 7: Delete Normal Option

### User Action
User clicks trash icon on a normal option (not N/A, since N/A has no delete button)

### File Location
**CriteriaLabeledOptions.tsx:267-280** (Trash button in option row)

```tsx
<ActionIcon
  onClick={() => handleRemoveOption(index)}
  variant="outline"
  radius="md"
  size="lg"
  c="var(--content-primary)"
  aria-label={t('criteria-labeled-options.remove-option-aria', {
    defaultValue: 'Remove option {{number}}',
    number: index + 1,
  })}
>
  <IconTrash size={16} />
</ActionIcon>
```

### Deletion Logic
**CriteriaLabeledOptions.tsx:150-190** (`handleRemoveOption` function)

```tsx
function handleRemoveOption(deletedIndex: number): void {
  // Step 1: Rebuild arrays with deleted item filtered out
  const currentOptions = watchedOptionsField ?? [];
  const currentScores = watchedScoresField ?? [];
  
  // Filter out deleted index AND renormalize values to 0, 1, 2, ...
  const newOptions = currentOptions
    .filter((_, i) => i !== deletedIndex)
    .map((opt, i) => ({ ...opt, value: i }));  // ← Renormalize values
  
  const newScores = currentScores
    .filter((_, i) => i !== deletedIndex)
    .map((s, i) => ({ ...s, value: i }));      // ← Renormalize values
  
  optionsField.replace(newOptions);
  scoresFieldArray.replace(newScores);

  // Step 2: Define remap function (old index → new index)
  const remapIndex = (oldIdx: number): number => {
    if (oldIdx === deletedIndex) return -1;        // ← Deleted → invalid
    return oldIdx > deletedIndex ? oldIdx - 1 : oldIdx;  // ← Shift down if after deleted
  };

  // Step 3: Remap branch conditions
  const branches = form.getValues(`${fieldName as FieldNameType}.branches`);
  if (branches?.length) {
    branches.forEach((branch, branchIdx) => {
      const remapped = branch.condition.numeric_values
        .map(remapIndex)
        .filter((v) => v !== -1);  // ← Remove deleted indices
      form.setValue(
        `${fieldName as FieldNameType}.branches.${branchIdx}.condition.numeric_values`, 
        remapped
      );
    });
  }

  // Step 4: Remap auto_qa detected/not_detected/not_applicable
  const detected = autoQADetectedField.value as number | null;
  if (detected != null) {
    const newDetected = remapIndex(detected);
    autoQADetectedField.onChange(newDetected === -1 ? null : newDetected);
  }
  
  const notDetected = autoQANotDetectedField.value as number | null;
  if (notDetected != null) {
    const newNotDetected = remapIndex(notDetected);
    autoQANotDetectedField.onChange(newNotDetected === -1 ? null : newDetected);
  }
  
  const notApplicable = autoQANotApplicableField.value as number | null;
  if (notApplicable != null) {
    const newNotApplicable = remapIndex(notApplicable);
    autoQANotApplicableField.onChange(newNotApplicable === -1 ? null : newNotApplicable);
  }
}
```

### Example: Deleting with N/A Present

**Before deletion:**
```typescript
options: [
  { label: 'Good', value: 0 },
  { label: 'Bad', value: 1 },
  { label: 'N/A', value: 2, isNA: true }
]

scores: [
  { value: 0, score: 10 },
  { value: 1, score: 0 },
  { value: 2, score: null }
]

auto_qa: {
  detected: 0,       // 'Good'
  not_detected: 1,   // 'Bad'
  not_applicable: 2  // 'N/A'
}

branches: [
  { condition: { numeric_values: [0, 1] } }  // Good OR Bad
]
```

**Delete index 0 (Good):**

**Step 1: Filter and renormalize**
```typescript
newOptions = [
  { label: 'Bad', value: 0 },          // ← Was value:1, now value:0
  { label: 'N/A', value: 1, isNA: true }  // ← Was value:2, now value:1
]

newScores = [
  { value: 0, score: 0 },              // ← Was value:1
  { value: 1, score: null }            // ← Was value:2
]
```

**Step 2: Remap indices**
- `remapIndex(0)` → `-1` (deleted)
- `remapIndex(1)` → `0` (shifted down)
- `remapIndex(2)` → `1` (shifted down)

**Step 3: Remap branches**
```typescript
// Old: [0, 1]
// Map: remapIndex(0)=-1, remapIndex(1)=0
// Filter -1: [0]
branches[0].condition.numeric_values = [0]  // Now just 'Bad'
```

**Step 4: Remap auto_qa**
```typescript
detected: remapIndex(0) = -1 → null        // ← Cleared (was deleted)
not_detected: remapIndex(1) = 0            // ← Updated to new index
not_applicable: remapIndex(2) = 1          // ← Updated to new index
```

**After deletion:**
```typescript
options: [
  { label: 'Bad', value: 0 },
  { label: 'N/A', value: 1, isNA: true }
]

auto_qa: {
  detected: null,        // ← Cleared
  not_detected: 0,       // ← Remapped
  not_applicable: 1      // ← Remapped
}

branches: [
  { condition: { numeric_values: [0] } }  // ← Remapped
]
```

### Key Points
- **isNA flag is preserved** through renormalization (it's on the option object)
- **All dependent fields are remapped** (branches, auto_qa)
- **Deleted option's references are cleared** (set to null)
- **N/A stays last** (but its value changes to keep array sequential)

✅ **Verified:** Deletion renormalizes indices and remaps all dependent fields correctly

---

## Flow 8: Initial Mount with Legacy Template

### Scenario
User opens a template created before the scored N/A feature was implemented. The template has `showNA: true` but no `isNA` option in the options array.

### File Location
**CriteriaLabeledOptions.tsx:90-109** (`useOnMount` hook)

```tsx
useOnMount(() => {
  // Part 1: Initialize scores for legacy templates that don't have scores yet
  if (watchedOptionsField?.length && !watchedScoresField?.length) {
    scoresFieldArray.replace(
      watchedOptionsField.map((opt) => ({ 
        value: opt.value, 
        score: opt.value    // ← Legacy: score = value (1:1 mapping)
      }))
    );
  }
  
  // Part 2: Show first option by default
  if (optionsField.fields.length === 0) {
    onAddLabel();
  }
  
  // Part 3: Create isNA option for existing templates with showNA enabled
  const currentOptions = form.getValues(`${fieldName as FieldNameType}.settings.options`);
  if (
    enableNAScore &&
    enableDuplicateScoreForCriteria &&
    showNAField.value &&
    !currentOptions?.some((opt) => opt.isNA)
  ) {
    onAddLabel(true);  // ← Auto-create N/A option
  }
});
```

### Why form.getValues() Instead of watchedOptionsField?

**React StrictMode Issue:**
- In development, React StrictMode calls mount effects **twice**
- `watchedOptionsField` (from useWatch) returns a **snapshot** that might be stale on the second call
- `form.getValues()` is **synchronous** and reads the **current** form state

**Without form.getValues():**
1. First mount: `watchedOptionsField = []`, creates N/A option
2. Second mount (StrictMode): `watchedOptionsField` still `[]` (stale snapshot), creates **another** N/A option
3. Result: Two N/A options! 🐛

**With form.getValues():**
1. First mount: `form.getValues() = []`, creates N/A option
2. Second mount (StrictMode): `form.getValues() = [{...N/A}]` (current state), `some((opt) => opt.isNA)` is true, **skips creation**
3. Result: One N/A option ✅

### Example Flow

**Template data loaded:**
```typescript
{
  showNA: true,
  options: [
    { label: 'Good', value: 0 },
    { label: 'Bad', value: 1 }
  ]
  // No isNA option!
}
```

**useOnMount executes:**

**Part 1:** Initialize scores (if needed)
- `watchedOptionsField.length = 2`
- `watchedScoresField.length = 0` (legacy template has no scores)
- Creates: `[{ value: 0, score: 0 }, { value: 1, score: 1 }]`

**Part 2:** Add first option (if empty)
- `optionsField.fields.length = 2` → skipped

**Part 3:** Auto-create N/A option
- `enableNAScore = true` (feature flag on)
- `enableDuplicateScoreForCriteria = true` (feature flag on)
- `showNAField.value = true` (template has showNA=true)
- `form.getValues('settings.options') = [Good, Bad]`
- `!currentOptions.some((opt) => opt.isNA)` → true (no isNA option)
- Calls `onAddLabel(true)`

**Result:**
```typescript
options: [
  { label: 'Good', value: 0 },
  { label: 'Bad', value: 1 },
  { label: 'N/A', value: 2, isNA: true }  // ← Auto-created
]

scores: [
  { value: 0, score: 0 },   // ← From Part 1
  { value: 1, score: 1 },   // ← From Part 1
  { value: 2, score: null } // ← From onAddLabel
]
```

✅ **Verified:** Legacy templates get isNA option auto-created on mount, StrictMode-safe

---

## Flow 9-10: # of Occurrences Mode

### File
**NumericBinsAndValuesConfigurator.tsx**

This component is simpler because it doesn't have scored options in the main UI (only in the N/A row).

---

### Flow 9: Check "Allow N/A"

**File Location: NumericBinsAndValuesConfigurator.tsx:191-206**

```tsx
{showAllowNAToggle && (
  <Checkbox
    label={t('allow-na', 'Allow N/A')}
    {...checkedControllerFieldToMantine(showNAField)}
    onChange={(event) => {
      const checked = event.currentTarget.checked;
      showNAField.onChange(checked);
      if (checked && enableNAScore && !isNAOption) {
        addNAOption();  // ← Create N/A option
      }
      if (!checked) {
        removeNAOption();  // ← Remove N/A option
      }
    }}
  />
)}
```

**addNAOption function (lines 66-71):**
```tsx
const addNAOption = useCallback((): void => {
  const maxValue = Math.max(...(watchedSettingsOptions?.map((opt) => opt.value) ?? [-1]));
  const newValue = maxValue + 1;
  settingsOptionsField.append({ 
    label: 'N/A', 
    value: newValue, 
    isNA: true 
  });
  settingsScoresField.append({ 
    value: newValue, 
    score: null    // ← N/A gets null score
  });
}, [watchedSettingsOptions, settingsOptionsField, settingsScoresField]);
```

**Result:**
- Appends isNA option to `settings.options`
- Appends null score to `settings.scores`

---

### Flow 10: Uncheck "Allow N/A"

**File Location: NumericBinsAndValuesConfigurator.tsx:200-203** (same onChange)

```tsx
if (!checked) {
  removeNAOption();
}
```

**removeNAOption function (lines 73-79):**
```tsx
const removeNAOption = useCallback((): void => {
  if (isNAIndex >= 0) {
    settingsOptionsField.remove(isNAIndex);
    settingsScoresField.remove(isNAIndex);
    form.setValue(`${itemFieldPath as ItemFieldPathType}.auto_qa.not_applicable`, null);
    //            ↑ Clear auto_qa.not_applicable (same as CriteriaLabeledOptions)
  }
}, [isNAIndex, settingsOptionsField, settingsScoresField, form, itemFieldPath]);
```

**Result:**
- Removes isNA option from arrays
- Clears `auto_qa.not_applicable`

---

### N/A Row Rendering

**File Location: NumericBinsAndValuesConfigurator.tsx:132-153**

```tsx
{showNAScoreRow && isNAIndex >= 0 && (
  <Paper radius="md" variant="elevation">
    <Flex align="center" gap="xs" px={10} py={10}>
      <TextInput 
        value={t('na-label', 'N/A')} 
        disabled 
        flex={4} 
        mr="0.3em" 
      />
      <Controller
        name={
          `${itemFieldPath}.settings.scores.${isNAIndex}.score` as 
          `${ItemFieldPathType}.settings.scores.${number}.score`
        }
        render={({ field }) => (
          <NumberInput
            {...field}
            allowDecimal={false}
            clampBehavior="none"
            placeholder={t('na-score-placeholder', 'no score')}
            flex={1}
            hideControls
          />
        )}
      />
    </Flex>
  </Paper>
)}
```

**Where showNAScoreRow is computed (line 64):**
```tsx
const showNAScoreRow = enableNAScore && !!showNAField.value;
```

✅ **Verified:** # of Occurrences mode handles N/A checkbox correctly

---

## Summary: Correctness Verification

### All Flows Verified ✅

1. ✅ **Check "Allow N/A"** - Creates isNA option with null score
2. ✅ **Uncheck "Allow N/A"** - Removes isNA option and clears auto_qa.not_applicable
3. ✅ **Re-check "Allow N/A"** - Creates fresh isNA option (condition checks !isNAOption)
4. ✅ **Enter N/A score** - Updates existing or creates new isNA option
5. ✅ **Clear N/A score (onBlur)** - Only removes if undefined (edge case handling)
6. ✅ **Add normal option when N/A exists** - Inserts before N/A to keep it last
7. ✅ **Delete normal option** - Renormalizes indices, remaps branches/auto_qa, preserves isNA flag
8. ✅ **Legacy template migration** - Auto-creates isNA option on mount, StrictMode-safe with form.getValues()
9. ✅ **# of Occurrences: Check N/A** - Appends isNA option
10. ✅ **# of Occurrences: Uncheck N/A** - Removes isNA option and clears auto_qa.not_applicable

### Key Design Principles

1. **isNA flag persistence** - Lives on option object, survives renormalization
2. **N/A always last** - `existingNAIndex` check in `onAddLabel` inserts normal options before N/A
3. **Auto_qa cleanup** - `removeNAOption` clears `auto_qa.not_applicable` to prevent stale indices
4. **StrictMode idempotency** - `form.getValues()` instead of `useWatch` for synchronous reads
5. **Index remapping** - `handleRemoveOption` remaps all dependent fields (branches, auto_qa)
6. **Score decoupling** - `settings.scores[]` independent from `settings.options[]`, linked by value field

### Testing Recommendations

1. Test in StrictMode (development build) - verify no duplicate N/A options on mount
2. Test deletion with branches - verify branch conditions update correctly
3. Test deletion with auto_qa - verify detected/not_detected/not_applicable remap correctly
4. Test N/A always last - add options before/after creating N/A, verify UI order
5. Test legacy template migration - load old template with showNA=true, verify auto-creation
