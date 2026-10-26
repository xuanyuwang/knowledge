# Solution B Feasibility: Pass Full Criteria to MapToScores

**Created:** 2026-04-13

## MapToScores Callers

Found **2 production callers** + 3 test calls:

### Caller 1: action_trigger_conversation_autoscoring.go

**Line 196-229:**
```go
// Line 196-201: ALREADY BUILDS BOTH MAPS!
criteriaByIdentifier := make(map[string]scoring.ScorecardTemplateCriterion)
criterionToAutoQa := make(map[string]*scoring.AutoQAConfig)
for i := range scorableItems {
    criteriaByIdentifier[scorableItems[i].GetIdentifier()] = scorableItems[i]  // ← Full criterion!
    criterionToAutoQa[scorableItems[i].GetIdentifier()] = scorableItems[i].GetAutoQA()
}

// Line 229: Currently only passes criterionToAutoQa
dbScores := autoqa.MapToScores(autoScoredResult.Items, criterionToAutoQa)

// Could easily pass both:
dbScores := autoqa.MapToScores(autoScoredResult.Items, criterionToAutoQa, criteriaByIdentifier)
```

**Data available:** ✅ YES - `criteriaByIdentifier` already exists!

---

### Caller 2: template_processor.go (backfill scorecards)

**Line 38-42 (struct fields):**
```go
type templateProcessor struct {
    // ...
    autoScoreableItems              []scoring.ScorecardTemplateCriterion
    criterionIdentifierToAutoConfig map[string]*scoring.AutoQAConfig
    criterionIdentifierToCriterion  map[string]scoring.ScorecardTemplateCriterion  // ← Full criteria!
}
```

**Line 140-145 (initialization):**
```go
criterionIdentifierToAutoConfig := make(map[string]*scoring.AutoQAConfig, len(autoScoreableItems))
criterionIdentifierToCriterion := make(map[string]scoring.ScorecardTemplateCriterion, len(autoScoreableItems))
for _, autoScoreableItem := range autoScoreableItems {
    criterionIdentifierToAutoConfig[autoScoreableItem.GetIdentifier()] = autoScoreableItem.GetAutoQA()
    criterionIdentifierToCriterion[autoScoreableItem.GetIdentifier()] = autoScoreableItem  // ← Full criterion!
}
```

**Line 317:**
```go
newScores := p.autoQA.MapToScores(autoScoredItems, p.criterionIdentifierToAutoConfig)

// Could easily pass both:
newScores := p.autoQA.MapToScores(autoScoredItems, p.criterionIdentifierToAutoConfig, p.criterionIdentifierToCriterion)
```

**Data available:** ✅ YES - `p.criterionIdentifierToCriterion` already exists!

---

## Key Finding

**BOTH callers ALREADY BUILD the full criteria map!**

They're just not passing it to MapToScores because the current signature doesn't accept it.

## Solution B Implementation

### Step 1: Update MapToScores Signature

**File:** `shared/scoring/autoqa_mapper.go`

**Current:**
```go
type AutoQAMapper interface {
    MapToScores(
        autoScoredItems []*autoqapb.AutoScoredItem,
        criterionToAutoQa map[string]*AutoQAConfig,
    ) []*coachingpb.Score
}

func (impl *autoQAImpl) MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,
    criterionToAutoQa map[string]*AutoQAConfig,
) []*coachingpb.Score
```

**New:**
```go
type AutoQAMapper interface {
    MapToScores(
        autoScoredItems []*autoqapb.AutoScoredItem,
        criterionToAutoQa map[string]*AutoQAConfig,
        criteriaByIdentifier map[string]ScorecardTemplateCriterion,  // NEW
    ) []*coachingpb.Score
}

func (impl *autoQAImpl) MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,
    criterionToAutoQa map[string]*AutoQAConfig,
    criteriaByIdentifier map[string]ScorecardTemplateCriterion,  // NEW
) []*coachingpb.Score
```

### Step 2: Update MapToScores Implementation

