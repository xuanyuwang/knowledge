# N/A Score — Full Lifecycle Reference

**Created:** 2026-04-29

This document traces the complete lifecycle of the N/A score feature (`enableNAScore` feature flag) through every stage: load, edit, save, grading, clone, and validation. It follows the same framework as `options-scores-lifecycle.md`.

---

## 1. N/A Score States

The N/A score feature introduces three distinct states:

| State | `showNA` | N/A in arrays | `not_applicable` | UI |
|-------|----------|---------------|-------------------|----|
| **OFF** | `false` | No | `null` | No N/A row |
| **ON, no score** | `true` | No | `null` | N/A row with empty score input |
| **ON, scored** | `true` | Yes (`isNA: true`) | Index of N/A option | N/A row with score value |

The "ON, no score" state is **intentionally identical** to legacy `showNA: true` behavior — the template JSON has no N/A in options/scores, preserving backwards compatibility.

### Key identifiers

- **`isNA: true`** — flag on `NumericInputOption` that marks a scored N/A option
- **`checkIsNAOption(opt)`** — helper in `template-builder/utils.ts` checks `opt.isNA === true`
- **`na_no_score`** — synthetic sentinel value for the AutoQA "not applicable" dropdown when N/A has no score
- **`INPUT_N_A_VALUE`** — legacy sentinel (`"N/A"` string) for unscored N/A in grading UI

---

## 2. Load Path (API → Form)

**Entry:** `TemplateBuilderForm.tsx` → `transformApiCriterionTemplateSettingsToForm()`

### 2a. Decoupled scoring path (has scores array)

```
API:  options=[{label:"Yes", value:0}, {label:"No", value:1}, {label:"N/A", value:2, isNA:true}]
      scores=[{value:0, score:1}, {value:1, score:0}, {value:2, score:5}]
                                    ↓
Form: options=[{label:"Yes", value:1}, {label:"No", value:0}, {label:"N/A", value:5, isNA:true}]
      scores unchanged
```

Each `option.value` is replaced by the matching `score.score`. **Exception:** if `score.score` is `null` or `undefined`, the option is left as-is (lines 684-685). This means an N/A option with null score keeps its original `value` (the index).

`isNA: true` is preserved via spread (line 691).

### 2b. Legacy migration path (no scores array)

`isNA: true` is preserved via spread (line 714). Options are renormalized to sequential indices.

**Bug (latent):** `auto_qa.not_applicable` is NOT remapped via `valueToIndexMap` — only `detected` and `not_detected` are (lines 731-740). If a legacy template has `not_applicable` set, it won't be correctly remapped. This is documented in `options-scores-lifecycle.md` section 8.7 / 9.9.

### 2c. Deferred N/A (no score → not in arrays)

When N/A has no score, the N/A option is simply **not present** in the `options`/`scores` arrays. The load path passes through these arrays without modification — no special N/A handling needed. State is: `showNA: true`, arrays contain only regular options, `not_applicable: null`.

### 2d. Mount-time migration (`CriteriaLabeledOptions.useOnMount`)

The `useOnMount` checks if the template needs legacy migration. It skips templates that **already have N/A options** (line 95: `const hasNAOption = currentOptions?.some(...)`) — this prevents the migration from breaking scored N/A templates.

**Verdict:** Load path is **correct** for all three N/A states.

---

## 3. Edit Path (Form Mutations)

### 3a. Allow N/A toggle — `handleAllowNAChange()`

**File:** `CriteriaLabeledOptions.tsx`, line 193

```
ON:  showNAField.onChange(true)  — NO array mutation (deferred approach)
OFF: showNAField.onChange(false)
     if N/A in arrays (isNAIndex >= 0): handleRemoveOption(isNAIndex)
     clear naScoreInput
```

Toggling ON does **not** add N/A to arrays. N/A is only added when a score is assigned (3c).
Toggling OFF removes any existing scored N/A and clears the input.

**Verdict:** Correct.

### 3b. Add option — `onAddLabel()`

**File:** `CriteriaLabeledOptions.tsx`, line 73

