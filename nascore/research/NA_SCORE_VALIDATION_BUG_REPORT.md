# N/A Score Validation Bug Report

**Date:** 2026-04-12  
**Feature:** enableNAScore  
**Bug:** "At least one score is not configured properly" validation error on new criteria

---

## Problem Summary

When creating a new criterion with default settings (Allow N/A enabled), the template fails validation with error:

```
At least one score is not configured properly
```

**Malformed scores data:**
```json
[
  {"score": 1},           // ❌ Missing "value" field
  {"score": 2},           // ❌ Missing "value" field  
  {"value": 2, "score": null}  // ✅ N/A score (correct)
]
```

**Expected scores data:**
```json
[
  {"value": 1, "score": 1},
  {"value": 0, "score": 0},
  {"value": 2, "score": null}
]
```

---

## Why This Bug Didn't Happen Before

### 1. Feature Flag Recently Enabled
The `enableNAScore` feature flag was recently enabled in production. Before this:
- N/A options existed but didn't have associated scores
- The scores array didn't need to handle `null` values
- Validation didn't need to distinguish between N/A and regular options

### 2. Decoupled Scoring Model
The decoupled scoring model separates options and scores into two parallel arrays:

```json
{
  "options": [
    {"label": "Yes", "value": 1},
    {"label": "No", "value": 0}
  ],
  "scores": [
    {"value": 1, "score": 1},    // Matches option with value=1
    {"value": 0, "score": 0}     // Matches option with value=0
  ]
}
```

The `value` field links options to their corresponding scores. With enableNAScore:
```json
{
  "options": [
    {"label": "Yes", "value": 1},
    {"label": "No", "value": 0},
    {"label": "N/A", "value": 2, "isNA": true}
  ],
  "scores": [
    {"value": 1, "score": 1},
    {"value": 0, "score": 0},
    {"value": 2, "score": null}   // N/A can have null score
  ]
}
```

### 3. Migration Code Designed for Existing Templates
The migration code in `CriteriaLabeledOptions.useOnMount` was designed to add N/A options to existing templates that had `showNA: true` but no N/A option yet (created before `enableNAScore` feature flag).

**It was NOT designed to handle NEW criteria being created in real-time.**

---

## Data Flow: How Criteria Are Created

### Flow 1: Creating a New Template (First Criterion)

```
1. User clicks "New Template"
   ↓
2. TemplateBuilderForm.calculateDefaultFormValues() runs
   - Returns: { template: { items: [] }, ... }
   - Empty items array - no criteria yet
   ↓
3. User clicks "Add Criterion" button
   ↓
4. handleAddCriterion() is called
   ↓
5. Create newCriterion from DEFAULT_CRITERION
   const newCriterion = {
     ...cloneDeep(DEFAULT_CRITERION),
     identifier: uuidv7()
   }
   
   DEFAULT_CRITERION contains:
   {
     type: CriterionTypes.LabeledRadios,
     settings: {
       options: [
         { label: 'Yes', value: 1 },
         { label: 'No', value: 0 }
       ],
       showNA: true,
       // ⚠️ NO scores field!
     }
   }
   ↓
6. defaultCriterion is undefined (no previous criterion)
   - Skip copying from defaultCriterion
   ↓
7. Initialize scores array (THE FIX)
   newCriterion.settings.scores = [
     { value: 1, score: 1 },  // Yes
     { value: 0, score: 0 }   // No
   ]
   
   📝 NOTE: Main branch doesn't do this step!
   - Main branch: scores = undefined
   - Main branch works because CriteriaLabeledOptions.useOnMount initializes scores
   - Feature branch NEEDS this because:
     a) Prevents React Hook Form from creating empty placeholders
     b) Allows N/A migration to work correctly
   ↓
8. form.setValue adds criterion to template.items
   ↓
9. CriteriaLabeledOptions mounts
   
   MAIN BRANCH (without fix):
   - watchedScoresField = undefined
   - Condition !watchedScoresField?.length = true
   - useOnMount initializes: [{ value: 1, score: 1 }, { value: 0, score: 0 }]
   - No N/A feature, done ✓
   
   FEATURE BRANCH (with fix):
   - watchedScoresField = [{ value: 1, score: 1 }, { value: 0, score: 0 }]
   - Condition !watchedScoresField?.length = false
   - useOnMount skips initialization (already done!)
   - N/A migration runs: adds { label: 'N/A', value: 2, isNA: true }
   - Adds score: { value: 2, score: null }
   ↓
10. Final state:
    options: [
      { label: 'Yes', value: 1 },
      { label: 'No', value: 0 },
      { label: 'N/A', value: 2, isNA: true }
    ]
    scores: [
      { value: 1, score: 1 },
      { value: 0, score: 0 },
      { value: 2, score: null }
    ]
```

