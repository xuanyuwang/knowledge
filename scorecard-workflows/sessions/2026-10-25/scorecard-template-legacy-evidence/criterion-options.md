# Criterion Options — How They Work Today

**Created:** 2026-03-26

## 1. Criterion Type Hierarchy

There are 6 criterion types, but only 3 are scorable and relevant here. The non-scorable types (`sentence`, `date`, `user`) are metadata fields that don't participate in QA scoring.

The three scorable types all extend `BaseCriterion` + a type-specific settings struct, sharing a common settings base:

```
BaseCriterion (weight, identifier, displayName, branches, perMessage, ...)
  │
  ├── NumericRadiosCriterion
  │     Settings: NumericRadiosCriterionSettings
  │       ├── CriterionWithValueSettings  ← shared base
  │       └── Range { Min, Max }
  │
  ├── LabeledRadiosCriterion
  │     Settings: LabeledCriterionTypeOptionSettings
  │       ├── CriterionWithValueSettings  ← shared base
  │       └── Options []{ Label, Value }
  │
  └── DropdownNumericCriterion
        Settings: LabeledCriterionTypeOptionSettings  ← same as labeled
          ├── CriterionWithValueSettings  ← shared base
          └── Options []{ Label, Value }
```

Source: `shared/scoring/scorecard_templates.go` lines 143-173, 302-339

## 2. Settings Fields — Purpose in UI Display vs Score Calculation

Template settings serve two purposes: (1) controlling what the grader sees and can select in the UI, and (2) defining how the raw selection feeds into score calculation.

### Shared Settings Base

```go
// scorecard_templates.go:332-339
type CriterionWithValueSettings struct {
    ShowNA                 *bool                   `json:"showNA"`
    AutoFail               *AutoFailConfig         `json:"autoFail,omitempty"`
    ExcludeOutcomeInsights *bool                   `json:"excludeOutcomeInsights"`
    ExcludeFromQAScores    *bool                   `json:"excludeFromQAScores"`
    Scores                 *[]CriterionScoreOption `json:"scores"`
    EnableMultiSelect      *bool                   `json:"enableMultiSelect"`
}
```

### Field-by-Field Breakdown

#### `options` (LabeledRadios & Dropdown only)

```go
type LabeledCriterionSettingOption struct {
    Label string `json:"label"`  // UI: "Value" column
    Value int    `json:"value"`  // hidden numeric value
}
```

- **UI display**: Renders the selectable options the grader clicks/selects. `Label` is shown as the clickable text. `Value` is the underlying numeric value submitted when selected.
- **Score calc**: Determines `MaxValue` (highest `Value` among options). When no value-score mapping exists, `Value` is used directly as the raw score input.

#### `range` (NumericRadios only)

```go
type RangeSettings struct {
    Min int `json:"min"`  // UI: "From"
    Max int `json:"max"`  // UI: "To"
}
```

- **UI display**: Renders radio buttons for each integer in [Min, Max]. Default 1–5.
- **Score calc**: `Max` is used as `MaxValue` for percentage normalization. E.g., grader selects 3 out of range 1–5 → percentage = 3/5 = 60%.

#### `scores` (Value-Score Mapping)

```go
type CriterionScoreOption struct {
    Value float64 `json:"value"`  // matches an option's .value or a range integer
    Score float64 `json:"score"`  // UI: "Score" column
}
```

- **UI display**: In the template builder, shown as the **"Score" column** next to options. Lets the admin configure a custom score for each option value.
- **Score calc**: **Optional.** When present, raw value is mapped to its corresponding `Score` before percentage calculation. `MaxScore` = highest `Score` in the array. When absent, raw value is used directly and `MaxValue` comes from `options` or `range`.

**Example — with mapping:**

```
Options:          { "Strong"=3, "Normal"=2, "Weak"=1 }
Value-Score Map:  { value:3→score:0, value:2→score:1, value:1→score:2 }

Grader selects "Strong" (value=3) → mapped score=0, maxScore=2
Percentage = 0/2 = 0%
```

**Example — without mapping:**