When N/A exists in arrays (isNAIndex ≥ 0), new options are inserted **before** the N/A option to keep it last:

```
if (isNAIndex >= 0):
  optionsField.insert(isNAIndex, ...)
  scoresFieldArray.insert(isNAIndex, ...)
else:
  optionsField.append(...)
  scoresFieldArray.append(...)
```

**Verdict:** Correct — N/A stays last.

### 3c. N/A score input — `handleNAScoreChange()`

**File:** `CriteriaLabeledOptions.tsx`, line 137

Three cases:

**Case 1: Score assigned, N/A not in arrays (first score entry)**
```typescript
const maxId = Math.max(...(watchedOptionsField?.map((f) => f.value) ?? [-1]));
const newValue = maxId + 1;
optionsField.append({ label: 'N/A', value: newValue, isNA: true });
scoresFieldArray.append({ value: newValue, score: numValue });
autoQANotApplicableField.onChange(newValue);   // ← sets not_applicable
```

**Fixed:** Previously `autoQANotApplicableField.onChange(newValue)` set `not_applicable` to `maxId + 1` (max option value + 1) instead of the array index. In decoupled mode, `option.value` = score, so these could diverge. Now uses `watchedOptionsField?.length ?? 0` — the correct index where N/A is appended.

**Case 2: Score changed, N/A already in arrays**
```typescript
form.setValue(`...settings.scores.${isNAIndex}.score`, numValue);
```
Only updates the score value. **Correct.**

**Case 3: Score cleared (handled on blur)**
See 3d below.

### 3d. N/A score blur — `handleNAScoreBlur()`

**File:** `CriteriaLabeledOptions.tsx`, line 158

When score input is cleared (non-number) and N/A is in arrays:
1. Remove N/A from both arrays
2. Renormalize remaining options/scores to sequential values
3. Set `auto_qa.not_applicable = null`
4. Remap `detected` and `not_detected` in case they referenced the removed N/A index

**Verdict:** Correct — properly handles all cross-references.

### 3e. Remove option — `handleRemoveOption()`

**File:** `CriteriaLabeledOptions.tsx`, line 204

Remaps all three auto_qa fields (`detected`, `not_detected`, `not_applicable`) and branch conditions. Renormalizes remaining options/scores.

**Verdict:** Correct — complete remapping including `not_applicable`.

### 3f. AutoQA dropdown — not_applicable selection

**File:** `TemplateBuilderAutoQA.tsx`, line 155-178

```
notApplicableFieldValue:
  if value != null → String(value)                    // normal: show index
  if showNA && !hasScoredNA → 'na_no_score'            // deferred: show synthetic entry
  else → null                                          // no N/A

onChangeNotApplicableField:
  if value === 'na_no_score' || null → set null         // deferred path
  else → parse to number (index)                        // normal path
```

The `hasScoredNA` check (line 162) ensures that when N/A IS in arrays (has a real index), `null` doesn't get mapped to `'na_no_score'`. This prevents a display bug where removing the not_applicable selection would show "N/A (no score)" instead of empty.

**Verdict:** Correct.

### 3g. NumericBinsAndValuesConfigurator (# of occurrences mode)

**File:** `NumericBinsAndValuesConfigurator.tsx`, lines 112-139

Same deferred N/A pattern as CriteriaLabeledOptions. Same `handleNAScoreChange` and `handleNAScoreBlur`.

**Fixed (same as 3c):** `not_applicable` now uses `watchedSettingsOptions?.length ?? 0` instead of `newValue`.

**Fixed:** Added `useEffect` sync for `naScoreInput` to match CriteriaLabeledOptions — form state changes from undo/reset/programmatic updates now sync to the local input state.

**Verdict:** Correct after fixes.

---

## 4. Save Path (Form → API)

**Entry:** `useSaveScorecardTemplate.ts` → `transformDropdownNumericInputOptionsToApi()`

### 4a. Transform

```typescript
options?.forEach((option, index) => {
  newOptions.push({
    label: option.label || '',
    value: index,                              // re-index to sequential
    ...(option.isNA ? { isNA: true } : {}),    // preserve isNA
  });
  newScores.push({
    value: index,
    score: scores?.[index]?.score ?? option.value,   // score from scores array
  });
});
```