### Flow 2: Creating a Second Criterion (defaultCriterion Set)

```
1. User already created and configured Criterion A
   - User changed options, added custom options, checked "Allow N/A"
   - TemplateBuilderCriterionConfiguration watches criterion
   - Sets defaultCriterion = Criterion A (including N/A option)
   ↓
2. User clicks "Add Criterion" again
   ↓
3. handleAddCriterion() is called
   ↓
4. Create newCriterion from DEFAULT_CRITERION
   const newCriterion = { ...cloneDeep(DEFAULT_CRITERION), ... }
   ↓
5. defaultCriterion IS set (contains Criterion A's settings)
   
   defaultCriterion.settings = {
     options: [
       { label: 'Yes', value: 1 },
       { label: 'No', value: 0 },
       { label: 'N/A', value: 2, isNA: true }  // ← Problem!
     ],
     scores: [{}, {}, {}]  // ← Malformed from React Hook Form!
   }
   ↓
6. Copy settings from defaultCriterion
   
   ⚠️ BEFORE THE FIX:
   newCriterion.settings.options = defaultCriterion.settings.options
   // Copies N/A option! ← Bug starts here
   
   ✅ AFTER THE FIX:
   const optionsWithoutNA = defaultCriterion.settings.options.filter(opt => !opt.isNA)
   newCriterion.settings.options = optionsWithoutNA
   // Filters out N/A ← Bug prevented
   ↓
7. Initialize scores array (THE FIX)
   // Never copy scores from defaultCriterion!
   newCriterion.settings.scores = optionsWithoutNA.map(opt => ({
     value: opt.value,
     score: opt.value
   }))
   ↓
8. Rest of flow same as Flow 1
   - form.setValue
   - CriteriaLabeledOptions mounts
   - N/A added dynamically if needed
```

### Flow 3: Creating a Criterion in a Chapter/Section

```
1. User clicks "Add Criterion" inside a Chapter
   ↓
2. handleAddCriterion(chapterFieldName) is called
   - chapterFieldName = "template.items.0" (path to chapter)
   ↓
3. Same creation logic as Flow 1 or Flow 2
   - Create from DEFAULT_CRITERION
   - Copy from defaultCriterion if set (filter N/A)
   - Initialize scores array
   ↓
4. Difference: criterion added to chapter.items instead of template.items
   const parentFieldName = chapterFieldName || 'template'
   const itemsListFieldName = `${parentFieldName}.items`
   
   form.setValue(itemsListFieldName, [...itemsList, newCriterion])
   ↓
5. Rest of flow is identical
   - CriteriaLabeledOptions mounts
   - N/A added if needed
```

---

## Root Causes

### Root Cause 1: defaultCriterion Pollution

**What is defaultCriterion?**
- When you edit a criterion, `TemplateBuilderCriterionConfiguration` saves it as `defaultCriterion`
- When creating the next new criterion, `handleAddCriterion` copies settings from `defaultCriterion`
- This provides a better UX: if you configure one criterion, the next one starts with similar settings
- See **Flow 2** above for the complete sequence

**The pollution:**
```javascript
// Flow 2, step 5: defaultCriterion contains Criterion A's settings
defaultCriterion.settings = {
  options: [
    { label: "Yes", value: 1 },
    { label: "No", value: 0 },
    { label: "N/A", value: 2, isNA: true }  // ← Should NOT be copied!
  ],
  scores: [{},{},{}]  // ← Malformed from React Hook Form placeholders
}

// Flow 2, step 6: BEFORE THE FIX
newCriterion.settings.options = defaultCriterion.settings.options;
// ⚠️ Copies N/A option to new criterion!
// ⚠️ N/A should be added dynamically by CriteriaLabeledOptions, not copied
```

