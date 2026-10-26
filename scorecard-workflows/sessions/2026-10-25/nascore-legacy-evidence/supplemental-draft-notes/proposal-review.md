# Proposal Review: Setting NotApplicable Based on Option Lookup

**Created:** 2026-04-13

## The Problem We're Solving

**Current behavior:**
```go
// In MapToScores (autoqa_mapper.go:81-93)
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NotApplicable = true  // ← Always true for NOT_APPLICABLE outcome
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    }
```

**The issue:**
- NotApplicable flag is determined by **outcome type** (NOT_APPLICABLE → true)
- But ANY outcome can map to ANY option type (regular OR N/A)
- Examples:
  - DETECTED can map to N/A option (should be NotApplicable=true)
  - NOT_APPLICABLE can map to regular option (should be NotApplicable=false)

## The Proposal: Two-Part Fix

### Part 1: Add Helper Function in scorecard_calculator.go

**Location:** After `mapScoresByCriterion` function

```go
// isNumericValueNAOption checks if the given numeric value points to an N/A option
func isNumericValueNAOption(numericValue float32, criterion ScorecardTemplateCriterion) bool {
    if criterion.GetSettings() == nil || criterion.GetSettings().GetOptions() == nil {
        return false
    }
    
    // Loop through options to find matching value
    for _, option := range *criterion.GetSettings().GetOptions() {
        // Match by criterion value (same way MapScoreValue matches in settings.scores)
        if option.Value == int(numericValue) {
            // Check if this option is marked as N/A
            if option.IsNA != nil && *option.IsNA {
                return true
            }
            break  // Found matching value, no need to continue
        }
    }
    return false
}
```

**Parallels MapScoreValue:**
- MapScoreValue: Looks up in `settings.scores` (ValueScores) to map value → score
- isNumericValueNAOption: Looks up in `settings.options` (Options) to check value → isNA

### Part 2: Modify mapScoresByCriterion

**Location:** scorecard_calculator.go:180-222

**Current code:**
```go
for _, score := range scores {
    _, isCriterion := criteria[score.CriterionId]  // Line 181
    if !isCriterion {
        continue
    }
    var numericValue *float32
    if score.NumericValue != nil {
        numericValue = score.NumericValue
    } else if score.AiScored {
        numericValue = score.AiValue
    }

    // ... messageID handling ...

    dbScores = append(dbScores, &model.Scores{
        // ...
        NumericValue:  converter.ConvertToNullFloat64From32Ref(numericValue),
        NotApplicable: converter.ConvertToNullBool(score.NotApplicable),  // ← Line 215: Just copying
        // ...
    })
}
```

**Modified code:**
```go
for _, score := range scores {
    criterion, isCriterion := criteria[score.CriterionId]  // ← Change: Get full criterion
    if !isCriterion {
        continue
    }
    var numericValue *float32
    if score.NumericValue != nil {
        numericValue = score.NumericValue
    } else if score.AiScored {
        numericValue = score.AiValue
    }

    // NEW: Determine NotApplicable based on option lookup
    var notApplicable bool
    if numericValue != nil {
        // Look up the option and check if it's N/A
        notApplicable = isNumericValueNAOption(*numericValue, criterion)
    } else {
        // No numeric value, fallback to what was set in the proto
        notApplicable = score.NotApplicable
    }

    // ... messageID handling ...

    dbScores = append(dbScores, &model.Scores{
        // ...
        NumericValue:  converter.ConvertToNullFloat64From32Ref(numericValue),
        NotApplicable: converter.ConvertToNullBool(notApplicable),  // ← Use computed value
        // ...
    })
}
```

**Key changes:**
1. Line 181: Change from `_` to `criterion` to get full criterion object
2. Add logic to compute `notApplicable` by calling `isNumericValueNAOption`
3. Line 215: Use computed `notApplicable` instead of copying from proto

### Part 3 (Optional): Simplify MapToScores

**Current MapToScores:**
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NotApplicable = true  // ← No longer needed
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    }
```

**Simplified MapToScores:**
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
        // NotApplicable will be set by mapScoresByCriterion based on option lookup
    }
```

**Note:** This is optional. We can leave MapToScores as-is since mapScoresByCriterion will override it anyway.

## What This Achieves

### ✅ Fixes All Scenarios