Key behaviors:
- `option.value` is re-indexed to sequential 0,1,2... (undoing decoupled mode)
- `isNA: true` is preserved via conditional spread
- Score comes from `scores[i].score` (authoritative), falling back to `option.value` (which is the score in decoupled mode)

### 4b. Auto QA pass-through

`auto_qa.detected`, `not_detected`, `not_applicable` are saved as indices — they were already indices in the form. The save transform re-indexes options to 0,1,2..., which matches the index-based auto_qa values.

### 4c. N/A states on save

| State | Saved `options` | Saved `scores` | Saved `not_applicable` |
|-------|----------------|----------------|----------------------|
| OFF | `[Yes(0), No(1)]` | `[{0,1}, {1,0}]` | `null` |
| ON, no score | `[Yes(0), No(1)]` | `[{0,1}, {1,0}]` | `null` |
| ON, scored (5) | `[Yes(0), No(1), N/A(2,isNA)]` | `[{0,1}, {1,0}, {2,5}]` | `2` |

"ON, no score" is **byte-identical** to legacy `showNA: true` — the deferred approach works correctly.

**Verdict:** Correct.

---

## 5. Grading Path (API → Grader UI)

**Entry:** `CriterionInputDisplay.tsx`

### 5a. Building grading options

For decoupled scoring criteria, options use `value: String(index)`. The N/A option (if scored) appears in the list with its index.

### 5b. `moveScoredNAOptionToHead()` (line 399)

If a scored N/A exists in `settings.options`, it's moved from its position (typically last) to the **front** of the grading option list for UX visibility.

### 5c. `addNAOption()` guard (line 426)

```typescript
const nonScoredNA =
  criterionTemplate.settings?.showNA &&
  !('options' in criterionTemplate.settings &&
    criterionTemplate.settings.options?.some(checkIsNAOption));
```

- **Scored N/A present:** `nonScoredNA = false` → no synthetic N/A added → scored N/A appears naturally with its real value
- **No scored N/A, showNA true:** `nonScoredNA = true` → synthetic `INPUT_N_A_VALUE` N/A prepended → legacy behavior

This guard prevents **duplicate N/A buttons** (scored + synthetic).

### 5d. Score lookup on grading

**File:** `scoring/utils.ts`, line 546-561

When user selects N/A (`INPUT_N_A_VALUE`):

```typescript
const isNAOption = criterion.settings.options?.find((opt) => checkIsNAOption(opt));
const naScore = isNAOption ? criterion.settings.scores?.find((s) => s.value === isNAOption.value) : undefined;
const hasNAScore = naScore?.score != null;
return [{
  numericValue: isNAOption?.value ?? null,
  notApplicable: !hasNAScore,    // scored N/A → false (participates), unscored → true (excluded)
}];
```

Two paths:
- **Scored N/A:** `notApplicable = false`, `numericValue = N/A option's value`. Goes through normal score aggregation.
- **Unscored N/A:** `notApplicable = true`, `numericValue = null`. Excluded from aggregation (legacy behavior).

**Verdict:** Correct — scored N/A participates in scoring, unscored N/A is excluded.

---

## 6. Clone Path

**Entry:** `TemplateBuilderFormConfigurationStep.tsx` → `handleAddCriterion()`, line 305

### 6a. Cloning with N/A

```typescript
const sourceOptions = defaultCriterion.settings.options;
const sourceScores = 'scores' in defaultCriterion.settings ? defaultCriterion.settings.scores : undefined;

// Pair options with scores, then filter out N/A
const paired = sourceOptions
  .map((opt, i) => ({ opt, score: sourceScores?.[i] }))
  .filter(({ opt }) => !checkIsNAOption(opt));

// Clear not_applicable since N/A was stripped
newAutoQA.auto_qa.not_applicable = null;

// Renormalize to sequential indexes
newCriterion.settings.options = paired.map(({ opt }, index) => ({
  label: opt.label, value: index,
}));

// Use score from scores array; fall back to opt.value (which is the score in decoupled mode)
(newCriterion.settings as SettingsWithScores).scores = paired.map(({ opt, score }, index) => ({
  value: index,
  score: score?.score ?? opt.value,
}));
```