```
Options:          { "Yes"=1, "No"=0 }
(no scores array)

Grader selects "Yes" (value=1), maxValue=1
Percentage = 1/1 = 100%
```

#### `weight`

- **UI display**: Shown as **"Weight"** input in the template builder criterion config panel.
- **Score calc**: Multiplier in weighted aggregation. When rolling up to chapter/overall: `summary.Total += percentage × weight`, `summary.Weight += weight`. Higher weight = more influence on the final score.

#### `showNA`

- **UI display**: When `true`, shows an **"Allow N/A"** checkbox in the template builder and an **N/A option** to the grader when scoring.
- **Score calc**: When grader selects N/A, the criterion is **completely excluded** — returns nil from `ComputeCriterionPercentageScore()`, its weight is not counted in the denominator. (See section 6 for full N/A flow.)

#### `autoFail`

```go
type AutoFailConfig struct {
    Comparator *AutoFailComparator `json:"comparator"`  // equal | less_than | greater_than
    Value      *int                `json:"value"`
}
```

- **UI display**: Shown as **"Auto Fail"** toggle + comparator dropdown + threshold input in the template builder.
- **Score calc**: Checked before percentage calculation. If the grader's score triggers the condition (e.g., value equals 0), the criterion's percentage is forced to **0%** and `AutoFailed=true` propagates up to parent chapters.

#### `excludeFromQAScoresexcludeFromQAScores`

- **UI display**: Shown as **"Evaluate scores"** toggle (⚠️ **inverted** — toggle ON = `excludeFromQAScores: false`).
- **Score calc**: When `true`, the criterion is **statically excluded** from QA score stats. Unlike N/A (dynamic, per-score), this is a permanent template-level setting. Filtered out in `getScoreableCriteria()` in analytics and skipped in `ComputeScores()`.

#### `enableMultiSelect` (Dropdown only)

- **UI display**: Shown as **"Allow multi-select"** toggle. When enabled, grader can select multiple options.
- **Score calc**: Switches to `computeMultiSelectScore()` path. Each selection gets `percentage = (numSelections × selectedScore) / sumAllScores` with weight distributed as `weight / numSelections`.

#### `excludeOutcomeInsights`

- **UI display**: Shown as **"Exclude from Outcome Insights"** toggle.
- **Score calc**: ❌ **No impact on QA scoring.** Only affects whether the criterion appears in the Outcome Insights view.

#### `displayName`

- **UI display**: Shown as **"Display Name"** — the label the grader sees for this criterion.
- **Score calc**: ❌ No impact.

#### `shortName`

- **UI display**: Shown as **"Short Name"** — abbreviated name for compact views.
- **Score calc**: ❌ No impact.

#### `helpText`

- **UI display**: Shown as **"Help Text"** — guidance tooltip for graders.
- **Score calc**: ❌ No impact.

#### `required`

- **UI display**: Shown as **"Required"** toggle — grader must fill in this criterion.
- **Score calc**: ❌ No impact on calculation (just validation).

### Summary Table


| Field                    | UI Display                      | Score Calc                        | UI Label                        |
| ------------------------ | ------------------------------- | --------------------------------- | ------------------------------- |
| `options`                | ✅ Renders selectable options    | ✅ Defines MaxValue                | "Value" (column)                |
| `range`                  | ✅ Renders numeric scale buttons | ✅ Defines MaxValue                | "From" / "To"                   |
| `scores`                 | ✅ Shows Score column in builder | ✅ Maps value→score for %          | "Score" (column)                |
| `weight`                 | ✅ Shown in builder              | ✅ Multiplier in aggregation       | "Weight"                        |
| `showNA`                 | ✅ Shows N/A option to grader    | ✅ Triggers skip-from-scoring      | "Allow N/A"                     |
| `autoFail`               | ✅ Config in builder             | ✅ Zeros score + flags             | "Auto Fail"                     |
| `excludeFromQAScores`    | ✅ Toggle in builder             | ✅ Static exclusion from analytics | "Evaluate scores" (inverted)    |
| `enableMultiSelect`      | ✅ Allows multiple selections    | ✅ Changes to multi-select path    | "Allow multi-select"            |
| `excludeOutcomeInsights` | ✅ Toggle in builder             | ❌                                 | "Exclude from Outcome Insights" |
| `displayName`            | ✅ Criterion label               | ❌                                 | "Display Name"                  |
| `shortName`              | ✅ Abbreviated label             | ❌                                 | "Short Name"                    |
| `helpText`               | ✅ Grader guidance               | ❌                                 | "Help Text"                     |
| `required`               | ✅ Validation only               | ❌                                 | "Required"                      |


