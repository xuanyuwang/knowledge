# Design: Giving N/A a Score

**Created:** 2026-03-26
**Updated:** 2026-03-27
**Status:** Draft

## Problem

Today, when a grader selects N/A for a criterion, it is completely excluded from scoring (no score, no weight in aggregation). Some customers want N/A to carry a configurable score value instead.

---

## Approach A: `scoreableNA` as Separate Option (Rejected)

Add a new field `scoreableNA` to `CriterionWithValueSettings` that is mutually exclusive with `showNA`. N/A becomes a regular scored option — grader submits `numeric_value`, not `not_applicable: true`.

**Rejected because:**
- **Value collision problem**: Need to pick a numeric value for the N/A option that doesn't collide with existing `options[].value`. No clean way to guarantee uniqueness since values are user-configurable.
- **Identity problem**: Need to mark the option as "special" (it's N/A, not a real option) while making it look "normal" to the scoring pipeline. Putting it in the `scores` array means no calc changes, but then it's indistinguishable from real options. Putting it outside means calc changes are needed anyway.
- **FE complexity**: Grader UI needs to know whether to submit `numeric_value` or `not_applicable: true` depending on which mode is active.

<details>
<summary>Full details of rejected approach</summary>

### Template Settings Change

```go
type ScoreableNAOption struct {
    Value int     `json:"value"`
    Score float64 `json:"score"`
}
```

### Three Modes

| State | Settings | Grader Submits | Scoring |
|-------|----------|----------------|---------|
| No N/A | `showNA: false`, no `scoreableNA` | — | Normal |
| Legacy N/A | `showNA: true`, no `scoreableNA` | `not_applicable: true` | Skip |
| Scored N/A | `showNA: false`, `scoreableNA: {value, score}` | `numeric_value: <value>` | Normal (zero calc changes) |

Pros: zero scoring code changes.
Cons: value collision, FE dual-mode submission, mutual exclusivity enforcement.

</details>

---

## Approach B: `NAScore` Field Alongside `showNA` (Current)

Add a single new field `NAScore` next to `showNA`. The two fields work together:

### Template Settings Change

```go
type CriterionWithValueSettings struct {
    ShowNA                 *bool                   `json:"showNA"`
    NAScore                *float64                `json:"naScore,omitempty"`  // NEW
    AutoFail               *AutoFailConfig         `json:"autoFail,omitempty"`
    ExcludeOutcomeInsights *bool                   `json:"excludeOutcomeInsights"`
    ExcludeFromQAScores    *bool                   `json:"excludeFromQAScores"`
    Scores                 *[]CriterionScoreOption `json:"scores"`
    EnableMultiSelect      *bool                   `json:"enableMultiSelect"`
}
```

### Three Combinations

| `showNA` | `NAScore` | Meaning | Grader Submits | Scoring |
|----------|-----------|---------|----------------|---------|
| `false`/nil | nil | No N/A option | — | Normal |
| `true` | nil | Legacy N/A | `not_applicable: true` | **Skip** — excluded from scoring |
| `true` | `<number>` | Scored N/A | `not_applicable: true` | **Scored** — NAScore used as percentage, weight included |

Invalid: `showNA: false` + `NAScore: <number>` → BE rejects (N/A not enabled, score meaningless).

### Why This Is Better

1. **No value collision**: Grader still submits `not_applicable: true`. No need to pick a fake numeric value.
2. **No FE grader change**: Grader UI behaves exactly the same — selects N/A, submits `not_applicable: true`. BE resolves whether to skip or score.
3. **Simple data model**: One new `float64` field. No new structs.
4. **Backward compatible**: `NAScore: nil` → all existing behavior unchanged.

### Scoring Code Changes

#### Design Principle: Separate Mutations from Calculations

The scoring pipeline has two distinct layers:

1. **Post-processing layer** — `ComputeScores()` (scorecard_calculator.go) enriches raw grader input with metadata: `Chapter`, `AiScored`, `AiValue`, `NumericValue` (cleared for N/A), `AutoFailed`. These mutations are **persisted to DB** via `UpdateScorecardAndScoresInDB()`. This is post-processing of scorecard user input, not pure calculation.

2. **Calculation layer** — `ComputeCriterionPercentageScore()` (scorecard_scores_dao.go) computes derived percentage values. These feed into `updateSummaryValues()` for chapter/overall aggregation and are written to the ClickHouse sync model, but do NOT modify the original PG score rows.

**We must NOT modify `NotApplicable` or `NumericValue` for NAScore in the post-processing layer** — doing so would overwrite the grader's original N/A selection in PG, losing the audit trail. If the template's NAScore config is later changed/removed, historical scores would be corrupted.

Instead, NAScore is handled in the **calculation layer** (`ComputeCriterionPercentageScore`) which computes derived values without modifying stored score rows.

#### No Change: `ComputeScores()` — scorecard_calculator.go:50-52

The existing N/A pre-processing stays as-is:

```go
if newScore.NotApplicable.Bool {
    newScore.NumericValue = sql.NullFloat64{}  // unchanged — clears value, persisted to DB
}
```

DB rows for N/A scores continue to have `not_applicable = true`, `numeric_value = NULL`.

#### Change: `ComputeCriterionPercentageScore()` — scorecard_scores_dao.go:562-596

Currently, N/A causes an early return at lines 582-584:

```go
// BEFORE
if scoreNotApplicable || len(scoreValidNumericValueList) == 0 {
    return nil, nil  // criterion skipped entirely
}
```

With NAScore, check if the criterion has a configured NAScore before skipping:

```go
// AFTER
if scoreNotApplicable || len(scoreValidNumericValueList) == 0 {
    if scoreNotApplicable {
        naScore := criterion.GetNAScore()
        if naScore != nil {
            // Scored N/A: return the configured percentage directly
            weight := float64(criterion.GetWeight())
            return []*CriterionPercentageScore{{
                PercentageScore: &sql.NullFloat64{Float64: *naScore, Valid: true},
                Weight:          weight,
            }}, nil
        }
    }
    return nil, nil  // legacy N/A or no valid values — skip as before
}
```

This is the **only scoring code change**. The rest of the pipeline (`computeScore`, `computeMultiSelectScore`, `computePerMessageScore`, `mapToPercentageScore`, `updateSummaryValues`, chapter aggregation) is untouched.

#### New Interface Method

```go
// On ScorecardTemplateCriterion interface
GetNAScore() *float64
```

Implementation checks `settings.NAScore`.

### Percentage Calculation for NAScore

NAScore is a **direct percentage value** (0.0 – 1.0), returned directly as `PercentageScore` from `ComputeCriterionPercentageScore`. It does NOT go through value-score mapping (`GetValueScores()` / `MapScoreValue()` / `mapToPercentageScore()`).

Example:
- Admin sets NAScore = 0 → N/A contributes 0% to the weighted average
- Admin sets NAScore = 0.5 → N/A contributes 50%
- Admin sets NAScore = 1.0 → N/A contributes 100%

### What Gets Stored in DB vs What Gets Calculated

| Field | PG (score row) | CH (scorecard_score / score) | Source |
|-------|---------------|------------------------------|--------|
| `not_applicable` | `true` (grader's choice, preserved) | `true` (synced from PG) | Grader input |
| `numeric_value` | `NULL` (unchanged from today) | `0` (default) | Post-processing clears it |
| `percentage_value` | N/A (not in PG scores table) | `<NAScore>` or `0`/`-1` (legacy) | Calculation layer → CH sync |
| `float_weight` | N/A (not in PG scores table) | `<criterion weight>` or `0` (legacy) | Calculation layer → CH sync |

**Key insight**: `percentage_value` and `float_weight` are NOT stored in PG score rows — they're computed by `ComputeCriterionPercentageScore()` and written directly to the CH sync model (scorecard_scores_dao.go:463-469). This means the calculation layer's output flows to CH without touching PG, which is exactly why it's safe to handle NAScore there.

To distinguish scored vs legacy N/A in analytics: `WHERE not_applicable = true AND percentage_value > 0`.

### FE Changes

#### Current State

- "Allow N/A" toggle is **only available for `labeled-radios` and `dropdown-numeric-values`** criterion types (not `numeric-radios`)
- Toggling ON only adds: an N/A **button** (labeled-radios) or N/A **dropdown option** (dropdown-numeric-values) in the grader preview
- There is **no config UI** for N/A value or score — only the N/A preview appears during config

#### Template Builder — New Behavior

When "Allow N/A" is toggled ON, the options config table should include an **N/A row** at the bottom, similar to other options but with special behavior:

1. **Label input**: disabled, fixed to "N/A" (not editable by admin)
2. **Score input**: editable number field, displays "no score" placeholder by default
3. When admin enters a score → set `NAScore: <value>` in settings
4. When admin clears the score → set `NAScore: nil` (reverts to legacy skip behavior)
5. When admin toggles OFF "Allow N/A" → remove the N/A row, clear both `showNA` and `NAScore`

The N/A row should look like other option rows (consistent UI) except for the disabled label.

#### Opera Integration Section

The N/A score config must also be **synced to the Opera Integration section** of the criterion config panel. If N/A has an associated Auto-QA trigger (policy/moment), the configured NAScore should be reflected there when the NOT_APPLICABLE outcome maps to a score.

#### FE Template Parsing

FE parses the template JSON from API response to:
- Display the N/A config row with current NAScore value (or "no score" if nil)
- On save, serialize `showNA` and `NAScore` back into the settings JSON sent to BE

#### Grader UI

No changes. Grader selects N/A → submits `not_applicable: true` as today. BE handles the rest.

### BE Changes

#### Template Settings
- Add `NAScore *float64` to `CriterionWithValueSettings`
- Add `GetNAScore() *float64` to `ScorecardTemplateCriterion` interface
- Implement on all criterion types (returns `settings.NAScore`)

#### Validation
- Reject `showNA: false` + `NAScore != nil` (N/A not enabled but score set)
- NAScore must be >= 0 (negative scores don't make sense)

#### Scoring — scorecard_scores_dao.go
- In `ComputeCriterionPercentageScore()`, before the N/A early return: check `GetNAScore()`, if non-nil return the configured percentage directly
- `ComputeScores()` (scorecard_calculator.go) is **NOT modified** — N/A pre-processing stays as-is, DB rows preserve grader's N/A selection

#### Auto-QA
- No changes to the auto-QA mapper. It still produces `NOT_APPLICABLE` outcome.
- The calculation layer (`ComputeCriterionPercentageScore`) handles the NAScore resolution.

#### Analytics / ClickHouse
- **No schema changes needed.** Both `scorecard_score` and `score` tables already have the necessary columns:
  - `not_applicable Bool` — synced from PG `score.NotApplicable.Bool` (scorecard_scores_dao.go:456)
  - `percentage_value Float64` — written from `ComputeCriterionPercentageScore()` output (scorecard_scores_dao.go:466)
  - `float_weight Float64` — written from percentage score weight (scorecard_scores_dao.go:468)
- **Today (legacy N/A)**: `not_applicable = true`, `percentage_value = 0`/`-1` (sentinel), `float_weight = 0`
- **With scored N/A**: `not_applicable = true`, `percentage_value = <NAScore>`, `float_weight = <weight>`
- No query changes needed. Queries using `percentage_value` will naturally pick up scored N/A values. Queries filtering `not_applicable = true` still identify all N/A scores.
- To distinguish scored vs legacy N/A in analytics: `not_applicable = true AND percentage_value > 0`

### What Stays Unchanged

| Component | Changed? |
|-----------|----------|
| `ComputeScores()` pre-processing | ❌ No — DB rows preserve grader's N/A selection |
| `computeScore()` / `computeMultiSelectScore()` / `computePerMessageScore()` | ❌ No |
| `mapToPercentageScore()` / `MapScoreValue()` | ❌ No |
| `updateSummaryValues()` | ❌ No |
| Chapter/overall aggregation | ❌ No |
| ClickHouse queries | ❌ No |
| `RetrieveQAScoreStats` | ❌ No |
| Auto-QA mapper | ❌ No |
| FE grader UI | ❌ No |
| `ComputeCriterionPercentageScore()` | ✅ Yes — NAScore check before N/A early return |
| Template settings struct | ✅ Yes — 1 new field |
| Criterion interface | ✅ Yes — 1 new method |
| Template builder UI | ✅ Yes — NAScore input |
| Validation | ✅ Yes — showNA/NAScore consistency |

---

## Approach C: N/A as an Explicit Option with `isNA` Flag (Alternative)

Instead of a separate field, let the admin add an option entry with a reserved `isNA` flag:

### Template Settings Change

```go
type LabeledCriterionSettingOption struct {
    Label string `json:"label"`
    Value int    `json:"value"`
    IsNA  *bool  `json:"isNA,omitempty"`  // NEW
}
```

### How It Works

- Admin toggles "Allow N/A" → adds an option with `isNA: true`, label fixed to "N/A", value and score configurable
- Grader selects "N/A" → submits `numeric_value: <value>`, `not_applicable: false` (it's just an option)
- Scoring pipeline sees a normal option → zero calc changes
- To revert to legacy skip N/A: remove the `isNA` option, set `showNA: true`

### Example

```json
{
  "settings": {
    "showNA": false,
    "options": [
      { "label": "Yes", "value": 1 },
      { "label": "No", "value": 0 },
      { "label": "N/A", "value": -1, "isNA": true }
    ],
    "scores": [
      { "value": 1, "score": 10 },
      { "value": 0, "score": 0 },
      { "value": -1, "score": 5 }
    ]
  }
}
```

### Pros
- Zero scoring code changes — N/A is just another option
- Reuses existing options/scores infrastructure
- Per-criterion granularity
- No value collision if admin picks a value not used by other options (FE can auto-assign)

### Cons
- **Only works for LabeledRadios and Dropdown** — NumericRadios uses `range`, not `options[]`, so this approach doesn't apply
- Mixes N/A into the regular options list — could be confusing in the template builder
- `showNA` and `isNA` option coexistence needs careful handling (mutually exclusive, like Approach A)
- FE grader change needed: submit `numeric_value` instead of `not_applicable: true` when the N/A option is selected

---

## Approach Comparison

| | A: scoreableNA | B: NAScore | C: isNA option flag |
|---|---|---|---|
| New fields | New struct + field | 1 float64 field | 1 bool on existing struct |
| Value collision risk | ⚠️ Yes | ✅ None | ⚠️ Mild (FE can auto-assign) |
| Scoring code changes | 0 | 1 spot | 0 |
| FE grader changes | ⚠️ Yes — dual-mode submission | ✅ None | ⚠️ Yes — submit value not N/A |
| FE builder changes | Yes | Yes | Yes |
| Auto-QA changes | ⚠️ Yes | ✅ None | ⚠️ Yes — mapper submits value |
| Works for NumericRadios | ✅ Yes | ✅ Yes | ❌ No (range, not options) |
| Semantic clarity | Clear (two modes) | Slightly coupled | Natural (N/A is just an option) |
| **Recommendation** | Rejected | **✅ Preferred** | Viable for labeled/dropdown |

---

## Open Questions

1. **NAScore semantics**: Is it a direct percentage (0.0–1.0), a raw value (like option values), or a mapped score (like `CriterionScoreOption.Score`)? Direct percentage is simplest — avoids all mapping logic.
2. **Migration**: Purely opt-in for new/edited templates. No migration of existing templates needed.
3. **N/A branch conditions**: When NAScore is set, `not_applicable: true` is still submitted by the grader, then flipped to `false` in scoring. Branch conditions checking `not_applicable` should fire **before** the scoring transform. Need to verify branch evaluation order.
4. **Auto-fail + scored N/A**: If NAScore produces a value that triggers auto-fail (e.g., NAScore=0 and auto-fail is "equal to 0"), should auto-fail fire? Probably yes — the admin configured both, and the score is real.
5. **Multi-select + NAScore**: N/A is typically exclusive. When grader selects N/A, no other options are selected. This should work naturally.
6. **PerMessage + NAScore**: Each message can independently be N/A with a score. Should work — Spot 1 transform runs per-score.
