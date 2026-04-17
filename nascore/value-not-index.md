# Correction: option.Value is NOT an Index

**Created:** 2026-04-13

## The User's Correction

I incorrectly said `option.Value` is an "index". 

**User correctly pointed out:**
```go
type AutoQAOptions struct {
    Value        float32  `json:"value"`         // Value of the criterion
}
```

The comment clearly says "**Value of the criterion**", NOT "index".

## What option.Value Actually Is

### AutoQAOptions Structure

```go
type AutoQAOptions struct {
    NumericFrom  *float32 `json:"numeric_from"`  // Bin range: from
    NumericTo    *float32 `json:"numeric_to"`    // Bin range: to
    Value        float32  `json:"value"`         // ← CRITERION VALUE (not index!)
}
```

**Purpose:** Maps occurrence count ranges → criterion values

**Example:**
```json
{
  "auto_qa": {
    "options": [
      {"numeric_from": 0, "numeric_to": 1, "value": 0},   // 0 occurrences → value 0
      {"numeric_from": 1, "numeric_to": 3, "value": 1},   // 1-2 occurrences → value 1
      {"numeric_from": 3, "numeric_to": 99, "value": 2}   // 3+ occurrences → value 2
    ]
  }
}
```

**What this means:**
- If 0 occurrences detected → use criterion value **0**
- If 1-2 occurrences detected → use criterion value **1**
- If 3+ occurrences detected → use criterion value **2**

These values (0, 1, 2) are **criterion values**, not array indices!

## How It Maps to Settings

### Criterion Settings

```json
{
  "settings": {
    "options": [
      {"label": "Never", "value": 0},      // Criterion value 0
      {"label": "Sometimes", "value": 1},  // Criterion value 1
      {"label": "Always", "value": 2}      // Criterion value 2
    ],
    "scores": [
      {"value": 0, "score": 0},   // Criterion value 0 → score 0
      {"value": 1, "score": 5},   // Criterion value 1 → score 5
      {"value": 2, "score": 10}   // Criterion value 2 → score 10
    ]
  }
}
```

### The Flow

1. **AutoQA detects:** 2 occurrences
2. **matchNumericAutoQaOption:** Finds bin [1, 3) → returns option with `value: 1`
3. **MapToScores:** Sets `NumericValue = 1` (criterion value)
4. **Later in scoring:** Maps criterion value 1 → score 5

## Why I Got Confused

In the **decoupled scoring model**, criterion values happen to be **sequential starting from 0**:
- value: 0, 1, 2, 3, ...

This **looks like** indices (0, 1, 2, 3), but they're actually **criterion values**.

**The coincidence:**
- Option at index 0 has value 0
- Option at index 1 has value 1  
- Option at index 2 has value 2

But if you had:
```json
"options": [
  {"label": "Low", "value": 10},
  {"label": "High", "value": 20}
]
```

Then criterion values are 10, 20 (NOT indices 0, 1).

## Corrected Understanding

### What NumericValue Contains

`mappedScore.NumericValue` always contains the **criterion value**, which is:

**For Simple DND:**
```go
mappedScore.NumericValue = nilOrFloat32(autoQaConfig.Detected)  // e.g., 5
```
→ Criterion value is 5 (directly the score)

**For # of Occurrences:**
```go
mappedScore.NumericValue = &option.Value  // e.g., 1
```
→ Criterion value is 1 (maps to option "Sometimes", which maps to score 5)

## How to Check if Value is N/A

```go
// Given a criterion value (e.g., 1)
criterionValue := *mappedScore.NumericValue

// Check if this value corresponds to an N/A option
for _, option := range criterion.Settings.Options {
    if option.Value == int(criterionValue) && option.IsNA {
        // This is an N/A option
        isNA = true
        break
    }
}
```

**Key:** We're matching criterion **values**, not indices.

## Why This Still Works

Even though I was wrong about "index vs value", the **check is the same**:

```go
// Check if criterion value points to N/A option
for _, option := range settings.options {
    if option.value == numericValue && option.isNA {
        return true
    }
}
```

This works because we're comparing:
- `NumericValue` (criterion value like 1, 2, etc.)
- `option.value` (criterion value in settings like 1, 2, etc.)
- NOT array indices!

## Summary

✅ **Corrected:** `option.Value` is a **criterion value** (e.g., 0, 1, 2, 10, 20)
❌ **Was wrong:** I said it was an "index" (e.g., position in array)

✅ **Still correct:** To check N/A, compare criterion values:
```go
settings.options.find(opt => opt.value == numericValue && opt.isNA)
```

Thank you for the correction!
