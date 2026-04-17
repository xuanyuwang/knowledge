# AutoQA Mapper Architecture Explained

**Created:** 2026-04-13

## Question 1: What is `autoQaConfig`?

### Structure Definition

```go
type AutoQAConfig struct {
    Triggers      *[]AutoQATrigger `json:"triggers"`
    Options       *[]AutoQAOptions `json:"options"`
    Detected      *int             `json:"detected"`
    NotDetected   *int             `json:"not_detected"`
    NotApplicable *int             `json:"not_applicable,omitempty"`
}
```

### What It Represents

**YES, it's the JSON template configuration from the frontend "Opera integration" / Auto QA section!**

When you configure a scorecard template criterion with Auto QA in the frontend:
1. User selects a trigger (e.g., "Greeting detected" behavior)
2. User maps outcomes to scores:
   - **Detected** → score value (e.g., 5)
   - **Not Detected** → score value (e.g., 0)
   - **Not Applicable** → score value (e.g., null or 0) ← NEW with scored N/A

This configuration is saved as `auto_qa` in the template JSON:

```json
{
  "identifier": "greeting-criterion",
  "type": "numeric-radios",
  "auto_qa": {
    "triggers": [
      {
        "type": "behavior",
        "resource_name": "customers/acme/profiles/default/behaviors/greeting"
      }
    ],
    "detected": 5,
    "not_detected": 0,
    "not_applicable": null  // ← This is NEW! Not configured in old templates
  }
}
```

### The Flow

**Frontend (Template Builder)** → saves template JSON to DB
↓
**Backend** loads template, extracts `auto_qa` config
↓
**MapToScores** uses `autoQaConfig` to map AutoQA outcomes to scores

## Question 2: What is the purpose of MapToScores?

### Function Signature

```go
func (impl *autoQAImpl) MapToScores(
    autoScoredItems []*autoqapb.AutoScoredItem,  // FROM: AutoQA service results
    criterionToAutoQa map[string]*AutoQAConfig,   // FROM: Template configuration
) []*coachingpb.Score                             // TO: Scorecard scores
```

### Purpose: Map AutoQA Results → Scorecard Scores

**Input 1: `autoScoredItems`** (from AutoQA service)
```go
[
  {
    CriterionId: "greeting-criterion",
    Outcome: DETECTED,  // AutoQA says: behavior was detected
  },
  {
    CriterionId: "empathy-criterion", 
    Outcome: NOT_DETECTED,  // AutoQA says: behavior was not detected
  },
  {
    CriterionId: "resolution-criterion",
    Outcome: NOT_APPLICABLE,  // AutoQA says: not applicable
  }
]
```

**Input 2: `criterionToAutoQa`** (from template)
```go
{
  "greeting-criterion": {
    Detected: 5,
    NotDetected: 0,
  },
  "empathy-criterion": {
    Detected: 3,
    NotDetected: 1,
  },
  "resolution-criterion": {
    Detected: 2,
    NotDetected: 0,
    NotApplicable: 0,  // ← Scored N/A configured!
  }
}
```

**Output: Scorecard scores**
```go
[
  {
    CriterionId: "greeting-criterion",
    NumericValue: 5,      // Outcome=DETECTED → use config.Detected
    NotApplicable: false,
  },
  {
    CriterionId: "empathy-criterion",
    NumericValue: 1,      // Outcome=NOT_DETECTED → use config.NotDetected
    NotApplicable: false,
  },
  {
    CriterionId: "resolution-criterion",
    NumericValue: 0,      // Outcome=NOT_APPLICABLE → use config.NotApplicable
    NotApplicable: true,
  }
]
```

### The Mapping Logic

```
AutoQA Outcome      →  Template Config Field  →  Score Result
─────────────────────────────────────────────────────────────
DETECTED            →  config.Detected        →  NumericValue=X, NotApplicable=false
NOT_DETECTED        →  config.NotDetected     →  NumericValue=Y, NotApplicable=false
NOT_APPLICABLE      →  config.NotApplicable   →  NumericValue=Z, NotApplicable=true
                       (if configured)
NOT_APPLICABLE      →  (not configured)       →  Skip score (continue)
                       config.NotApplicable=nil
```

## Question 3: Do we need to handle N/A for each outcome type?

### Short Answer: NO

AutoQA only returns **ONE outcome** per criterion per evaluation:
- Either `DETECTED`
- Or `NOT_DETECTED`
- Or `NOT_APPLICABLE`

You CANNOT have multiple outcomes for the same criterion in a single scorecard.

### Current Code Structure is Correct

```go
switch scoredItem.Outcome {
case autoqapb.AutoScoreOutcome_DETECTED:
    // Handle detected case
    mappedScore.NumericValue = config.Detected

case autoqapb.AutoScoreOutcome_NOT_DETECTED:
    // Handle not detected case
    mappedScore.NumericValue = config.NotDetected

case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    // Handle not applicable case
    if config.NotApplicable != nil {
        mappedScore.NotApplicable = true
        mappedScore.NumericValue = config.NotApplicable
    } else {
        continue  // Skip if not configured
    }
}
```

### Why Your Concern Makes Sense (But Doesn't Apply Here)

You might be thinking:
> "What if a user selects N/A option for a criterion that was DETECTED?"

**This is a different scenario:**
- **Manual scoring:** User manually selects N/A from UI → handled in frontend/scoring UI
- **Auto scoring (MapToScores):** AutoQA returns one outcome → we map it to a score

### Where N/A Option Selection Happens

**Scenario 1: Manual Scorecard**
```
User opens scorecard UI
  → Sees criterion with options: [Yes (1), No (0), N/A]
  → User clicks "N/A"
  → Frontend sends: { value: [INPUT_N_A_VALUE] }
  → Backend saves: { NumericValue: <index of N/A>, NotApplicable: true }
```

**Scenario 2: Auto Scorecard (MapToScores)**
```
AutoQA evaluates conversation
  → Returns: { Outcome: NOT_APPLICABLE }
  → MapToScores checks: Does config have NotApplicable value?
    → YES: Create score with NotApplicable=true + NumericValue=config.NotApplicable
    → NO: Skip score creation (continue)
```

## Summary

### 1. autoQaConfig = Template Configuration
- ✅ Yes, it's the "Opera integration" / Auto QA config from frontend
- Defines what score value to assign for each AutoQA outcome
- Stored in template JSON as `auto_qa` field

### 2. MapToScores = Outcome → Score Converter
- Takes AutoQA service results (outcomes)
- Uses template config (autoQaConfig) to map outcomes to score values
- Returns scorecard scores ready to save to DB

### 3. No Need for N/A in Each Outcome Branch
- ❌ AutoQA returns ONE outcome per criterion (mutually exclusive)
- ✅ Current switch-case structure is correct
- ✅ N/A handling is only in the NOT_APPLICABLE case
- Manual N/A selection is a separate flow (frontend → scoring UI)

## The tinglinliu Fix Explained

**Problem:** Current code always sets `NotApplicable=true` when AutoQA returns NOT_APPLICABLE

**tinglinliu's feedback:** Only set when configured in template

**Solution:**
```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if config.NotApplicable != nil {  // ← CONDITIONAL
        mappedScore.NotApplicable = true
        mappedScore.NumericValue = config.NotApplicable
    } else {
        continue  // ← Skip score if not configured
    }
```

**Behavior:**
- Template HAS `not_applicable: 0` configured → Create scored N/A
- Template has NO `not_applicable` field → Skip score (legacy behavior)
