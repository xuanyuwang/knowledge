# Scored N/A Feature - Code Review

**Created:** 2026-04-11  
**Updated:** 2026-04-12  
**How to use:** Cmd+Click (Mac) or Ctrl+Click (Windows) on file paths to jump to code

## Changelog

### 2026-04-12: Simplified N/A Score Handling

**Commit 1:** Unify N/A score with standard Controller pattern
- ✅ **Removed** `handleNAScoreChange` function (21 lines)
  - Deleted Case B (creating N/A without checkbox) - unreachable in normal flow
  - Removed custom state management logic
- ✅ **Unified** N/A score with normal scores using standard `Controller` pattern
  - N/A score now works exactly like normal option scores
  - No special handling needed
- ✅ **Removed** unused `NumericInputOption` import

**Commit 2:** Remove redundant rendering check (CriteriaLabeledOptions)
- ✅ **Removed** `isNAIndex >= 0` from `showNAScoreRow && isNAIndex >= 0`
  - Invariant: `showNAScoreRow=true` implies `isNAIndex >= 0`
  - Checkbox onChange creates option when checked → guaranteed to exist
  - Fail loudly if state is corrupt rather than silently hiding

**Commit 3:** Remove additional redundant checks (both files)
- ✅ **NumericBinsAndValuesConfigurator:** Removed same rendering check
  - Same invariant applies: checkbox creates option when checked
- ✅ **CriteriaLabeledOptions:** Simplified `isNAScore` computation
  - Removed `isNAIndex >= 0 ? ... : undefined` ternary
  - Array access with -1 returns undefined anyway
- ✅ **Kept** defensive `if (isNAIndex >= 0)` in `removeNAOption` callbacks
  - Protects against double-removal (checkbox uncheck + input blur)

**Commit 4:** Make N/A score onChange consistent with normal scores
- ✅ **Added** explicit `onChange` handler to N/A score
  - Calls `resetAutoQADetectedFields()` like normal scores
  - Ensures consistent behavior: changing any score clears autoQA selections

**Commit 5:** Unify N/A score handling with normal score patterns (CriteriaLabeledOptions)
- ✅ **Removed** custom `onBlur` handler from N/A score input
  - N/A score now uses `onChange` only, exactly like normal scores
  - No special edge case handling needed
- ✅ **Removed** `removeNAOption` function
  - Merged into `handleRemoveOption` - it already handles N/A through its remapping logic (lines 155-159)
  - Checkbox onChange now calls `handleRemoveOption(isNAIndex)` instead of `removeNAOption()`
- ✅ **Removed** unused `isNAScore` variable
- ✅ **Added** defensive check to checkbox onChange: `if (!checked && isNAIndex >= 0)`
  - Prevents calling `handleRemoveOption(-1)` if option doesn't exist

**Commit 6:** Make N/A score identical to normal scores in decoupled mode
- ✅ **Removed** onChange handler from N/A score input (lines 271-274)
  - In decoupled mode, autoQA stores **indices** (not score values)
  - Changing a score doesn't invalidate autoQA mappings → no need to reset
  - Normal scores in decoupled mode don't have onChange handler
  - Only legacy mode needs `resetAutoQADetectedFields()` (line 231) because it stores values
- ✅ **Reused** isNAIndex in onAddLabel instead of recomputing
  - Moved isNAOption/isNAIndex/showNAScoreRow declarations before onAddLabel
  - Eliminates redundant findIndex call

**Commit 7:** Remove enableDuplicateScoreForCriteria feature flag (sync with main)
- ✅ **Removed** enableDuplicateScoreForCriteria declaration
  - Feature flag was already cleaned up from main on April 9, 2026 (commit c4e5df3298)
  - Kept useFeatureFlag import for enableNAScore
- ✅ **Removed** resetAutoQADetectedFields callback
  - No longer used after legacy mode removal
- ✅ **Removed** ternary for normal options (lines 190-229)
  - Deleted entire legacy mode branch using `settings.options[index].value`
  - Kept only decoupled mode using `settings.scores[index].score`
- ✅ **Simplified** conditions throughout:
  - showNAScoreRow: removed redundant enableDuplicateScoreForCriteria check
  - useOnMount N/A creation: simplified condition
  - Checkbox onChange: removed redundant check

**Net result:** -68 lines total across 7 commits. Legacy mode code completely removed, consistent with main branch.

### Defensive Checks We Kept (Intentionally)

These checks remain because they protect against edge cases:

1. **Checkbox onChange:** `if (!checked && isNAIndex >= 0)`
   - In CriteriaLabeledOptions: Prevents calling `handleRemoveOption(-1)` if N/A option doesn't exist
   - Edge case: if state is corrupted (showNA=true but no isNA option), fail safely

