# Design: Giving N/A a Score

**Created:** 2026-03-26
**Updated:** 2026-04-01
**Status:** Draft

## Problem

Today, when a grader selects N/A for a criterion, it is completely excluded from scoring (no score, no weight in aggregation). Some customers want N/A to carry a configurable score value instead.

Additionally, AutoQA currently maps NOT_APPLICABLE outcomes to `Score.NotApplicable = true` unconditionally (autoqa_mapper.go:81-82). There is no way to associate NOT_APPLICABLE with a specific criterion option in the Opera Integration → Behaviour section. Only DETECTED and NOT_DETECTED can be mapped to criterion options.

---

## Background: Option Wiring Architecture

Understanding how options, scores, and AutoQA are wired together is critical to the design.

### Option → Score Mapping

```
LabeledCriterionSettingOption          CriterionScoreOption
(scorecard_templates.go:318-322)       (scorecard_templates.go:325-328)
┌───────────┬────────────┐            ┌──────────────┬──────────────┐
│ Label     │ Value      │ ──wired──▶ │ Value        │ Score        │
│ string    │ int        │    by      │ float64      │ float64      │
├───────────┼────────────┤  value     ├──────────────┼──────────────┤
│ "Yes"     │   1        │            │   1          │  10          │
│ "No"      │   0        │            │   0          │   0          │
└───────────┴────────────┘            └──────────────┴──────────────┘
```

- `LabeledCriterionSettingOption.Value` (`int`) is an internally generated index (auto-assigned by FE)
- `CriterionScoreOption.Value` (`float64`) matches the option value — this is the wiring key
- Note: types differ (`int` vs `float64`) but represent the same logical integer; verified by `GetCriterionLabelForValue()` (line 206-215) which matches `option.Value == value`

### AutoQA → Option Mapping

```
AutoQAConfig (scorecard_templates.go:224-229)
┌──────────────┬──────────┐
│ Detected     │ *int = 1 │ ──▶ maps to option value=1 ("Yes") via nilOrFloat32()
│ NotDetected  │ *int = 0 │ ──▶ maps to option value=0 ("No") via nilOrFloat32()
└──────────────┴──────────┘
```

- `AutoQAConfig.Detected` / `NotDetected` (`*int`, json: `"detected"` / `"not_detected"`) store criterion option values
- In `autoqa_mapper.go:69-79`, `nilOrFloat32()` converts `*int` → `*float32`, set as `Score.NumericValue`
- NOT_APPLICABLE (line 81-82) currently has NO option mapping — always sets `Score.NotApplicable = true`
- For # of Occurrences mode: `AutoQAOptions.Value` (`float32`, json: `"value"`) is used instead

### Wiring Key Type Summary

| Component | Field | Go Type | JSON | Role |
|-----------|-------|---------|------|------|
| `LabeledCriterionSettingOption` | `Value` | `int` | `"value"` | Option identifier |
| `CriterionScoreOption` | `Value` | `float64` | `"value"` | Score lookup key |
| `AutoQAConfig` | `Detected`/`NotDetected` | `*int` | `"detected"`/`"not_detected"` | AutoQA → option mapping |
| `AutoQAOptions` | `Value` | `float32` | `"value"` | # of Occurrences option value |
| `Score.NumericValue` | — | `*float32` | — | Runtime score value |

Types are inconsistent (`int`, `*int`, `float64`, `float32`) but represent the same logical integer. Conversions happen at runtime via `nilOrFloat32()` (autoqa_mapper.go:144-150).

### Key Insight

All three systems (options, scores, AutoQA) use the same logical integer as the wiring key, despite different Go types. Adding a new scoreable option means adding entries to all three with a consistent value.

---

## Current Approach: N/A as Option with `isNA` Flag (D+C)

**Rationale (2026-04-01):**

1. AutoQA outcome types must wire to options via `value` → N/A must be a real option in the options/scores arrays
2. Labels are i18n-dependent → cannot reliably identify the N/A option by label string → need `isNA` flag
3. N/A as a real option means **zero BE scoring pipeline changes** — scoring sees a normal option

### Template Data Model

