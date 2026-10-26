# Setting NotApplicable in ComputeScores Instead of MapToScores

**Created:** 2026-04-13

## The Discovery

After investigating where `MapToScores` output is used downstream, I found the perfect place to set the `NotApplicable` flag:

**`mapScoresByCriterion` in `scorecard_calculator.go`**

## The Flow

### Current Architecture

```
1. MapToScores (autoqa_mapper.go)
   Input:  AutoQA outcomes + autoQaConfig
   Output: []*coachingpb.Score with NumericValue
   ↓

2. mapScoresByCriterion (scorecard_calculator.go:173-248)
   Input:  []*coachingpb.Score + full template structure
   Action: Convert coachingpb.Score → model.Scores for DB
   Line 179: Gets criteria map from template
   Line 180-184: Iterates over scores
   Line 215: Copies NotApplicable flag
   Output: map[string][]*model.Scores
   ↓

3. GetValidScorecardScores (scorecard_scores_dao.go:594)
   Input:  []*model.Scores + template structure
   Action: Validates scores
   Output: Valid scores
```

## Why mapScoresByCriterion is the Perfect Place

### Advantages

1. **Has all required data:**
   - Full template structure (line 176)
   - Criteria map (line 179): `_, criteria := template.GetChaptersAndCriteria()`
   - Score with NumericValue (line 180)
   - Can look up: `criterion, isCriterion := criteria[score.CriterionId]` (line 181)

2. **Right architectural layer:**
   - This is where proto scores → DB model conversion happens
   - Already enriching scores with additional info (lines 205-222)
   - Before validation (GetValidScorecardScores) runs

3. **Solves the fundamental issue:**
   - Works for ALL outcomes (DETECTED, NOT_DETECTED, NOT_APPLICABLE)
   - Not hardcoded to outcome type
   - Based on actual option type lookup

### Code Location

**File:** `/Users/xuanyu.wang/repos/go-servers-na-score/shared/scoring/scorecard_calculator.go`

**Function:** `mapScoresByCriterion` (lines 173-248)

**Current code (lines 180-222):**
```go
for _, score := range scores {
    _, isCriterion := criteria[score.CriterionId]
    if !isCriterion {
        continue
    }
    var numericValue *float32
    if score.NumericValue != nil {
        numericValue = score.NumericValue
    } else if score.AiScored {
        numericValue = score.AiValue
    }

    // ... message ID handling ...

    dbScores = append(dbScores, &model.Scores{
        Customer:            scorecard.Customer,
        Profile:             scorecard.Profile,
        UsecaseID:           scorecard.UsecaseID,
        ResourceID:          dbuuid.NewString(),
        ScorecardID:         scorecard.ResourceID,
        CriterionIdentifier: score.CriterionId,
        MessageID:           messageID,
        NumericValue:        converter.ConvertToNullFloat64From32Ref(numericValue),
        TextValue:           converter.ConvertRefToNullString(score.TextValue),
        NotApplicable:       converter.ConvertToNullBool(score.NotApplicable),  // ← Line 215
        Comment:             converter.ConvertRefToNullString(score.Comment),
        CommentAccessRoles:  commentAccessRoles,
        AiScored:            converter.ConvertToNullBool(false),
        AiValue:             converter.ConvertToNullFloat64From32Ref(nil),
        AutoFailed:          converter.ConvertToNullBool(false),
    })
}
```

## Proposed Solution

### Step 1: Add Helper Function

```go
// isNumericValueNAOption checks if the given numeric value points to an N/A option
func isNumericValueNAOption(numericValue float32, criterion ScorecardTemplateCriterion) bool {
    if criterion.GetSettings() == nil || criterion.GetSettings().GetOptions() == nil {
        return false
    }
    
    for _, option := range *criterion.GetSettings().GetOptions() {
        // Match by value (criterion value, not index)
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

### Step 2: Modify mapScoresByCriterion

```go
for _, score := range scores {
    criterion, isCriterion := criteria[score.CriterionId]  // ← Get full criterion
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

    // ... message ID handling ...

    dbScores = append(dbScores, &model.Scores{
        Customer:            scorecard.Customer,
        Profile:             scorecard.Profile,
        UsecaseID:           scorecard.UsecaseID,
        ResourceID:          dbuuid.NewString(),
        ScorecardID:         scorecard.ResourceID,
        CriterionIdentifier: score.CriterionId,
        MessageID:           messageID,
        NumericValue:        converter.ConvertToNullFloat64From32Ref(numericValue),
        TextValue:           converter.ConvertRefToNullString(score.TextValue),
        NotApplicable:       converter.ConvertToNullBool(notApplicable),  // ← Use computed value
        Comment:             converter.ConvertRefToNullString(score.Comment),
        CommentAccessRoles:  commentAccessRoles,
        AiScored:            converter.ConvertToNullBool(false),
        AiValue:             converter.ConvertToNullFloat64From32Ref(nil),
        AutoFailed:          converter.ConvertToNullBool(false),
    })
}
```

## What This Solves

### ✅ Handles ALL Outcome Types

**Scenario 1: DETECTED maps to N/A option**
```json
{
  "options": [
    {"label": "Yes", "value": 2},
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "auto_qa": {
    "detected": 0  // ← Maps to N/A!
  }
}
```
- MapToScores: Sets `NumericValue = 0`
- mapScoresByCriterion: Looks up option with value=0 → finds isNA=true → sets `NotApplicable = true`

**Scenario 2: NOT_APPLICABLE maps to regular option**
```json
{
  "options": [
    {"label": "No", "value": 1},
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "auto_qa": {
    "not_applicable": 1  // ← Maps to "No"!
  }
}
```
- MapToScores: Sets `NumericValue = 1`
- mapScoresByCriterion: Looks up option with value=1 → no isNA flag → sets `NotApplicable = false`

**Scenario 3: # of Occurrences system**
```json
{
  "auto_qa": {
    "options": [
      {"numeric_from": 0, "numeric_to": 1, "value": 0},   // → N/A option
      {"numeric_from": 1, "numeric_to": 99, "value": 1}   // → Regular option
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
- MapToScores: Sets `NumericValue = 0` or `1` based on occurrence count
- mapScoresByCriterion: Looks up option → sets `NotApplicable` accordingly

## Impact on MapToScores

### Can Simplify MapToScores

With this approach, MapToScores doesn't need to worry about setting NotApplicable at all!

**Current MapToScores (can be simplified):**
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NotApplicable = true  // ← No longer needed!
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    } else {
        continue
    }
```

**Simplified MapToScores:**
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    } else {
        continue
    }
```

Same for DETECTED and NOT_DETECTED cases!

### MapToScores Responsibility

MapToScores only needs to:
1. Map outcome → numeric value (criterion value)
2. Skip if no value configured

mapScoresByCriterion takes care of:
1. Looking up the option
2. Setting NotApplicable based on option type

## Benefits of This Architecture

1. **Separation of concerns:**
   - MapToScores: Outcome → value mapping (data layer has no template knowledge)
   - mapScoresByCriterion: Value → option metadata (has full template context)

2. **Works for all code paths:**
   - AutoQA scoring
   - Manual scoring (if user sets a value, we still look it up)
   - Import/export flows

3. **Single source of truth:**
   - NotApplicable is ALWAYS determined by option lookup
   - Not dependent on how the score was created

4. **Maintainable:**
   - One place to update if logic changes
   - Clear responsibility for each function

## Next Steps

1. Implement `isNumericValueNAOption` helper function
2. Modify `mapScoresByCriterion` to use option lookup
3. Simplify `MapToScores` (remove explicit NotApplicable setting)
4. Update tests
5. Commit changes
