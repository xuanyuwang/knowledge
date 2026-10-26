# Complete Call Chain: MapToScores → MapScoreValue

**Created:** 2026-04-13

## The Full Call Chain

### 1. MapToScores (autoqa_mapper.go)

**Returns:** `[]*coachingpb.Score`

Each score has:
```go
type Score struct {
    CriterionId   string
    NumericValue  *float32  // ← Set to criterion value (e.g., 2)
    NotApplicable bool
    // ...
}
```

**Called from:** `action_trigger_conversation_autoscoring.go:229`
```go
dbScores := autoqa.MapToScores(autoScoredResult.Items, criterionToAutoQa)
```

---

### 2. ComputeScores (scorecard_calculator.go:20)

**Called from:** `action_trigger_conversation_autoscoring.go:231`
```go
dbScoresByCriterion, err := scoring.ComputeScores(dbScorecard, dbScores, template.TemplateStructure, true)
```

**Function signature:**
```go
func ComputeScores(
    scorecard *model.Scorecards,
    scores []*coachingpb.Score,  // ← From MapToScores
    template ScorecardTemplateStructure,
    aiScored bool,
) (map[string][]*model.Scores, error)
```

**Line 30:**
```go
dbScoresByCriterion, err = mapScoresByCriterion(scorecard, scores, template)
```

---

### 3. mapScoresByCriterion (scorecard_calculator.go:173)

**Converts proto scores → DB model scores**

```go
func mapScoresByCriterion(
    scorecard *model.Scorecards,
    scores []*coachingpb.Score,  // ← From MapToScores
    template ScorecardTemplateStructure,
) (map[string][]*model.Scores, error) {
    dbScores := make([]*model.Scores, 0)
    _, criteria := template.GetChaptersAndCriteria()
    
    // Line 180-222: For each score
    for _, score := range scores {
        var numericValue *float32
        if score.NumericValue != nil {
            numericValue = score.NumericValue  // ← Extract from proto
        }
        
        // Create DB model score
        dbScores = append(dbScores, &model.Scores{
            CriterionIdentifier: score.CriterionId,
            NumericValue: converter.ConvertToNullFloat64From32Ref(numericValue),  // ← Copy to DB
            // ...
        })
    }
    
    // Line 224: Validate
    validScores, err := GetValidScorecardScores(dbScores, template)
    
    // Line 228-246: Group by criterion
    scoreByCriterion := make(map[string][]*model.Scores)
    for _, score := range validScores {
        scoreByCriterion[score.CriterionIdentifier] = append(...)
    }
    
    return scoreByCriterion, nil
}
```

**Returns:** `map[string][]*model.Scores` where each `model.Scores` has `NumericValue` from MapToScores

---

### 4. Back in ComputeScores (scorecard_calculator.go:39)

```go
for _, criterion := range criteria {
    scores, ok := dbScoresByCriterion[criterion.GetIdentifier()]  // ← Get scores from map
    if !ok || len(scores) == 0 {
        continue
    }
    
    // Line 75: Compute percentage score for this criterion
    percentageScores, err := ComputeCriterionPercentageScore(criterion, scores)
    // ...
}
```

---

### 5. ComputeCriterionPercentageScore (scorecard_scores_dao.go:563)

```go
func ComputeCriterionPercentageScore(
    criterion ScorecardTemplateCriterionWithChapter,
    scores []*dbmodel.Scores,  // ← Has NumericValue from MapToScores
) ([]*CriterionPercentageScore, error) {
    // Line 571-576: Collect numeric values
    scoreValidNumericValueList := make([]float64, 0, len(scores))
    for _, score := range scores {
        if score.NumericValue.Valid {
            scoreValidNumericValueList = append(scoreValidNumericValueList, score.NumericValue.Float64)
        }
    }
    
    // Line 582-591: Route to appropriate compute function
    switch {
    case criterion.IsMultiSelect():
        return computeMultiSelectScore(criterion, scores)
    case criterion.IsPerMessage():
        return computePerMessageScore(criterion, scores)
    case criterion.GetAutoQA() != nil && (*criterion.GetAutoQA().Triggers)[0].Type == TriggerTypeMetadata:
        return computeOutcomeCriterionScore(criterion, scores)
    default:
        return computeScore(criterion, scores)  // ← Most common path
    }
}
```

---

### 6. computeScore (scorecard_scores_dao.go:927)

**For single-select criteria (most common)**

```go
func computeScore(criterion ScorecardTemplateCriterionWithChapter, scores []*dbmodel.Scores) ([]*CriterionPercentageScore, error) {
    score := scores[0]  // Single-select: only one score
    maxScore := GetCriterionMaxScore(criterion)
    weight := float64(criterion.GetWeight())
    
    // Line 944-952: If has value-to-score mapping
    if criterion.GetValueScores() != nil && len(*criterion.GetValueScores()) > 0 {
        percentageScore, err := mapToPercentageScore(score, criterion, maxScore)
        if err != nil {
            return nil, err
        }
        if percentageScore == nil {
            return nil, nil
        }
        return []*CriterionPercentageScore{{PercentageScore: percentageScore, Weight: weight}}, nil
    }
    
    // Line 955-959: Simple DND (no mapping)
    percentageScore, err := mapToPercentageScore(score, criterion, maxScore)
    if err != nil {
        return nil, err
    }
    return []*CriterionPercentageScore{{PercentageScore: percentageScore, Weight: weight}}, nil
}
```