```
Template Settings (JSONB)
├── showNA: true                              ← controls N/A button in grader UI
├── options: [
│   { label: "Yes", value: 0 },
│   { label: "No",  value: 1 },
│   { label: "N/A", value: 2, isNA: true }   ← NEW: isNA flag identifies N/A option
│ ]
├── scores: [
│   { value: 0, score: 10 },
│   { value: 1, score: 0 },
│   { value: 2, score: 5 }                   ← N/A score, normal entry
│ ]

AutoQAConfig
├── detected: 0
├── not_detected: 1
├── not_applicable: 2                         ← NEW: maps NOT_APPLICABLE to N/A option
```

- `showNA` stays `true` — controls whether the grader sees the N/A button
- The `isNA` option is a regular option with a score — scoring pipeline treats it normally
- `AutoQAConfig.NotApplicable` wires NOT_APPLICABLE outcome to the N/A option value

### Grader Submission (Manual Grading)

**Current grader flow** (`CriterionInputDisplay.tsx → utils.ts:getPartialScoreForNumericValue()`):

- N/A button uses sentinel `INPUT_N_A_VALUE = '__director_n/a_value__'`
- When sentinel detected + `showNA`: submits `{ notApplicable: true, numericValue: null }`
- Normal option: submits `{ notApplicable: false, numericValue: <value> }`

**New grader flow** (when `isNA` option exists):

- N/A button still uses the same sentinel `INPUT_N_A_VALUE`
- When sentinel detected + `isNA` option exists: submits `{ notApplicable: true, numericValue: <isNA_option_value> }`
- When sentinel detected + no `isNA` option (legacy): submits `{ notApplicable: true, numericValue: null }` (unchanged)

Submitting **both** `notApplicable: true` AND `numericValue` ensures:

- Analytics can distinguish scored N/A from legacy N/A via `not_applicable = true AND numeric_value IS NOT NULL`
- The `not_applicable` flag is preserved for audit trail and analytics filtering

### AutoQA Submission

**Current**: `autoqa_mapper.go:81-82` always sets `NotApplicable = true` for NOT_APPLICABLE outcome.

**New**: When `AutoQAConfig.NotApplicable` is configured:

```go
case autoqapb.AutoScoreOutcome_NOT_APPLICABLE:
    if autoQaConfig.NotApplicable != nil {
        mappedScore.NotApplicable = true   // preserve N/A flag for analytics
        mappedScore.NumericValue = nilOrFloat32(autoQaConfig.NotApplicable)
    } else {
        mappedScore.NotApplicable = true   // legacy behavior
    }
```

Both manual and AutoQA paths produce: `not_applicable: true, numeric_value: <value>`.

### BE Scoring Pipeline Changes

Two changes needed to handle `not_applicable: true` + `numeric_value: <value>`:

**Change 1: `ComputeScores()` — scorecard_calculator.go:50-52**

Current code unconditionally clears `NumericValue` for N/A:

```go
if newScore.NotApplicable.Bool {
    newScore.NumericValue = sql.NullFloat64{}  // always clears
}
```

New code — don't clear when `NumericValue` is already set (scored N/A):

```go
if newScore.NotApplicable.Bool && !newScore.NumericValue.Valid {
    newScore.NumericValue = sql.NullFloat64{}  // only clear for legacy N/A
}
```

**Change 2: `ComputeCriterionPercentageScore()` — scorecard_scores_dao.go:582-584**

Current code skips when any score is N/A:

```go
if scoreNotApplicable || len(scoreValidNumericValueList) == 0 {
    return nil, nil
}
```

New code — only skip when N/A AND no valid numeric values:

```go
if scoreNotApplicable && len(scoreValidNumericValueList) == 0 {
    return nil, nil  // legacy N/A (no numeric value) — skip as before
}
// scored N/A: has numeric value, proceed to normal scoring
```

**Why `||` → `&&` is safe**: Legacy N/A always has `NumericValue = NULL` (empty list), so `scoreNotApplicable = true && len(...) == 0` still returns nil. Scored N/A has valid `NumericValue`, so it falls through to normal percentage calculation.

### What Gets Stored in DB


