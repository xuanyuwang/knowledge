# NumericValue as Lookup Key for Options

**Created:** 2026-04-13

## User's Question

> "So in MapToScores, `mappedScore.NumericValue` is actually the Value which can be used to find options?"

## Answer: YES!

`mappedScore.NumericValue` contains a **criterion value** that can be used to look up the corresponding option in `settings.options`.

## How the Lookup Works

### Template Configuration

```json
{
  "settings": {
    "options": [
      {"label": "Yes", "value": 2},
      {"label": "No", "value": 1},
      {"label": "N/A", "value": 0, "isNA": true}
    ]
  },
  "auto_qa": {
    "detected": 2,      // ← Criterion value
    "not_detected": 1,  // ← Criterion value
    "not_applicable": 0 // ← Criterion value
  }
}
```

### MapToScores Sets NumericValue

```go
// For DETECTED outcome
mappedScore.NumericValue = nilOrFloat32(autoQaConfig.Detected)  // = 2
```

### Lookup the Option

```go
// Find the option with value = 2
for _, option := range criterion.Settings.Options {
    if option.Value == 2 {
        // Found: {"label": "Yes", "value": 2}
        // Check if N/A:
        if option.IsNA {
            // This is an N/A option
        }
        break
    }
}
```

## Both Systems Use Same Lookup

### System 1: Simple DND (no AutoQA.Options)

```go
mappedScore.NumericValue = nilOrFloat32(autoQaConfig.Detected)  // e.g., 2
```

**Lookup:**
```go
settings.options.find(opt => opt.value == 2)  // ✅ Works!
```

### System 2: # of Occurrences (with AutoQA.Options)

```go
option := matchNumericAutoQaOption(autoQaConfig, evidenceCount)
mappedScore.NumericValue = &option.Value  // e.g., 1
```

**Lookup:**
```go
settings.options.find(opt => opt.value == 1)  // ✅ Works!
```

## The Fix We Need

Now we can implement the N/A check for ALL outcomes:

```go
func isNumericValueNAOption(numericValue float32, criterion ScorecardTemplateCriterion) bool {
    if criterion.Settings == nil || criterion.Settings.Options == nil {
        return false  // No options defined
    }
    
    for _, option := range *criterion.Settings.Options {
        if option.Value == int(numericValue) && option.IsNA != nil && *option.IsNA {
            return true
        }
    }
    return false
}
```

**Then in MapToScores:**

```go
// For ANY outcome (DETECTED, NOT_DETECTED, NOT_APPLICABLE)
var mappedValue *int
switch scoredItem.Outcome {
case DETECTED:
    mappedValue = autoQaConfig.Detected
case NOT_DETECTED:
    mappedValue = autoQaConfig.NotDetected
case NOT_APPLICABLE:
    mappedValue = autoQaConfig.NotApplicable
}

if mappedValue == nil {
    continue  // No value configured, skip
}

mappedScore.NumericValue = float32(*mappedValue)

// Check if this value points to an N/A option
isNA := isNumericValueNAOption(float32(*mappedValue), criterion)
mappedScore.NotApplicable = isNA
```

## Example Scenarios

### Scenario 1: DETECTED maps to N/A

**Template:**
```json
{
  "options": [
    {"label": "Yes", "value": 2},
    {"label": "No", "value": 1},
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "auto_qa": {
    "detected": 0  // ← Maps to N/A option!
  }
}
```

**Flow:**
1. AutoQA returns DETECTED
2. MapToScores: `NumericValue = 0`
3. Lookup: `options.find(opt => opt.value == 0)` → N/A option
4. Set: `NotApplicable = true` ✅

### Scenario 2: NOT_APPLICABLE maps to Regular Option

**Template:**
```json
{
  "options": [
    {"label": "Yes", "value": 2},
    {"label": "No", "value": 1},
    {"label": "N/A", "value": 0, "isNA": true}
  ],
  "auto_qa": {
    "not_applicable": 1  // ← Maps to "No" option!
  }
}
```

**Flow:**
1. AutoQA returns NOT_APPLICABLE
2. MapToScores: `NumericValue = 1`
3. Lookup: `options.find(opt => opt.value == 1)` → "No" option (not N/A)
4. Set: `NotApplicable = false` ✅

## Key Insight

**`NumericValue` is a lookup key**, not a score!

- ✅ Use it to find the option
- ✅ Check if that option is N/A
- ✅ Set `NotApplicable` flag accordingly
- ❌ Don't assume it's a score (score comes later via `settings.scores` mapping)

## Summary

**Yes, you can use `NumericValue` to find options!**

This is the key to fixing the bug: instead of assuming outcome type determines NotApplicable flag, we **look up the actual option** and check its `isNA` property.