### Key Functions for Score Calculation


| Function                 | File:Line                       | Purpose                                                |
| ------------------------ | ------------------------------- | ------------------------------------------------------ |
| `GetValueScores()`       | scorecard_templates.go:564-582  | Return the Scores array from settings                  |
| `MapScoreValue()`        | scorecard_scores_dao.go:775-786 | Map raw value → score (or return raw if no mapping)    |
| `GetCriterionMaxScore()` | scorecard_scores_dao.go:759-773 | Max of mapped scores, or `GetMaxValue()` if no mapping |
| `GetMatchedValueScore()` | scorecard_templates.go:824-833  | Used in auto-fail checks                               |


## 3. Percentage Score Calculation

### Standard (single-select, single score)

`computeScore()` at scorecard_scores_dao.go:931-964:

```
1. Get maxScore = GetCriterionMaxScore(criterion)
     - If value-score mapping exists: max of all Score values
     - Otherwise: criterion.GetMaxValue()
2. Map the raw value: scoreValue = MapScoreValue(numericValue, criterion)
3. percentage = scoreValue / maxScore
4. Return { PercentageScore: percentage, Weight: criterion.GetWeight() }
```

### Multi-Select

`computeMultiSelectScore()` at scorecard_scores_dao.go:788-833:

```
1. sumScore = Σ(all valueScore.Score)
2. For each selected value:
     selectedScore = mapped score for that value
     percentage = (numSelections × selectedScore) / sumScore
     weight = criterion.Weight / numSelections
```

### Per-Message

`computePerMessageScore()` at scorecard_scores_dao.go:835-862:

Each message scored independently, weight = `criterion.Weight / numMessages`.

## 4. N/A Handling — Current Behavior

### What N/A Means Today

When `showNA: true` in settings, a grader can mark a criterion as "Not Applicable." This **completely removes the criterion from scoring** — it contributes nothing (no score, no weight) to chapter or overall scores.

### How It's Stored

In the `Score` proto (`cresta-proto/cresta/v1/coaching/scorecard.proto:153-203`):

```protobuf
message Score {
  string criterion_id = 1;
  optional float numeric_value = 3;
  bool not_applicable = 6;           // ← the N/A flag
  // ...
}
```

In the DB model (`model.Scores`):

```go
NotApplicable: sql.NullBool{Valid: true, Bool: true}
NumericValue:  sql.NullFloat64{}  // cleared/invalid
```

### N/A Flow Through the Scoring Pipeline

#### Step 1: NumericValue is cleared

`scorecard_calculator.go:50-52`:

```go
for _, newScore := range scores {
    if newScore.NotApplicable.Bool {
        newScore.NumericValue = sql.NullFloat64{}  // invalidated
    }
}
```

#### Step 2: ComputeCriterionPercentageScore returns nil

`scorecard_scores_dao.go:562-595`:

```go
func ComputeCriterionPercentageScore(...) ([]*CriterionPercentageScore, error) {
    scoreNotApplicable := false
    scoreValidNumericValueList := make([]float64, 0)

    for _, score := range scores {
        if score.NotApplicable.Bool {
            scoreNotApplicable = true
        }
        if score.NumericValue.Valid {
            scoreValidNumericValueList = append(...)
        }
    }

    // If ANY score is N/A OR no valid values → return nil
    if scoreNotApplicable || len(scoreValidNumericValueList) == 0 {
        return nil, nil  // ← criterion completely skipped
    }
    // ... normal computation
}
```

**Important**: If even ONE score for a criterion is N/A (e.g., in multi-score scenarios), the **entire criterion** is skipped.