| Case          | `not_applicable` | `numeric_value` | `percentage_value` (CH) | `float_weight` (CH) |
| ------------- | ---------------- | --------------- | ----------------------- | ------------------- |
| Normal option | `false`          | `<value>`       | `<computed>`            | `<weight>`          |
| Legacy N/A    | `true`           | `NULL`          | `-1` (sentinel)         | `0`                 |
| Scored N/A    | `true`           | `<isNA value>`  | `<computed>`            | `<weight>`          |


Analytics can distinguish: `WHERE not_applicable = true AND numeric_value IS NOT NULL` → scored N/A.

### Analytics Compatibility (Verified)

The analytics service (`insights-server/internal/analyticsimpl/`) handles scored N/A correctly with **zero changes**:

**Row filtering** (`common_clickhouse.go:622-637`):

- `includeNaScored = false` → `not_applicable <> true` excludes ALL N/A rows (legacy + scored). Correct.
- `includeNaScored = true` → no N/A filter, rows included.

**Aggregation** (`retrieve_qa_score_stats_clickhouse.go:151-152`):

```sql
SUM(percentage_value * float_weight) FILTER (WHERE percentage_value >= 0)
SUM(float_weight) FILTER (WHERE percentage_value >= 0)
```

- Legacy N/A: `percentage_value = -1` → excluded from SUM (even when `includeNaScored = true`)
- Scored N/A: `percentage_value >= 0` → **included in SUM** when `includeNaScored = true`

**Individual scores** (`retrieve_qa_conversations_clickhouse.go`): Returns both `not_applicable` and `numeric_value` — consumers can distinguish all three cases.

### Three Modes


| `showNA` | `isNA` option          | AutoQA `NotApplicable` | Manual Grading                      | AutoQA Grading                                 |
| -------- | ---------------------- | ---------------------- | ----------------------------------- | ---------------------------------------------- |
| `true`   | none                   | nil                    | N/A → `{na: true, nv: null}` → skip | NOT_APPLICABLE → `{na: true, nv: null}` → skip |
| `true`   | `{value: 2, score: 5}` | nil                    | N/A → `{na: true, nv: 2}` → scored  | NOT_APPLICABLE → `{na: true, nv: null}` → skip |
| `true`   | `{value: 2, score: 5}` | `2`                    | N/A → `{na: true, nv: 2}` → scored  | NOT_APPLICABLE → `{na: true, nv: 2}` → scored  |


### UI Specification

When "Allow N/A" is **unchecked** → everything unchanged.

When "Allow N/A" is **checked**, four areas are affected:

#### Area 1: Preview Section (Grader N/A Button)

**Location**: `CriterionInputDisplay.tsx` — `addNAOption()` / `getPartialScoreForNumericValue()`

**Change**: When `isNA` option exists, N/A button submits `{ notApplicable: true, numericValue: <isNA_value> }` instead of `{ notApplicable: true, numericValue: null }`. Visual appearance unchanged.

#### Area 2: Criterion Scoring Details — N/A Row

**Location**: `CriteriaLabeledOptions.tsx`

When "Allow N/A" is checked, a new row appears at the bottom of the options table:

```
┌─────────────────────────────────────────────────┐
│  Value (label)                    │  Score       │
├───────────────────────────────────┼──────────────┤
│  [Yes              ]              │  [10 ] [🗑]  │
│  [No               ]              │  [0  ] [🗑]  │
│  [N/A          ] (disabled)       │  [no score]  │  ← NEW
├───────────────────────────────────┴──────────────┤
│                    ☑ Allow N/A      [+ Add Option]│
└─────────────────────────────────────────────────┘
```

- **Label**: disabled TextInput, fixed to "N/A"
- **Score**: NumberInput, placeholder "no score", editable integer
- **No delete button** (controlled by "Allow N/A" toggle)

On score enter → create `isNA` option + score entry in options/scores arrays.
On score clear → remove `isNA` option/score entries (reverts to legacy N/A).
On "Allow N/A" uncheck → remove `isNA` option/score, clear `showNA`.

#### Area 3: Opera Integration → Behavior — NOT_APPLICABLE Association

**Location**: `TemplateBuilderAutoQA.tsx:379-437`

