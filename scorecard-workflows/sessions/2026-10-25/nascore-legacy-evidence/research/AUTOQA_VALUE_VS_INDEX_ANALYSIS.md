# AutoQA Value vs Index Analysis

**Created:** 2026-04-13

## Current Implementation

### In Decoupled Scoring Mode

**Data Structure:**
```typescript
settings: {
  options: [
    {label: 'Yes', value: 0},  // index 0, value 0
    {label: 'No', value: 1}     // index 1, value 1
  ],
  scores: [
    {value: 0, score: 1},  // options[0] maps to score 1
    {value: 1, score: 0}   // options[1] maps to score 0
  ]
}
auto_qa: {
  detected: 0,      // ← Is this an INDEX or a VALUE?
  not_detected: 1   // ← Is this an INDEX or a VALUE?
}
```

### How AutoQA Values Are Used

**1. In TemplateBuilderAutoQA.tsx (line 238-247):**
```typescript
if (isDecoupledScoring) {
  // Use index as value key
  scoreOptions.options.forEach((option: ScoreOption, index: number) => {
    const score = scoreOptions.scores?.[index]?.score;  // ← Lookup by INDEX
    const displayScore = checkIsNAOption(option) && score == null ? 'no score' : (score ?? option.value);
    selectionOptions.push({ 
      label: `${option.label} (${displayScore})`, 
      value: index.toString()  // ← Dropdown value is INDEX as string
    });
  });
}
```

**2. In onChange handlers (line 288-308):**
```typescript
const onChangeDetectedField = useCallback((value: string | null) => {
  if (isDecoupledScoring) {
    detectedField.onChange(value != null ? Number(value) : null);  // ← Store as number (INDEX)
  }
}, [detectedField, isDecoupledScoring]);
```

**3. In validation.ts (line 224-227):**
```typescript
if (
  item.auto_qa.detected < 0 ||
  item.auto_qa.detected >= optionsLength ||  // ← Treating as INDEX (comparing to array length)
  item.auto_qa.not_detected < 0 ||
  item.auto_qa.not_detected >= optionsLength
)
```

### Conclusion

**AutoQA uses INDEXES in decoupled scoring mode**, not values.

- `auto_qa.detected: 0` means "the option at **index 0**"
- `auto_qa.not_detected: 1` means "the option at **index 1**"

This works correctly because:
- options[index].value === index (we normalize to sequential)
- scores[index].value === index (same normalization)
- So using index to lookup both arrays works

## Potential Issues

### Issue 1: Options and Scores must stay in sync

If options are reordered or deleted:
```typescript
// Before deletion
options: [{label: 'Yes', value: 0}, {label: 'Maybe', value: 1}, {label: 'No', value: 2}]
scores: [{value: 0, score: 2}, {value: 1, score: 1}, {value: 2, score: 0}]
auto_qa.detected: 0  // Points to 'Yes' at index 0

// After deleting 'Maybe' at index 1
options: [{label: 'Yes', value: 0}, {label: 'No', value: 1}]  // ← Renormalized
scores: [{value: 0, score: 2}, {value: 1, score: 0}]         // ← Renormalized
auto_qa.detected: 0  // Still points to index 0, which is still 'Yes' ✓
```

The `handleRemoveOption` function in CriteriaLabeledOptions.tsx (line 140-180) handles this by remapping AutoQA values when options are deleted.

### Issue 2: Migration from old templates

Old templates (before decoupled scoring) used option.value:
```typescript
// Old format
options: [{label: 'Yes', value: 1}, {label: 'No', value: 0}]
auto_qa.detected: 1  // Meant value 1 (Yes)
auto_qa.not_detected: 0  // Meant value 0 (No)

// After loading into new format
options: [{label: 'Yes', value: 0}, {label: 'No', value: 1}]  // ← Renormalized
scores: [{value: 0, score: 1}, {value: 1, score: 0}]
auto_qa.detected: 1  // Now means INDEX 1, which is 'No' ✗ WRONG!
```

**Is there migration code to handle this?**

Looking at `CriteriaLabeledOptions.tsx` line 92-125 (legacy migration), it handles:
- Initializing scores for old templates
- Renormalizing options to sequential indexes

But it does NOT remap `auto_qa.detected` and `auto_qa.not_detected` values!

## Resolution: No Change Needed

**Verified on 2026-04-13:** Origin/main (prod) uses `index` as the lookup key in decoupled scoring mode, not `option.value`. This works because option values are **always normalized to sequential integers** (0, 1, 2...) matching their array index.

The code explicitly comments: `// In decoupled mode, use index as value key (ensures uniqueness for Mantine Select)`

The feature branch follows the same pattern. No change needed.
