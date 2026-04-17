# Revert to Main Branch Pattern - Analysis

**Created:** 2026-04-13

## Problem

The "fix" introduced in commit `4754e67d0a` (initializing scores in handleAddCriterion) added unnecessary complexity. It made watchedScoresField always have a value, which prevented the natural flow in CriteriaLabeledOptions.useOnMount.

## User Insight

> "The fix introduced the bug itself? The main branch behaviour looks reasonable enough."

The main branch pattern is simpler:
- Don't initialize scores in handleAddCriterion
- Let CriteriaLabeledOptions.useOnMount handle it

## Solution

### 1. Revert score initialization in handleAddCriterion

**REMOVE** (lines 325-344):
```typescript
// Normalize options to use sequential indexes (0, 1, 2...) for decoupled scoring model
// and initialize scores array (excluding N/A which will be added later by CriteriaLabeledOptions)
type OptionWithNA = { label: string; value: number; isNA?: boolean };
const currentOptions = newCriterion.settings.options as OptionWithNA[];
const optionsWithoutNA = currentOptions.filter((opt) => !opt.isNA);

// In the old model, option.value WAS the score (e.g., Yes=1, No=0)
// In the new decoupled model, we use sequential indexes for options and store scores separately
newCriterion.settings.options = optionsWithoutNA.map((opt, index) => ({
  label: opt.label,
  value: index,
}));

type SettingsWithScores = typeof newCriterion.settings & {
  scores: { value: number; score: number }[];
};
(newCriterion.settings as SettingsWithScores).scores = optionsWithoutNA.map((opt, index) => ({
  value: index,
  score: opt.value, // The original value IS the score
}));
```

**Result:** handleAddCriterion just copies options from defaultCriterion (with N/A filtered out), doesn't touch scores.

### 2. Fix legacy migration in CriteriaLabeledOptions.useOnMount

**BEFORE:**
```typescript
const initialScores = normalizedOptions.map((opt, index) => ({ value: index, score: 0 }));
```

**AFTER:**
```typescript
// In old templates, option.value WAS the score, so preserve it
const initialScores = normalizedOptions.map((opt, index) => ({
  value: index,
  score: opt.value  // Use original value as score
}));
```

## Flow Analysis

### Input (from defaultCriterion)
```typescript
options: [
  { label: 'Yes', value: 1 },
  { label: 'No', value: 0 }
]
scores: undefined  // Not initialized by handleAddCriterion
```

### CriteriaLabeledOptions.useOnMount sees:
- `currentOptions?.length = 2` ✅
- `!currentScores?.length = true` ✅ (because undefined)
- Condition is true → Initialize scores

### Normalization:
```typescript
// Renormalize options to sequential indexes
normalizedOptions = [
  { label: 'Yes', value: 0 },   // Was value: 1
  { label: 'No', value: 1 }      // Was value: 0
]

// Create scores using ORIGINAL value as score
initialScores = [
  { value: 0, score: 1 },  // Yes: index 0, original value 1 → score 1
  { value: 1, score: 0 }   // No: index 1, original value 0 → score 0
]
```

### Output:
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

✅ **Perfect!** Preserves the score mapping from the old model.

## Why This Is Better

1. **Simpler**: Follows main branch pattern, less code in handleAddCriterion
2. **Single responsibility**: CriteriaLabeledOptions is responsible for ALL score-related logic
3. **No dual initialization**: Only one place initializes scores
4. **Main branch compatibility**: Same pattern as main branch

## Testing Checklist

- [ ] Create new criterion with AutoQA disabled → scores initialized correctly
- [ ] Create new criterion with AutoQA enabled → scores initialized correctly
- [ ] Toggle AutoQA off → scores stay correct
- [ ] Check "Allow N/A" → N/A option added with null score
- [ ] Uncheck "Allow N/A" → N/A option removed
- [ ] Old templates without scores array → migrated correctly
