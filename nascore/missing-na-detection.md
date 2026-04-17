# Critical Gap: N/A Option Can Be Mapped from Any Outcome

**Created:** 2026-04-13

## The Problem

**Current assumption (WRONG):**
- DETECTED → always maps to regular option
- NOT_DETECTED → always maps to regular option  
- NOT_APPLICABLE → always maps to N/A option

**Reality (CORRECT):**
- DETECTED → can map to N/A option
- NOT_DETECTED → can map to N/A option
- NOT_APPLICABLE → can map to regular option

## Example Scenario

```json
{
  "settings": {
    "options": [
      {"label": "Yes", "value": 0},
      {"label": "No", "value": 1},
      {"label": "N/A", "value": 2, "isNA": true}
    ],
    "scores": [
      {"value": 0, "score": 10},
      {"value": 1, "score": 0},
      {"value": 2, "score": 5}  // N/A has score!
    ]
  },
  "auto_qa": {
    "detected": 2,        // ← Maps to N/A option!
    "not_detected": 1,    // ← Maps to "No" 
    "not_applicable": 0   // ← Maps to "Yes" option!
  }
}
```

### What Should Happen

**When AutoQA returns DETECTED:**
1. Get mapped value: `detected = 2`
2. Check if value 2 is N/A option: **YES** (isNA=true)
3. Check if N/A has score: **YES** (score=5)
4. Create: `{NotApplicable: true, NumericValue: 2}`

**When AutoQA returns NOT_APPLICABLE:**
1. Get mapped value: `not_applicable = 0`
2. Check if value 0 is N/A option: **NO** (regular option "Yes")
3. Create: `{NotApplicable: false, NumericValue: 0}`

### What Current Code Does (WRONG)

**When AutoQA returns DETECTED:**
```go
case autoqapb.AutoScoreOutcome_DETECTED:
    mappedScore.NotApplicable = false  // ← WRONG! Value 2 is N/A
    mappedScore.NumericValue = 2
```

**When AutoQA returns NOT_APPLICABLE:**
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    mappedScore.NotApplicable = true  // ← WRONG! Value 0 is "Yes"
    mappedScore.NumericValue = 0
```

## Proposed Solution

### Unified Logic for All Outcomes

```go
func mapOutcomeToScore(
    outcome autoqapb.AutoScoreOutcome,
    autoQaConfig *AutoQAConfig,
    criterion ScorecardTemplateCriterion,  // Need full criterion to check options
) (*coachingpb.Score, bool) {
    
    // Step 1: Get mapped value from config
    var mappedValue *int
    switch outcome {
    case autoqapb.AutoScoreOutcome_DETECTED:
        mappedValue = autoQaConfig.Detected
    case autoqapb.AutoScoreOutcome_NOT_DETECTED:
        mappedValue = autoQaConfig.NotDetected
    case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
        mappedValue = autoQaConfig.NotApplicable
    default:
        return nil, false  // Unknown outcome
    }
    
    // Step 2: If no value configured, skip score
    if mappedValue == nil {
        return nil, false  // Skip this score
    }
    
    // Step 3: Check if mapped value is an N/A option
    isNAOption := checkIfOptionIsNA(criterion, *mappedValue)
    
    // Step 4: Check if N/A has score configured
    if isNAOption {
        hasScore := checkIfNAHasScore(criterion, *mappedValue)
        if !hasScore {
            return nil, false  // N/A without score, skip
        }
    }
    
    // Step 5: Create score
    mappedScore := &coachingpb.Score{
        NumericValue: float32(*mappedValue),
        NotApplicable: isNAOption,  // Set based on option type, not outcome
    }
    
    return mappedScore, true
}
```

### Helper Functions Needed

```go
func checkIfOptionIsNA(criterion ScorecardTemplateCriterion, value int) bool {
    // Check if the option at this value has isNA=true
    options := criterion.GetSettings().GetOptions()
    for _, opt := range options {
        if opt.Value == value && opt.IsNA {
            return true
        }
    }
    return false
}

func checkIfNAHasScore(criterion ScorecardTemplateCriterion, value int) bool {
    // Check if this value has a score configured in settings.scores
    scores := criterion.GetSettings().GetScores()
    for _, scoreMapping := range scores {
        if scoreMapping.Value == value {
            return scoreMapping.Score != nil  // Has score configured
        }
    }
    return false
}
```

## Current Code Structure Problem

**MapToScores signature:**
```go
func MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,
    criterionToAutoQa map[string]*AutoQAConfig,  // ← Only has AutoQAConfig
) []*coachingpb.Score
```

**We need:**
```go
func MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,
    criterionToAutoQa map[string]*AutoQAConfig,
    criterionToSettings map[string]*CriterionSettings,  // ← NEW: Full settings
) []*coachingpb.Score
```

Or enhance AutoQAConfig:
```go
type AutoQAConfig struct {
    Triggers      *[]AutoQATrigger
    Options       *[]AutoQAOptions
    Detected      *int
    NotDetected   *int
    NotApplicable *int
    
    // NEW: Metadata about mapped values
    DetectedIsNA      bool  // Is detected value an N/A option?
    NotDetectedIsNA   bool  // Is not_detected value an N/A option?
    NotApplicableIsNA bool  // Is not_applicable value an N/A option?
}
```

## Impact Analysis

### Templates That Could Break Current Code

**Scenario 1: DETECTED maps to N/A**
- AutoQA returns DETECTED
- Template has `detected: 2` where value 2 is N/A option with score
- Current code: NotApplicable=false (wrong)
- Should be: NotApplicable=true

**Scenario 2: NOT_APPLICABLE maps to regular option**
- AutoQA returns NOT_APPLICABLE  
- Template has `not_applicable: 0` where value 0 is "No" option
- Current code: NotApplicable=true (wrong)
- Should be: NotApplicable=false

**Scenario 3: DETECTED maps to N/A without score**
- AutoQA returns DETECTED
- Template has `detected: 2` where value 2 is N/A option WITHOUT score
- Current code: NotApplicable=false, NumericValue=2 (wrong)
- Should be: Skip score creation (or NotApplicable=true without value)

## Recommended Fix

### Option A: Enhanced AutoQAConfig (Simpler)

When building `criterionToAutoQa` map, also check if each value is an N/A option:

```go
autoQAConfig := &AutoQAConfig{
    Detected: criterion.AutoQA.Detected,
    DetectedIsNA: isNAOption(criterion.Settings.Options, criterion.AutoQA.Detected),
    // ... same for NotDetected and NotApplicable
}
```

Then in MapToScores:
```go
case autoqapb.AutoScoreOutcome_DETECTED:
    if autoQaConfig.DetectedIsNA {
        mappedScore.NotApplicable = true
    } else {
        mappedScore.NotApplicable = false
    }
    mappedScore.NumericValue = autoQaConfig.Detected
```

### Option B: Pass Full Criterion (More flexible)

Change MapToScores to accept full criterion data and do the lookup there.

## Decision Needed

Which approach should we take:
1. **Option A:** Add isNA flags to AutoQAConfig
2. **Option B:** Pass full criterion settings to MapToScores
3. **Option C:** Leave as-is and document that users shouldn't map outcomes to wrong option types

**Recommendation:** Option A is the least disruptive and most aligned with current architecture.
