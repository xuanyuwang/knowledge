# Data Flow: MapToScores → ComputeScores → mapScoresByCriterion

**Created:** 2026-04-13

## The Complete Flow

### Step 1: MapToScores Returns Scores

**File:** `shared/scoring/autoqa_mapper.go`

**Returns:** `[]*coachingpb.Score`

**Each score contains:**
```go
type Score struct {
    CriterionId    string    // "greeting_detection"
    NumericValue   *float32  // 2 (criterion value/lookup key)
    NotApplicable  bool      // Currently set in MapToScores
    TextValue      *string
    Comment        *string
    MessageName    *string
    // ... other fields
}
```

### Step 2: ComputeScores Receives Scores

**File:** `shared/scoring/scorecard_calculator.go`

**Line 20-28:**
```go
func ComputeScores(
    scorecard *model.Scorecards,
    scores []*coachingpb.Score,  // ← MapToScores output
    template ScorecardTemplateStructure,
    aiScored bool,
) (
    dbScoresByCriterion map[string][]*model.Scores,
    err error,
) {
```

**Line 30:**
```go
dbScoresByCriterion, err = mapScoresByCriterion(scorecard, scores, template)
```

Immediately passes the scores array to mapScoresByCriterion.

### Step 3: mapScoresByCriterion Processes Each Score

**File:** `shared/scoring/scorecard_calculator.go`

**Line 173-227:**
```go
func mapScoresByCriterion(
    scorecard *model.Scorecards,
    scores []*coachingpb.Score,  // ← From MapToScores
    template ScorecardTemplateStructure,  // ← Has full criterion data
) (scoreByCriterion map[string][]*model.Scores, err error) {
    dbScores := make([]*model.Scores, 0)
    _, criteria := template.GetChaptersAndCriteria()  // Line 179: Get criteria map
    
    for _, score := range scores {  // Line 180: Iterate over each score from MapToScores
        // Line 181: Look up the criterion for this score
        _, isCriterion := criteria[score.CriterionId]
        if !isCriterion {
            continue
        }
        
        // Line 186-190: Extract NumericValue from the score
        var numericValue *float32
        if score.NumericValue != nil {
            numericValue = score.NumericValue  // ← This is from MapToScores!
        } else if score.AiScored {
            numericValue = score.AiValue
        }

        // ... message handling ...

        // Line 205-222: Convert proto score → DB model score
        dbScores = append(dbScores, &model.Scores{
            Customer:            scorecard.Customer,
            Profile:             scorecard.Profile,
            UsecaseID:           scorecard.UsecaseID,
            ResourceID:          dbuuid.NewString(),
            ScorecardID:         scorecard.ResourceID,
            CriterionIdentifier: score.CriterionId,  // ← From MapToScores
            MessageID:           messageID,
            NumericValue:        converter.ConvertToNullFloat64From32Ref(numericValue),  // ← From MapToScores
            TextValue:           converter.ConvertRefToNullString(score.TextValue),
            NotApplicable:       converter.ConvertToNullBool(score.NotApplicable),  // ← Line 215: Currently just copying!
            Comment:             converter.ConvertRefToNullString(score.Comment),
            CommentAccessRoles:  commentAccessRoles,
            AiScored:            converter.ConvertToNullBool(false),
            AiValue:             converter.ConvertToNullFloat64From32Ref(nil),
            AutoFailed:          converter.ConvertToNullBool(false),
        })
    }
    // Line 224: Validate scores
    validScores, err := GetValidScorecardScores(dbScores, template)
    // ...
}
```

## The Key Insight

**At line 215, we're currently just copying `score.NotApplicable` from MapToScores output!**

But look at what we have available at that point:

1. **Line 179:** `criteria` map with full criterion data including Settings.Options
2. **Line 181:** Can look up the criterion: `criteria[score.CriterionId]`
3. **Line 186-190:** Already extracted `numericValue` from the score
4. **Line 215:** About to set NotApplicable

**We have everything needed to:**
- Look up the criterion
- Look up the option using numericValue
- Check if that option has isNA=true
- Set NotApplicable based on the lookup instead of blindly copying

## Proposed Modification

**Before line 205 (where we create the DB score), add:**

```go
// Line 181: We already checked isCriterion, now get the actual criterion
criterion, _ := criteria[score.CriterionId]  // Safe because we checked isCriterion above

// Determine NotApplicable based on option lookup
var notApplicable bool
if numericValue != nil {
    // Look up the option and check if it's N/A
    notApplicable = isNumericValueNAOption(*numericValue, criterion)
} else {
    // No numeric value, fallback to what was set in the proto
    notApplicable = score.NotApplicable
}
```

**Then at line 215, use the computed value:**

```go
NotApplicable: converter.ConvertToNullBool(notApplicable),  // ← Use lookup result
```

## Why This Works Perfectly

### Data Flow Summary

```
MapToScores
  ↓ Returns: []*coachingpb.Score with NumericValue
  ↓
ComputeScores (line 22: receives scores)
  ↓ Line 30: passes to mapScoresByCriterion
  ↓
mapScoresByCriterion
  Line 179: Gets criteria map from template
  Line 180: Iterates over each score
  Line 181: Looks up criterion by score.CriterionId
  Line 186: Extracts score.NumericValue
  Line 215: Can now look up option and set NotApplicable correctly!
```

### What We're Using

From MapToScores output (the `score` object):
- `score.CriterionId` - to look up criterion
- `score.NumericValue` - to look up option

From template (available in mapScoresByCriterion):
- `criteria[score.CriterionId]` - the full criterion
- `criterion.Settings.Options` - to check if value points to N/A option

### Perfect Separation of Concerns

**MapToScores:**
- Maps AutoQA outcome → criterion value
- Doesn't need to know about option types
- Just sets NumericValue

**mapScoresByCriterion:**
- Has full template context
- Can look up options
- Sets NotApplicable based on option metadata

## Conclusion

The answer to "how mapScoresByCriterion is using the return of MapToScores":

**It iterates over each score in the returned array, and for each score:**
1. Uses `score.CriterionId` to look up the criterion in the template
2. Uses `score.NumericValue` as the criterion value
3. Currently just copies `score.NotApplicable` to the DB model

**We can enhance step 3 to:**
- Look up the option using `score.NumericValue`
- Check if that option is N/A
- Set `NotApplicable` based on actual option metadata instead of blindly copying
