# Skip N/A Options Without Score

**Created:** 2026-04-13

## The Improvement

After implementing the option lookup to determine `NotApplicable` flag, we added logic to **skip creating scores for N/A options that don't have a score configured**.

## Why This is Needed

Some scorecard templates may have N/A options that are for display purposes only and shouldn't contribute to scoring:

```json
{
  "settings": {
    "options": [
      {"label": "Yes", "value": 2},
      {"label": "No", "value": 1},
      {"label": "N/A", "value": 0, "isNA": true}  // ← No score configured!
    ],
    "scores": [
      {"value": 2, "score": 10},
      {"value": 1, "score": 0}
      // ← Note: value 0 (N/A) has NO score mapping!
    ]
  },
  "auto_qa": {
    "detected": 2,
    "not_detected": 0  // ← Maps to N/A option without score
  }
}
```

**In this case:**
- AutoQA returns NOT_DETECTED
- Maps to value 0 (N/A option)
- But value 0 has no score configured in `settings.scores`
- We should **skip creating this score** entirely

## Implementation

### Added Helper Function

```go
// doesNAOptionHaveScore checks if an N/A option (identified by numericValue) has a score configured
func doesNAOptionHaveScore(numericValue float32, criterion ScorecardTemplateCriterion) bool {
	valueScores := criterion.GetValueScores()
	if valueScores == nil || len(*valueScores) == 0 {
		// No value-to-score mapping defined, value IS the score (Simple DND)
		return true
	}

	// Check if this value has a score configured
	for _, scoreMapping := range *valueScores {
		if int(scoreMapping.Value) == int(numericValue) {
			return true
		}
	}
	return false
}
```

**What it does:**
- Takes a numeric value (criterion value) and a criterion
- Checks if that value has a score mapping in `criterion.GetValueScores()`
- Returns true if score exists, false otherwise
- For Simple DND (no ValueScores), returns true (value IS the score)

### Updated All Outcome Logic

**Pattern applied to ALL cases:**

```go
mappedScore.NumericValue = &value
mappedScore.NotApplicable = isNumericValueNAOption(value, criterion)
// NEW: If mapped to N/A option, check if it has a score configured
if mappedScore.NotApplicable && !doesNAOptionHaveScore(value, criterion) {
	continue // Skip creating score for N/A option without score
}
```

**Applied to:**
1. DETECTED with Options (# of Occurrences)
2. DETECTED without Options (Simple DND)
3. NOT_DETECTED with Options
4. NOT_DETECTED without Options
5. NOT_APPLICABLE (always Simple DND)
6. Metadata triggers:
   - StringValue
   - BooleanValue
   - NumberValue (with and without options)

## What This Achieves

### Before This Change

**Scenario:** AutoQA returns NOT_DETECTED, maps to N/A option with value 0, but no score configured

**Old behavior:**
```go
mappedScore = {
  NumericValue: 0,
  NotApplicable: true  // ← Score created!
}
```
Score gets created and later fails during score calculation when trying to map value 0 → score.

### After This Change

**New behavior:**
```go
if mappedScore.NotApplicable && !doesNAOptionHaveScore(0, criterion) {
  continue  // ← Skip! No score created
}
```
No score is created, avoiding downstream errors.

## Edge Cases Handled

### Case 1: N/A with Score (Should Create)

```json
{
  "options": [
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "scores": [
    {"value": 0, "score": 5}  // ← N/A has score!
  ]
}
```

✅ **Result:** Score created with `NotApplicable=true`, `NumericValue=0`

### Case 2: N/A without Score (Should Skip)

```json
{
  "options": [
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "scores": [
    {"value": 1, "score": 10}  // ← N/A has NO score mapping!
  ]
}
```

✅ **Result:** No score created (skipped via continue)

### Case 3: Regular Option without Score

```json
{
  "options": [
    {"label": "Yes", "value": 1},
    {"label": "No", "value": 0}
  ],
  "scores": [
    {"value": 1, "score": 10}  // ← value 0 has no score!
  ]
}
```

**Behavior:** Score IS created with `NotApplicable=false`, `NumericValue=0`
- Downstream MapScoreValue will error: "score '0' not found in value scores"
- This is correct - it's a configuration error in the template
- Not our responsibility to catch this here

**Note:** The skip logic ONLY applies when `NotApplicable=true`. Regular options are expected to have scores.

### Case 4: Simple DND (No ValueScores)

```json
{
  "options": [
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "scores": []  // ← No scores array! Value IS the score
}
```

✅ **Result:** Score created with `NotApplicable=true`, `NumericValue=0`
- `doesNAOptionHaveScore` returns true (no ValueScores means value is score)

## Test Results

All tests continue to pass:

```
--- PASS: TestAutoQAMapper (7.41s)
    --- PASS: TestAutoQAMapper/TestMapToScores (0.00s)
    --- PASS: TestAutoQAMapper/TestMapToScoresMetadataCriteria (0.00s)
    --- PASS: TestAutoQAMapper/TestMapToScoresPerMessageCriteria (0.00s)
PASS
```

## Summary

**What we check:**
1. Is this value mapped to an N/A option? (`isNumericValueNAOption`)
2. Does that N/A option have a score configured? (`doesNAOptionHaveScore`)

**What we do:**
- If N/A AND has score: Create score with `NotApplicable=true` ✅
- If N/A AND no score: Skip creating score (continue) ✅
- If regular option: Create score with `NotApplicable=false` ✅

**Prevents:**
- Creating N/A scores that would fail downstream when trying to calculate percentage scores
- Configuration errors where N/A options exist but aren't meant to be scored