2. **`removeNAOption` in NumericBinsAndValuesConfigurator:** `if (isNAIndex >= 0) { ... }`
   - This file still has its own `removeNAOption` function (different context, no `handleRemoveOption` equivalent)
   - Protects against double-removal from checkbox onChange
   - Prevents `.remove(-1)` errors

---

## Quick Reference

| Flow | File | Lines | Status |
|------|------|-------|--------|
| Check "Allow N/A" | [CriteriaLabeledOptions.tsx](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L287) | 287-299 | ✅ |
| Uncheck "Allow N/A" | [CriteriaLabeledOptions.tsx](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L297) | 297-299 | ✅ |
| Enter N/A Score | [CriteriaLabeledOptions.tsx](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L260) | 260-274 | ✅ **UNIFIED** |
| Delete Option | [CriteriaLabeledOptions.tsx](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L120) | 120-160 | ✅ |
| Legacy Migration | [CriteriaLabeledOptions.tsx](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L90) | 90-108 | ✅ |
| # of Occurrences | [NumericBinsAndValuesConfigurator.tsx](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/NumericBinsAndValuesConfigurator.tsx#L191) | 191-206 | ✅ |

---

## Flow 1: Check "Allow N/A" ✅

### Code Location
[CriteriaLabeledOptions.tsx:316-325](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L316)

### Flow
1. User clicks "Allow N/A" checkbox
2. Checkbox onChange fires → `showNAField.onChange(true)`
3. Condition check: `checked && enableNAScore && enableDuplicateScoreForCriteria && !isNAOption`
4. Calls `onAddLabel(true)` → creates isNA option

### Key Code: onAddLabel
[CriteriaLabeledOptions.tsx:72-88](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L72)

```tsx
const onAddLabel = useCallback(
  (isNA?: boolean): void => {
    const maxId = Math.max(...(watchedOptionsField?.map((field) => field.value) ?? [-1]));
    const newValue = maxId + 1;
    const newOption = { 
      label: isNA ? 'N/A' : '', 
      value: newValue, 
      ...(isNA && { isNA: true })  // ← Only add flag if isNA=true
    };
    const newScore = { 
      value: newValue, 
      score: isNA ? null : 0  // ← N/A gets null score
    };
    
    // Keep N/A last by inserting normal options before it
    const existingNAIndex = isNA ? -1 : (watchedOptionsField?.findIndex((opt) => opt.isNA) ?? -1);
    
    if (existingNAIndex >= 0) {
      optionsField.insert(existingNAIndex, newOption);
      scoresFieldArray.insert(existingNAIndex, newScore);
    } else {
      optionsField.append(newOption);
      scoresFieldArray.append(newScore);
    }
  },
  [watchedOptionsField, optionsField, scoresFieldArray]
);
```

### Result
- Creates: `{ label: 'N/A', value: maxId+1, isNA: true }`
- Score: `{ value: maxId+1, score: null }`
- UI: N/A row appears with disabled "N/A" label and empty score input

### Verification
- ✅ isNA flag is set
- ✅ Score is null (not 0)
- ✅ Appends to end of array
- ✅ showNA field updated to true

---

## Flow 2: Uncheck "Allow N/A" ✅

### Code Location
[CriteriaLabeledOptions.tsx:297-299](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L297)

### Flow
1. User unchecks "Allow N/A" checkbox
2. Checkbox onChange fires → `showNAField.onChange(false)`
3. Defensive check: `if (!checked && isNAIndex >= 0)`
4. Calls `handleRemoveOption(isNAIndex)`

### Key Code
[CriteriaLabeledOptions.tsx:297-299](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L297)

```tsx
if (!checked && isNAIndex >= 0) {
  handleRemoveOption(isNAIndex);  // ← Uses same function as normal options
}
```

Where `isNAIndex` is computed at [line 117](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L117):
```tsx
const isNAIndex = watchedOptionsField?.findIndex((opt) => opt.isNA) ?? -1;
```

### How handleRemoveOption Handles N/A
The general `handleRemoveOption` function already handles N/A through its remapping logic at [lines 155-159](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L155):

```tsx
const notApplicable = autoQANotApplicableField.value as number | null;
if (notApplicable != null) {
  const newNotApplicable = remapIndex(notApplicable);
  autoQANotApplicableField.onChange(newNotApplicable === -1 ? null : newNotApplicable);
}
```

When the deleted option **is** the N/A option, `remapIndex(notApplicable)` returns `-1`, which sets `auto_qa.not_applicable` to `null`.