**Code location:**
```typescript
// TemplateBuilderFormConfigurationStep.tsx (BEFORE FIX)
if ('options' in defaultCriterion.settings && defaultCriterion.settings.options) {
  newCriterion.settings.options = defaultCriterion.settings.options;  // ← Copies N/A!
}

// AFTER FIX: Filter out N/A
const optionsWithoutNA = defaultCriterion.settings.options.filter(opt => !opt.isNA);
newCriterion.settings.options = optionsWithoutNA;  // ← N/A filtered out ✓
```

### Root Cause 2: React Hook Form's useFieldArray Behavior

When `useFieldArray` is initialized on an undefined field with existing options, it creates **placeholder empty objects**.

**What happens (see Flow 1, steps 8-9):**

```typescript
// Step 8: form.setValue adds criterion with options but NO scores
form.setValue('template.items', [{
  settings: {
    options: [
      { label: 'Yes', value: 1 },
      { label: 'No', value: 0 }
    ]
    // scores: undefined ← Not set!
  }
}]);

// Step 9: CriteriaLabeledOptions mounts
const scoresFieldArray = useFieldArray({
  name: 'template.items.0.settings.scores'  // undefined at this point
});

// React Hook Form sees options array has length 2
// Assumes scores array should also have length 2
// Creates placeholder empty objects:
currentScores: [{}, {}]  // ← Empty objects with no fields!
```

**Why empty objects?**
- useFieldArray tracks arrays by index
- If options[0] and options[1] exist, it assumes scores[0] and scores[1] should exist
- It creates empty objects as placeholders until values are set
- These empty objects have no `value` or `score` fields → validation fails

### Root Cause 3: Timing Issue

The initialization code in `CriteriaLabeledOptions.useOnMount` runs **too late**:

```
Flow 1, Step 7 (BEFORE FIX):
  handleAddCriterion creates criterion with options but NO scores field
  newCriterion.settings.scores = undefined
  ↓
Flow 1, Step 8:
  form.setValue adds criterion to form
  ↓
Flow 1, Step 9:
  CriteriaLabeledOptions mounts
  useFieldArray initializes
  ↓
  ⚠️ PROBLEM: useFieldArray sees 2 options, creates [{}, {}] placeholders
  currentScores = [{}, {}]  // ← Too early! Before useOnMount runs
  ↓
  useOnMount runs
  ↓
  ⚠️ PROBLEM: Initialization condition fails
  if (currentOptions?.length && !currentScores?.length) {
    // Initialize scores
  }
  // currentScores.length = 2 (not 0!), so condition is false
  // Initialization skipped!  ← Too late to fix!
```

**The fix:** Initialize scores in Step 7 (handleAddCriterion) BEFORE Step 8 (form.setValue), so useFieldArray never sees undefined scores.

**Visual comparison:**

```
BEFORE FIX:
  handleAddCriterion                CriteriaLabeledOptions
  ─────────────────                 ──────────────────────
  Create criterion
  scores: undefined
       ↓
  form.setValue ──────────────────→ Mounts
       ↓                                 ↓
       ✓                            useFieldArray sees undefined
                                    Creates [{}, {}] placeholders
                                         ↓
                                    useOnMount runs
                                    Tries to initialize
                                    But length = 2, skips!
                                         ↓
                                    ❌ Malformed scores

AFTER FIX:
  handleAddCriterion                CriteriaLabeledOptions
  ─────────────────                 ──────────────────────
  Create criterion
  scores: undefined
       ↓
  Initialize scores ← FIX
  scores: [{value:1,score:1}, {value:0,score:0}]
       ↓
  form.setValue ──────────────────→ Mounts
       ↓                                 ↓
       ✓                            useFieldArray sees initialized array
                                    Uses existing scores ✓
                                         ↓
                                    useOnMount runs
                                    Adds N/A if needed
                                         ↓
                                    ✓ Properly structured scores
```

