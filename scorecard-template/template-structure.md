# Scorecard Template Structure — Deep Dive

**Created:** 2026-03-26

## 1. What Is a Scorecard Template?

A reusable blueprint that defines evaluation criteria for assessing agent performance. Templates are:

- **Multi-versioned** with revision tracking
- **Customer + profile scoped**
- **Use-case associated** (one template can serve multiple use cases)
- **Audience-controlled** (which agents are evaluated)
- Supports both **conversation-based** and **process-based** scorecards

### Database Table: `director.scorecard_templates`

Key fields (from `apiserver/sql-schema/gen/model/scorecard_templates.go`):


| Field          | Type     | Description                         |
| -------------- | -------- | ----------------------------------- |
| Customer       | string   | Customer ID                         |
| Profile        | string   | Profile ID                          |
| ResourceID     | string   | Template ID                         |
| Revision       | string   | Template revision                   |
| Title          | string   | Display name                        |
| Template       | JSONB    | **The actual template structure**   |
| UsecaseIds     | []string | Associated use case IDs             |
| Type           | int      | 1=Conversation, 2=Process           |
| Status         | int      | 1=Active, 2=Inactive, 3=Archived    |
| Audience       | JSONB    | Who can access (users/teams/groups) |
| Permissions    | JSONB    | Role-based permissions              |
| QaTaskConfig   | JSONB    | QA task configuration               |
| QaScoreConfig  | JSONB    | QA scoring configuration            |
| AutoqaTriggers | []string | Auto-QA trigger resource names      |


### Proto Definition

```protobuf
message ScorecardTemplate {
  string name = 1;                                    // Auto-generated
  string title = 2;                                   // REQUIRED
  google.protobuf.Struct template = 3;               // REQUIRED - scorecard structure
  Audience audience = 4;
  ResolvedTemplateAudience resolved_audience = 5;    // OUTPUT_ONLY
  google.protobuf.Timestamp create_time = 6;         // OUTPUT_ONLY
  repeated string usecase_names = 9;
  cresta.v1.qa.QATaskConfig qa_task_config = 10;
  Permissions permissions = 12;
  ScorecardTemplateType type = 13;
  ScorecardTemplateStatus status = 14;
  cresta.v1.qa.QAScoreConfig qa_score_config = 15;
}
```

### Enums

```
ScorecardTemplateType:  UNSPECIFIED=0, CONVERSATION=1, PROCESS=2
ScorecardTemplateStatus: UNSPECIFIED=0, ACTIVE=1, INACTIVE=2, ARCHIVED=3
ScorecardScoringSubject: UNSPECIFIED=0, CONVERSATION=1, MESSAGE=2
```

---

## 2. Template Structure — Hierarchy

The `Template` JSONB field is parsed via `ParseScorecardTemplateStructure()` in `shared/scoring/scorecard_template_parser.go`.

Two versions exist. **Most templates in production are V2.**

### V1 — Flat List (legacy)

```go
type ScorecardTemplateStructureV1 struct {
    Criteria                  []ScorecardTemplateCriterion
    ShouldDisplayCommentField *bool
}
```

### V2 — Hierarchical with Chapters (current, used by most templates)

```go
type ScorecardTemplateStructureV2 struct {
    Version                   int
    Items                     []ScorecardTemplateStructureNode  // Chapter or Criterion
    ShouldDisplayCommentField *bool
}
```

### Hierarchy:

```
Template
├── Chapter (section)
│   ├── Chapter (nested section)
│   │   └── Criterion
│   └── Criterion
└── Criterion (top-level, with optional branches)
    └── Branch (conditional)
        └── Child Criterion
```

Chapters can nest arbitrarily. A `ScorecardTemplateStructureNode` is either a **Chapter** or a **Criterion**.

### Full Visual Template Structure (V2)

This shows where every struct lives in the JSONB template field:

```
ScorecardTemplateStructureV2
│
├── version: 2
├── shouldDisplayCommentField: bool
│
└── items[]: (Chapter | Criterion)
    │
    ├── ── Chapter ──────────────────────────────────────────────
    │   ├── identifier: string
    │   ├── displayName: string
    │   └── items[]: (Chapter | Criterion)  ← recursive
    │
    └── ── Criterion (scorable) ─────────────────────────────────
        │
        ├── ── BaseCriterion ────────────────────────────────────
        │   ├── type: "numeric-radios" | "labeled-radios" | "dropdown-numeric-values"
        │   ├── identifier: string
        │   ├── displayName: string
        │   ├── weight: int                     ← relative importance in scoring
        │   ├── shortName: *string
        │   ├── helpText: *string
        │   ├── required: *bool
        │   ├── notRemovable: *bool
        │   ├── perMessage: *bool               ← per-message scoring
        │   ├── shouldDisplayCommentField: *bool
        │   │
        │   └── branches[]: Branch              ← conditional sub-criteria
        │       ├── identifier: string
        │       ├── condition: CriterionCondition
        │       │   ├── numericValues: []float32
        │       │   └── notApplicable: *bool
        │       └── children[]: Criterion       ← child criteria
        │
        ├── ── CriterionWithValueSettings ───────────────────────
        │   ├── showNA: *bool                   ← allow N/A selection
        │   ├── autoFail: *AutoFailConfig
        │   │   ├── comparator: "equal" | "less_than" | "greater_than"
        │   │   └── value: *int
        │   ├── excludeFromQAScores: *bool      ← "Evaluate scores" toggle (inverted)
        │   ├── excludeOutcomeInsights: *bool
        │   └── enableMultiSelect: *bool        ← dropdown only
        │
        ├── ── Type-Specific Settings ───────────────────────────
        │   │
        │   ├── (numeric-radios)
        │   │   ├── range: { min: int, max: int }
        │   │   └── scores[]: CriterionScoreOption
        │   │
        │   ├── (labeled-radios)
        │   │   ├── options[]: LabeledCriterionSettingOption
        │   │   │   ├── label: string           ← display text (unique)
        │   │   │   └── value: int              ← internal index (wiring key)
        │   │   └── scores[]: CriterionScoreOption
        │   │       ├── value: float64          ← matches option.value (wiring key)
        │   │       └── score: float64          ← the actual score
        │   │
        │   └── (dropdown-numeric-values)
        │       ├── options[]: LabeledCriterionSettingOption  (same as labeled-radios)
        │       ├── scores[]: CriterionScoreOption            (same as labeled-radios)
        │       └── enableMultiSelect: *bool
        │
        └── ── AutoQAConfig ─────────────────────────────────────
            ├── triggers[]: AutoQATrigger
            │   ├── type: "policy" | "sdx_moment" | "moment" | "behavior" | "metadata"
            │   └── resourceName: string
            │
            ├── ── Behavior Done/Not Done Mode ──────────────────
            │   ├── detected: *int              ← option value when DETECTED
            │   └── notDetected: *int           ← option value when NOT_DETECTED
            │
            └── ── # of Occurrences Mode ────────────────────────
                └── options[]: AutoQAOptions
                    ├── triggerValue: *string    ← "detected", "not detected" (metadata)
                    ├── numericFrom: *float32    ← bin range start
                    ├── numericTo: *float32      ← bin range end
                    ├── numericValue: *float32   ← exact match
                    └── value: float32           ← criterion option value to assign
```

### Option Wiring Diagram

All three systems use `value` as the wiring key:

```
options[]                    scores[]                  auto_qa
┌───────┬───────┐           ┌───────┬───────┐        ┌──────────────┬───────┐
│ label │ value │──────────▶│ value │ score │        │ detected     │ value │──┐
├───────┼───────┤   wired   ├───────┼───────┤        │ notDetected  │ value │──┤
│ "Yes" │   1   │   by      │   1   │  10   │        └──────────────┴───────┘  │
│ "No"  │   0   │  value    │   0   │   0   │                                  │
└───────┴───────┘           └───────┴───────┘        wires to option value ◀────┘
```

