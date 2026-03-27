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

### Scoring Code Changes (2 spots)

#### Spot 1: `ComputeScores()` — scorecard_calculator.go:50-52

Currently clears NumericValue for N/A scores. With NAScore, inject the score instead:

```go
for _, newScore := range scores {
    if newScore.NotApplicable.Bool {
        naScore := criterion.GetNAScore()  // NEW method
        if naScore != nil {
            // Scored N/A: inject NAScore as the numeric value
            newScore.NumericValue = sql.NullFloat64{Float64: *naScore, Valid: true}
            newScore.NotApplicable = sql.NullBool{Bool: false, Valid: true}  // treat as scored
        } else {
            // Legacy N/A: clear as before
            newScore.NumericValue = sql.NullFloat64{}
        }
    }
}
```

After this transform, the rest of the pipeline sees a regular scored criterion. No further changes needed.

#### Spot 2: `ComputeCriterionPercentageScore()` — scorecard_scores_dao.go:562-595

Actually, **no change needed here** if Spot 1 correctly transforms the score. By the time `ComputeCriterionPercentageScore` runs:
- `NotApplicable` is `false` (was flipped in Spot 1)
- `NumericValue` is valid (was set to NAScore in Spot 1)
- The existing code processes it normally

So effectively **only 1 code change spot** in the scoring pipeline.

#### New Interface Method

```go
// On ScorecardTemplateCriterion interface
GetNAScore() *float64
```

Implementation checks `settings.NAScore`.

### Percentage Calculation for NAScore

NAScore is a **direct percentage value** (0.0 – 1.0), not a raw value that goes through value-score mapping. This avoids needing to integrate with `GetValueScores()` / `MapScoreValue()`.

Example:
- Admin sets NAScore = 0 → N/A contributes 0% to the weighted average
- Admin sets NAScore = 0.5 → N/A contributes 50%
- Admin sets NAScore = 1.0 → N/A contributes 100%

The injected `NumericValue` in Spot 1 would be set to `NAScore * maxScore` so that the existing `scoreValue / maxScore` normalization produces the correct percentage. Alternatively, NAScore could bypass `MapScoreValue` entirely — this is a detail to decide during implementation.

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

#### Scoring — scorecard_calculator.go
- In `ComputeScores()`, before the main loop: when `NotApplicable` and `GetNAScore() != nil`, inject the score and flip `NotApplicable` to false (Spot 1 above)

#### Auto-QA
- No changes to the auto-QA mapper. It still produces `NOT_APPLICABLE` outcome.
- The scoring pipeline (Spot 1) handles the NAScore resolution when computing scores.

#### Analytics / ClickHouse
- No query changes. Score rows will have valid `percentage_value` and `float_weight` for scored N/A criteria.

### What Stays Unchanged

| Component | Changed? |
|-----------|----------|
| `ComputeCriterionPercentageScore()` | ❌ No |
| `computeScore()` / `computeMultiSelectScore()` / `computePerMessageScore()` | ❌ No |
| `mapToPercentageScore()` / `MapScoreValue()` | ❌ No |
| `updateSummaryValues()` | ❌ No |
| Chapter/overall aggregation | ❌ No |
| ClickHouse queries | ❌ No |
| `RetrieveQAScoreStats` | ❌ No |
| Auto-QA mapper | ❌ No |
| FE grader UI | ❌ No |
| `ComputeScores()` pre-processing | ✅ Yes — 1 spot (NAScore injection) |
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