### Root Cause 4: N/A Migration Not Designed for New Criteria

The N/A migration in `useOnMount` was designed for existing templates:
```typescript
// Scenario: Existing template with showNA enabled but no N/A option
{
  "settings": {
    "showNA": true,        // ← Setting exists from before enableNAScore
    "options": [           // ← But no N/A option with isNA flag
      {"label": "Yes", "value": 1},
      {"label": "No", "value": 0}
    ]
  }
}
// Migration adds: {"label": "N/A", "value": 2, "isNA": true}
```

It was NOT designed for new criteria being created in real-time, where options and scores are being built up dynamically and N/A might get added multiple times or at the wrong time.

---

## The Fix

### Solution: Initialize Scores in handleAddCriterion

Initialize the scores array when creating the criterion, BEFORE adding it to the form:

```typescript
// TemplateBuilderFormConfigurationStep.tsx

const newCriterion = {
  ...cloneDeep(DEFAULT_CRITERION),
  identifier: newCriterionIdentifier,
};

// 1. Filter out N/A from defaultCriterion (don't copy it)
if (defaultCriterion?.settings?.options) {
  const optionsWithoutNA = defaultCriterion.settings.options.filter((opt) => !opt.isNA);
  newCriterion.settings.options = optionsWithoutNA;
}

// 2. Initialize scores array BEFORE form.setValue
// Keep original option values (Yes=1, No=0) - no need to normalize
const currentOptions = newCriterion.settings.options;
const optionsWithoutNA = currentOptions.filter((opt) => !opt.isNA);

newCriterion.settings.scores = optionsWithoutNA.map((opt) => ({
  value: opt.value,   // Keep original value
  score: opt.value    // In DEFAULT_CRITERION, value IS the score
}));

// Result:
// options: [{"label":"Yes","value":1}, {"label":"No","value":0}]
// scores:  [{"value":1,"score":1}, {"value":0,"score":0}]

// 3. Add to form (now useFieldArray sees properly initialized scores)
form.setValue(itemsListFieldName, [...itemsList, newCriterion]);

// 4. CriteriaLabeledOptions mounts and adds N/A option if needed
// Adds: {"label":"N/A","value":2,"isNA":true}
// Adds: {"value":2,"score":null}
```

### What Changed

**File: `TemplateBuilderFormConfigurationStep.tsx`**
1. Filter out N/A option when copying from defaultCriterion (it should be added dynamically)
2. Initialize scores array BEFORE adding criterion to form (prevents React Hook Form placeholders)
3. Keep original option values (Yes=1, No=0) - match each option.value to its score
4. Never copy scores from defaultCriterion

**File: `CriteriaLabeledOptions.tsx`**
1. Simplify useOnMount (initialization now happens earlier in handleAddCriterion)
2. Keep N/A migration for existing templates only

---

## Why This Fix Is Better Than Alternatives

### ❌ Alternative 1: Fix Initialization in CriteriaLabeledOptions

**Approach:** Detect malformed scores in useOnMount and fix them

**Problems:**
- Runs too late (after useFieldArray creates empty objects)
- Complex logic to detect all malformed patterns
- Doesn't fix the root cause (initialization timing)

**Code would look like:**
```typescript
useOnMount(() => {
  const scores = form.getValues('settings.scores');
  // Fix empty objects: [{}, {}] → [...]
  // Fix missing value field: [{"score":1}] → [{"value":0,"score":1}]
  // Fix duplicates, wrong indexes, etc.
  // Very fragile!
});
```

### ❌ Alternative 2: Don't Use defaultCriterion

**Approach:** Always start from DEFAULT_CRITERION, ignore previous criterion

**Problems:**
- Loses valuable UX feature
- Users expect new criteria to copy settings from previous one
- More clicks to configure each criterion

**User impact:**
```
Before: Create criterion with 5 custom options → Next criterion starts with those 5
After:  Create criterion with 5 custom options → Next criterion starts with Yes/No only
        ↑ User has to recreate the 5 options every time
```

