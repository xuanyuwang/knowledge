# CONVI-6709: NRG Scorecard UI Displaying Reversed Predicted CSAT / Resolution

**Created:** 2026-05-02
**Updated:** 2026-05-02

## Overview

NRG's "Checklist Performance – Canada" scorecard in Closed Conversations renders conversation outcome criteria (predicted-csat, predicted-resolution) **reversed** in the scorecard panel. The filtering logic and conversation-detail view show correct values.

- Filter `predicted-resolution = "Not Resolved"` → scorecard panel shows "Resolved"
- Filter `predicted-resolution = "Resolved"` → scorecard panel shows "Not Resolved"
- Filter `predicted-csat = "low CSAT"` → scorecard panel shows "high CSAT"
- Filter `predicted-csat = "High"` → scorecard panel shows NA, Low, or High inconsistently
- Conversation-detail view and reports show **correct** values in all cases

**Namespace:** nrg-us-east-1, usecase: Canada – Retention

## Root Cause (CONFIRMED)

### The bug: `CriterionInputDisplay` in read-only mode uses array index for decoupled scoring, but the field value is `option.value` from auto-scoring

**The scorecard panel renders `CriterionInputDisplay` in read-only mode** (via `CriterionMain.tsx:151-160` → `CriterionInput` → `CriterionInputDisplay`). The rendering path is:

1. **Form initialization** (`utils.ts:710-712`): `score.numericValue` → `String(score.numericValue)` → field value
2. **Decoupled scoring options** (`CriterionInputDisplay.tsx:243-250`): options use `String(index)` as value
3. **Read-only display** (`CriterionInputDisplay.tsx:262-270`): finds option where `fieldValue.includes(option.value)` → gets label

When `option.value ≠ array index`, the field value (derived from `numericValue` = `option.value`) matches the **wrong** array index.

### Concrete example with NRG template

```
predicted-resolution template:
  settings.options: [0] value=1 label="Not Resolved"
                    [1] value=0 label="Resolved"

Auto-scoring for "resolved" conversation:
  trigger_value="resolved" → auto_qa option value=0 → numericValue=0

Form field value: ["0"]

CriterionInputDisplay decoupled options:
  [0] value="0" label="Not Resolved"   ← index 0
  [1] value="1" label="Resolved"       ← index 1

Read-only lookup: fieldValue "0" matches index 0 → "Not Resolved"  ❌
Expected: "Resolved"
```

The **real value 0** means "Resolved" (per template config), but **index 0** in the options array points to "Not Resolved".

### Why `useScoreLabelProvider` gets it RIGHT

`useScoreLabelProvider` (line 703-711) maps by `option.value`, not index:
```typescript
const optionsMap = new Map<number, string>(
  options.map((option) => [option.value, option.label])
);
return optionsMap.get(score.numericValue) ?? '';
```
This correctly maps `numericValue=0 → option with value=0 → "Resolved"`.

But this function is only used for the **label text** in appeal sections and calibration display — not for the main criterion input control.

### Why `CriterionInputDisplay` gets it WRONG for auto-scored outcomes

In `hasDecoupledScoring` mode (criteria with `settings.scores`), `CriterionInputDisplay` uses array **index** as the option value:
```typescript
const existingOptions = hasDecoupledScoring(dropdownNumericTemplate)
  ? options.map((option, index) => ({
      value: String(index),  // ← BUG: uses array index
      label: option.label,
    }))
```

This works correctly for **manual scoring** because the form submits the **index** and `calculateCorrectScoreForDropdownNumericAndLabeledRadioTypes` converts it back. But for **auto-scored** values, the DB stores `option.value` (not index), so the read-only display maps the wrong index.

## DB Verification

Queried actual scores for the two conversations from the bug report:

```
Conversation 019db831 (predicted-resolution): numeric_value=0, ai_value=0
  → trigger was "resolved" → option.value=0 ✅ correct in DB
  → But CriterionInputDisplay shows index 0 = "Not Resolved" ❌

Conversation 019db83c (predicted-resolution): numeric_value=1, ai_value=1
  → trigger was "unresolved" → option.value=1 ✅ correct in DB
  → But CriterionInputDisplay shows index 1 = "Resolved" ❌

Conversation 019db831 (predicted-csat): numeric_value=1, ai_value=1
  → numeric_from=0, numeric_to=6 → option.value=1 ✅ correct in DB
  → CriterionInputDisplay shows index 1 = "high CSAT" ❌ (should be "low CSAT")

Conversation 019db83c (predicted-csat): numeric_value=2, ai_value=2
  → numeric_from=6, numeric_to=10 → option.value=2 ✅ correct in DB
  → CriterionInputDisplay: no index 2 → shows "--" ❌ (should be "high CSAT")
```