New third row (when "Behavior Done/Not Done" is selected):

- "If behavior is done" → Select dropdown (maps to `auto_qa.detected`)
- "If behavior is not done" → Select dropdown (maps to `auto_qa.not_detected`)
- "If the behavior is not applicable" → Select dropdown (maps to `auto_qa.not_applicable`) ← NEW

Same `behaviorScoreSelectionOptions` (all criterion options including N/A).

#### Area 4: Opera Integration → # of Occurrences — N/A Entry

**Location**: `NumericBinsAndValuesConfigurator.tsx`

When "Allow N/A" is checked, auto-add a read-only N/A card at the bottom:

```
┌──────────────────────────────────────────────┐
│  [N/A  ] (disabled)   [<score>] (disabled)   │
│  (no value range — N/A is a special outcome) │
└──────────────────────────────────────────────┘
```

- Label/Score: auto-filled from the `isNA` option's config, disabled
- No value range, no delete button
- Read-only reflection of Area 2

### BE Changes Summary

#### Template Structs

- Add `IsNA *bool` to `LabeledCriterionSettingOption` (`json:"isNA,omitempty"`)
- Add `NotApplicable *int` to `AutoQAConfig` (`json:"not_applicable,omitempty"`)

#### Scoring — 2 changes

1. `ComputeScores()` (scorecard_calculator.go:50-52): `&&` guard — don't clear `NumericValue` when already set
2. `ComputeCriterionPercentageScore()` (scorecard_scores_dao.go:582-584): `||` → `&&` — only skip when N/A AND no numeric values

#### AutoQA Mapper — 1 change

- `autoqa_mapper.go:81-82`: When `config.NotApplicable` set, map to `NumericValue` (keep `NotApplicable = true`)

#### Validation

- `isNA` option must have a score entry in scores array
- At most one `isNA` option per criterion

#### No changes needed

- `ComputeScores()` pre-processing logic (just add `&&` guard)
- `computeScore()` / `computeMultiSelectScore()` / `computePerMessageScore()`
- `mapToPercentageScore()` / `MapScoreValue()` / `updateSummaryValues()`
- Chapter/overall aggregation
- ClickHouse schema
- Analytics service queries

### FE Changes Summary

`**scoring.ts**` (director-api types):

- Add `isNA?: boolean` to `LabeledCriterionSettingOption` type (or wherever option type is defined)
- Add `not_applicable?: number | null` to `ScorecardTemplateAutoQA`

`**CriteriaLabeledOptions.tsx**` (Area 2):

- When `showNA` checked, render N/A row with disabled label + score input
- On score change: create/update option with `isNA: true` + matching score entry
- Gate behind `enableNAScore` feature flag

`**CriteriaRangeOptions.tsx**`:

- No change (numeric-radios has no options array — N/A stays as legacy skip)

`**utils.ts**` — `getPartialScoreForNumericValue()` (Area 1):

- When `isNA` option found: submit `{ notApplicable: true, numericValue: <isNA_value> }`

`**TemplateBuilderAutoQA.tsx**` (Area 3):

- Add NOT_APPLICABLE dropdown row

`**NumericBinsAndValuesConfigurator.tsx**` + `**NumericOutcomeRangeConfiguration.tsx**` (Area 4):

- Read-only N/A card

`**useSaveScorecardTemplate.ts**`:

- No special transform needed — `isNA` option is a regular option, passes through normally

### Remaining Concerns

**1. NumericRadios scored N/A (future work)**: NumericRadios has `range` {min, max} instead of `options[]`, so the D+C `isNA` option approach doesn't apply directly. The grader UI does show an N/A button (`showNA: true`), but N/A stays as legacy skip for now.

Three approaches explored for future support:
- **Approach A: `NAScore` on `CriterionWithValueSettings`** — Add a direct percentage field (0.0–1.0). `ComputeCriterionPercentageScore` checks: if N/A + `NAScore` set → return it. Simple but introduces a second scoring code path separate from D+C.
- **Approach B: Synthesize options array** — FE creates `options: [{label: "N/A", value: <max+1>, isNA: true}]` + `scores: [{value: <max+1>, score: <configured>}]` even for NumericRadios. `NumericRadiosCriterionSettings` already embeds `CriterionWithValueSettings` which has `Scores *[]CriterionScoreOption`. `MapScoreValue()` (`scorecard_scores_dao.go:775`) would find the value in scores → return the configured score → `percentage = score / maxScore`. Zero additional BE scoring changes. Piggybacks entirely on D+C.
- **Approach C: Skip** — Current state. NumericRadios N/A stays as legacy skip.

