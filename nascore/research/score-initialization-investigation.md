# Score Initialization Investigation

## Issue Report

**User's observation:**
> In existing prod, the default value of Score box are always empty, and will trigger "Value is required" alert. In current branch, those box will not trigger alert when they're empty.

## Current Code Analysis

### Line 79: New option creation
```tsx
const newScore = { value: newValue, score: isNA ? null : 0 };
```
- Normal options: `score: 0`
- N/A options: `score: null`

### Line 98-99: Auto-create first option
```tsx
if (optionsField.fields.length === 0) {
  onAddLabel();  // Creates first option with score: 0
}
```

### Main Branch (Same!)
```tsx
scoresFieldArray.append({ value: maxId + 1, score: 0 });
```

## Question

If both main and current branch set `score: 0`, why would the behavior be different?

### Hypothesis 1: Should be `undefined` instead of `0`

Maybe the initial score should be:
```tsx
const newScore = { value: newValue, score: isNA ? null : undefined };
```

This would:
- ✅ Show empty input (undefined doesn't display)
- ✅ Trigger validation error (undefined fails `required` rule)
- ✅ Force user to explicitly enter a score

### Hypothesis 2: Should NOT auto-create first option

Maybe line 98-99 shouldn't exist? User should manually click "Add Option"?

### Hypothesis 3: NumberInput behavior changed

Maybe NumberInput in current branch treats `0` differently than before?

## Testing Plan

1. Check if removing auto-creation (line 98-99) fixes the issue
2. Check if changing `score: 0` to `score: undefined` fixes the issue
3. Check prod behavior - does first option auto-create or not?
