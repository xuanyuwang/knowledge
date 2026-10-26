# Alternative Solutions: Where to Set NotApplicable Flag

**Created:** 2026-04-13

## The Two Approaches

### Solution A: Set in mapScoresByCriterion (Downstream)
- **Where:** scorecard_calculator.go (mapScoresByCriterion)
- **When:** During proto → DB model conversion
- **Data available:** Full template structure passed to ComputeScores

### Solution B: Set in MapToScores (Upstream)
- **Where:** autoqa_mapper.go (MapToScores)
- **When:** During AutoQA outcome → score conversion
- **Data needed:** Pass full criterion data to MapToScores

## Solution A: Downstream (Current Proposal)

### Current Signature
```go
func MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,
    criterionToAutoQa map[string]*AutoQAConfig,  // Only has AutoQA config
) []*coachingpb.Score
```

**No changes to MapToScores signature or caller.**

### Implementation
Add lookup in `mapScoresByCriterion`:
```go
criterion, _ := criteria[score.CriterionId]
notApplicable := isNumericValueNAOption(*numericValue, criterion)
```

### Pros
✅ No API changes - MapToScores signature stays the same
✅ All callers unaffected
✅ Clean separation: MapToScores = data mapping, mapScoresByCriterion = metadata enrichment
✅ Works for all code paths (AutoQA, manual, imports)
✅ Already have the data in mapScoresByCriterion

### Cons
❌ NotApplicable set later in pipeline (but this is actually fine - it's enrichment)
❌ Two steps: MapToScores sets value, mapScoresByCriterion sets flag

---

## Solution B: Upstream (Pass More Data)

### New Signature Option 1: Pass Full Criteria Map
```go
func MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,
    criterionToAutoQa map[string]*AutoQAConfig,
    criteriaByIdentifier map[string]ScorecardTemplateCriterion,  // NEW
) []*coachingpb.Score
```

### Data Available at Call Site

Looking at `action_trigger_conversation_autoscoring.go:196-229`:

```go
// Line 196-201: Build BOTH maps from scorableItems
criteriaByIdentifier := make(map[string]scoring.ScorecardTemplateCriterion)
criterionToAutoQa := make(map[string]*scoring.AutoQAConfig)
for i := range scorableItems {
    criteriaByIdentifier[scorableItems[i].GetIdentifier()] = scorableItems[i]
    criterionToAutoQa[scorableItems[i].GetIdentifier()] = scorableItems[i].GetAutoQA()
}

// Line 229: Currently only pass criterionToAutoQa
dbScores := autoqa.MapToScores(autoScoredResult.Items, criterionToAutoQa)

// Could pass both:
dbScores := autoqa.MapToScores(autoScoredResult.Items, criterionToAutoQa, criteriaByIdentifier)
```

**The data IS available!** We already have `criteriaByIdentifier` built, just not passing it.

### Implementation in MapToScores
```go
func MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,
    criterionToAutoQa map[string]*AutoQAConfig,
    criteriaByIdentifier map[string]ScorecardTemplateCriterion,  // NEW
) []*coachingpb.Score {
    // ...
    
    for _, scoredItem := range autoScoredItems {
        criterion, hasCriterion := criteriaByIdentifier[scoredItem.Identifier]  // NEW: Get criterion
        if !hasCriterion {
            continue
        }
        
        autoQaConfig, hasConfig := criterionToAutoQa[scoredItem.Identifier]
        if !hasConfig {
            continue
        }
        
        // ...
        
        var mappedValue *int
        switch scoredItem.Outcome {
        case autoqapb.AutoScoreOutcome_DETECTED:
            mappedValue = autoQaConfig.Detected
        case autoqapb.AutoScoreOutcome_NOT_DETECTED:
            mappedValue = autoQaConfig.NotDetected
        case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
            mappedValue = autoQaConfig.NotApplicable
        }
        
        if mappedValue == nil {
            continue
        }
        
        mappedScore.NumericValue = nilOrFloat32(mappedValue)
        
        // NEW: Set NotApplicable based on option lookup
        mappedScore.NotApplicable = isNumericValueNAOption(float32(*mappedValue), criterion)
    }
}
```

### Pros
✅ NotApplicable set immediately when score created
✅ All logic in one place (MapToScores)
✅ Easier to understand flow (one function does everything)
✅ No need to override later

### Cons
❌ API change - need to update signature
❌ Need to update caller (action_trigger_conversation_autoscoring.go)
❌ Need to check if MapToScores is called from other places
❌ Helper function `isNumericValueNAOption` needs to be accessible to autoqa_mapper.go
❌ Tight coupling: MapToScores now needs full criterion structure

---

## Comparison

| Aspect | Solution A (Downstream) | Solution B (Upstream) |
|--------|------------------------|----------------------|
| **API Changes** | None | Change MapToScores signature |
| **Caller Changes** | None | Update action_trigger_conversation_autoscoring.go |
| **Data Availability** | Already has it | Need to pass it |
| **Helper Function Location** | scorecard_calculator.go | Need to move to shared location |
| **When Flag Set** | During DB conversion | During score creation |
| **Separation of Concerns** | Better (mapping vs enrichment) | Worse (all in one place) |
| **Code Paths Affected** | All (AutoQA, manual, import) | Only AutoQA |
| **Backward Compatibility** | Perfect | Need to check other callers |

## Other Callers of MapToScores?

Need to check if MapToScores is called from anywhere else besides `action_trigger_conversation_autoscoring.go`:

```bash
# Search for MapToScores calls
grep -r "MapToScores(" --include="*.go"
```

If it's only called from one place, Solution B is easier.
If called from multiple places, Solution A is safer.

## Recommendation

**I recommend Solution A (Downstream in mapScoresByCriterion)** because:

1. **No API changes** - cleaner, safer
2. **Better architecture** - clear separation:
   - MapToScores: AutoQA outcome → score value (pure data mapping)
   - mapScoresByCriterion: Enrich with metadata (NotApplicable, chapters, etc.)
3. **Works for ALL code paths** - not just AutoQA
4. **Already established pattern** - mapScoresByCriterion already enriches scores with:
   - Chapter info (line 49)
   - AiScored flag (line 54)
   - AiValue (line 56)
   - AutoFailed (line 67)
   
   Adding NotApplicable enrichment fits this pattern perfectly!

5. **Parallel to MapScoreValue** - Both use NumericValue as lookup key:
   - MapScoreValue: value → score (in settings.scores)
   - isNumericValueNAOption: value → isNA (in settings.options)

## But Solution B is Also Valid

If you prefer to set NotApplicable early, Solution B is perfectly valid because:
- Data is available at call site
- Simple to pass additional parameter
- All logic in one place

**The choice is architectural preference:**
- Solution A: Separation of concerns (mapping vs enrichment)
- Solution B: All-in-one (set everything when creating score)

## Let's Check Other Callers

Before deciding, we should check if MapToScores is called from other places.
