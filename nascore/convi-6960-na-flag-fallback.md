# CONVI-6960: Stored Scored N/A Must Fall Back When `enableNAScore` Is Off

**Created:** 2026-06-01
**Issue:** [CONVI-6960](https://linear.app/cresta/issue/CONVI-6960/scorecard-na-option-inherits-stored-numeric-score-after-enablenascore)
**PR:** [director#19236](https://github.com/cresta/director/pull/19236)

## Investigation

Closed-conversation scoring and scorecard hydration paths were still willing to treat a stored scored-N/A selection as an ordinary numeric option, even after the `enableNAScore` feature flag was disabled.

The result was inconsistent fallback behavior:

- the UI could still surface the stored scored-N/A option
- hydration could reopen scorecards with that numeric option selected
- saving and aggregation could continue to persist and interpret the numeric N/A value instead of converting back to legacy `notApplicable`

## Root Cause

This was an independent feature-flag fallback bug:

1. scoring-side helpers did not consult `enableNAScore`
2. stored scored-N/A values therefore remained numeric through rendering, hydration, submission, and closed-conversation scoring flows even when the flag was off

## Solution

Implemented in `director` on branch `xwang/convi-6960-na-flag-fallback`.

- Kept the fallback logic inside existing scoring files instead of adding standalone helper modules.
- Consolidated the repeated scored-N/A lookup/fallback helpers into `components/scoring/utils.ts`.
- When `enableNAScore` is off:
  - real scored-N/A options are filtered out of the rendered choices
  - the legacy synthetic N/A option is shown instead
  - stored scored-N/A values hydrate as legacy N/A
  - submitted N/A values persist as `notApplicable: true` with no numeric value
- Threaded the flag through existing flows:
  - option rendering
  - score extraction
  - default form hydration
  - process scorecard scoring
  - score save/update flows

## Code Flow Diagram

```mermaid
flowchart TD
  A["Scoring UI reads enableNAScore feature flag"] --> B["CriterionInputDisplay"]
  B --> C["Build effective option list"]
  C --> D{"enableNAScore?"}
  D -->|true| E["Keep scored-N/A option visible"]
  D -->|false| F["Filter option where isNA is true"]

  B --> G["Decide whether legacy N/A sentinel is needed"]
  G --> H{"showNA enabled and legacy fallback needed?"}
  H -->|yes| I["Append INPUT_N_A_VALUE sentinel option"]
  H -->|no| J["Do not append synthetic N/A"]

  K["Existing scorecard opens"] --> L["calculateDefaultFormValues"]
  L --> M["calculateDefaultCriterionFormValue"]
  M --> N["Compare stored numericValue with option marked isNA"]
  N --> O{"Stored numericValue matches scored-N/A option while flag is off?"}
  O -->|yes| P["Hydrate as INPUT_N_A_VALUE"]
  O -->|no| Q["Hydrate as stored numeric value"]

  R["User saves scorecard"] --> S["getScorecardDataFromForm"]
  S --> T["getScoresForCriterion"]
  T --> U["Build partial numeric score"]
  U --> V{"Selected value is INPUT_N_A_VALUE?"}
  V -->|yes, flag on and N/A has score| W["Persist scored N/A numericValue"]
  V -->|yes, flag off or no N/A score| X["Persist legacy N/A: notApplicable true, no numericValue"]
  V -->|no| Y["Persist regular numeric score"]

  Z["Closed-conversation / process scoring"] --> AA["Pass enableNAScore into score extraction and aggregation helpers"]
  AA --> N
```

## Helper Functions

The fix is intentionally built from small helper functions because the same N/A concept is read and written through several paths: option rendering, default hydration, score extraction, and score labels. The important rule is: when `enableNAScore` is off, any stored scored-N/A value is treated as legacy N/A.

### Shared helper layer

`components/scoring/utils.ts` is the shared home for scoring-side N/A helpers.

| Helper | Purpose |
| --- | --- |
| `isOptionBackedNumericCriterion(...)` | Narrows a criterion to labeled radios or dropdown numeric values. These are the only numeric criterion types that can contain a real template option marked `isNA`. |
| `getScoredNAOption(...)` | Finds the real scored-N/A option in `settings.options`. This is different from legacy N/A, which is represented by `INPUT_N_A_VALUE`. |
| `isEffectiveLegacyNASelection(...)` | Treats a stored scored-N/A numeric value as legacy N/A when `enableNAScore` is off. This is the key fallback guard for old scorecards created while scored N/A was enabled. |

This avoids three separate `getScoredNAOption` implementations across:

- `components/scoring/utils.ts`
- `components/scoring/scorecard-criterion/utils.ts`
- `components/scoring/criterion-input/CriterionInputDisplay.tsx`

It also avoids duplicating the effective legacy fallback check between save/hydration logic and scorecard criterion label logic.

### UI helper layer

`components/scoring/criterion-input/CriterionInputDisplay.tsx` keeps UI-specific helpers local because they are about rendering choices, not interpreting saved scores.

| Helper | Purpose |
| --- | --- |
| `getEffectiveOptions(...)` | Removes the real scored-N/A option from rendered choices when `enableNAScore` is off. |
| `shouldRenderLegacyNAOption(...)` | Decides whether the UI should show the legacy `INPUT_N_A_VALUE` sentinel option. It returns true when scored N/A is disabled, or when the template has `showNA` but no real scored-N/A option. |
| `addNAOption(...)` | Adds the legacy sentinel N/A option after the real option list has already been filtered/reordered. This prevents rendering both N/A modes at the same time. |
| `moveScoredNAOptionToHead(...)` | Keeps scored-N/A display order compatible with legacy N/A by moving the real scored-N/A option to the front when scored N/A is enabled. |

## Guarded Paths

### 1. Option rendering guards the button/dropdown UI

The input renderer reads:

```ts
const enableNAScore = useFeatureFlag('enableNAScore');
```

For labeled radios and dropdown numeric criteria, it removes scored-N/A options from rendered choices when the feature flag is off:

```ts
return options.filter((option) => !checkIsNAOption(option));
```

The same file decides whether to append the synthetic sentinel N/A option:

```ts
return !enableNAScore || getScoredNAOption(criterion) == null;
```

So when the flag is off, scored N/A is hidden and the legacy N/A sentinel is shown instead.

Scenario guarded: a closed conversation has a template option marked `isNA`, but `enableNAScore` is now off. The UI must show one legacy N/A choice, not the stored scored-N/A numeric option and not two N/A choices.

### 2. Score label conversion guards criterion display state

`getScoreFromFieldValue` converts form values into saved `Score` objects. It treats either the explicit sentinel or an effective legacy scored-N/A selection as N/A:

```ts
const isFieldValueNA =
  fieldValue[0] === INPUT_N_A_VALUE ||
  isEffectiveLegacyNASelection(criterionTemplate, numericValue, enableNAScore);
```

When `isFieldValueNA` is true, the saved score becomes legacy N/A:

```ts
notApplicable: true,
numericValue: undefined,
```

Scenario guarded: a scorecard criterion receives a numeric value that used to mean scored N/A. With the flag off, label/display logic must classify it as N/A instead of a regular numeric score.

### 3. Save extraction guards what is persisted

`getScorecardDataFromForm` accepts `enableNAScore` and passes it into `getScoresForCriterion`. The numeric score extraction path applies the same feature flag when the user selects the N/A sentinel. If `enableNAScore` is on and the N/A option has a score, it persists the scored-N/A numeric option. Otherwise it persists legacy N/A.

Scenario guarded: the user selects N/A while the flag is off. The submit payload must persist `notApplicable: true` with no numeric value, rather than re-saving the scored-N/A numeric option.

### 4. Default hydration guards reopened scorecards

`calculateDefaultCriterionFormValue` handles reopening an existing scorecard. It maps either `score.notApplicable` or a stored numeric value that points at the scored-N/A option while the flag is off to `INPUT_N_A_VALUE`, so a scorecard saved while scored N/A was enabled reopens as legacy N/A after the flag is disabled.

The default hydration functions also accept the flag:

```ts
calculateDefaultFormValues(..., enableNAScore)
calculateDefaultAppealFormValues(..., enableNAScore)
```

Both paths eventually call the same numeric hydration logic with the same flag. This keeps initial form state, score extraction, and appeal hydration consistent.

Scenario guarded: a previously submitted scorecard stored scored N/A numerically. Reopening it with the flag off must initialize the form as legacy N/A, not as a percentage-backed numeric selection.

### 5. Scoring entry points pass the feature flag

The scorecard form, save mutation, criterion card body, and process scorecard scoring code now read or receive `enableNAScore` and pass it into the helper layer.

That closes the original gap: closed-conversation scoring no longer interprets stored scored-N/A values as regular numeric selections after the feature flag is turned off.

## Verification

The simplified PR intentionally removes the added test files and does not add new source files. Validation should focus on reviewing the existing paths above and manually exercising:

- opening a scorecard with stored scored-N/A while `enableNAScore` is off
- confirming the UI shows one legacy N/A option, not the scored-N/A numeric option
- saving without changing the selection and confirming the saved score is `notApplicable: true` with no numeric value
- turning the flag on and confirming existing scored-N/A behavior remains unchanged

Local validation after helper deduplication:

- `biome check --write packages/director-app/src/components/scoring/utils.ts packages/director-app/src/components/scoring/scorecard-criterion/utils.ts packages/director-app/src/components/scoring/criterion-input/CriterionInputDisplay.tsx`
- `biome check packages/director-app/src/components/scoring/utils.ts packages/director-app/src/components/scoring/scorecard-criterion/utils.ts packages/director-app/src/components/scoring/criterion-input/CriterionInputDisplay.tsx`
- `git diff --check`
- `yarn workspace @cresta/director-app tsc --pretty false`

Latest local commit:

- `b630cbee50 Deduplicate scored N/A helpers`