**Scenario 1: DETECTED → N/A option**
```json
{
  "options": [
    {"label": "Yes", "value": 2},
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "auto_qa": {"detected": 0}
}
```
- MapToScores: Sets `NumericValue = 0`
- mapScoresByCriterion: Looks up option with value=0 → finds `isNA=true` → sets `NotApplicable = true` ✅

**Scenario 2: NOT_APPLICABLE → regular option**
```json
{
  "options": [
    {"label": "No", "value": 1},
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "auto_qa": {"not_applicable": 1}
}
```
- MapToScores: Sets `NumericValue = 1`
- mapScoresByCriterion: Looks up option with value=1 → no `isNA` → sets `NotApplicable = false` ✅

**Scenario 3: # of Occurrences**
```json
{
  "auto_qa": {
    "options": [
      {"numeric_from": 0, "numeric_to": 1, "value": 0},
      {"numeric_from": 1, "numeric_to": 99, "value": 1}
    ]
  },
  "settings": {
    "options": [
      {"label": "N/A", "value": 0, "isNA": true},
      {"label": "Found", "value": 1}
    ]
  }
}
```
- MapToScores: Sets `NumericValue = 0` or `1` based on count
- mapScoresByCriterion: Looks up option → sets `NotApplicable` correctly ✅

### ✅ Works for All Code Paths

- AutoQA scoring ✅
- Manual scoring ✅
- Import/export ✅
- Any flow that goes through mapScoresByCriterion ✅

### ✅ Single Source of Truth

NotApplicable is ALWAYS determined by:
1. Looking up the option using NumericValue
2. Checking if that option has `isNA=true`

Not dependent on:
- Outcome type
- How the score was created
- Which code path set the score

## Architecture Benefits

### Separation of Concerns

**MapToScores:**
- Responsibility: Map AutoQA outcome → criterion value
- Has: AutoQA config
- Missing: Full criterion settings
- Sets: NumericValue only

**mapScoresByCriterion:**
- Responsibility: Convert proto → DB model, enrich with metadata
- Has: Full template structure with criteria
- Can: Look up options and scores
- Sets: NotApplicable based on option lookup

### Parallel with MapScoreValue

**MapScoreValue (existing):**
- Uses: NumericValue as lookup key
- Looks up in: `settings.scores` (ValueScores)
- Returns: Actual score value
- Called from: mapToPercentageScore (later in pipeline)

**isNumericValueNAOption (new):**
- Uses: NumericValue as lookup key
- Looks up in: `settings.options` (Options)
- Returns: Whether option is N/A
- Called from: mapScoresByCriterion (earlier in pipeline)

Both use NumericValue as a lookup key, just in different tables!

## Testing Considerations

### Test Cases to Add/Update

1. **DETECTED maps to N/A option:**
   - Create template with `auto_qa.detected = 0`, `options[0] = {value: 0, isNA: true}`
   - Expect: `NotApplicable = true`

2. **NOT_APPLICABLE maps to regular option:**
   - Create template with `auto_qa.not_applicable = 1`, `options[1] = {value: 1}`
   - Expect: `NotApplicable = false`

3. **# of Occurrences with N/A:**
   - Create template with occurrence bins, one mapping to N/A option
   - Expect: NotApplicable based on which bin matched

4. **No options defined:**
   - Template with no `settings.options`
   - Expect: `NotApplicable = false` (helper returns false)

5. **No numeric value:**
   - Score with no NumericValue (text-based criterion)
   - Expect: Fall back to `score.NotApplicable` from proto

### Existing Tests to Update

- `TestMapToScores`: May need to update if we change MapToScores behavior
- Any tests that check NotApplicable flag on scores

## Implementation Steps

1. Add `isNumericValueNAOption` helper function to scorecard_calculator.go
2. Modify `mapScoresByCriterion` to use option lookup
3. (Optional) Simplify MapToScores to remove NotApplicable setting
4. Update tests
5. Test scenarios manually
6. Commit changes

## Questions for Review

1. **Do we want to simplify MapToScores** (Part 3) or leave it as-is for backward compatibility?
2. **Should we add validation** in `isNumericValueNAOption` to error if option not found, or silently return false?
3. **Test coverage:** Do we need integration tests or just unit tests?
4. **Do we need to handle the case** where `settings.options` doesn't exist (old templates)?