Example: `detected: 1` means "when AI detects behavior → assign option value 1 → grader sees 'Yes' → score lookup finds 10".

### Chapter

```go
type ScorecardTemplateChapter struct {
    Identifier  string
    DisplayName string
    Items       []ScorecardTemplateStructureNode
}
```

---

## 3. Criterion Types

From `shared/scoring/scorecard_templates.go` lines 79-88:


| Type                      | Scorable? | Description                                |
| ------------------------- | --------- | ------------------------------------------ |
| `numeric-radios`          | ✅         | Numeric scale (default 1-5)                |
| `labeled-radios`          | ✅         | Custom labeled options with numeric values |
| `dropdown-numeric-values` | ✅         | Dropdown with numeric values               |
| `sentence`                | ❌         | Free text field (not relevant to scoring)  |
| `date`                    | ❌         | Date picker (not relevant to scoring)      |
| `user`                    | ❌         | User picker (not relevant to scoring)      |


**Focus on the three scorable types** — `sentence`, `date`, and `user` are non-scorable metadata fields that don't participate in QA score calculations.

### Criterion Fields

```go
type BaseCriterion struct {
    Type                      CriterionType
    Weight                    int              // Relative importance
    Identifier                string           // Unique ID
    DisplayName               string           // Label
    ShortName                 *string
    HelpText                  *string
    ShouldDisplayCommentField *bool
    Required                  *bool
    NotRemovable              *bool
    Branches                  *[]Branch        // Conditional sub-criteria
    PerMessage                *bool            // Per-message scoring
}
```

### Important Settings (on scorable criteria)


| Setting               | Description                                   |
| --------------------- | --------------------------------------------- |
| `Weight`              | Integer, relative importance in scoring       |
| `ExcludeFromQAScores` | Hide from QA score calculations               |
| `ShowNA`              | Allow "Not Applicable" selection              |
| `AutoFail`            | Auto-fail config with comparator + threshold  |
| `EnableMultiSelect`   | Allow multiple selections (dropdown only)     |
| `PerMessage`          | Score per message instead of per conversation |
| `ValueScores`         | Custom value→score mapping                    |


### Value-Score Mapping

```go
type CriterionScoreOption struct {
    Value float64  // The raw value (e.g., 1, 2, 3)
    Score float64  // The mapped score for this value
}
```

Example: Values `[1→10, 2→20, 3→50]` — if agent scores 2, the mapped score is 20 out of max 50.

### Auto-Fail

```go
type AutoFailConfig struct {
    Comparator *AutoFailComparator  // equal | less_than | greater_than
    Value      *int                 // Threshold
}
```

When triggered: criterion score becomes **0**, `AutoFailed=true` propagates up to parent chapters.

### Branching (Conditional Criteria)

```go
type Branch struct {
    Identifier string
    Condition  *CriterionCondition  // NumericValues or NotApplicable
    Children   *[]ScorecardTemplateStructureNode
}
```

---

## 4. Auto-QA (Automated Scoring)

Auto-QA allows AI to automatically score criteria based on detected behaviors. The `AutoQAConfig` sits on each criterion and wires AI outcomes to criterion option values.

### AutoQAConfig Struct

```go
type AutoQAConfig struct {
    Triggers    *[]AutoQATrigger  // What behavior(s) to detect
    Options     *[]AutoQAOptions  // # of Occurrences mode: maps count ranges to options
    Detected    *int              // Behavior DND mode: option value when DETECTED
    NotDetected *int              // Behavior DND mode: option value when NOT_DETECTED
}
```

### Two Scoring Modes

**Mode 1: Behavior Done/Not Done** — Simple binary mapping using `Detected` and `NotDetected` fields:

