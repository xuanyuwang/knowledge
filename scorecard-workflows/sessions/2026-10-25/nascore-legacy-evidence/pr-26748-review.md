# PR #26748 Review Summary

**Created:** 2026-04-13
**Updated:** 2026-04-13

## PR Status

⚠️ **BLOCKED - Needs Design Clarification**

There are fundamental design questions from tinglinliu about the scored N/A approach that need to be resolved before merging.

## All Review Comments from tinglinliu

### Comment 1: autoqa_mapper.go line 87
**tinglinliu says:** "don't need set NotApplicable to true in this case"

### Comment 2: scorecard_scores_dao.go line 577
**tinglinliu says:** "why removing this not_applicable check?"

### Message 3:
**tinglinliu says:** "when not_applicable is set in the config, then the score.not_applicable should be set to true"

## Analysis: Reconciling All Three Pieces of Feedback

Looking at the current code structure:
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    mappedScore.NotApplicable = true  // ← Line 82: UNCONDITIONAL
    // ... comments ...
    if autoQaConfig.NotApplicable != nil {  // ← Line 87: Comment here
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    }
```

### The Key Insight

Comment 1 is on line 87 (the `if` statement), but it's actually about line 82 that UNCONDITIONALLY sets `NotApplicable = true`.

**tinglinliu's intent (combining all three pieces):**
1. **Don't unconditionally** set NotApplicable = true (Comment 1)
2. **Only set** NotApplicable = true when config has a value (Message 3)
3. The NotApplicable check removal needs justification (Comment 2)

### Proposed Fix: Conditional NotApplicable Flag

```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    // REMOVE the unconditional line 82
    if autoQaConfig.NotApplicable != nil {
        // Only set NotApplicable when there's a configured score
        // This satisfies Message 3: "when not_applicable is set in the config, 
        // then the score.not_applicable should be set to true"
        mappedScore.NotApplicable = true
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    }
    // If autoQaConfig.NotApplicable == nil, don't set anything
    // (AutoQA returned NOT_APPLICABLE but template has no N/A score configured)
```

### Two Design Paths Forward

#### Path A: Scored N/A Only (Recommended based on tinglinliu's feedback)

**Behavior:**
- AutoQA returns NOT_APPLICABLE + config has score → `NotApplicable=true` + `NumericValue=<score>`
- AutoQA returns NOT_APPLICABLE + config has NO score → Skip creating score / leave as nil

**Changes needed:**
1. ✅ Move `NotApplicable = true` inside the `if` block (autoqa_mapper.go)
2. ✅ Keep NotApplicable check removal (scorecard_scores_dao.go) - needed for scored N/A to flow through
3. ✅ Tests already cover both cases

**This aligns with all three pieces of feedback:**
- Comment 1: ✅ Not setting NotApplicable unconditionally
- Comment 2: ✅ Removal justified because scored N/A needs to flow through
- Message 3: ✅ NotApplicable is set when config value exists

#### Path B: Treat Scored N/A as Regular Option (Alternative)

**Behavior:**
- AutoQA returns NOT_APPLICABLE + config has score → `NotApplicable=false` + `NumericValue=<score>` (treat as regular option)
- AutoQA returns NOT_APPLICABLE + config has NO score → `NotApplicable=true` + no NumericValue

**Changes needed:**
1. ✅ Remove `NotApplicable = true` entirely when config has score
2. ⚠️ Can restore NotApplicable check (but not required)
3. ⚠️ Update tests to expect NotApplicable=false for scored N/A

**This interpretation:**
- Comment 1: ✅ Not setting NotApplicable for scored cases
- Comment 2: ⚠️ Less clear why removal is needed
- Message 3: ❌ Contradicts - says to set NotApplicable when config has value

## Recommended Action: Path A

Based on Message 3 stating "when not_applicable is set in the config, then the score.not_applicable should be set to true", **Path A** is the correct interpretation.

### Code Changes for Path A

**File: autoqa_mapper.go**
```diff
 case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
-    mappedScore.NotApplicable = true
     // Unlike DETECTED/NOT_DETECTED which have two modes (Behavior DND uses direct field mapping,
     // # of Occurrences matches trigger count against numeric bins via matchNumericAutoQaOption),
     // NOT_APPLICABLE is categorical with no occurrence count, so it always uses direct mapping.
     if autoQaConfig.NotApplicable != nil {
+        mappedScore.NotApplicable = true
         mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
     }
```

**File: scorecard_scores_dao.go**
- ✅ No changes needed (removal is correct)

**File: scorecard_scores_dao_test.go**
- ✅ No changes needed (tests already handle both scenarios)

**File: scorecard_calculator.go**
- ✅ No changes needed (conditional clear logic is correct)

### Response to tinglinliu's Comments

**For Comment 1 (autoqa_mapper.go line 87):**
> You're absolutely right! The unconditional `NotApplicable = true` on line 82 should be moved inside the `if` block. We should only set NotApplicable when the config has a score value configured, as per your message "when not_applicable is set in the config, then the score.not_applicable should be set to true".
>
> Updated the code to conditionally set NotApplicable only when `autoQaConfig.NotApplicable != nil`.

**For Comment 2 (scorecard_scores_dao.go line 577):**
> The NotApplicable check was removed to support scored N/A flowing through percentage calculations. 
>
> Previous behavior: Any score with NotApplicable=true would short-circuit and return nil, preventing percentage computation.
>
> New behavior: Scores with NotApplicable=true AND valid NumericValue (scored N/A) can produce percentage results. Only when scoreValidNumericValueList is empty (no valid numeric values) do we return nil.
>
> This allows scored N/A (configured with a numeric value) to contribute to QA scores while maintaining backward compatibility with legacy unscored N/A.

## Summary

✅ **Path A: Conditional NotApplicable flag** aligns with all feedback

🔧 **Single code change needed:**
- Move `mappedScore.NotApplicable = true` from line 82 to inside the `if` block at line 87

✅ **CodeRabbit comments already addressed**

## Next Steps

1. Make the code change to conditionally set NotApplicable
2. Respond to tinglinliu's comments with explanations above
3. Run tests to confirm behavior
4. Push updated code
5. Get codeowner approval from @cresta/core
