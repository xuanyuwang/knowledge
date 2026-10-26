# NumericValue Usage: Correction

**Created:** 2026-04-13

## The Question

User asked: "From the usage of score.NumericValue, it's not a lookup key?"

**Answer: You're RIGHT to question this!**

## What Actually Happens in mapScoresByCriterion

Looking at the code (scorecard_calculator.go:180-222):

```go
for _, score := range scores {
    _, isCriterion := criteria[score.CriterionId]
    if !isCriterion {
        continue
    }
    var numericValue *float32
    if score.NumericValue != nil {
        numericValue = score.NumericValue  // ← Extract from proto
    } else if score.AiScored {
        numericValue = score.AiValue
    }

    dbScores = append(dbScores, &model.Scores{
        // ...
        NumericValue: converter.ConvertToNullFloat64From32Ref(numericValue),  // ← Just copy!
        // ...
    })
}
```

**It's NOT doing any lookup here! It's just copying the value.**

## Where the ACTUAL Lookup Happens

The lookup happens MUCH LATER in the scoring pipeline:

### Step 1: mapToPercentageScore (scorecard_scores_dao.go:860)

```go
func mapToPercentageScore(
    score *dbmodel.Scores,  // ← Has NumericValue from DB
    criterion ScorecardTemplateCriterionWithChapter,
    maxScore float64,
) (*sql.NullFloat64, error) {
    var numericValue float64
    if score.NumericValue.Valid {
        numericValue = score.NumericValue.Float64  // ← Extract from DB
    }
    
    // Line 879: Call MapScoreValue to do the lookup!
    scoreValue, err := MapScoreValue(numericValue, criterion)
    // ...
}
```

### Step 2: MapScoreValue (scorecard_scores_dao.go:771)

```go
func MapScoreValue(numericValue float64, criterion ...) (float64, error) {
    if criterion.GetValueScores() == nil {
        return numericValue, nil  // Simple DND: value IS the score
    }
    
    // ← HERE is the actual lookup!
    for _, score := range *criterion.GetValueScores() {
        if numericValue == score.Value {  // Match by value
            return score.Score, nil  // Return the mapped score
        }
    }
    
    return 0, status.Errorf(...)  // Not found error
}
```

## What This Means

### NumericValue is stored, not used for lookup in mapScoresByCriterion

**In mapScoresByCriterion:**
- We just copy `score.NumericValue` from proto → DB model
- No lookup happens
- It's just data transfer

**The lookup happens later:**
- In `mapToPercentageScore` (during score calculation)
- Calls `MapScoreValue` which loops through `criterion.GetValueScores()`
- Matches `numericValue == score.Value`
- Returns `score.Score`

### But We CAN Still Do a Lookup for Options!

Even though mapScoresByCriterion doesn't currently do any lookup, **we can add one** because we have:

1. **The numeric value:** `numericValue` (extracted from score)
2. **The criterion:** `criterion, _ := criteria[score.CriterionId]`
3. **The options:** `criterion.GetSettings().GetOptions()`

We can add a lookup to check if the option is N/A:

```go
// In mapScoresByCriterion, after extracting numericValue
criterion, _ := criteria[score.CriterionId]

var notApplicable bool
if numericValue != nil {
    // NEW: Look up the option to check if it's N/A
    // This is similar to how MapScoreValue looks up in ValueScores
    // but we're looking up in Options instead
    notApplicable = isNumericValueNAOption(*numericValue, criterion)
} else {
    notApplicable = score.NotApplicable
}
```

## Two Different Lookups

There are TWO different lookup tables in the template:

### 1. settings.scores (ValueScores) - For score mapping
```json
{
  "settings": {
    "scores": [
      {"value": 0, "score": 0},   // NumericValue=0 → score 0
      {"value": 1, "score": 5},   // NumericValue=1 → score 5
      {"value": 2, "score": 10}   // NumericValue=2 → score 10
    ]
  }
}
```
**Used in:** `MapScoreValue` (line 771) - maps value → score
**When:** During percentage score calculation

### 2. settings.options (Options) - For option metadata
```json
{
  "settings": {
    "options": [
      {"label": "Never", "value": 0},
      {"label": "Sometimes", "value": 1},
      {"label": "N/A", "value": 2, "isNA": true}  // ← Has isNA flag!
    ]
  }
}
```
**Used in:** Currently NOT used in backend scoring!
**Could use in:** mapScoresByCriterion to check if option is N/A

## Clarification of My Previous Understanding

### What I Said Before (PARTIALLY WRONG)

I said: "NumericValue is a lookup key used in mapScoresByCriterion"

**Correction:** 
- NumericValue IS a lookup key
- But NOT used for lookup in mapScoresByCriterion (just copied)
- The lookup happens LATER in MapScoreValue

### What IS Correct

- NumericValue contains a criterion value (not index, not direct score)
- This value CAN be used to look up in BOTH:
  - `settings.scores` (ValueScores) → to get the score (happens in MapScoreValue)
  - `settings.options` (Options) → to get option metadata like isNA (doesn't happen, but we can add it!)

## So Can We Still Use It?

**YES!** Even though mapScoresByCriterion doesn't currently do lookups, we can add one:

```go
func isNumericValueNAOption(numericValue float32, criterion ScorecardTemplateCriterion) bool {
    if criterion.GetSettings() == nil || criterion.GetSettings().GetOptions() == nil {
        return false
    }
    
    // Look up in Options (similar to how MapScoreValue looks up in ValueScores)
    for _, option := range *criterion.GetSettings().GetOptions() {
        if option.Value == int(numericValue) {  // Match by value
            if option.IsNA != nil && *option.IsNA {
                return true  // This option is marked as N/A
            }
            break
        }
    }
    return false
}
```

Use it in mapScoresByCriterion:
```go
criterion, _ := criteria[score.CriterionId]
notApplicable := false
if numericValue != nil {
    notApplicable = isNumericValueNAOption(*numericValue, criterion)
}
```

## Summary

**User's observation was correct:**
- mapScoresByCriterion is NOT currently using NumericValue for lookup
- It's just copying the value from proto → DB

**But my proposal is still valid:**
- We can ADD a lookup in mapScoresByCriterion
- Similar to how MapScoreValue looks up in ValueScores
- But we look up in Options to check isNA flag
- This would set NotApplicable based on actual option metadata