### Result
- Removes isNA from `settings.options` (renormalizes indices)
- Removes score from `settings.scores` (renormalizes indices)
- Clears `auto_qa.not_applicable` to null (via remapping logic)
- UI: N/A row disappears

### Verification
- ✅ Both arrays updated and renormalized
- ✅ auto_qa field cleared through remapping
- ✅ No special N/A logic needed - unified with normal option deletion

---

## Flow 3: Enter N/A Score ✅ (IDENTICAL)

### Code Location
[CriteriaLabeledOptions.tsx:260-271](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L260)

### Flow
1. User types "5" in N/A score input
2. Controller's onChange fires (from `{...field}` spread)
3. Form field `settings.scores[isNAIndex].score` updated directly

### Evolution: From Custom Logic to Standard Pattern

**Before (Commit 1):** Custom `handleNAScoreChange` with Case A/B logic (21 lines)  
**Commit 4:** Standard Controller + explicit onChange with `resetAutoQADetectedFields()`  
**Commit 5:** Removed `onBlur` handler  
**Now (Commit 6):** Removed onChange handler → **IDENTICAL to normal scores**

### Current Implementation

```tsx
<Controller
  name={`${fieldName}.settings.scores.${isNAIndex}.score`}
  render={({ field }) => (
    <NumberInput
      {...field}  // ← Standard react-hook-form binding
      allowDecimal={false}
      clampBehavior="none"
      placeholder={t('criteria-labeled-options.na-score-placeholder', 'no score')}
      hideControls
    />
  )}
/>
```

### Comparison with Normal Score (Decoupled Mode)
[CriteriaLabeledOptions.tsx:199-215](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L199)

```tsx
<Controller
  name={`${fieldName}.settings.scores.${index}.score`}
  rules={{ required: '...' }}  // ← Only difference: N/A doesn't require validation
  render={({ field }) => (
    <NumberInput
      {...field}
      allowDecimal={false}
      min={0}  // ← Only difference: N/A doesn't have min (can be null)
      clampBehavior="none"
      placeholder={t('criteria-labeled-options.value-placeholder', 'Value')}
      hideControls
    />
  )}
/>
```

**Core structure is IDENTICAL. Only differences:**
1. ✅ N/A has no `required` rule (can be null)
2. ✅ N/A has no `min={0}` (can be null)
3. ✅ Different placeholder ("no score" vs "Value")

### Why No onChange Handler?

In **decoupled mode** (`enableDuplicateScoreForCriteria = true`):
- AutoQA stores **indices** (not score values)
- Changing a score doesn't invalidate autoQA mappings
- Therefore, no need to call `resetAutoQADetectedFields()`

In **legacy mode** (lines 217-235), normal scores DO have onChange:
```tsx
onChange={(value) => {
  field.onChange(value);
  resetAutoQADetectedFields();  // ← Correct: legacy mode stores VALUES
}}
```

But N/A scores only exist in decoupled mode, so they never need the handler.

### Invariant
`showNAScoreRow=true` implies `isNAIndex >= 0` (checkbox onChange creates isNA option when checked).

No defensive check needed - if state is corrupt, fail loudly with invalid Controller name.

### Result
- N/A score uses standard Controller pattern
- No onChange handler, no onBlur handler, no custom state management
- Works exactly like normal option scores in decoupled mode

### Verification
- ✅ IDENTICAL to normal scores in decoupled mode
- ✅ No special N/A handling whatsoever
- ✅ Only expected differences: no validation, different placeholder

---

## Flow 4: Delete Normal Option ✅

### Code Location  
[CriteriaLabeledOptions.tsx:150-190](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L150)

### Flow
1. User clicks trash icon on option at index 0
2. Calls `handleRemoveOption(0)`
3. Renormalizes all indices
4. Remaps all dependent fields

### Key Code: handleRemoveOption

