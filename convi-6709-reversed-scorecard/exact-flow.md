# Exact Flow: Template Fetch → Wrong Label in Scorecard Panel

Using **predicted-resolution** criterion for conversation `019db831-fdbf-7cc0-959a-4c41360833f7` (a "resolved" conversation that the scorecard panel incorrectly shows as "Not Resolved").

---

## Step 1: Template structure in DB

Template ID `01990cb4-dec8-719f-b0eb-7713d43856a8`, revision `5e2a5bcc`.

The predicted-resolution criterion (`019d879f-7e40-7685-8489-04d2cd879364`) has type `DropdownNumericValues` and this structure:

```json
{
  "settings": {
    "options": [
      { "value": 1, "label": "Not Resolved" },
      { "value": 0, "label": "Resolved" }
    ],
    "scores": [
      { "value": 1, "score": 0 },
      { "value": 0, "score": 1 }
    ],
    "showNA": true,
    "excludeFromQAScores": true
  },
  "auto_qa": {
    "triggers": [{ "resource_name": "...", "type": "metadata" }],
    "options": [
      { "trigger_value": "unresolved", "value": 1 },
      { "trigger_value": "resolved",   "value": 0 }
    ]
  }
}
```

Key observation: **`settings.options` is ordered `[Not Resolved, Resolved]` but their `.value` fields are `[1, 0]` — reversed from their array indices `[0, 1]`.**

The `.value` field on each option is a **lookup key** (an identifier that links `settings.options`, `settings.scores`, and `auto_qa.options` together), **not** the actual score. The actual score is in `settings.scores[].score` (e.g., `score: 1` for "Resolved", `score: 0` for "Not Resolved").

### How did `option.value ≠ array index` happen?

Normally, saving a template through the template builder renormalizes `option.value` to match the array index via `transformDropdownNumericInputOptionsToApi` (`useSaveScorecardTemplate.ts:126-137`):
```typescript
options?.forEach((option, index) => {
  newOptions.push({ label: option.label, value: index, ... });  // value = index
  newScores.push({ value: index, score: scores?.[index]?.score ?? option.value });
});
```

**But outcome criteria skip this renormalization.** The save path (`extractCriterionSettingsForApi`, line 159-167) calls:
```typescript
transformDropdownNumericInputOptionsToApi(
  options, scores,
  enableMultiSelect || isNonOutcomePerformanceCriterion,  // false for outcome
  isCriterionOutcome(criterion)                           // true for outcome
)
```
And inside the function (line 139-141):
```typescript
if (isEvaluativeOutcome) {
    return { options, scores };   // ← PASSTHROUGH! No renormalization!
}
```

Additionally, `transformAutoQAToApi` (`useSaveScorecardTemplate.ts:52-59`) is a pure passthrough — it does NOT renormalize `auto_qa.options[].value` either.

So when the scoring-details UI (`getScoringConfigurationWithOptionBins` in `scoring-details/utils.ts:29-77`) rebuilds arrays grouped by score bin, the array order may differ from the `value` order. The three arrays stay internally consistent (`settings.options[].value === auto_qa.options[].value === settings.scores[].value`), but the array **index** no longer equals the `value`. And since the outcome save path doesn't renormalize, this mismatch persists to the DB.

---

## Step 2: Auto-scoring writes `option.value` (the lookup key) as `numeric_value` in DB

The conversation's moment annotation had `stringValue = "resolved"`.

### 2a. `MapToScores` sets `NumericValue = option.Value`

`autoqa_mapper.go:99-109` — iterates `auto_qa.options`, finds `trigger_value == "resolved"` at `auto_qa.options[1]`, stores its `.value` field (the lookup key, `0`) as `NumericValue`:

```go
// autoqa_mapper.go:104-109
for _, option := range *autoQaConfig.Options {
    if option.TriggerValue != nil && *option.TriggerValue == v.StringValue {
        optionValue := option.Value   // option.Value = 0 (the lookup key, NOT the score)
        mappedScore.NumericValue = &optionValue
        break
    }
}
```

Evidence that `option.Value` is the lookup key, not the score:

- `AutoQAOptions` struct (`scorecard_templates.go:237-243`): `Value float32 \`json:"value"\`` — comment says "Value of the criterion", maps to `auto_qa.options[].value` in template JSON
- `LabeledCriterionSettingOption` struct (`scorecard_templates.go:320-324`): `Value int \`json:"value"\`` — same `value` field in `settings.options`
- `CriterionScoreOption` struct (`scorecard_templates.go:327-329`): has **both** `Value float64` and `Score float64` — `Value` is the lookup key, `Score` is the actual score
- In our template: `auto_qa.options[1].value = 0`, `settings.options[1].value = 0`, `settings.scores[1] = { value: 0, score: 1 }` — they all share the same lookup key `0`, and the actual score for "Resolved" is `1`

### 2b. `MapToScores` result flows to DB via `ComputeScores` → `mapScoresByCriterion`

Call chain (`action_trigger_conversation_autoscoring.go:224-233`):
```go
dbScores := autoqa.MapToScores(autoScoredResult.Items, criterionToAutoQa)
dbScoresByCriterion, err := scoring.ComputeScores(dbScorecard, dbScores, template.TemplateStructure, true)
computedScores := scoring.FlattenCriterionScores(dbScoresByCriterion)
err = scoring.CreateScorecardAndScoresInDB(db, dbScorecard, computedScores, false)
```

Inside `ComputeScores` → `mapScoresByCriterion` (`scorecard_calculator.go:186-213`), the proto `NumericValue` is directly converted to the DB model:
```go
// scorecard_calculator.go:185-187
var numericValue *float32
if score.NumericValue != nil {
    numericValue = score.NumericValue    // passes through unchanged
}

// scorecard_calculator.go:213
NumericValue: converter.ConvertToNullFloat64From32Ref(numericValue),  // 0 → sql.NullFloat64{Float64: 0, Valid: true}
```

No transformation — `auto_qa.options[].value` passes straight through to `director.scores.numeric_value`.

### 2c. Verified in DB

```
| conversation_id                        | criterion_identifier                   | numeric_value | ai_value |
|----------------------------------------|----------------------------------------|---------------|----------|
| 019db831-fdbf-7cc0-959a-4c41360833f7   | 019d879f-7e40-7685-8489-04d2cd879364   | 0             | 0        |
```

**`numeric_value = 0` is the lookup key for "Resolved"** (matching `settings.options` where `value=0, label="Resolved"`). It is not the score (the score for "Resolved" is `1`, stored in `settings.scores`).

---

## Step 3: Frontend fetches scorecard + template

The scorecard panel in Closed Conversations loads the scorecard (scores) and the criterion template. At this point we have:

- `score.numericValue = 0` (the lookup key, read from DB via `convertScoreToPB` at `transformers.go:619`)
- `criterionTemplate` with the structure from Step 1

---

## Step 4: Form initialization — `numericValue` → field value

`utils.ts:710-712` converts scores to form field values:

```typescript
// utils.ts:710-712
value = scores.every((score) => score.numericValue == null)
  ? [INPUT_NOT_SET_VALUE]
  : [...scores.filter((score) => score.numericValue != null)
       .map((score) => String(score.numericValue))];
```

`String(score.numericValue)` = `String(0)` = `"0"`

**Field value is now `["0"]`.**

This value is the raw lookup key from the DB — it is **not** an array index. No conversion happens here.

---

## Step 5: `CriterionMain` renders `CriterionInput` in read-only mode

`CriterionMain.tsx:151-160` — the scorecard panel renders the criterion input:

```tsx
// CriterionMain.tsx:151-160
<CriterionInput
  onChange={onCriterionValueChange}
  criterionTemplate={criterionTemplate}
  fieldNameBase={criteriaFieldNameBase}
  readOnly={readOnly || !!calibrationScorecard}   // readOnly = true for auto-scored view
  tooltip={calibrationReadonlyTooltip}
  templateType={ScorecardTemplateType.SCORECARD_TEMPLATE_TYPE_CONVERSATION}
  isOutcome={isOutcome}
  isAppeal={isInAppeal}
/>
```

### How `CriterionInput` wraps `CriterionInputDisplay`

`CriterionInput.tsx:28-88` — `CriterionInput` is a thin form-binding wrapper:

1. Reads the form field via `useController({ name: \`${fieldNameBase}.value\` })` (line 41-55) — this retrieves the field value `["0"]` set in Step 4
2. Creates a change handler that calls `field.onChange(value)` (line 57-67)
3. Renders `CriterionInputDisplay` directly, passing:
   - `field={field}` — the react-hook-form field object (contains `field.value = ["0"]`)
   - `readOnly={readOnly}` — `true` in this case
   - `criterionTemplate={criterionTemplate}` — the template structure from Step 1