Template revision used for all scorecards: `5e2a5bcc`

## Fix Options

### Option A: Fix `CriterionInputDisplay` read-only path to use `option.value` for auto-scored outcomes

When rendering read-only and the criterion has auto-scored values, use `option.value` instead of array index. However, this only fixes the read-only display — the interactive SegmentedControl/Select would still use index-based values for decoupled scoring.

### Option B: Fix form initialization to convert `numericValue` → index for decoupled scoring

In `utils.ts:710-712`, when the criterion has decoupled scoring, convert the stored `numericValue` (which is `option.value`) to the corresponding array index before setting the form field value. This would fix both read-only and interactive displays.

### Option C: Fix auto-scoring to store array index instead of `option.value`

Change `autoqa_mapper.go` to store the array index instead of `AutoQAOptions.Value`. This would align auto-scoring with how `CriterionInputDisplay` expects values in decoupled mode. **Risk**: breaks existing scored data — would need a migration.

### Recommended: Option B

Option B is the cleanest fix — it converts at the form boundary without changing the DB schema or the auto-scoring logic. The conversion would be:
1. Find the option in `settings.options` where `option.value === numericValue`
2. Use that option's array index as the form field value
3. Only apply this for criteria with `hasDecoupledScoring()`

## Data Flow Trace

```
Moment Annotation (ES)
  stringValue = "resolved"
       │
       ▼
Auto-scoring (autoqa_mapper.go:99-114)
  Matches trigger_value="resolved" → AutoQAOptions.Value=0 → NumericValue=0
       │
       ▼
Score in DB: NumericValue = 0
       │
       ▼
Form initialization (utils.ts:710-712)
  String(0) → fieldValue = ["0"]
       │
       ├──────────────────────────────────┐
       ▼                                  ▼
CriterionInputDisplay (read-only)      useScoreLabelProvider
  Decoupled options:                    Maps option.value → label:
  [0] value="0" label="Not Resolved"   { 0 → "Resolved", 1 → "Not Resolved" }
  [1] value="1" label="Resolved"       numericValue=0 → "Resolved" ✅
  fieldValue "0" → index 0 →
  "Not Resolved" ❌ WRONG!
```

## Investigation Summary

### Finding 1: RetrieveClosedConversations filter logic is NOT the bug

The `momentGroups` filter in `RetrieveClosedConversations` correctly builds the ES query. For an outcome filter with `stringValue = "resolved"`, it produces a correct nested filter on `moment_annotations.payload.conversationOutcome.stringValue.keyword`.

### Finding 2: Auto-scoring stores correct values in DB

Auto-scoring maps `trigger_value → option.value` and stores `option.value` as `NumericValue`. DB values are correct: `numericValue=0` for "resolved", `numericValue=1` for "unresolved".

### Finding 3: The bug is in how `CriterionInputDisplay` renders auto-scored values in decoupled scoring mode

The scorecard panel renders `CriterionInputDisplay` (via `CriterionMain` → `CriterionInput`) in read-only mode. The form field stores `String(numericValue)` = `String(option.value)`. In decoupled scoring mode, `CriterionInputDisplay` builds options using **array index** as the value key. When `option.value ≠ array index` (which happens when options are not in sequential order), the lookup returns the wrong label.

### Finding 4: `useScoreLabelProvider` is NOT the bug

`useScoreLabelProvider` correctly maps by `option.value` and would show the right label. But it's only used for appeal/calibration label text, not for the main criterion display.

## Key Files

| File | Line | Purpose |
|------|------|---------|
| `CriterionInputDisplay.tsx` | 243-250, 262-270 | **BUG**: read-only display uses array index for decoupled scoring |
| `CriterionMain.tsx` | 151-160 | Renders `CriterionInput` with `readOnly` in scorecard panel |
| `ScorecardCriterionCardBody.tsx` | 674-718 | `useScoreLabelProvider` — correct but not used for main display |
| `utils.ts` | 710-712 | Form initialization: `String(numericValue)` as field value |
| `autoqa_mapper.go` | 99-114 | Auto-scoring: stores `option.value` as NumericValue (correct) |

## Log History

| Date | Summary |
|------|---------|
| 2026-05-02 | Full investigation: traced all paths, confirmed root cause is CriterionInputDisplay decoupled scoring index mismatch with auto-scored option.value. DB data verified. |