#### Step 3: ComputeScores skips the criterion

`scorecard_calculator.go:75-81`:

```go
percentageScores, err := ComputeCriterionPercentageScore(criterion, scores)
if len(percentageScores) == 0 {
    continue  // ← no contribution to chapter or overall
}
```

#### Step 4: Weight excluded from aggregation

Since the criterion is skipped, its weight is never added to the denominator:

```go
// This never executes for N/A criteria:
summary.Total += score × weight
summary.Weight += weight
```

### Net Effect


| Scenario                          | Result                                     |
| --------------------------------- | ------------------------------------------ |
| 1 of 3 criteria is N/A            | Overall = weighted avg of the other 2 only |
| All criteria in a chapter are N/A | Chapter has no score                       |
| All criteria are N/A              | Overall scorecard has no valid score       |


### Auto-QA N/A

Auto-QA can also produce N/A outcomes (`AutoScoreOutcome_NOT_APPLICABLE`). This only works when the criterion has `showNA: true` — otherwise the N/A outcome is ignored (`autoqa_scoring_test.go:123-137`).

### N/A as Branch Condition

Branches can trigger on N/A selection:

```json
{
  "identifier": "branch-1",
  "condition": { "not_applicable": true },
  "children": [...]
}
```

## 5. Auto-Fail Interaction

Auto-fail is checked **before** N/A handling in `ComputeScores()`:

```go
// scorecard_calculator.go:64-68
scoreAutoFailed := criterion.IsValueAutoFailed(numericValues)
// ... then later:
// scorecard_calculator.go:75
percentageScores, err := ComputeCriterionPercentageScore(criterion, scores)
```

But since N/A clears `NumericValue` at line 50-52, `numericValues` will be empty for N/A scores, so auto-fail won't trigger.

## 6. ExcludeFromQAScores vs N/A

These are different mechanisms:


|               | ExcludeFromQAScores                      | N/A                                                    |
| ------------- | ---------------------------------------- | ------------------------------------------------------ |
| Who decides   | **Admin** configures per-criterion in template builder (UI: "Evaluate scores" toggle) | **Grader** decides per-criterion per-scorecard when scoring |
| Granularity   | Per-criterion setting in the template | Per-criterion, per-scorecard decision |
| When applies  | Fixed at template design time — applies to every scorecard graded with this template | Only the specific scorecard where grader selects N/A   |
| Overridable?  | No — grader cannot override at scoring time | Yes — grader chooses per scorecard              |
| Analytics     | Filtered out in `getScoreableCriteria()` | Score row exists but has no percentage contribution    |
| Weight        | Never counted                            | Not counted only when selected                         |


## 7. UI ↔ Backend Name Mapping

The template builder UI (in `director/packages/director-app/src/features/admin/coaching/template-builder/`) uses different names than the backend structs. This is a common source of confusion.

### Criterion Type Names


| Backend (`CriterionType`) | UI Label ("Score Type") | Source                             |
| ------------------------- | ----------------------- | ---------------------------------- |
| `numeric-radios`          | **Number Scale**        | `TemplateBuilderScoreType.tsx:206` |
| `labeled-radios`          | **Button Select**       | `TemplateBuilderScoreType.tsx:207` |
| `dropdown-numeric-values` | **Dropdown**            | `TemplateBuilderScoreType.tsx:209` |


### Settings Fields


