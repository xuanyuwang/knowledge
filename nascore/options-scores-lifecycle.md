# Options, Scores & Auto QA — Full Lifecycle

**Created:** 2026-04-25

This document maps every stage in the life of `settings.options`, `settings.scores`, and `auto_qa` for a scorecard criterion template, from DB to form to DB again, and shows where N/A score logic currently hooks in.

---

## 1. Data Model (API / DB shape)

```
criterion: {
  type: 'labeled_radios' | 'dropdown_numeric_values' | 'numeric_radios' | ...
  settings: {
    options: [{ label: string, value: number, isNA?: boolean }]   // ordered list
    scores:  [{ value: number, score: number }]                   // parallel to options
    range:   { min: number, max: number }                         // NumericRadios only
    showNA:  boolean                                              // legacy N/A toggle
    enableMultiSelect: boolean                                    // DropdownNumericValues only
    autoFail: { value, comparator }
    excludeFromQAScores: boolean
    commentSettings: { ... }
  }
  auto_qa: {
    triggers:       [{ type, resource_name }]
    detected:       number | null      // option value (DB) or index (form)
    not_detected:   number | null
    not_applicable: number | null
    options:        [{ ... }]          // "# of occurrences" mode only
  }
  branches: [{ condition: { numeric_values: number[] }, children: [...] }]
}
```

### Current observations (not universal invariants)

The following relationships are **intended** but not uniformly enforced at every stage. In particular, the decoupled load path (section 2a) breaks the first rule in form state.

| Observation | Holds when | Breaks when |
|-------------|-----------|-------------|
| `options[i].value === scores[i].value` | After mutations in CriteriaLabeledOptions (which renormalize); after save transform; in DB | **In form state after decoupled load** — `option.value` is rewritten to `score.score` while `scores[i].value` keeps the original index (section 2a) |
| `options.length === scores.length` | After every mutation in CriteriaLabeledOptions; after save | After load of legacy templates before mount-time migration runs |
| `detected`, `not_detected`, `not_applicable` ∈ `[0..options.length)` | After validation runs | Intermediate form states during editing; `not_applicable` can also be `null` |
| `detected ≠ not_detected ≠ not_applicable` (when non-null) | Enforced by validation.ts + TemplateBuilderAutoQA | — |
| N/A option has `isNA: true` | Everywhere — survives API round trip | — |
| N/A option is always last in arrays | After mutations in CriteriaLabeledOptions (inserts before isNAIndex) | No enforcement in load path or save path |

### Target invariants (after refactor)

After the proposed refactor (section 9), the canonical form state should enforce:

| Invariant | Enforcement |
|-----------|-------------|
| `option.value` = sequential index (identity key), never the actual score | `fromApiToForm()` at load boundary |
| `options[i].value === scores[i].value === i` | All mutations via option-state module |
| `scores[i].score` = actual score | All mutations via option-state module |
| `options.length === scores.length` | All mutations via option-state module |
| `auto_qa.detected/not_detected/not_applicable` and `branch.condition.numeric_values` reference option indices | All mutations via option-state module |

---

## 2. Load Path (API → Form)

**Entry:** `TemplateBuilderForm.tsx` → `calculateDefaultFormValues()` → `addItemTypeToCriterionTemplate()` → `transformApiCriterionTemplateSettingsToForm()`

### 2a. Decoupled scoring path (has scores array)

**Condition:** `(enableMultiSelect || (eligible type && scores.length > 0)) && !isNumOccurrences`

```
API:   options=[{label:"Yes", value:0}, {label:"No", value:1}]  scores=[{value:0, score:1}, {value:1, score:0}]
                                           ↓
Form:  options=[{label:"Yes", value:1}, {label:"No", value:0}]  (value ← score.score)
       scores unchanged (still [{value:0, score:1}, {value:1, score:0}])
```

**What happens:** Each `option.value` is replaced by the matching `score.score`. The `scores` array is NOT modified. This means in the form, `option.value` no longer matches `scores[i].value` — the two arrays are "decoupled" by this transform.

**N/A note:** If an N/A option exists with `score: null`, the option is left as-is (line 684-685). `isNA: true` is preserved.

### 2b. Legacy migration path (no scores array)

**Condition:** Eligible type, no scores, has options, not outcome, not # of occurrences

```
API:   options=[{label:"Yes", value:1}, {label:"No", value:0}]  scores=[]
                                           ↓
Form:  options=[{label:"Yes", value:0}, {label:"No", value:1}]  (value ← index)
       scores=[{value:0, score:1}, {value:1, score:0}]          (created from original values)
       auto_qa.detected/not_detected: remapped via valueToIndexMap
       branches.condition.numeric_values: remapped via valueToIndexMap
```

