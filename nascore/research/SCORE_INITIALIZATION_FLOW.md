# Score Initialization Flow - After Fix

**Created:** 2026-04-13

## Flow Trace

### Step 1: handleAddCriterion

```typescript
// defaultCriterion has:
options: [
  { label: 'Yes', value: 1 },
  { label: 'No', value: 0 }
]

// After filtering N/A:
newCriterion.settings.options = [
  { label: 'Yes', value: 1 },
  { label: 'No', value: 0 }
]
newCriterion.settings.scores = undefined  // NOT initialized
```

### Step 2: form.setValue

Form receives:
```typescript
options: [
  { label: 'Yes', value: 1 },
  { label: 'No', value: 0 }
]
scores: undefined
```

### Step 3: CriteriaLabeledOptions.useOnMount

Condition check:
```typescript
currentOptions = [
  { label: 'Yes', value: 1 },
  { label: 'No', value: 0 }
]
currentScores = undefined

currentOptions?.length = 2 ✅
!currentScores?.length = true ✅

→ Enter initialization block
```

### Step 4: Score Initialization Logic

```typescript
// 1. Filter N/A (none in this case)
const optionsWithoutNA = [
  { label: 'Yes', value: 1 },  // ← Original value 1
  { label: 'No', value: 0 }    // ← Original value 0
];

// 2. Renormalize options to sequential indexes
const normalizedOptions = optionsWithoutNA.map((opt, index) => ({
  ...opt,
  value: index  // Override with sequential index
}));
// Result:
normalizedOptions = [
  { label: 'Yes', value: 0 },  // ← NEW value 0 (index)
  { label: 'No', value: 1 }    // ← NEW value 1 (index)
];

// 3. Create scores using ORIGINAL values
const initialScores = optionsWithoutNA.map((opt, index) => ({
  value: index,      // Sequential index
  score: opt.value   // ORIGINAL value (1 and 0)
}));
// Result:
initialScores = [
  { value: 0, score: 1 },  // Yes: index 0, original value 1
  { value: 1, score: 0 }   // No: index 1, original value 0
];
```

### Step 5: Final State

```typescript
options: [
  { label: 'Yes', value: 0 },
  { label: 'No', value: 1 }
]
scores: [
  { value: 0, score: 1 },
  { value: 1, score: 0 }
]
```

## Key Fix

**The crucial part:** Use `optionsWithoutNA` for creating scores, not `normalizedOptions`!

```typescript
// ❌ WRONG (old code):
const normalizedOptions = optionsWithoutNA.map((opt, index) => ({ ...opt, value: index }));
const initialScores = normalizedOptions.map((opt, index) => ({
  value: index,
  score: opt.value  // This would be the NEW index, not original score!
}));

// ✅ CORRECT (new code):
const normalizedOptions = optionsWithoutNA.map((opt, index) => ({ ...opt, value: index }));
const initialScores = optionsWithoutNA.map((opt, index) => ({
  value: index,
  score: opt.value  // This uses the ORIGINAL value
}));
```

## Why This Works

1. `optionsWithoutNA` preserves original values: `[Yes=1, No=0]`
2. `normalizedOptions` uses sequential indexes: `[Yes=0, No=1]`
3. `initialScores` maps:
   - Index → sequential value field
   - Original value → score field
4. Result: Decoupled scoring with preserved score mapping!
