# Refactor Plan: Canonical Score Option State for NA / Scores / AutoQA

## Summary

Refactor the template-builder and runtime scoring code around one canonical FE model for scorable option criteria.

For all decoupled criteria (`labeled-radios`, `dropdown-numeric-values`, multi-select dropdowns, textual/numeric outcome options), the form state will use a single indexed option-key domain:

- `settings.options[].value` = stable option key used only for identity
- `settings.scores[].value` = same option key
- `settings.scores[].score` = actual score
- `auto_qa.detected`, `not_detected`, `not_applicable`, and `auto_qa.options[].value` all reference that same option key
- branch condition `numeric_values` also reference that same option key

Occurrence-based behavior criteria remain on their existing literal-value model and are explicitly excluded from this indexed helper flow.

## Investigation Result

The current FE issue is structural, not just patch quality. One logical concept, “criterion score option state,” is split across these carriers:

- `settings.showNA`
- `settings.options`
- `settings.scores`
- `auto_qa.{detected, not_detected, not_applicable, options}`
- branch condition `numeric_values`

Those are updated in different places, with mixed value domains:

- sometimes `option.value` is treated as the persisted score
- sometimes it is treated as an indexed key
- sometimes AutoQA and branch references are reindexed manually

That is why NA-related changes are fragile.

### Current lifecycle touchpoints

- Load from API into form:
  - `packages/director-app/src/features/admin/coaching/template-builder/TemplateBuilderForm.tsx`
  - `transformApiCriterionTemplateSettingsToForm`
- Initialize / mutate in builder:
  - `TemplateBuilderFormConfigurationStep.tsx`
  - `CriteriaLabeledOptions.tsx`
  - `NumericBinsAndValuesConfigurator.tsx`
  - `TemplateBuilderAutoQA.tsx`
- Save back to API:
  - `useSaveScorecardTemplate.ts`
- Runtime read / scorecard rendering:
  - `packages/director-app/src/components/scoring/utils.ts`
  - `CriterionInputDisplay.tsx`

### Main findings

- The form layer is doing legacy migration and normalization in multiple places instead of one boundary.
- New criteria still start from defaults that are not fully in the canonical decoupled-score shape, so components repair them later.
- N/A is not first-class. It is partly modeled by `showNA`, partly by a synthetic UI option, and partly by `auto_qa.not_applicable`.
- `NumericBinsAndValuesConfigurator` and `CriteriaLabeledOptions` both own reindexing logic; they should not.
- Runtime scoring currently depends on a builder helper for `isNA`, which is the wrong dependency direction.

### Where NA should fit in the lifecycle

- `showNA=true` means N/A is allowed
- unscored N/A means no persisted N/A option row and `auto_qa.not_applicable = null`
- scored N/A means a real N/A option exists in `settings.options` + `settings.scores`, and `auto_qa.not_applicable` points at that same option key
- all other references must use the same option-key domain

## Key Changes

### 1. Introduce shared score-option domain helpers

Add shared pure helpers in `director-api` for read semantics used by both builder and runtime:

- `isNAOption`
- `findNAOption`
- `getScoreForOptionValue`
- `usesIndexedScoreValues`
- `isOccurrenceBasedAutoQACriterion`

This removes the current dependency from runtime scoring code back into template-builder internals.

### 2. Add one builder-side option-state module

Create a single pure module in template-builder for indexed-option lifecycle operations. It owns:

- normalize API/form option state into canonical indexed form
- migrate legacy option-only criteria into `{ options + scores }`
- remove an option and reindex all references together
- add/update scored N/A
- remove scored N/A
- remap:
  - `settings.options`
  - `settings.scores`
  - `auto_qa.options`
  - `auto_qa.detected`
  - `auto_qa.not_detected`
  - `auto_qa.not_applicable`
  - branch condition `numeric_values`

No component should manually reindex arrays after this change.

### 3. Move normalization to boundaries, not components

Change lifecycle boundaries so normalization happens exactly here:

- API -> form in `TemplateBuilderForm`
  - normalize existing decoupled criteria once
  - migrate legacy templates once
  - stop converting option values back and forth inside UI components
- new criterion defaults
  - make `DEFAULT_CRITERION` already canonical by including `scores`
  - copied criteria should use normalized option state
  - copying strips scored N/A rows but preserves `showNA`
- component mutations
  - `CriteriaLabeledOptions` and `NumericBinsAndValuesConfigurator` call the shared option-state module for add/remove/NA operations
  - remove local remap logic from those components

### 4. Keep save path as the boundary back to persisted shape

Keep the persisted API shape unchanged.

Update `useSaveScorecardTemplate` only as needed so it consumes canonical form state instead of compensating for mixed in-form representations. The save path remains responsible for serializing the canonical FE model back into existing template JSON.

### 5. Unify runtime NA read semantics

Update runtime scoring/rendering helpers to use the shared `director-api` helpers:

- `CriterionInputDisplay`
- `components/scoring/utils.ts`

This ensures manual scoring, AutoQA display, and score lookup all resolve N/A the same way.

## Public Interfaces / Types

No backend API or DB schema changes.

Frontend-only additions:

- shared FE helper functions in `director-api`
- a new template-builder internal option-state utility module

Canonical FE invariant for indexed criteria:

- option identity and score value are separate concepts
- `option.value` is never treated as the actual score in canonical form state

## Test Plan

Add unit tests for the new option-state module covering:

- legacy options-only migration to indexed `options + scores`
- deleting a normal option reindexes:
  - options
  - scores
  - AutoQA references
  - AutoQA option values
  - branch conditions
- adding scored N/A creates:
  - N/A option row
  - score row
  - `auto_qa.not_applicable`
- removing scored N/A clears the same consistently

Add or update targeted tests for runtime helpers:

- unscored N/A resolves to not-applicable / no score
- scored N/A resolves through score lookup correctly
- `CriterionInputDisplay` uses indexed option keys for decoupled criteria
- occurrence-based behavior criteria do not switch to indexed helpers

Smoke-check scenarios after implementation:

- create new criterion
- copy existing criterion
- toggle allow N/A on/off
- give N/A a score, then clear it
- delete middle option
- configure behavior DND
- configure number-range / exact-value AutoQA options
- load existing template and save without semantic drift

## Assumptions

- This refactor is FE-only; persisted API shape and backend semantics stay unchanged.
- `showNA=true` means N/A is available even when there is no scored N/A row.
- A scored N/A remains represented by a real option+score row plus `auto_qa.not_applicable`.
- Occurrence-based behavior criteria keep their current literal-value behavior and are not forced into indexed remapping.
- The goal is architecture cleanup and lifecycle correctness, not changing scoring policy.