```tsx
function handleRemoveOption(deletedIndex: number): void {
  // Step 1: Rebuild arrays with renormalized indices
  const currentOptions = watchedOptionsField ?? [];
  const currentScores = watchedScoresField ?? [];
  
  const newOptions = currentOptions
    .filter((_, i) => i !== deletedIndex)
    .map((opt, i) => ({ ...opt, value: i }));  // ← Renormalize to 0,1,2...
  
  const newScores = currentScores
    .filter((_, i) => i !== deletedIndex)
    .map((s, i) => ({ ...s, value: i }));
  
  optionsField.replace(newOptions);
  scoresFieldArray.replace(newScores);

  // Step 2: Remap function
  const remapIndex = (oldIdx: number): number => {
    if (oldIdx === deletedIndex) return -1;           // Deleted → invalid
    return oldIdx > deletedIndex ? oldIdx - 1 : oldIdx;  // Shift down
  };

  // Step 3: Remap branch conditions
  const branches = form.getValues(`${fieldName}.branches`);
  if (branches?.length) {
    branches.forEach((branch, branchIdx) => {
      const remapped = branch.condition.numeric_values
        .map(remapIndex)
        .filter((v) => v !== -1);  // Remove deleted
      form.setValue(`${fieldName}.branches.${branchIdx}.condition.numeric_values`, remapped);
    });
  }

  // Step 4: Remap auto_qa fields
  const detected = autoQADetectedField.value;
  if (detected != null) {
    const newDetected = remapIndex(detected);
    autoQADetectedField.onChange(newDetected === -1 ? null : newDetected);
  }
  // ... same for not_detected and not_applicable
}
```

### Example: Delete index 0 when N/A exists

**Before:**
```typescript
options: [
  { label: 'Good', value: 0 },
  { label: 'Bad', value: 1 },
  { label: 'N/A', value: 2, isNA: true }
]

auto_qa: { detected: 0, not_detected: 1, not_applicable: 2 }
branches: [{ condition: { numeric_values: [0, 1] } }]
```

**After deleting index 0:**
```typescript
options: [
  { label: 'Bad', value: 0 },          // ← Was 1, now 0
  { label: 'N/A', value: 1, isNA: true }  // ← Was 2, now 1
]

auto_qa: { detected: null, not_detected: 0, not_applicable: 1 }  // ← Remapped
branches: [{ condition: { numeric_values: [0] } }]  // ← 0 deleted, 1→0
```

### Result
- Values renormalized to sequential 0, 1, 2...
- isNA flag preserved (on option object)
- Branches remapped (deleted indices filtered out)
- auto_qa fields remapped (deleted → null)

### Verification
- ✅ Index renormalization correct
- ✅ isNA flag preserved
- ✅ All dependent fields remapped
- ✅ N/A stays last (but value changes)

---

## Flow 5: Legacy Template Migration ✅

### Code Location
[CriteriaLabeledOptions.tsx:90-109](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L90)

### Scenario
Template created before scored N/A feature:
- Has `showNA: true`
- Has no isNA option in options array
- May have no scores array (legacy)

### Key Code: useOnMount

```tsx
useOnMount(() => {
  // Part 1: Initialize scores for legacy templates
  if (watchedOptionsField?.length && !watchedScoresField?.length) {
    scoresFieldArray.replace(
      watchedOptionsField.map((opt) => ({ value: opt.value, score: opt.value }))
    );
  }
  
  // Part 2: Show first option by default
  if (optionsField.fields.length === 0) {
    onAddLabel();
  }
  
  // Part 3: Auto-create isNA option for legacy templates
  const currentOptions = form.getValues(`${fieldName}.settings.options`);
  if (
    enableNAScore &&
    enableDuplicateScoreForCriteria &&
    showNAField.value &&
    !currentOptions?.some((opt) => opt.isNA)
  ) {
    onAddLabel(true);  // ← Create N/A option
  }
});
```

### Why form.getValues() instead of watchedOptionsField?

**React StrictMode Issue:**
- StrictMode calls mount effects **twice** in development
- `watchedOptionsField` (useWatch) returns a **snapshot** (may be stale on 2nd call)
- `form.getValues()` is **synchronous** (reads current state)

**Without form.getValues():**
1. First call: `watchedOptionsField = []`, creates N/A
2. Second call: `watchedOptionsField = []` (stale), creates **another** N/A ❌

**With form.getValues():**
1. First call: `form.getValues() = []`, creates N/A
2. Second call: `form.getValues() = [{...N/A}]`, `some(opt => opt.isNA)` is true, **skips** ✅

### Example Flow

**Template loaded:**
```typescript
{
  showNA: true,
  options: [
    { label: 'Good', value: 0 },
    { label: 'Bad', value: 1 }
  ]
  // No isNA option, no scores array
}
```

**After useOnMount:**
```typescript
{
  showNA: true,
  options: [
    { label: 'Good', value: 0 },
    { label: 'Bad', value: 1 },
    { label: 'N/A', value: 2, isNA: true }  // ← Auto-created
  ],
  scores: [
    { value: 0, score: 0 },   // ← Part 1
    { value: 1, score: 1 },   // ← Part 1
    { value: 2, score: null } // ← Part 3
  ]
}
```

### Result
- Legacy templates auto-upgrade on mount
- StrictMode-safe (no duplicates)
- Scores initialized with 1:1 mapping

