# Value vs Score: Understanding the Confusion

**Created:** 2026-04-13

## The Question

User saw: `mappedScore.NumericValue = &option.Value`

**Question:** Is `option.Value` the score?

## The Answer: It Depends on the Criterion Type!

There are **TWO different systems** in MapToScores:

### System 1: Simple Detected/Not Detected (Behavior DND)

**Template config:**
```json
{
  "auto_qa": {
    "detected": 5,
    "not_detected": 0
  }
}
```

**Code path:**
```go
if autoQaConfig.Options != nil && len(*autoQaConfig.Options) > 0 {
    // Not this path
} else {
    // THIS PATH - Simple DND
    mappedScore.NumericValue = nilOrFloat32(autoQaConfig.Detected)  // 5
}
```

**Result:** `NumericValue = 5` (direct score value)

### System 2: # of Occurrences with Bins

**Template config:**
```json
{
  "auto_qa": {
    "options": [
      {"numeric_from": 0, "numeric_to": 1, "value": 0},  // 0 occurrences → option index 0
      {"numeric_from": 1, "numeric_to": 3, "value": 1},  // 1-2 occurrences → option index 1
      {"numeric_from": 3, "numeric_to": 99, "value": 2}  // 3+ occurrences → option index 2
    ]
  },
  "settings": {
    "options": [
      {"label": "Never", "value": 0},
      {"label": "Sometimes", "value": 1},
      {"label": "Always", "value": 2}
    ],
    "scores": [
      {"value": 0, "score": 0},
      {"value": 1, "score": 5},
      {"value": 2, "score": 10}
    ]
  }
}
```

**Code path:**
```go
if autoQaConfig.Options != nil && len(*autoQaConfig.Options) > 0 {
    // THIS PATH - # of Occurrences
    option := matchNumericAutoQaOption(autoQaConfig, float32(len(scoredItem.Evidences)))
    mappedScore.NumericValue = &option.Value  // 0, 1, or 2 (OPTION INDEX, not score!)
}
```

**Result:** `NumericValue = 1` (option index, NOT the score 5!)

## The Key Distinction

### In Simple DND:
- `NumericValue` = **actual score** (5, 0, etc.)
- No `settings.scores` mapping needed

### In # of Occurrences:
- `NumericValue` = **option index** (0, 1, 2, etc.)
- Later mapped to score via `settings.scores`

## Where Score Mapping Happens

**Step 1: MapToScores** (autoqa_mapper.go)
```go
mappedScore.NumericValue = 1  // Could be index or score depending on type
```

**Step 2: ComputeScores** (scorecard_calculator.go)
```go
// For criteria with value-to-score mapping
func MapScoreValue(numericValue float64, criterion) (float64, error) {
    if criterion.GetValueScores() == nil {
        return numericValue, nil  // Simple DND: already a score
    }
    // # of Occurrences: map index → score
    for _, score := range criterion.GetValueScores() {
        if score.Value == numericValue {  // numericValue=1 matches value=1
            return score.Score, nil  // return 5
        }
    }
}
```

## Back to the Original Question: Checking N/A

When we need to check "is this value an N/A option?", we need to check:

### For Simple DND (no Options):
```go
// Check if value matches an N/A option in settings.options
isNA := checkIfValueIsNAInOptions(criterion.Settings.Options, *mappedValue)
```

### For # of Occurrences (with Options):
```go
// Check if value (which is an index) points to N/A option
isNA := checkIfValueIsNAInOptions(criterion.Settings.Options, *mappedValue)
```

**Same check for both!** Because:
- Simple DND: value might be 0, 1, 2 (option values)
- # of Occurrences: value is 0, 1, 2 (option indices)
- In decoupled scoring, option values ARE indices!

## The Confusion Resolved

```go
mappedScore.NumericValue = &option.Value
```

**What is `option.Value`?**
- For # of Occurrences: It's the **option index** (0, 1, 2)
- This index points to an entry in `settings.options`
- The actual score comes from `settings.scores[index].score`

**So to check if it's N/A:**
```go
// Get the option at this index
option := settings.options[numericValue]
if option.isNA {
    // It's an N/A option
}
```

## Implementation Note

For our fix, we need to:
1. Get the numeric value (which could be index or direct value)
2. Check if this value corresponds to an N/A option in `settings.options`
3. Set `NotApplicable` flag accordingly

The check is the same for both systems because in decoupled scoring, option values ARE indices!