**N/A note:** `auto_qa.not_applicable` is NOT remapped here (only detected/not_detected are). This is a latent bug.

### 2c. Passthrough path

All other types (NumericRadios, outcomes, # of occurrences): returned as-is, only branches are recursively processed.

### 2d. Secondary migration: `CriteriaLabeledOptions.useOnMount()`

After the form loads, each `CriteriaLabeledOptions` instance runs an `useOnMount` that re-checks if the criterion needs legacy migration (has options but no valid scores and no NA option). This is **redundant** with 2b above but exists as a safety net.

---

## 3. Edit Path (Form mutations)

Mutations to options/scores/auto_qa happen in multiple components — not just the two option editors.

**Primary option editors** (own the options/scores arrays directly):
- **CriteriaLabeledOptions.tsx** — LabeledRadios/DropdownNumeric behavior DND
- **NumericBinsAndValuesConfigurator.tsx** — # of occurrences

**Other mutation sites** (directly `form.setValue` on settings/auto_qa fields):
- **TemplateBuilderCriterionConfiguration.tsx** `resetSettings()` — resets `settings.options`, `settings.scores`, `auto_qa.options`, `auto_qa.detected`, `auto_qa.not_detected` when switching between Behavior DND and # of Occurrences (lines 162-181)
- **TemplateBuilderScoreType.tsx** `handleTypeFieldChange()` — resets `settings` entirely when switching to/from Sentence type, resets `settings.range` when switching to NumericRadios (lines 93-104)
- **TemplateBuilderFormConfigurationStep.tsx** `handleAddCriterion()` — initializes options/scores/auto_qa when creating or cloning criteria (lines 282-355)
- **TemplateBuilderAutoQA.tsx** — mutates `auto_qa.detected`, `auto_qa.not_detected`, `auto_qa.not_applicable` via Select dropdowns

Any refactor that centralizes mutation logic must cover all of these sites, not just the option editors.

### 3a. Add option — `onAddLabel()`

```
newValue = max(options.map(o => o.value)) + 1
if (isNAIndex >= 0):
  insert at isNAIndex (before N/A, to keep it last)
else:
  append
→ options.push/insert({ label: '', value: newValue })
→ scores.push/insert({ value: newValue, score: 0 })
```

### 3b. Remove option — `handleRemoveOption(deletedIndex)`

```
1. Filter out options[deletedIndex] and scores[deletedIndex]
2. Renormalize remaining: option.value = new index, score.value = new index
3. Remap branch conditions via remapIndex(old → new)
4. Remap auto_qa.detected/not_detected/not_applicable via remapIndex
```

### 3c. Edit label

Direct form field: `settings.options.${index}.label` — no side effects.

### 3d. Edit score

Direct form field: `settings.scores.${index}.score` — no side effects.

### 3e. Toggle "Allow N/A" — `handleAllowNAChange()`

```
ON:  showNAField.onChange(true)  — no array mutation
OFF: showNAField.onChange(false)
     if N/A in arrays: handleRemoveOption(isNAIndex)
     clear naScoreInput
```

### 3f. N/A score input — `handleNAScoreChange()` + `handleNAScoreBlur()`

```
onChange(value):
  if number AND N/A not in arrays:
    append { label:'N/A', value:maxId+1, isNA:true } to options
    append { value:maxId+1, score:value } to scores
    set auto_qa.not_applicable = maxId+1
  if number AND N/A in arrays:
    update scores[isNAIndex].score = value

onBlur:
  if input is empty AND N/A in arrays:
    remove N/A from both arrays
    renormalize remaining
    set auto_qa.not_applicable = null
```

### 3g. Auto QA detected/not_detected/not_applicable

Managed in **TemplateBuilderAutoQA.tsx** via Select dropdowns. Values are indices into `settings.options`.

`not_applicable` has a special `na_no_score` sentinel: when N/A has no score (not in arrays), the dropdown shows "N/A (no score)" with value `'na_no_score'`. Selecting it sets `not_applicable = null`.

### 3h. Score type switch (Behavior DND ↔ # of Occurrences)

In **TemplateBuilderCriterionConfiguration.tsx** `resetSettings()` (line 162):
- To Behavior DND: clear `auto_qa.options`, reset to Yes/No `settings.options` + default `settings.scores`
- To # of Occurrences: clear `auto_qa.detected`/`not_detected` to null, clear `settings.options`/`settings.scores` to `[]`

### 3i. Criterion type change (Score Type dropdown)

In **TemplateBuilderScoreType.tsx** `handleTypeFieldChange()` (line 93):
- To Sentence: `settings` set to `undefined` entirely (wipes options, scores, range, showNA)
- From Sentence to any scorable type: `settings` set to `{ showNA: true }` (no options, no scores)
- To NumericRadios: resets `settings.range` to `DEFAULT_CRITERION_SETTINGS_RANGE`
- If branches exist: shows confirmation modal first, resets all `branch.condition.numeric_values` to `[]`

### 3j. Criterion creation / cloning

In **TemplateBuilderFormConfigurationStep.tsx** `handleAddCriterion()` (line 282):
- New criterion: clones `DEFAULT_CRITERION`, initializes `auto_qa` with `{ triggers: [], detected: 1, not_detected: 0, not_applicable: null }`
- Cloning: copies from `defaultCriterion`, filters out N/A options (`checkIsNAOption`), renormalizes values to sequential indices, creates matching scores array, sets `auto_qa.not_applicable = null`

---

## 4. Save Path (Form → API)

**Entry:** `useSaveScorecardTemplate.ts` → `transformTemplateStructureToApi()` → `transformItemToApi()` → `extractCriterionSettingsForApi()`

### 4a. Settings transform by type

| Type | enableMultiSelect | isOutcome | Transform |
|------|-------------------|-----------|-----------|
| NumericRadios | — | no | Strip scores (set to []) |
| NumericRadios | — | yes | Keep scores |
| DropdownNumericValues | true | any | `transformDropdownNumericInputOptionsToApi` (renormalize) |
| DropdownNumericValues | false | yes | Keep as-is |
| DropdownNumericValues | false | no | Strip scores |
| LabeledRadios | — | no, not #occ | `transformDropdownNumericInputOptionsToApi(enableMultiSelect=true)` |
| LabeledRadios | — | no, #occ | Keep options+scores |
| LabeledRadios | — | yes | Keep options+scores |

### 4b. `transformDropdownNumericInputOptionsToApi()` (the key transform)

```
For each option at index i:
  newOption = { label, value: i, isNA? }     // value reset to index
  newScore  = { value: i, score: scores[i]?.score ?? option.value }
```

This is the **inverse** of the load-time decoupling: the form has `option.value = score` (from load), and the save re-indexes `option.value = index` while preserving the actual score in `scores[i].score`.

**N/A note:** `isNA: true` is preserved via spread.

### 4c. Auto QA transform

`transformAutoQAToApi()` — essentially passes through if valid (`isValidAutoQA()`), otherwise returns `undefined`.

**Critical:** auto_qa.detected/not_detected/not_applicable are saved as **indices** (they were already indices in the form). The save path does NOT remap them. This works because option values are re-indexed to 0,1,2... in 4b, which matches the index-based auto_qa values.

### 4d. Other save-time cleanup

- `excludeFromQAScores && !isOutcome` → scores stripped
- Sentence/Date/User types → settings and auto_qa stripped entirely
- Branches → recursively transformed

---

## 5. Grading Path (API → Grader UI)

**Entry:** `CriterionInputDisplay.tsx`

### 5a. Building grader options

For each criterion type, options are built differently:

- **NumericRadios:** Generated from `range.min..range.max`, NOT from settings.options
- **LabeledRadios / DropdownNumericValues with decoupled scoring:** `value: String(index)` — index-based
- **LabeledRadios / DropdownNumericValues without decoupled scoring:** `value: String(option.value)` — original value
- **"# of occurrences":** Uses original values (NOT indices), detected by `isNumOccurrencesCriterion()`

### 5b. `hasDecoupledScoring()` decision

```
return !!criterion.settings?.scores?.length && !isNumOccurrencesCriterion(criterion)
```

This determines whether grading uses index-based or value-based option keys.

### 5c. N/A in grading — `addNAOption()`

Two kinds of N/A can appear:

1. **Scored N/A** (`isNA: true` in options array): Stays in the options list with its real value. `moveScoredNAOptionToHead()` moves it to position 0 for display.

2. **Legacy display N/A** (`showNA: true` but no isNA option): `addNAOption()` prepends a synthetic entry with `value: INPUT_N_A_VALUE` sentinel.

**Guard:** If a scored N/A exists (`options.some(checkIsNAOption)`), the legacy display N/A is NOT added. This prevents duplicate N/A buttons.

### 5d. Score lookup on grading

`calculateCorrectScoreForDropdownNumericAndLabeledRadioTypes()` in `scoring/utils.ts`:

```
scoreValue = criterion.settings.scores.find(s => s.value === selectedValue)?.score
```

For N/A with `INPUT_N_A_VALUE`, `getPartialScoreForNumericValue()` checks:
- If N/A option has a score → treated as regular scored option (notApplicable = false)
- If N/A option has no score → marked `notApplicable = true`, `numericValue = null`

---

## 6. Value Semantics Summary

The most confusing aspect: `option.value` means different things at different stages.

| Stage | option.value means | auto_qa.detected means |
|-------|-------------------|----------------------|
| **DB / API** | Unique ID (often sequential index) | Index into options array |
| **Form (after load)** | The score itself (decoupled) OR index (legacy migrated) | Index into options array |
| **Save (before API)** | Re-indexed to 0,1,2... | Index (unchanged) |
| **Grading (decoupled)** | String(index) | — |
| **Grading (non-decoupled)** | String(original value) | — |

---

## 7. Where N/A Score Currently Hooks In

| Stage | What happens | File | Lines |
|-------|-------------|------|-------|
| **Load (decoupled)** | N/A option with null score left as-is | TemplateBuilderForm.tsx | 684-685 |
| **Load (legacy)** | `isNA: true` preserved in normalization | TemplateBuilderForm.tsx | 691, 714 |
| **Mount migration** | N/A options skipped in legacy migration | CriteriaLabeledOptions.tsx | 95-100 |
| **Edit: Allow N/A** | Only toggles showNA, no array mutation | CriteriaLabeledOptions.tsx | 174-183 |
| **Edit: N/A score input** | Adds/removes N/A from arrays on score change | CriteriaLabeledOptions.tsx | 132-172 |
| **Edit: Remove option** | Remaps auto_qa.not_applicable | CriteriaLabeledOptions.tsx | 220-224 |
| **Edit: Clone criterion** | Strips N/A from copied options, nulls not_applicable | ConfigurationStep.tsx | 318-332 |
| **AutoQA dropdown** | na_no_score sentinel for scoreless N/A | TemplateBuilderAutoQA.tsx | 155-174 |
| **Save** | isNA preserved via spread | useSaveScorecardTemplate.ts | 130 |
| **Grading display** | moveScoredNAOptionToHead + addNAOption guard | CriterionInputDisplay.tsx | 399-448 |
| **Grading score lookup** | Checks isNA for notApplicable flag | scoring/utils.ts | 546-561 |
| **Validation** | Allows null score for N/A, bounds check | validation.ts | various |

---

## 8. Pain Points & Fragility

### 8.1 Dual migration paths
`transformApiCriterionTemplateSettingsToForm()` (load-time) and `CriteriaLabeledOptions.useOnMount()` (mount-time) both do legacy migration. The mount-time one is a fallback for edge cases the load-time one misses, but this makes the flow hard to reason about.

### 8.2 Decoupling semantics change option.value meaning
After load, `option.value` holds the score value (not the index), but `scores[i].value` still holds the original index. This means `option.value !== scores[i].value` in the form, which is confusing and error-prone.

### 8.3 Renormalization scattered across handlers
Every handler that modifies options (add, remove, N/A score change, N/A score blur, allow N/A uncheck) independently renormalizes values to 0,1,2... and keeps scores in sync. This is the same logic copy-pasted with slight variations.

### 8.4 Auto QA index remapping is manual
When an option is deleted, detected/not_detected/not_applicable must be remapped. This is done inline in `handleRemoveOption`. If any new auto_qa field is added, this remapping must be updated in every component that removes options.

### 8.5 N/A position management is ad-hoc
N/A must be last in arrays (for correct index alignment) but first in grading UI (for UX). This is managed by: (a) inserting new options before isNAIndex in `onAddLabel`, (b) `moveScoredNAOptionToHead()` in grading. If either is missed, position bugs appear.

### 8.6 No single source of truth for "has scored N/A"
Multiple places check `options.some(checkIsNAOption)` vs `showNA` vs `isNAIndex >= 0` independently. Each makes its own determination of N/A state.

### 8.7 Load transform doesn't remap not_applicable
In the legacy migration path of `transformApiCriterionTemplateSettingsToForm()` (line 731-741), `detected` and `not_detected` are remapped via `valueToIndexMap`, but `not_applicable` is NOT. This is a latent bug — if a legacy template has `not_applicable` set, it won't be correctly remapped.

### 8.8 Save-time index renormalization depends on form state
`transformDropdownNumericInputOptionsToApi()` re-indexes options to 0,1,2... But it takes `scores[i]?.score ?? option.value` as fallback. Since `option.value` in the form is the score (from decoupling), this works by accident — if the form shape changes, this could break.

---

## 9. Proposed Refactoring Direction

> Cross-validated against `codex-refactor-design.md` (independently produced Codex design).
> The two analyses agree on the core problem and converge on the same architecture.
> Differences are reconciled below — this section represents the merged plan.

### 9.1 Canonical form invariant

**`option.value` is never treated as the actual score in canonical form state.**

- `option.value` = sequential index (identity key, 0-based)
- `scores[i].value` = same index (matches `option.value`)
- `scores[i].score` = actual score
- `auto_qa.detected`, `not_detected`, `not_applicable` = option indices
- `branch.condition.numeric_values` = option indices

This eliminates the current confusion where `option.value` means "score" after load but "index" after save.

### 9.2 Two-level module split

Splitting into two packages (from Codex design) fixes the dependency direction: runtime scoring currently imports `checkIsNAOption` from template-builder internals, which is wrong.

**Level 1 — Shared read helpers in `director-api`:**

```typescript
// Pure read-only helpers used by both builder and runtime
function isNAOption(opt: { isNA?: boolean }): boolean;
function findNAOption(options: NumericInputOption[]): NumericInputOption | undefined;
function getScoreForOptionValue(scores: SettingsScore[], value: number): number | undefined;
function usesIndexedScoreValues(criterion: ScorableScorecardCriterionTemplate): boolean;
function isOccurrenceBasedAutoQACriterion(criterion: ScorecardCriterionTemplate): boolean;
```

**Level 2 — Builder-side option-state module in template-builder:**

```typescript
// Pure mutation functions — no React, no form dependency
type OptionsState = {
  options: NumericInputOption[];
  scores: SettingsScore[];
  autoQa: { detected: number | null; not_detected: number | null; not_applicable: number | null };
  branches: ScorecardCriterionBranch[];
  showNA: boolean;
};

// Mutations (return new state, always renormalize internally)
function addOption(state: OptionsState, label: string, score: number): OptionsState;
function removeOption(state: OptionsState, index: number): OptionsState;
function updateScore(state: OptionsState, index: number, score: number): OptionsState;
function updateLabel(state: OptionsState, index: number, label: string): OptionsState;

// N/A operations
function addNAWithScore(state: OptionsState, score: number): OptionsState;
function removeNA(state: OptionsState): OptionsState;
function updateNAScore(state: OptionsState, score: number): OptionsState;
function toggleShowNA(state: OptionsState, enabled: boolean): OptionsState;

// Queries (delegate to director-api helpers)
function getNAIndex(state: OptionsState): number;
function hasNA(state: OptionsState): boolean;
function hasScoredNA(state: OptionsState): boolean;
function getNAScore(state: OptionsState): number | null;

// Load boundary transform
function fromApiToForm(criterion: ScorecardCriterionTemplate): OptionsState;

// Save-time helpers (shared canonicalization, NOT a single serializer —
// type-specific save rules in extractCriterionSettingsForApi remain)
function reindexOptionsAndScores(state: OptionsState): { options, scores };
function canonicalizeAutoQA(state: OptionsState): { detected, not_detected, not_applicable };
```

**Occurrence-based criteria excluded:** They keep their literal-value model and are not forced into indexed remapping. The module should check `isOccurrenceBasedAutoQACriterion` and skip reindexing for those.

### 9.3 Move normalization to boundaries

| Boundary | Current | Proposed |
|----------|---------|----------|
| **API → Form** | Load-time `transformApiCriterionTemplateSettingsToForm` + mount-time `CriteriaLabeledOptions.useOnMount` | Single `fromApiToForm()` at load. Remove mount-time migration entirely. |
| **New criterion defaults** | `DEFAULT_CRITERION` has options but NO scores array — components repair later | `DEFAULT_CRITERION` includes `scores` to be canonical from the start |
| **Component mutations** | Inline renormalize + remap in each handler across 4+ components | Call option-state module functions from all mutation sites (including `resetSettings`, `handleTypeFieldChange`, `handleAddCriterion`) |
| **Form → API** | Type-specific switch in `extractCriterionSettingsForApi` | **Keep the type-specific save rules** — they encode deliberate per-type semantics (non-outcome NumericRadios strips scores, evaluative outcomes keep raw options/scores, non-outcome dropdowns reindex, # of occurrences keeps literal values). Extract shared canonicalization helpers (e.g., reindex options/scores to 0,1,2..., preserve `isNA`) but do NOT collapse into a single uniform serializer. |

### 9.4 Fix `DEFAULT_CRITERION`

Currently missing `scores`, forcing mount-time migration:
```typescript
// Current (incomplete)
DEFAULT_CRITERION = {
  settings: {
    options: [{ label: 'Yes', value: 1 }, { label: 'No', value: 0 }],
    showNA: true,
  },
};

// Proposed (canonical)
DEFAULT_CRITERION = {
  settings: {
    options: [{ label: 'Yes', value: 0 }, { label: 'No', value: 1 }],
    scores: [{ value: 0, score: 1 }, { value: 1, score: 0 }],
    showNA: true,
  },
};
```

### 9.5 Save-time fallback risk

**Must fix during refactor:** `transformDropdownNumericInputOptionsToApi` uses `scores[i]?.score ?? option.value` as fallback. Currently this works because the decoupled load put the score into `option.value`. In canonical form (`option.value = index`), this fallback would return the index instead of the score, silently corrupting data. When adopting canonical form state, the save path must be updated to never fall back to `option.value`.

### 9.6 Benefits

1. **Single renormalization**: `removeOption` always renormalizes and remaps everything. No duplicated logic.
2. **N/A invariants enforced**: N/A always last in arrays, `auto_qa.not_applicable` always in sync.
3. **Testable**: Pure functions, easy to unit test every mutation.
4. **Load boundary is the single normalization point**: `fromApiToForm` handles all legacy/decoupled/passthrough logic in one place.
5. **Save path stays intentional**: Type-specific save rules preserved; shared helpers avoid duplicating reindex logic.
6. **New fields safe**: Adding a new auto_qa field? Update `removeOption` once.
7. **Correct dependency direction**: Runtime scoring imports from `director-api`, not from template-builder.
8. **No mount-time migration**: `DEFAULT_CRITERION` is already canonical. Load transform is the only normalization point.

### 9.7 Component integration

The React components would:
1. Read `OptionsState` from form (via `useWatch`)
2. Call pure functions to compute new state
3. Write new state back to form (via `form.setValue` or field array methods)

This decouples the mutation logic from the form framework.

### 9.8 Runtime scoring updates

Update `CriterionInputDisplay.tsx` and `scoring/utils.ts` to use `director-api` helpers:
- Replace `checkIsNAOption` import from template-builder with `isNAOption` from `director-api`
- Replace inline `hasDecoupledScoring` with `usesIndexedScoreValues`
- `addNAOption` + `moveScoredNAOptionToHead` + sentinel → single `getGradingOptions(criterion)` function

### 9.9 Pre-refactor bug fix

**Fix before or during refactor:** In `transformApiCriterionTemplateSettingsToForm()` (TemplateBuilderForm.tsx, line 731-741), the legacy migration path remaps `detected` and `not_detected` via `valueToIndexMap` but skips `not_applicable`. Add the same remapping for `not_applicable`.

---

## 10. Cross-Validation Notes

This document was cross-validated against `codex-refactor-design.md` (an independently produced Codex analysis of the same codebase). Key reconciliation points:

| Topic | This doc | Codex doc | Resolution |
|-------|----------|-----------|------------|
| Module structure | Single `CriterionOptionsManager` | Two-level: `director-api` helpers + builder module | **Adopted Codex's split** — fixes dependency direction |
| `DEFAULT_CRITERION` fix | Mentioned as pain point | Explicit proposal to add scores | **Adopted Codex's explicit fix** |
| Canonical invariant | Documented confusion, no crisp target | "option.value is never the score" | **Adopted Codex's statement** as section 9.1 |
| `not_applicable` remap bug | Identified as latent bug | Not mentioned | **Kept** — added as pre-refactor fix (9.9) |
| Save fallback risk | Identified `scores[i]?.score ?? option.value` issue | "Update save path as needed" | **Kept detailed analysis** — added as 9.5 |
| Value semantics table | Full stage-by-stage table (section 6) | Not present | **Kept** — critical implementation reference |
| Grading path detail | Full flow documented (section 5) | Mentioned but not traced | **Kept** — needed for runtime helper updates |
| Occurrence-based criteria | Noted as excluded | Explicit exclusion + `isOccurrenceBasedAutoQACriterion` | **Aligned** — both agree, Codex names the helper |