| Backend Field                     | UI Label                                  | UI Component                                                   | Notes                                                        |
| --------------------------------- | ----------------------------------------- | -------------------------------------------------------------- | ------------------------------------------------------------ |
| `weight`                          | **Weight**                                | `TemplateBuilderCriterionConfiguration.tsx:301`                |                                                              |
| `displayName`                     | **Display Name**                          | `TemplateBuilderCriterionConfiguration.tsx:213`                |                                                              |
| `shortName`                       | **Short Name**                            | `TemplateBuilderCriterionConfiguration.tsx:266`                |                                                              |
| `helpText`                        | **Help Text**                             | `TemplateBuilderCriterionConfiguration.tsx:246`                |                                                              |
| `required`                        | **Required**                              | `TemplateBuilderCriterionControls.tsx:113`                     |                                                              |
| `settings.showNA`                 | **Allow N/A**                             | `CriteriaLabeledOptions.tsx:67`, `CriteriaRangeOptions.tsx:67` |                                                              |
| `settings.excludeFromQAScores`    | **Evaluate scores**                       | `TemplateBuilderScoreType.tsx:272`                             | ⚠️ **Inverted!** UI toggle ON = `excludeFromQAScores: false` |
| `settings.excludeOutcomeInsights` | **Exclude from Outcome Insights**         | `TemplateBuilderCriterionConfiguration.tsx:382`                |                                                              |
| `settings.autoFail`               | **Auto Fail**                             | `TemplateBuilderCriterionConfiguration.tsx:321`                |                                                              |
| `settings.autoFail.comparator`    | **Less than** / **More than** / **Equal** | `TemplateBuilderAutoFail.tsx:26-35`                            |                                                              |
| `settings.enableMultiSelect`      | **Allow multi-select**                    | `TemplateBuilderScoreType.tsx:303`                             | Dropdown only                                                |
| `settings.scores`                 | **Score** (column header)                 | `CriteriaLabeledOptions.tsx:143`                               | The value→score mapping                                      |
| `settings.options[].label`        | **Value** (column header)                 | `CriteriaLabeledOptions.tsx:137`                               | ⚠️ Confusing: "Value" in UI = `label` in BE                  |
| `settings.options[].value`        | (numeric input next to label)             | `CriteriaLabeledOptions.tsx`                                   | The integer value                                            |
| `settings.range.min`              | **From**                                  | `CriteriaRangeOptions.tsx:53`                                  | NumericRadios only                                           |
| `settings.range.max`              | **To**                                    | `CriteriaRangeOptions.tsx:53`                                  | NumericRadios only                                           |


### Fields NOT Exposed in UI


| Backend Field         | Notes                                                   |
| --------------------- | ------------------------------------------------------- |
| `displayCommentField` | Set to `undefined` in `consts.ts:46`, not user-editable |
| `perMessage`          | Set to `undefined` in `consts.ts:49`, not user-editable |
| `notRemovable`        | System flag for locked criteria                         |
| `identifier`          | Auto-generated internal ID                              |


### Key Confusing Points

1. **"Evaluate scores" is inverted** — UI toggle ON means `excludeFromQAScores: false` (criterion IS included in QA scores)
2. **"Value" column in UI = `options[].label` in backend** — the text label shown to the user is called "Value" in the UI but `label` in the data model
3. **"Score" column in UI = `scores[].score` in backend** — this is the mapped score, not the raw option value
4. **Criterion type names differ** — "Number Scale" vs `numeric-radios`, "Button Select" vs `labeled-radios`, "Dropdown" vs `dropdown-numeric-values`

### i18n Source

All labels from: `director/packages/director-app/locales/en-US/director-app-admin.json` (lines 3671-4230)

Key prefix: `template-builder.configuration.{section}.{field-key}`

---

## 8. Summary: What Would Change for "N/A with a Score"

Today's invariant: **N/A = no score, no weight contribution**.

The proposed change would break this invariant by allowing N/A to carry a configurable score value. Key areas that would need modification:

1. **Template settings** (`CriterionWithValueSettings`): Add N/A score field alongside `showNA`
2. **ComputeCriterionPercentageScore()**: Instead of returning `nil` for N/A, compute a percentage from the configured N/A score
3. **ComputeScores() N/A clearing**: Don't clear NumericValue when N/A has a configured score
4. **Auto-fail**: Decide if N/A-with-score should trigger auto-fail checks
5. **Value-score mapping**: How does N/A score interact with the existing mapping?
6. **Multi-select**: Can N/A coexist with other selections?
7. **Auto-QA**: Should NOT_APPLICABLE outcome produce the N/A score instead of skipping?
8. **Analytics/ClickHouse**: Score rows would now have percentage values for N/A criteria
9. **Branch conditions**: N/A branch conditions still need to fire

