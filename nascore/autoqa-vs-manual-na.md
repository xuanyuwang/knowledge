# AutoQA NOT_APPLICABLE vs Manual N/A Selection

**Created:** 2026-04-13

## The User's Question

> "Since a user can select non-na option for NOT_APPLICABLE, we need to handle that, right?"

This reveals an important distinction between:
1. **AutoQA-driven N/A** (what MapToScores handles)
2. **Manual override/editing** (different code path)

## Two Different Scenarios

### Scenario 1: AutoQA Returns NOT_APPLICABLE

**Flow:**
1. AutoQA evaluates conversation
2. AutoQA service returns: `{ Outcome: NOT_APPLICABLE }`
3. **MapToScores runs** (the code we're fixing)
4. Creates initial score based on template config

**Current behavior after our fix:**
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NotApplicable = true  // Mark as N/A
        mappedScore.NumericValue = config.NotApplicable  // With score
    } else {
        continue  // Skip if not configured
    }
```

**Result:** 
- Scorecard created with `NotApplicable=true`
- User sees this in UI as an N/A score

### Scenario 2: User Manually Overrides N/A

**Flow:**
1. Scorecard already exists (created by AutoQA)
2. User opens scorecard in UI
3. User sees criterion marked as "N/A" (from AutoQA)
4. User disagrees and wants to change to "Yes"
5. User clicks to edit and selects "Yes" option
6. **UPDATE scorecard API called** (different code path, NOT MapToScores)

**This is handled by:**
- Frontend: Scorecard editing UI
- Backend: `UpdateScorecard` / `UpdateScores` APIs
- NOT `MapToScores` (which only runs once during initial scoring)

## The Key Insight

**MapToScores only runs ONCE when AutoQA initially scores the conversation.**

After that, any manual edits go through the **scorecard update flow**, which:
1. Accepts whatever value the user selected
2. Marks the score as manually edited
3. Overrides the AI value

## Does MapToScores Need to Handle Manual Selection?

**NO!** Here's why:

### MapToScores is for: AutoQA → Initial Score
```
Input:  AutoQA service result (NOT_APPLICABLE)
Output: Initial scorecard score
Runs:   Once per conversation, automatically
```

### Manual editing is handled by: UpdateScorecard APIs
```
Input:  User's selected option from UI
Output: Updated scorecard score  
Runs:   When user manually edits the score
```

## What if AutoQA Returns NOT_APPLICABLE but Template Maps it to Regular Option?

This is an interesting edge case. Let's look at the config:

```json
{
  "auto_qa": {
    "triggers": [...],
    "detected": 2,
    "not_detected": 1,
    "not_applicable": 0  // ← Maps to value 0
  },
  "settings": {
    "options": [
      {"label": "No", "value": 0},
      {"label": "Partial", "value": 1},
      {"label": "Yes", "value": 2}
    ]
  }
}
```

**Question:** When AutoQA returns NOT_APPLICABLE and config has `not_applicable: 0`, should we:
- **Option A:** Set NotApplicable=true + NumericValue=0 (current behavior)
- **Option B:** Set NotApplicable=false + NumericValue=0 (treat as regular option)

### Current Implementation (Option A)

```go
mappedScore.NotApplicable = true  // ← Always true
mappedScore.NumericValue = 0      // ← Maps to "No" option
```

**Semantic meaning:** "This criterion is not applicable, but has a score of 0"

### Alternative (Option B)

```go
mappedScore.NotApplicable = false  // ← Treat as regular option
mappedScore.NumericValue = 0       // ← Maps to "No" option
```

**Semantic meaning:** "This criterion is applicable, value is 'No'"

## Which is Correct?

**I believe Option A (current) is correct** because:

1. **AutoQA semantics:** When AutoQA returns `NOT_APPLICABLE`, it means "criterion doesn't apply to this conversation"
   - This is different from "criterion applies and the answer is No"
   
2. **Example:** Greeting detection in a chat conversation
   - NOT_APPLICABLE: "No greeting expected in chat" → NotApplicable=true
   - NOT_DETECTED: "Greeting expected but not found" → NotApplicable=false, value=0

3. **Scored N/A purpose:** Allow N/A to contribute to scoring without meaning "this applied but failed"

## tinglinliu's Perspective

tinglinliu said: "when not_applicable is set in the config, then score.not_applicable should be set to true"

This confirms **Option A is correct:**
- When config has `not_applicable` value → set `NotApplicable=true` + value
- The flag indicates semantic meaning (not applicable)
- The value indicates contribution to score

## Summary

### Your Question Answered

> "Since a user can select non-na option for NOT_APPLICABLE, we need to handle that, right?"

**Answer:** No, MapToScores doesn't need to handle manual selection because:

1. **MapToScores = AutoQA → Initial Score** (one-time, automatic)
2. **Manual override = Update APIs** (separate code path)
3. **When AutoQA returns NOT_APPLICABLE:**
   - We create score with `NotApplicable=true` (semantic meaning preserved)
   - User can later manually edit to any option (handled by update flow)

### The Code is Correct

```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NotApplicable = true  // ✅ Correct
        mappedScore.NumericValue = config.NotApplicable  // ✅ Correct
    } else {
        continue  // ✅ Correct
    }
```

**Rationale:**
- `NotApplicable=true` preserves semantic meaning from AutoQA
- `NumericValue` allows scored N/A to contribute to calculations
- Manual overrides happen in a different code path (UpdateScorecard)