---

### 7. mapToPercentageScore (scorecard_scores_dao.go:860)

```go
func mapToPercentageScore(
    score *dbmodel.Scores,  // ← Has NumericValue from MapToScores
    criterion ScorecardTemplateCriterionWithChapter,
    maxScore float64,
) (*sql.NullFloat64, error) {
    // Line 865-873: Extract numeric value from DB model
    var numericValue float64
    switch {
    case score.NumericValue.Valid:
        numericValue = score.NumericValue.Float64  // ← Extract from DB
    case score.AiValue.Valid:
        numericValue = score.AiValue.Float64
    default:
        return nil, nil
    }
    
    if maxScore <= 0 {
        return nil, status.Errorf(...)
    }
    
    // Line 879: THE LOOKUP!
    scoreValue, err := MapScoreValue(numericValue, criterion)
    if err != nil {
        return nil, err
    }
    
    // Line 887: Convert to percentage
    percentage := converter.ConvertToNullFloat64(scoreValue / maxScore)
    return &percentage, nil
}
```

---

### 8. MapScoreValue (scorecard_scores_dao.go:771) - THE ACTUAL LOOKUP!

```go
func MapScoreValue(
    numericValue float64,  // ← From MapToScores originally!
    criterion ScorecardTemplateCriterionWithChapter,
) (float64, error) {
    // Line 772-774: If no value-to-score mapping, value IS the score (Simple DND)
    if criterion.GetValueScores() == nil || len(*criterion.GetValueScores()) == 0 {
        return numericValue, nil
    }
    
    // Line 775-779: Lookup in settings.scores
    for _, score := range *criterion.GetValueScores() {
        if sharedmath.InDelta(numericValue, score.Value, Delta) {  // ← LOOKUP by value!
            return score.Score, nil  // ← Return the mapped score
        }
    }
    
    // Line 781: Not found error
    return 0, status.Errorf(codes.InvalidArgument, "score '%v' not found in value scores for criterion '%s'", numericValue, criterion.GetIdentifier())
}
```

---

## Data Flow Summary

```
MapToScores (autoqa_mapper.go)
  Sets: score.NumericValue = 2 (criterion value)
  Returns: []*coachingpb.Score
  ↓
ComputeScores (scorecard_calculator.go:20)
  Receives: scores with NumericValue = 2
  ↓
mapScoresByCriterion (scorecard_calculator.go:173)
  Line 186: numericValue = score.NumericValue (= 2)
  Line 213: dbScore.NumericValue = converter.From32Ref(numericValue) (= 2.0)
  Returns: map[string][]*model.Scores with NumericValue = 2.0
  ↓
ComputeScores (scorecard_calculator.go:75)
  Line 75: ComputeCriterionPercentageScore(criterion, scores)
  ↓
ComputeCriterionPercentageScore (scorecard_scores_dao.go:563)
  Line 574: Collects score.NumericValue.Float64 (= 2.0)
  Line 590: computeScore(criterion, scores)
  ↓
computeScore (scorecard_scores_dao.go:927)
  Line 940: score = scores[0] (has NumericValue = 2.0)
  Line 945: mapToPercentageScore(score, criterion, maxScore)
  ↓
mapToPercentageScore (scorecard_scores_dao.go:860)
  Line 868: numericValue = score.NumericValue.Float64 (= 2.0)
  Line 879: scoreValue = MapScoreValue(numericValue, criterion)
  ↓
MapScoreValue (scorecard_scores_dao.go:771) ← THE LOOKUP!
  Line 776: if numericValue == score.Value  // Check if 2.0 == 2
  Line 777: return score.Score  // Return the mapped score (e.g., 10)
```

## Validation

**NumericValue is indeed a lookup key!**

### The Journey of NumericValue = 2

1. **MapToScores:** Sets `NumericValue = 2` (criterion value)
2. **mapScoresByCriterion:** Copies to `dbScore.NumericValue = 2.0` (DB model)
3. **mapToPercentageScore:** Extracts `numericValue = 2.0` from DB
4. **MapScoreValue:** Loops through `settings.scores` looking for `score.Value == 2`
5. **Found:** Returns `score.Score = 10` (the actual score)

### Example Template

```json
{
  "settings": {
    "options": [
      {"label": "Never", "value": 0},
      {"label": "Sometimes", "value": 1},
      {"label": "Always", "value": 2}  // ← MapToScores sets NumericValue = 2
    ],
    "scores": [
      {"value": 0, "score": 0},
      {"value": 1, "score": 5},
      {"value": 2, "score": 10}  // ← MapScoreValue looks up value=2, returns score=10
    ]
  }
}
```

**Flow:**
- MapToScores: `NumericValue = 2`
- Stored in DB as: `NumericValue = 2.0`
- MapScoreValue: Looks up `value: 2` in settings.scores
- Returns: `score: 10`

## Conclusion

**Yes, NumericValue is a lookup key!**

It's used to look up the actual score in `settings.scores` (ValueScores) via MapScoreValue.

**And we can add another lookup** in mapScoresByCriterion to look up the option in `settings.options` to check if it's N/A!