### Verification
- ✅ Auto-creates N/A for legacy templates
- ✅ StrictMode idempotent (form.getValues)
- ✅ Initializes scores array

---

## Flow 6: # of Occurrences Mode ✅

### Code Location
[NumericBinsAndValuesConfigurator.tsx:191-206](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/NumericBinsAndValuesConfigurator.tsx#L191)

### Differences from CriteriaLabeledOptions
- Simpler: No scored options in main UI
- N/A only appears in separate card (no insertion logic needed)
- Same checkbox behavior

### Key Code: Checkbox

```tsx
<Checkbox
  label={t('allow-na', 'Allow N/A')}
  {...checkedControllerFieldToMantine(showNAField)}
  onChange={(event) => {
    const checked = event.currentTarget.checked;
    showNAField.onChange(checked);
    if (checked && enableNAScore && !isNAOption) {
      addNAOption();  // ← Just append
    }
    if (!checked) {
      removeNAOption();  // ← Remove and clear auto_qa.not_applicable
    }
  }}
/>
```

### addNAOption
[NumericBinsAndValuesConfigurator.tsx:66-71](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/NumericBinsAndValuesConfigurator.tsx#L66)

```tsx
const addNAOption = useCallback((): void => {
  const maxValue = Math.max(...(watchedSettingsOptions?.map((opt) => opt.value) ?? [-1]));
  const newValue = maxValue + 1;
  settingsOptionsField.append({ label: 'N/A', value: newValue, isNA: true });
  settingsScoresField.append({ value: newValue, score: null });
}, [watchedSettingsOptions, settingsOptionsField, settingsScoresField]);
```

### removeNAOption
[NumericBinsAndValuesConfigurator.tsx:73-79](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/NumericBinsAndValuesConfigurator.tsx#L73)

```tsx
const removeNAOption = useCallback((): void => {
  if (isNAIndex >= 0) {
    settingsOptionsField.remove(isNAIndex);
    settingsScoresField.remove(isNAIndex);
    form.setValue(`${itemFieldPath}.auto_qa.not_applicable`, null);  // ← Clear auto_qa
  }
}, [isNAIndex, settingsOptionsField, settingsScoresField, form, itemFieldPath]);
```

### Result
- Same behavior as CriteriaLabeledOptions
- Simpler implementation (no insertion logic)

### Verification
- ✅ Creates/removes isNA option
- ✅ Clears auto_qa.not_applicable on remove
- ✅ null score for N/A

---

## Summary: All Flows Verified ✅

| Flow | Status | Key Points |
|------|--------|-----------|
| Check "Allow N/A" | ✅ | Creates isNA option with null score |
| Uncheck "Allow N/A" | ✅ | Removes option + clears auto_qa.not_applicable |
| Re-check "Allow N/A" | ✅ | Creates fresh option (!isNAOption condition) |
| Enter N/A Score | ✅ **SIMPLIFIED** | Uses standard Controller (unified with normal scores) |
| Clear N/A Score | ✅ | Only removes if undefined (edge case) |
| Add Normal Option | ✅ | Inserts before N/A (keeps it last) |
| Delete Option | ✅ | Renormalizes + remaps branches/auto_qa |
| Legacy Migration | ✅ | Auto-creates N/A on mount, StrictMode-safe |
| # of Occurrences | ✅ | Same logic, simpler implementation |

### Recent Improvements (2026-04-12)

**Removed `handleNAScoreChange` function:**
- Deleted 21 lines of custom state management
- Removed Case B (unreachable defensive code)
- No more special handling for N/A scores

**Unified Pattern:**
- N/A score now uses same `Controller` pattern as normal options
- Consistent with existing codebase patterns
- Easier to maintain and understand

---

## How Options and AutoQA Stay in Sync

### Data Structure

The scorecard template has two related data structures:

```typescript
// User-visible options
settings: {
  options: [
    { label: 'Good', value: 0 },
    { label: 'Bad', value: 1 },
    { label: 'N/A', value: 2, isNA: true }
  ],
  scores: [
    { value: 0, score: 10 },  // Good scores 10 points
    { value: 1, score: 0 },   // Bad scores 0 points
    { value: 2, score: null } // N/A has no score
  ]
}

// AutoQA outcome mappings (what they reference depends on scoring mode)
auto_qa: {
  detected: 0,        
  not_detected: 1,    
  not_applicable: 2   
}
```

**Key insight:** What `auto_qa` fields store depends on scoring mode:
- **Decoupled mode** (scored N/A feature): Stores **array indices** (0, 1, 2...)
- **Legacy mode**: Stores **option.value** (can be 10, 0, 100...)

In decoupled mode, when options array changes, indices must be updated. In legacy mode, values are stable.

### Sync Mechanisms

#### 1. Removing N/A Option
[CriteriaLabeledOptions.tsx:121-127](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L121)

```tsx
const removeNAOption = useCallback(() => {
  if (isNAIndex >= 0) {
    optionsField.remove(isNAIndex);              // Remove from options
    scoresFieldArray.remove(isNAIndex);          // Remove from scores
    autoQANotApplicableField.onChange(null);     // ← Clear auto_qa.not_applicable
  }
}, [isNAIndex, optionsField, scoresFieldArray, autoQANotApplicableField]);
```

**Why clear `not_applicable`?**
- When N/A option is removed, its index becomes invalid
- If `not_applicable` still points to old index, AutoQA will select wrong option
- Must clear to prevent stale references

#### 2. Deleting Normal Options (with Renormalization)
[CriteriaLabeledOptions.tsx:129-169](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L129)

**Example: Delete option at index 0**

Before:
```typescript
options: [
  { label: 'Good', value: 0 },      // ← DELETE THIS
  { label: 'Bad', value: 1 },
  { label: 'N/A', value: 2, isNA: true }
]
auto_qa: { detected: 0, not_detected: 1, not_applicable: 2 }
```

After deletion:
```typescript
// Step 1: Filter and renormalize values
options: [
  { label: 'Bad', value: 0 },       // ← Was index 1, now index 0
  { label: 'N/A', value: 1, isNA: true }  // ← Was index 2, now index 1
]

// Step 2: Remap auto_qa indices
auto_qa: {
  detected: null,        // ← Was 0 (deleted), now null
  not_detected: 0,       // ← Was 1, now 0 (shifted down)
  not_applicable: 1      // ← Was 2, now 1 (shifted down)
}
```

**Remapping logic:**
```tsx
const remapIndex = (oldIdx: number): number => {
  if (oldIdx === deletedIndex) return -1;  // Deleted → invalid
  return oldIdx > deletedIndex ? oldIdx - 1 : oldIdx;  // After deleted → shift down
};

// Apply to auto_qa fields
const detected = autoQADetectedField.value;
if (detected != null) {
  const newDetected = remapIndex(detected);
  autoQADetectedField.onChange(newDetected === -1 ? null : newDetected);
}
// ... same for not_detected and not_applicable
```

#### 3. Branch Conditions (Also Remapped)

Branches also reference options by index, so they need remapping too:

```tsx
// Remap branch conditions
const branches = form.getValues(`${fieldName}.branches`);
if (branches?.length) {
  branches.forEach((branch, branchIdx) => {
    const remapped = branch.condition.numeric_values
      .map(remapIndex)
      .filter((v) => v !== -1);  // Remove deleted indices
    form.setValue(`${fieldName}.branches.${branchIdx}.condition.numeric_values`, remapped);
  });
}
```

**Example:**
- Before: Branch triggers if user selects Good (0) OR Bad (1)
- After deleting Good: Branch triggers only if user selects Bad (now 0)

### Why This Matters

**Without sync:**
```typescript
// User deletes "Good" option
options: [{ label: 'Bad', value: 0 }]
auto_qa: { detected: 0, not_detected: 1 }  // ← STALE!
```

AutoQA tries to select index 1 (not_detected) → **out of bounds error** or selects wrong option!

**With sync:**
```typescript
options: [{ label: 'Bad', value: 0 }]
auto_qa: { detected: null, not_detected: 0 }  // ← CORRECT!
```

AutoQA correctly selects Bad (index 0) when not detected.

### Invariants

1. **AutoQA indices must be valid:** `0 <= index < options.length` or `null`
2. **N/A option cleared when removed:** `not_applicable` set to `null` when N/A deleted
3. **Indices renormalized on deletion:** All references updated atomically
4. **isNA flag preserved:** Survives renormalization (on option object, not index-based)

---

## How AutoQA Dropdowns Stay in Sync

AutoQA dropdowns ("If behavior is done/not done/not applicable") need to stay in sync with the options array. This happens through two mechanisms:

### 1. Options → Dropdown: Reactive Sync

[TemplateBuilderAutoQA.tsx:231-274](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/TemplateBuilderAutoQA/TemplateBuilderAutoQA.tsx#L231)

```tsx
const behaviorScoreSelectionOptions = useMemo((): ComboboxItem[] => {
  let selectionOptions: ComboboxItem[] = [];
  
  if (
    (scoreType === CriterionTypes.LabeledRadios || 
     scoreType === CriterionTypes.DropdownNumericValues) &&
    scoreOptions.options
  ) {
    if (isDecoupledScoring) {
      // Decoupled mode: use array INDEX as select value
      scoreOptions.options.forEach((option: ScoreOption, index: number) => {
        if (option.value !== undefined) {
          const score = scoreOptions.scores?.[index]?.score;
          selectionOptions.push({ 
            label: `${option.label} (${score ?? option.value})`,  // "Good (10)"
            value: index.toString()  // "0", "1", "2" ← INDEX as string
          });
        }
      });
    } else {
      // Legacy mode: use option VALUE as select value
      const uniqueValueOptions = uniqBy(scoreOptions.options, 'value');
      uniqueValueOptions.forEach((option: ScoreOption) => {
        if (option.value !== undefined) {
          selectionOptions.push({ 
            label: option.label,
            value: option.value.toString()  // Option's value field
          });
        }
      });
    }
  }
  
  return selectionOptions;
}, [scoreOptions.options, scoreOptions.scores, scoreOptions.range, scoreType, isDecoupledScoring]);
```

**Key points:**
- `useMemo` depends on `scoreOptions.options` and `scoreOptions.scores`
- When user adds/removes/edits options → useMemo reruns → dropdown updates
- **Decoupled mode:** Dropdown values are **indices** ("0", "1", "2")
- **Legacy mode:** Dropdown values are **option values** ("0", "10", "100")

**Example (Decoupled Mode):**
```typescript
// Settings
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

// Dropdown options generated
behaviorScoreSelectionOptions = [
  { label: "Good (10)", value: "0" },     // ← Index 0
  { label: "Bad (0)", value: "1" },       // ← Index 1
  { label: "N/A (null)", value: "2" }     // ← Index 2, shows "null"
]
```

### 2. Dropdown Selection → Form: Value Conversion

[TemplateBuilderAutoQA.tsx:285-305](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/TemplateBuilderAutoQA/TemplateBuilderAutoQA.tsx#L285)

When user selects from dropdown:

```tsx
const onChangeDetectedField = useCallback(
  (value: string | null) => {  // ← Mantine Select passes string
    if (isDecoupledScoring) {
      detectedField.onChange(value != null ? Number(value) : null);  // "0" → 0
    } else {
      detectedField.onChange(Number(value));  // "10" → 10
    }
  },
  [detectedField, isDecoupledScoring]
);

// Same pattern for notDetectedField and notApplicableField
```

**Flow:**
1. User selects "Good (10)" from dropdown
2. Mantine Select calls `onChange("0")` (string)
3. `onChangeDetectedField` converts `"0"` → `0` (number)
4. Stores `auto_qa.detected = 0` (array index)

### 3. Form → Dropdown: Display Selected Value

[TemplateBuilderAutoQA.tsx:307-308](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/TemplateBuilderAutoQA/TemplateBuilderAutoQA.tsx#L307)

```tsx
const detectedFieldValue = detectedField.value != null ? String(detectedField.value) : null;
const notDetectedFieldValue = notDetectedField.value != null ? String(notDetectedField.value) : null;
```

**Flow:**
1. Form has `auto_qa.detected = 0` (number)
2. Convert to string: `detectedFieldValue = "0"`
3. Pass to Select: `<Select value={detectedFieldValue} />`
4. Mantine Select highlights option with `value="0"` → "Good (10)"

### Complete Round-Trip Example

**Scenario:** User adds option, then selects it in AutoQA

```typescript
// 1. User adds "Excellent" option
CriteriaLabeledOptions.onAddLabel(false)
→ options: [..., { label: 'Excellent', value: 3 }]
→ scores: [..., { value: 3, score: 20 }]

// 2. behaviorScoreSelectionOptions reruns (useMemo dependency changed)
→ Dropdown options: [..., { label: "Excellent (20)", value: "3" }]

// 3. User selects "Excellent (20)" from "If behavior is done" dropdown
→ Mantine Select calls onChangeDetectedField("3")

// 4. onChangeDetectedField converts string to number
→ detectedField.onChange(3)

// 5. Form updates
→ auto_qa.detected = 3

// 6. On next render, display selected value
→ detectedFieldValue = String(3) = "3"
→ Select shows "Excellent (20)" as selected
```

### Why String Conversion?

Mantine Select requires:
- **values** (keys) to be **strings**
- **onChange** receives **string**
- **value** prop expects **string**

But our form stores:
- **auto_qa.detected/not_detected/not_applicable** as **numbers**

So we need bidirectional conversion:
- **Form → UI:** `Number → String` (for display)
- **UI → Form:** `String → Number` (for storage)

### Edge Case: N/A Score is null

```typescript
// Option with null score
{ label: 'N/A', value: 2, isNA: true }
{ value: 2, score: null }

// Dropdown label shows
label: "N/A (null)"  // ← Falls through to show "null" literally

// This is cosmetic - the value="2" (index) is what matters
```

### Visual Summary: Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                  USER ADDS/EDITS OPTIONS                        │
│  CriteriaLabeledOptions: Add "Excellent" with score 20         │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    FORM STATE UPDATES                           │
│  settings.options: [..., { label: 'Excellent', value: 3 }]     │
│  settings.scores: [..., { value: 3, score: 20 }]               │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼ (useMemo dependency triggers)
┌─────────────────────────────────────────────────────────────────┐
│           DROPDOWN OPTIONS RECALCULATED (useMemo)               │
│  behaviorScoreSelectionOptions = [                              │
│    { label: "Good (10)", value: "0" },                          │
│    { label: "Bad (0)", value: "1" },                            │
│    { label: "N/A (null)", value: "2" },                         │
│    { label: "Excellent (20)", value: "3" }  ← NEW               │
│  ]                                                              │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│              USER SELECTS FROM DROPDOWN                         │
│  Clicks "Excellent (20)" in "If behavior is done" dropdown     │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼ (onChange callback)
┌─────────────────────────────────────────────────────────────────┐
│                STRING → NUMBER CONVERSION                       │
│  onChangeDetectedField("3")                                     │
│  → detectedField.onChange(3)                                    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                  FORM STATE UPDATED                             │
│  auto_qa.detected = 3                                           │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼ (Next render)
┌─────────────────────────────────────────────────────────────────┐
│              NUMBER → STRING CONVERSION                         │
│  detectedFieldValue = String(3) = "3"                           │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│              DROPDOWN SHOWS SELECTED VALUE                      │
│  <Select value="3" /> highlights "Excellent (20)"              │
└─────────────────────────────────────────────────────────────────┘
```

**Key Takeaway:** The dropdown options are **derived** from `settings.options` via `useMemo`, ensuring they always stay in sync. Changes flow unidirectionally: Options → Dropdown → Selection → Form.

---

## N/A Score: Consistent with Normal Scores + Edge Case Cleanup

[CriteriaLabeledOptions.tsx:275-290](/Users/xuanyu.wang/repos/director/packages/director-app/src/features/admin/coaching/template-builder/configuration/CriteriaLabeledOptions.tsx#L275)

**Both normal and N/A scores now have consistent behavior:**

```tsx
<NumberInput
  {...field}
  onChange={(value) => {
    field.onChange(value);            // Update score
    resetAutoQADetectedFields();      // Clear auto_qa.detected/not_detected
  }}
  // N/A-specific: additional onBlur for edge case cleanup
  onBlur={() => {
    field.onBlur();
    if (isNAScore === undefined) {
      removeNAOption();  // ← Defensive: remove if corrupted
    }
  }}
/>
```

**Why reset autoQA when score changes?**
- Changing score changes dropdown label: "Good (10)" → "Good (5)" or "N/A (null)" → "N/A (5)"
- Clearing selections prevents confusion about which option is selected after labels change
- Consistent behavior whether changing normal or N/A scores

**Why onBlur for N/A only?**
- Edge case: If `isNAScore === undefined`, option data is corrupted (shouldn't happen if invariants hold)
- `removeNAOption()` cleans up on blur
- Normal options can't have this corruption (they always have a score from creation)
- This is N/A-specific defensive programming

---

## Key Design Principles

1. **isNA flag persistence** - Lives on option object, survives renormalization
2. **N/A always last** - `existingNAIndex` check in `onAddLabel`
3. **Auto_qa cleanup** - `removeNAOption` clears `auto_qa.not_applicable`
4. **StrictMode idempotency** - `form.getValues()` for synchronous reads
5. **Index remapping** - `handleRemoveOption` remaps all dependent fields
6. **Score decoupling** - `settings.scores[]` independent, linked by value

---

## Testing Checklist

- [ ] Test in StrictMode - no duplicate N/A on mount
- [ ] Delete option with branches - verify remapping
- [ ] Delete option with auto_qa selected - verify remapping
- [ ] Add option when N/A exists - verify N/A stays last
- [ ] Load legacy template with showNA=true - verify auto-creation
- [ ] Check/uncheck/recheck Allow N/A - verify no stale state
- [ ] Enter N/A score without checking box - verify option creation
- [ ] Delete option that auto_qa.detected points to - verify cleared