Approach B is preferred if we pursue this — it reuses the D+C pipeline with no BE changes. The only concern is that the synthetic option value must be outside the range to avoid collisions with real grader selections.

**2. Backward compatibility**: Legacy templates have `showNA: true` with no `isNA` option. They continue to work as before — grader submits `{ notApplicable: true, numericValue: null }` → `ComputeScores` clears value → `ComputeCriterionPercentageScore` skips.

**3. Design principle**: `ComputeScores()` is post-processing that persists to DB. The `&& !newScore.NumericValue.Valid` guard preserves the grader's `NumericValue` for scored N/A while still clearing it for legacy N/A. This is safe because graders only submit `NumericValue` for scored N/A when the `isNA` option exists.

**4. Pre-existing bug: stale AutoQA dropdown values on criterion recreate**. When a criterion is deleted and a new one is created at the same form array index, `auto_qa.detected`, `auto_qa.not_detected`, and `auto_qa.not_applicable` retain stale values from the deleted criterion. Root cause: `handleAddCriterion()` in `TemplateBuilderFormConfigurationStep.tsx` (line 279) creates a `newAutoQA` object with defaults but never spreads it into the new criterion passed to `form.setValue()` (line 319). The new criterion only contains `...DEFAULT_CRITERION` + `identifier` + `itemType`, with no `auto_qa` key — so react-hook-form fields at that path keep their previously registered values. This affects all three AutoQA wiring fields equally and predates the scored N/A feature.

---

## Historical Reference: Previous Approaches

Approach A: scoreableNA (Rejected)

Separate `scoreableNA` struct mutually exclusive with `showNA`. Rejected: value collision, dual-mode FE submission, identity problem.



Approach B: Pure naScore field

Single `NAScore *float64` on settings as direct percentage (0.0-1.0). Used by `ComputeCriterionPercentageScore()` only. Simple but no AutoQA wiring capability — `NOT_APPLICABLE` always skips. Superseded because we need AutoQA NOT_APPLICABLE → option association.

Key design principle: **separate mutations from calculations**. `ComputeScores()` (post-processing, persists to DB) should not be modified for N/A scoring. `ComputeCriterionPercentageScore()` (calculation layer, feeds CH) handles scoring.



Hybrid B+D: naScore field + N/A option

Combined Approach B (BE uses naScore for manual grading) with Approach D (FE creates N/A option for AutoQA wiring). FE dual-stores in both naScore and option/score arrays. Superseded by D+C which is simpler: single source of truth in the option/score arrays, no dual-store sync.



---

## Open Questions

All resolved.

1. ~~**NAScore semantics**~~: Resolved — N/A score is the `CriterionScoreOption.Score` value of the `isNA` option. Normalized to percentage by the normal scoring pipeline.
2. ~~**Migration**~~: Resolved — purely opt-in for new/edited templates. No migration needed.
3. ~~**N/A branch conditions**~~: Resolved — `validateChildrenRecursively()` (`scorecard_scores_dao.go:672-693`) checks both `numericMatch` (line 687) and `naMatch` (line 689), OR'd together (line 693). Scored N/A has `NotApplicable = true` AND valid `NumericValue`, so it can match both an N/A branch condition and a numeric-value branch condition simultaneously. This is correct behavior — the branch children become valid either way.
4. ~~**Auto-fail + scored N/A**~~: Resolved — admin configured both the N/A score and auto-fail threshold. If scored N/A triggers the threshold, auto-fail should fire. Expected behavior.
5. ~~**Multi-select + scored N/A**~~: Resolved — N/A button is exclusive in the grader UI by existing behavior. No change needed.
6. ~~**PerMessage + scored N/A**~~: Resolved — scoring runs per-score independently. Works naturally.

