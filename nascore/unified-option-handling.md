# Unified Option Handling Flow

**Created:** 2026-04-13

## The Refactoring

Refactored the option handling logic to follow a cleaner, unified flow for all outcome types.

## New Flow

For ALL outcomes (DETECTED, NOT_DETECTED, NOT_APPLICABLE) and metadata triggers:

### Step 1: Find/Determine the Value

```go
var mappedValue *int
// Determine value based on outcome/trigger type
```

### Step 2: Check if Value Points to N/A Option

```go
if mappedValue != nil {
    isNA := isNumericValueNAOption(float32(*mappedValue), criterion)
```

### Step 3: Handle N/A Option

```go
    if isNA {
        // N/A option - check if it has a score configured
        if !doesNAOptionHaveScore(float32(*mappedValue), criterion) {
            continue // Skip creating score for N/A option without score
        }
        mappedScore.NotApplicable = true
    }
```

### Step 4: Handle Regular Option

```go
    else {
        // Regular option
        mappedScore.NotApplicable = false
    }
    mappedScore.NumericValue = nilOrFloat32(mappedValue)
}
```

## Example: DETECTED Outcome

### Before (Old Approach)

```go
case autoqapb.AutoScoreOutcome_DETECTED:
    if autoQaConfig.Options != nil && len(*autoQaConfig.Options) > 0 {
        option := matchNumericAutoQaOption(autoQaConfig, float32(len(scoredItem.Evidences)))
        if option != nil {
            mappedScore.NumericValue = &option.Value
            mappedScore.NotApplicable = isNumericValueNAOption(option.Value, criterion)
            if mappedScore.NotApplicable && !doesNAOptionHaveScore(option.Value, criterion) {
                continue
            }
        } else {
            mappedScore.NotApplicable = true
        }
    } else {
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.Detected)
        if mappedScore.NumericValue != nil {
            mappedScore.NotApplicable = isNumericValueNAOption(*mappedScore.NumericValue, criterion)
            if mappedScore.NotApplicable && !doesNAOptionHaveScore(*mappedScore.NumericValue, criterion) {
                continue
            }
        }
    }
```

### After (New Approach)

```go
case autoqapb.AutoScoreOutcome_DETECTED:
    // Step 1: Find the value
    var mappedValue *int
    if autoQaConfig.Options != nil && len(*autoQaConfig.Options) > 0 {
        option := matchNumericAutoQaOption(autoQaConfig, float32(len(scoredItem.Evidences)))
        if option != nil {
            value := int(option.Value)
            mappedValue = &value
        } else {
            mappedScore.NotApplicable = true
        }
    } else {
        mappedValue = autoQaConfig.Detected
    }

    // Step 2-4: Check option type and handle
    if mappedValue != nil {
        isNA := isNumericValueNAOption(float32(*mappedValue), criterion)
        if isNA {
            // N/A option - check if it has a score configured
            if !doesNAOptionHaveScore(float32(*mappedValue), criterion) {
                continue // Skip creating score for N/A option without score
            }
            mappedScore.NotApplicable = true
        } else {
            // Regular option
            mappedScore.NotApplicable = false
        }
        mappedScore.NumericValue = nilOrFloat32(mappedValue)
    }
```

## Benefits

### ✅ Clearer Logic Flow

**Old:** Set NumericValue, then check NA, then check has score
**New:** Find value → check NA → handle NA vs regular

### ✅ Single Responsibility

Each step has one responsibility:
1. **Find value:** Get the mapped value
2. **Check type:** Is it NA?
3. **Handle NA:** Check score, skip or continue
4. **Handle regular:** Set NotApplicable=false

### ✅ Consistent Pattern

Same pattern for ALL cases:
- DETECTED with/without Options
- NOT_DETECTED with/without Options
- NOT_APPLICABLE
- StringValue trigger
- BooleanValue trigger
- NumberValue trigger (with/without Options)

### ✅ Easier to Understand

Reading the code, it's clear:
1. First, we find what value we're mapping to
2. Then, we check if it's an N/A option
3. If N/A: check score existence, skip or mark NotApplicable=true
4. If regular: mark NotApplicable=false

### ✅ Easier to Maintain

Future changes only need to touch one part:
- Adding new option types? Update step 2 (check NA)
- Changing skip logic? Update step 3 (handle NA)
- Adding new outcome types? Follow the same 4-step pattern

## All Cases Updated

1. ✅ **DETECTED** (with/without Options)
2. ✅ **NOT_DETECTED** (with/without Options)
3. ✅ **NOT_APPLICABLE**
4. ✅ **StringValue** (metadata trigger)
5. ✅ **BooleanValue** (metadata trigger)
6. ✅ **NumberValue** (metadata trigger, with/without Options)

## Test Results

All tests pass with the refactored approach:

```
--- PASS: TestAutoQAMapper (6.93s)
    --- PASS: TestAutoQAMapper/TestMapToScores (0.00s)
    --- PASS: TestAutoQAMapper/TestMapToScoresMetadataCriteria (0.00s)
    --- PASS: TestAutoQAMapper/TestMapToScoresPerMessageCriteria (0.00s)
PASS
```

## Summary

**Old approach:**
- Set NumericValue immediately
- Then check if it's NA
- Then check if has score
- Repeated logic in multiple places

**New approach:**
1. Find scoreOption by looking up value
2. Check if scoreOption is NA option
3. If NA: check if has score → skip or continue with NotApplicable=true
4. If regular: process as normal option with NotApplicable=false

**Result:** Cleaner, more maintainable, easier to understand! ✅