Key behaviors:
- N/A options are **stripped** from the clone (filtered by `checkIsNAOption`)
- `not_applicable` is reset to `null`
- Options are paired with scores **before** filtering, so the correct score is used
- Score fallback `score?.score ?? opt.value` handles both decoupled (has scores) and legacy (no scores) paths

### 6b. What showNA does

`showNA` is copied from the source criterion (line 315), so the clone inherits the "Allow N/A" toggle state. But since N/A is stripped from arrays, the clone starts in the "ON, no score" state — the user must re-enter an N/A score if desired.

**Verdict:** Correct — clone produces a clean criterion without N/A in arrays.

---

## 7. Validation

**Entry:** `validation.ts` → `validateBehaviorDNDScoreTypeConfigurationItem()`, line 150

### 7a. Save-time validation

**Uniqueness checks (lines 183-206):**
1. `detected !== not_detected` — always checked
2. `detected !== not_applicable` — checked only when `not_applicable != null`
3. `not_detected !== not_applicable` — checked only when `not_applicable != null`

**Bounds check (lines 218-224):** For NumericRadios, checks `detected` and `not_detected` are within `range.min..range.max`. Does NOT check `not_applicable` against range — correct, since N/A may have a score outside the normal range.

### 7b. Form-time validation (TemplateBuilderAutoQA.tsx)

**Cross-field validation via `useController` rules (lines 98-152):**
- `detected` validates against `not_detected` and `not_applicable`
- `not_detected` validates against `detected` and `not_applicable`
- `not_applicable` validates against `detected` and `not_detected`

Each field triggers re-validation of siblings on change (via `form.trigger`).

### 7c. Score validation

In `validateCriterion()` (called for all criteria), the existing validation allows `null` scores for N/A options — the `excludeFromQAScores` logic and score range checks handle N/A gracefully.

**Verdict:** Correct — proper uniqueness and range validation for all three auto_qa fields.

---

## 8. Bugs Found & Fixed

### 8.1 `not_applicable` set to value instead of index — FIXED

**Files:** `CriteriaLabeledOptions.tsx`, `NumericBinsAndValuesConfigurator.tsx`

When first assigning an N/A score, `not_applicable` was set to `maxId + 1` (max option VALUE + 1) instead of the array index. In decoupled mode, `option.value` = score (not index), so these could diverge.

**Fix:** Changed to `watchedOptionsField?.length ?? 0` / `watchedSettingsOptions?.length ?? 0` — the correct index where N/A is appended.

### 8.2 Legacy load doesn't remap `not_applicable` (P3, latent, unfixed)

**File:** `TemplateBuilderForm.tsx` lines 731-740

The legacy migration path remaps `detected` and `not_detected` via `valueToIndexMap` but skips `not_applicable`. If a legacy template has `not_applicable` set (unlikely — feature is new), it won't be correctly remapped.

**Impact:** Effectively zero — `not_applicable` only exists in templates created with the `enableNAScore` flag, which always have a scores array (so they take the decoupled path, not legacy).

### 8.3 `NumericBinsAndValuesConfigurator` missing `useEffect` sync — FIXED

**File:** `NumericBinsAndValuesConfigurator.tsx`

Added `useEffect` to sync `naScoreInput` with form state when it changes externally (undo, reset, programmatic update), matching the pattern already present in CriteriaLabeledOptions.

---

## 9. State Diagram