```
AI outcome DETECTED     → assign criterion option value = AutoQAConfig.Detected     → "Yes" (score 10)
AI outcome NOT_DETECTED → assign criterion option value = AutoQAConfig.NotDetected  → "No"  (score 0)
AI outcome NOT_APPLICABLE → Score.NotApplicable = true → criterion skipped (NO option mapping)
```

**Mode 2: # of Occurrences** — Maps evidence counts or metadata values to options via `AutoQAOptions[]`:

```
AutoQAOptions:
  { triggerValue: "detected",     value: 1 }  → "Yes"
  { triggerValue: "not detected", value: 0 }  → "No"
  { numericFrom: 3, numericTo: 5, value: 2 }  → "Moderate" (3-5 occurrences)
```

### Trigger Types

| Type         | Description                   |
| ------------ | ----------------------------- |
| `policy`     | Policy violation detection    |
| `sdx_moment` | Structured behavior moment    |
| `moment`     | Keyword/moment annotation     |
| `behavior`   | Behavior pattern              |
| `metadata`   | Conversation metadata/outcome |

Only `numeric-radios`, `labeled-radios`, and `dropdown-numeric-values` support Auto-QA.

### Outcome Values and Mapping (`autoqa_mapper.go`)

| Outcome | Priority | Behavior DND Mode | # of Occurrences Mode |
|---------|----------|-------------------|-----------------------|
| `DETECTED` | 3 | `NumericValue = config.Detected` | Match via `AutoQAOptions` |
| `NOT_DETECTED` | 2 | `NumericValue = config.NotDetected` | Match via `AutoQAOptions` (count=0) |
| `NOT_APPLICABLE` | 1 | `NotApplicable = true` (always, no option mapping) | `NotApplicable = true` |

**Current limitation**: NOT_APPLICABLE always sets `Score.NotApplicable = true` unconditionally (`autoqa_mapper.go:81-82`). There is no way to map it to a criterion option value. This is being addressed in the NAScore feature (see `na-score-design.md`).

---

## 5. Scoring Algorithm

### Entry Point: `ComputeScores()` in `scorecard_calculator.go`

```
ComputeScores()
├─ GetChaptersAndCriteria() — extract from template
├─ mapScoresByCriterion() — group scores by criterion ID
└─ For each criterion:
   ├─ Check IsValueAutoFailed() → set AutoFailed, score=0
   ├─ Check IsExcludeFromQAScores() → skip
   ├─ ComputeCriterionPercentageScore()
   │  ├─ IsMultiSelect? → computeMultiSelectScore()
   │  ├─ IsPerMessage? → computePerMessageScore()
   │  ├─ IsOutcomeCriterion? → computeOutcomeCriterionScore()
   │  └─ else → computeScore()
   └─ updateSummaryValues(percentage, weight, autoFailed)
      └─ Recursively update all ancestor chapters
└─ computePercentage() for each chapter and overall
```

### Standard Scoring (`computeScore`)

1. Get max score: either max from value-score mapping, or criterion's `MaxValue` (default 5)
2. Percentage = `scoreValue / maxScore`
3. Return `CriterionPercentageScore{PercentageScore, Weight}`

Example: Value scores `[1→10, 2→20, 3→50]`, score=2 → `20/50 = 0.4 (40%)`

### Multi-Select Scoring (`computeMultiSelectScore`)

1. Sum all possible scores: `sumScore = Σ(valueScore.Score)`
2. For each selection: `percentage = (numSelections × selectedScore) / sumScore`
3. Weight distributed: `weight / numSelections`

Example: Values `{1→10, 2→20, 3→20}`, sum=50, selections=[1,2]:

- Score 1: `(2×10)/50 = 0.4`, weight=`5/2=2.5`
- Score 2: `(2×20)/50 = 0.8`, weight=`5/2=2.5`

### Per-Message Scoring (`computePerMessageScore`)

Each message scored independently, weight distributed equally: `weight / numMessages`

### Weight Aggregation