```tsx
// CriterionInput.tsx:74-88
return (
  <CriterionInputDisplay
    criterionTemplate={criterionTemplate}
    readOnly={readOnly}
    tooltip={props.tooltip}
    field={field}                    // field.value = ["0"]
    onChangeHandler={handleChangeHandler}
    user={user.data}
    templateAudience={templateAudience}
    templateType={templateType}
    fullWidth={fullWidth}
    isForOutcome={isOutcome}
  />
);
```

No transformation of `field.value` occurs between `CriterionInput` and `CriterionInputDisplay`.

---

## Step 6: `CriterionInputDisplay` checks `hasDecoupledScoring`

`CriterionInputDisplay.tsx:391-393`:

```typescript
function hasDecoupledScoring(criterion): boolean {
  return !!criterion.settings?.scores?.length && !isNumOccurrencesCriterion(criterion);
}
```

- `criterion.settings.scores` = `[{ value: 1, score: 0 }, { value: 0, score: 1 }]` → length 2 → truthy
- `isNumOccurrencesCriterion` (`CriterionInputDisplay.tsx:378-384`): checks `criterion.auto_qa.triggers.some(t => t.type === 'behavior')` → this criterion's trigger type is `"metadata"` → `false`

**Result: `hasDecoupledScoring = true`**

---

## Step 7: `CriterionInputDisplay` builds options using array INDEX

`CriterionInputDisplay.tsx:243-250` — the `DropdownNumericValues` branch:

```typescript
const existingOptions = hasDecoupledScoring(dropdownNumericTemplate)
  ? dropdownNumericTemplate.settings.options
      .filter((option) => notUndefined(option.label as string | undefined))
      .map((option, index) => ({
        value: String(index),    // ← uses array INDEX, not option.value
        label: option.label,
      }))
  : /* non-decoupled path uses option.value */;
```

Iterating `settings.options`:

| Array index | `option.value` (ignored) | `option.label` | Resulting `value` key |
|---|---|---|---|
| 0 | 1 | "Not Resolved" | **`"0"`** |
| 1 | 0 | "Resolved" | **`"1"`** |

**Built options: `[{ value: "0", label: "Not Resolved" }, { value: "1", label: "Resolved" }]`**

---

## Step 8: Read-only display looks up field value in index-keyed options

`CriterionInputDisplay.tsx:262-270`:

```typescript
if (readOnly) {
  let matchingLabel = '--';
  if (fieldValue) {
    const matchingOption = options.find(
      (option) => fieldValue.includes(option.value)
    );
    if (matchingOption) {
      matchingLabel = matchingOption.label;
    }
  }
  return <>{matchingLabel}</>;
}
```

- `fieldValue = ["0"]` (from Step 4 — this is `String(numericValue)` = `String(option.value)` = the lookup key)
- `options.find(o => ["0"].includes(o.value))`
- Matches `{ value: "0", label: "Not Resolved" }` — array index 0

**Displayed: "Not Resolved"** ❌

**Expected: "Resolved"** (because `numericValue = 0` is the lookup key for `settings.options` entry with `value = 0` and `label = "Resolved"`)

---

## The Mismatch

| What | Keyed by | Value `"0"` maps to |
|---|---|---|
| Auto-scoring stores | `option.value` (lookup key) | 0 → "resolved" trigger → label "Resolved" |
| Form field contains | `String(option.value)` | `"0"` |
| Display options keyed by | `String(array index)` | `"0"` → index 0 → label "Not Resolved" |

The field value `"0"` means **`option.value = 0` = "Resolved"**, but `CriterionInputDisplay` interprets it as **array index 0 = "Not Resolved"**.

---

## Same trace for predicted-csat

Template `settings.options`:

| Index | `option.value` | `option.label` |
|---|---|---|
| 0 | 1 | "low CSAT" |
| 1 | 2 | "high CSAT" |

Conversation `019db83c` has `numeric_value = 2` (high CSAT range 6-10).

- Field value: `["2"]`
- Decoupled options: `[{ value: "0", label: "low CSAT" }, { value: "1", label: "high CSAT" }]`
- Lookup: `["2"].includes(option.value)` → no match for `"0"` or `"1"`
- **Displayed: `"--"`** ❌ (expected "high CSAT")

This explains the bug report's observation that predicted-csat "inconsistently shows NA, Low, or High" — when `option.value` exceeds the max array index, no option matches and it falls through to `"--"`.