```
┌─────────────────────────────────────────────────────────┐
│  Allow N/A: OFF                                         │
│  options: [Yes(0), No(1)]                               │
│  scores:  [{0,1}, {1,0}]                                │
│  not_applicable: null                                   │
│  showNA: false                                          │
└─────────┬───────────────────────────────────────────────┘
          │ toggle Allow N/A ON
          ▼
┌─────────────────────────────────────────────────────────┐
│  Allow N/A: ON, no score (= legacy behavior)            │
│  options: [Yes(0), No(1)]          ← unchanged          │
│  scores:  [{0,1}, {1,0}]          ← unchanged          │
│  not_applicable: null              ← unchanged          │
│  showNA: true                                           │
│  UI: N/A row with empty NumberInput                     │
└──────┬──────────────────────┬───────────────────────────┘
       │ enter score = 5      │ toggle Allow N/A OFF
       ▼                      ▼ (back to OFF state)
┌─────────────────────────────────────────────────────────┐
│  Allow N/A: ON, scored                                  │
│  options: [Yes(0), No(1), N/A(2, isNA:true)]            │
│  scores:  [{0,1}, {1,0}, {2,5}]                         │
│  not_applicable: 2                                      │
│  showNA: true                                           │
│  UI: N/A row with score = 5                             │
└──────┬──────────────────────┬───────────────────────────┘
       │ clear score (blur)   │ toggle Allow N/A OFF
       ▼                      ▼
       (back to ON-no-score)  (back to OFF: N/A removed
                               from arrays, input cleared)
```

---

## 10. Cross-reference: AutoQA Dropdown States

| N/A State | `behaviorScoreSelectionOptions` | `notApplicableSelectionOptions` | `notApplicableFieldValue` |
|-----------|-------------------------------|-------------------------------|--------------------------|
| OFF | `[Yes(0), No(1)]` | N/A row hidden | — |
| ON, no score | `[Yes(0), No(1)]` | `[Yes(0), No(1), "N/A (no score)"]` | `'na_no_score'` |
| ON, scored(5) | `[Yes(0), No(1), N/A(2)]` | `[Yes(0), No(1), N/A(2)]` | `'2'` |

The `behaviorScoreSelectionOptions` (used for detected/not_detected dropdowns) includes N/A when scored — this means N/A can be selected as the "if behavior done" or "not done" value. Semantically questionable, but validation catches conflicts (8.1 uniqueness check).

---

## 11. Grading Path — N/A Score Lookup Detail

```
User selects N/A in grading UI
         │
         ├── Scored N/A (isNA option in arrays)
         │   └── value = String(index of N/A option)
         │       → scores.find(s => s.value === index)
         │       → numericValue = N/A value, notApplicable = false
         │       → participates in QA score aggregation
         │
         └── Unscored N/A (legacy, INPUT_N_A_VALUE sentinel)
             └── value = "N/A" (INPUT_N_A_VALUE)
                 → isNAOption = options.find(checkIsNAOption) → undefined
                 → hasNAScore = false
                 → numericValue = null, notApplicable = true
                 → excluded from QA score aggregation
```

This distinction is critical: scored N/A counts toward the agent's QA score (with its configured score value), while unscored N/A is excluded entirely (neither rewards nor penalizes).

---

## 12. Summary

| Lifecycle Stage | Verdict | Notes |
|-----------------|---------|-------|
| **Load (decoupled)** | ✅ Correct | `isNA` preserved, null-score N/A left as-is |
| **Load (legacy)** | ⚠️ Latent bug | `not_applicable` not remapped (8.2, impact: ~zero) |
| **Load (deferred)** | ✅ Correct | Not in arrays = nothing to process |
| **Mount migration** | ✅ Correct | Skips templates with N/A options |
| **Edit: Allow N/A** | ✅ Correct | Deferred — no array mutation on toggle |
| **Edit: N/A score** | ✅ Fixed | `not_applicable` now uses array index (8.1) |
| **Edit: Remove** | ✅ Correct | Full remapping of all auto_qa fields |
| **Edit: AutoQA dropdown** | ✅ Correct | `na_no_score` sentinel works properly |
| **Save** | ✅ Correct | `isNA` preserved, re-indexed, deferred = legacy JSON |
| **Grading display** | ✅ Correct | moveScoredNAOptionToHead + addNAOption guard |
| **Grading scoring** | ✅ Correct | Scored N/A participates, unscored excluded |
| **Clone** | ✅ Correct | N/A stripped, paired scores, not_applicable nulled |
| **Validation** | ✅ Correct | Uniqueness for all three fields, bounds check skips N/A |