```go
summary.Total += score × weight
summary.Weight += weight
percentage = (summary.Total × 100 × 10 / summary.Weight) / 10  // rounds to 1 decimal
```

### Chapter Aggregation

Each criterion score updates its direct parent chapter AND recursively all ancestor chapters. Chapters aggregate all descendant criterion scores.

```
BigChapter (chapter-1)
├─ CriterionA (weight=1, score=80%)
├─ CriterionB (weight=1, score=33%)
└─ NestedChapter (chapter-1-1)
   ├─ CriterionC (weight=1, score=100%)
   └─ CriterionD (weight=1, score=60%)

NestedChapter = (100+60)/2 = 80%
BigChapter = (80+33+100+60)/4 = 68.25%  ← all descendants, not just direct children
```

### Not Applicable Handling

When `NotApplicable=true`:

- NumericValue is nullified
- Criterion is **skipped** entirely — no impact on chapter or overall scores

---

## 6. Analytics Pipeline — How Templates Feed into QA Score Stats

### Entry Point: `RetrieveQAScoreStats` RPC

File: `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`

Two calculation paths:

#### Path A: Direct ClickHouse (RespectTemplateQaScoreConfig=false)

Simple weighted average from ClickHouse score rows.

#### Path B: Template-Aware (RespectTemplateQaScoreConfig=true)

1. **Load templates** via `qa.ListCurrentScorecardTemplateIDs()`
2. **Group by QA score config** — templates with config get individual queries
3. **Extract scoreable criteria** via `getScoreableCriteria()`:
  - Parse template JSON → `ScorecardTemplateStructure`
  - Filter out `IsExcludeFromQAScores()` criteria
4. **Apply conversation duration filters** from template's `QaScoreConfig`
5. **Build ClickHouse query**

### ClickHouse Aggregation

```sql
-- For score-level aggregation:
SUM(percentage_value * float_weight) / SUM(float_weight)

-- For scorecard-level aggregation:
SUM(score / 100.0)  -- with weight 1.0 per scorecard
```

### Data Flow: Scorecard Creation → ClickHouse

```
API Create/Update/Submit Scorecard
  ↓ [Synchronous]
Write to director.scorecards + director.scores
  ↓ [Asynchronous, 10 min timeout]
1. Transform → historic.scorecard_scores (Postgres)
2. Copy to ClickHouse score table
3. Update ClickHouse scorecards table
4. Reindex conversation in Elasticsearch (if conversation template)
```

### Score Row in ClickHouse

From `BuildScoreRows()` in `shared/clickhouse/conversations/conversation.go`:

- `FloatWeight` — from `scorecardScore.FloatWeight`
- `PercentageValue` — percentage score (0.0-1.0)
- `NumericValue` — raw criterion value
- `Weight` — integer weight from score

---

## 7. Template Versioning

```go
const (
    LatestRevisionName = "latest"
    RevisionWildcard   = "*"
)
```

- Each template edit creates a new **revision**
- Multiple revisions coexist in the DB
- "latest" resolves to the most recent by creation time
- Analytics can use specific revisions when looking at historical scores (e.g., conversation examples drawer uses `useRevisionScorecardTemplates` hook)

---

## 8. Permissions & Audience

### Audience (who gets evaluated)

```json
{
  "users": ["customers/{customer}/users/{user_id}"],
  "teams": ["customers/{customer}/teams/{team_id}"],
  "groups": ["customers/{customer}/groups/{group_id}"]
}
```

### Permissions (who can do what)

```json
{
  "template_editors": [...],
  "scorecard_viewers": [...],
  "scorecard_graders": [...],
  "scorecard_appealers": [...]
}
```

---

## 9. Open Questions / Areas to Explore Further

- How exactly does the `QaScoreConfig.ConversationDurationBuckets` filter work?
- Process scorecard vs conversation scorecard differences in detail
- Calibration scorecard flow
- Consistency score calculation details
- Template builder UI ↔ backend mapping