### ❌ Alternative 3: Add Scores to DEFAULT_CRITERION

**Approach:** Define scores in the constant

```typescript
export const DEFAULT_CRITERION = {
  settings: {
    options: [{"label":"Yes","value":1}, {"label":"No","value":0}],
    scores: [{"value":1,"score":1}, {"value":0,"score":0}]
  }
};
```

**Problems:**
- Violates separation of concerns (constant shouldn't know about decoupled scoring)
- Tight coupling between DEFAULT_CRITERION and enableNAScore feature
- Doesn't solve defaultCriterion pollution (would still copy N/A from previous criterion)
- Hard to maintain when features change
- Duplicates data (options already define values, scores repeat them)

### ❌ Alternative 4: Migrate on Form Load

**Approach:** Run migration in TemplateBuilderForm.calculateDefaultFormValues

**Problems:**
- TemplateBuilderForm shouldn't know about decoupled scoring details
- Separation of concerns: scoring logic belongs in scoring component
- Doesn't prevent the problem for new criteria
- Only fixes loaded templates, not newly created ones

---

## ✅ Why Our Solution Is Best

### 1. **Prevents Problem at Source**
- Initialize scores when creating criterion, BEFORE React Hook Form touches it
- No empty objects, no malformed data, no timing issues

### 2. **Maintains Separation of Concerns**
- `handleAddCriterion`: Creates criterion with proper initial state
- `CriteriaLabeledOptions`: Adds N/A option if enabled
- Clear boundaries, single responsibility

### 3. **Preserves defaultCriterion Feature**
- Still copies options from previous criterion
- Just filters out N/A (which should be added dynamically)
- Users get the UX benefit without the bug

### 4. **Minimal Code Changes**
- Only touched handleAddCriterion (criterion creation)
- Removed complex migration logic from CriteriaLabeledOptions
- Cleaner, simpler code overall

### 5. **Clear Data Flow**
```
1. handleAddCriterion:
   - Create criterion with options from DEFAULT_CRITERION
   - Filter out N/A if present (from defaultCriterion pollution)
   - Initialize scores [{value:1,score:1}, {value:0,score:0}]
   
2. form.setValue:
   - Add criterion to form state
   
3. CriteriaLabeledOptions mounts:
   - See scores already exist ✓
   - Add N/A option if showNA=true and enableNAScore=true
   - Add N/A score {value:2, score:null}
   
4. Validation:
   - All scores have {value, score} structure ✓
   - N/A score can be null ✓
   - Pass validation ✓
```

---

## Testing Checklist

- [x] Create new criterion with default settings → validates ✓
- [x] Create criterion with N/A enabled → validates ✓
- [x] Create criterion, edit it, create another → second criterion doesn't copy N/A ✓
- [x] Load existing template with N/A → scores properly structured ✓
- [x] Save template with N/A null score → succeeds ✓
- [x] Publish template with N/A null score → succeeds ✓

---

## Key Learnings

### 1. **Component Lifecycle Matters**
React Hook Form's useFieldArray runs BEFORE useOnMount. Initialize data before components mount.

### 2. **Migration Code Scope**
Migration code should have clear scope: "existing templates" vs "new data". Don't use the same code for both.

### 3. **defaultCriterion is a Reference**
When copying from defaultCriterion, be careful about:
- Feature-specific data (like N/A options)
- Form-managed data (like scores arrays)
- Filter what you copy!

### 4. **Validation Requirements Drive Data Structure**
Validation requires both `value` and `score` fields. Initialize with this structure from the start, don't try to fix it later.

### 5. **Separation of Concerns**
- Constants define static defaults
- Form handlers create dynamic initial state
- Components add feature-specific enhancements
- Each layer has clear responsibility

---

## Related Files

- `TemplateBuilderFormConfigurationStep.tsx` (lines 297-350) - Criterion creation with score initialization
- `CriteriaLabeledOptions.tsx` (lines 79-145) - N/A option addition
- `validation.ts` (lines 534-566) - Score validation logic
- `VALIDATION_FIX_ANALYSIS.md` - Validation fix for N/A score support
