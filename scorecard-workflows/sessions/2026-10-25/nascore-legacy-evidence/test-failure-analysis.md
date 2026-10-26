# Test Failure Analysis - PR #26748

**Created:** 2026-04-13

## Test Failure After Conditional NotApplicable Change

### What Failed

Test: `TestAutoQAMapper/TestMapToScores`

**Expected:**
```go
{
    CriterionId:   "labeled-radios-na",
    NotApplicable: true,
    NumericValue:  nil,
}
```

**Actual (after our change):**
No score created at all (filtered out)

### Root Cause

The test fixture `LabeledRadiosNA` has:
- **Settings:** `ShowNA: true` (allows N/A selection)
- **AutoQA config:** `Detected: 3`, `NotDetected: 1`, **NO NotApplicable field**

When AutoQA returns NOT_APPLICABLE:
- **Before our change:** Creates score with `NotApplicable=true`, `NumericValue=nil`
- **After our change:** Doesn't set anything (because `autoQaConfig.NotApplicable == nil`)

### The Conflict

**tinglinliu's message says:**
"when not_applicable is set **in the config**, then score.not_applicable should be set to true"

**Our interpretation:**
- ONLY set NotApplicable=true when `autoQaConfig.NotApplicable != nil`
- Don't create score when `autoQaConfig.NotApplicable == nil`

**But the test expects:**
- Create score with NotApplicable=true even when `autoQaConfig.NotApplicable == nil`
- This represents **legacy N/A behavior**: templates that allow N/A but don't score it

## Two Possible Interpretations

### Interpretation A: Always Create N/A Score (Keep Old Behavior)

```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    mappedScore.NotApplicable = true  // ALWAYS set
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    }
    // Result: NotApplicable=true, NumericValue=nil when config not set
```

**Pros:**
- ✅ Tests pass
- ✅ Supports legacy N/A (ShowNA=true but no score)
- ✅ Always creates a score record when AutoQA returns NOT_APPLICABLE

**Cons:**
- ❌ Contradicts tinglinliu's "when not_applicable is set in the config" (implies conditional)
- ❌ This is the CURRENT code that tinglinliu is questioning

### Interpretation B: Conditional N/A Score (Our Change)

```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NotApplicable = true
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    }
    // Result: No score created when config not set
```

**Pros:**
- ✅ Aligns with tinglinliu's "when not_applicable is set in the config" (conditional)
- ✅ Simpler: don't create scores for unconfigured cases

**Cons:**
- ❌ Breaks existing test
- ❌ Breaks legacy templates that have ShowNA=true but no NotApplicable score configured
- ❌ These templates would stop creating N/A scores

### Interpretation C: Hybrid Approach

Maybe tinglinliu meant something different?

**"when not_applicable is set in the config" could mean:**
1. When `autoQaConfig.NotApplicable != nil` (our interpretation)
2. When the template has `ShowNA: true` in settings (different interpretation)

If #2, then the code should be:
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    // Check if template allows N/A (but we don't have access to template settings here!)
    mappedScore.NotApplicable = true
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    }
```

But this is the same as Interpretation A (current code).

## Questions for tinglinliu

### Question 1: Legacy N/A Support

There are existing templates where:
- Template has `ShowNA: true` (allows users to select N/A)
- But AutoQA config has NO `NotApplicable` value configured
- When AutoQA returns NOT_APPLICABLE:
  - **Current behavior:** Creates score with `NotApplicable=true`, `NumericValue=nil`
  - **After conditional change:** No score created

**Should we maintain support for these legacy templates?**

### Question 2: Clarify "in the config"

When you said "when not_applicable is set in the config", did you mean:
- A) When `autoQaConfig.NotApplicable` field exists (is not nil)?
- B) When template settings have `ShowNA: true`?
- C) Something else?

### Question 3: Test Expectations

The test `TestMapToScores` expects a score with `NotApplicable=true` for a template that has NO `NotApplicable` in AutoQA config.

**Is this test correct, or should we update it?**

## Recommendation

**Wait for tinglinliu's clarification before pushing.**

Current state:
- ✅ CodeRabbit comments addressed (docstring + test assertions) - committed
- ⚠️ Conditional NotApplicable change - made but not committed (breaks test)

Need to know:
1. Should we support legacy N/A (ShowNA but no score config)?
2. What does "in the config" specifically refer to?
3. Should tests be updated?