```go
func (impl *autoQAImpl) MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,
    criterionToAutoQa map[string]*AutoQAConfig,
    criteriaByIdentifier map[string]ScorecardTemplateCriterion,
) []*coachingpb.Score {
    mappedScores := make([]*coachingpb.Score, 0)
    
    for _, scoredItem := range autoScoredItems {
        // Get both AutoQA config and full criterion
        autoQaConfig, hasConfig := criterionToAutoQa[scoredItem.Identifier]
        if !hasConfig {
            continue
        }
        
        criterion, hasCriterion := criteriaByIdentifier[scoredItem.Identifier]
        if !hasCriterion {
            continue
        }
        
        mappedScore := &coachingpb.Score{
            CriterionId: scoredItem.Identifier,
        }
        
        // Map outcome to value
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
        
        mappedScores = append(mappedScores, mappedScore)
    }
    
    return mappedScores
}
```

### Step 3: Add Helper Function

**Location:** Could go in `autoqa_mapper.go` or `scorecard_templates.go` (shared location)

```go
func isNumericValueNAOption(numericValue float32, criterion ScorecardTemplateCriterion) bool {
    if criterion.GetSettings() == nil || criterion.GetSettings().GetOptions() == nil {
        return false
    }
    
    for _, option := range *criterion.GetSettings().GetOptions() {
        if option.Value == int(numericValue) {
            if option.IsNA != nil && *option.IsNA {
                return true
            }
            break
        }
    }
    return false
}
```

### Step 4: Update Caller 1

**File:** `action_trigger_conversation_autoscoring.go`

**Line 229:** Change from
```go
dbScores := autoqa.MapToScores(autoScoredResult.Items, criterionToAutoQa)
```

To:
```go
dbScores := autoqa.MapToScores(autoScoredResult.Items, criterionToAutoQa, criteriaByIdentifier)
```

### Step 5: Update Caller 2

**File:** `template_processor.go`

**Line 317:** Change from
```go
newScores := p.autoQA.MapToScores(autoScoredItems, p.criterionIdentifierToAutoConfig)
```

To:
```go
newScores := p.autoQA.MapToScores(autoScoredItems, p.criterionIdentifierToAutoConfig, p.criterionIdentifierToCriterion)
```

### Step 6: Update Tests

**File:** `autoqa_mapper_test.go`

All 3 test calls need to pass the additional parameter:

**Lines 87, 160, 236:** Add criteria map to test calls
```go
// Build criteriaByIdentifier from test data
criteriaByIdentifier := make(map[string]scoring.ScorecardTemplateCriterion)
for identifier, config := range criterionToAutoQa {
    // Build test criterion with settings
    criteriaByIdentifier[identifier] = buildTestCriterion(identifier, config)
}

actual := s.autoQAMapper.MapToScores(given, criterionToAutoQa, criteriaByIdentifier)
```

## Comparison with Solution A

| Aspect | Solution A (Downstream) | Solution B (Upstream) |
|--------|------------------------|----------------------|
| **Changes Required** | 1 file (scorecard_calculator.go) | 5 files (autoqa_mapper.go, 2 callers, test) |
| **API Changes** | None | Interface + implementation |
| **Test Changes** | None | All 3 tests need update |
| **Data Availability** | Already has it | Already has it (just not passing) |
| **Coupling** | Loose (separate concerns) | Tighter (MapToScores knows about Settings) |
| **When Set** | During enrichment | During creation |

## Both Solutions Are Feasible!

The key insight: **Both callers already have the full criteria map.** They just don't pass it because MapToScores doesn't accept it.

So both solutions are equally feasible from a data availability perspective!

## Recommendation

I still lean toward **Solution A** because:
1. **Simpler** - 1 file change vs 5 file changes
2. **No API changes** - safer for potential other callers we missed
3. **Better architecture** - clear separation of concerns
4. **Established pattern** - mapScoresByCriterion already enriches with other metadata

But **Solution B is equally valid** and has the advantage of setting NotApplicable earlier in the pipeline.

**Your preference?**
