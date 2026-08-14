# CONVI-7280 investigation and implementation plan

**Date:** 2026-08-10
**Primary domain:** `training-simulator`
**Primary subdomain:** `training-content`
**Source repo:** `/Users/xuanyu.wang/repos/director`
**Branch/worktree context:** `/Users/xuanyu.wang/repos/director-convi-7280` on `convi-7280-add-warning-for-opera-rules-deactivated-in-module-view`, based on `origin/main` at `9bc70a6187`; the original checkout's unrelated modification remains untouched
**Ticket:** [CONVI-7280](https://linear.app/cresta/issue/CONVI-7280/add-warning-for-opera-rules-deactivated-in-module-view)

## Request

Investigate and plan the ticket using the Training Simulator domain context. The ticket asks to reuse Performance Config's pattern in two places:

1. the Training Simulator modules table;
2. the affected evaluation criterion in the module editor.

The supplied screenshots show an orange alert-circle tooltip beside an affected Performance Template title and orange warning text on an affected criterion.

## Inputs reviewed

- Training Simulator domain `project.yaml`, `README.md`, training-content and evaluation subdomain notes.
- Linear CONVI-7280, parent CONVI-7362, sibling CONVI-7281, and the ticket screenshots. CONVI-7280 has no comments, relations, PR, or implementation commit.
- Current `director/origin/main` after fetch, including recent Training Simulator UI changes through 2026-07-29 and newer quiz/module work.
- Current `go-servers/origin/main` and `cresta-proto/origin/main` for contract verification.
- Performance Config warning implementation:
  - `ScorecardTemplates.tsx`
  - `useScorecardTemplatesColumns.tsx`
  - `TemplateBuilderCriterion.tsx`
  - `TemplateBuilderAutoQA.tsx`
  - `useTriggerActivationStatusByName.ts`
- Training Simulator implementation:
  - `tabs/lesson-configuration/ModulesTab.tsx`
  - `tabs/lesson-configuration/types.ts`
  - `create-module/CreateModule.tsx`
  - `create-module/evaluation-criteria/EvaluationCriteriaStep.tsx`
  - `create-module/evaluation-criteria/CriterionListCard.tsx`
  - `create-module/evaluation-criteria/CriterionForm.tsx`
  - `create-module/evaluation-criteria/getTriggerOptions.ts`
  - `create-module/form/mappers.ts`
  - `hooks/coaching/useAutoQATriggers.ts`
- Training Simulator contracts and backend:
  - `cresta/v1/trainingsimulator/training_module.proto`
  - `cresta/v1/trainingsimulator/training_simulator_service.proto`
  - `apiserver/internal/trainingsimulator/action_list_training_modules.go`
  - policy-to-behavior status propagation in `policy_db_model_builder.go`

## Current implementation map

### Module table

`ModulesTab` calls `useListTrainingModules` and retains the full `TrainingModule` on each `ModuleRow`. The module name cell is the correct location for the table warning. Current `origin/main` already wraps/truncates the module name with `SharedTooltip`, so the alert icon should be added to that current layout rather than the stale local version.

Every returned module includes `evaluationConfig.criteria`; only scenarios are conditionally hydrated. No backend change is needed to obtain criterion behavior IDs.

### Module editor

`CreateModule` loads a module through `ListTrainingModules({ trainingModuleNames: [...] })` and separately calls `useAutoQATriggers`. `trainingModuleToFormValues` resolves each stored bare `behaviorId` to the full behavior resource name when possible.

`EvaluationCriteriaStep` renders left-side `CriterionListCard` components. It builds display data with `getTriggerOptions`, but currently excludes inactive/archived behaviors because it passes neither a selected default nor `shouldIncludeInactiveAndArchived=true`. An existing linked inactive criterion can therefore show “Select a behavior.”

`CriterionForm` renders the right-side behavior selector. It passes the selected resource name to `getTriggerOptions`, so an already-selected inactive behavior stays visible and receives the existing “(inactive)” or “(archived)” label suffix.

### Activation source

`useAutoQATriggers` loads:

- Opera policies in ACTIVE and INACTIVE states;
- behaviors in ACTIVE, INACTIVE, and ARCHIVED states;
- outcome metadata.

The behavior record is sufficient for this ticket. Policy state is propagated onto behavior state:

- active policy → ACTIVE behavior;
- inactive/draft policy → INACTIVE behavior;
- archived/deleted policy → ARCHIVED behavior.

The module's `EvaluationCriterion.behavior_id` is actually a behavior ID, despite the proto comment calling it an Opera rule ID.

## Performance Config precedent

Performance Config derives a map of policy/behavior resource name to `isActive`:

- table: scan template criteria; show `IconAlertCircle` in `var(--extended-content-orange)` with tooltip “This template includes a deactivated behavior”;
- criterion tree: show orange text “This criterion is linked to a deactivated behavior”;
- right-side selector: show an alert icon and remediation tooltip.

CONVI-7280 and its two screenshots ask for the first two surfaces. The third right-panel warning is a useful precedent but is not required by the ticket as written.

## Enterprise context, vetted

- [CONVI-7280](https://linear.app/cresta/issue/CONVI-7280/add-warning-for-opera-rules-deactivated-in-module-view), updated 2026-07-27 — official requirement, exact relevance, high confidence.
- [Training Simulator inactive-criteria incident](https://cresta.enterprise.slack.com/archives/C04NB5AMV0F/p1784585156708259), updated 2026-07-28 — cross-functional incident discussion, exact relevance, high confidence. It distinguishes deactivation from active criteria returning N/A because of targeting or absent annotations.
- [CONVI-7209](https://linear.app/cresta/issue/CONVI-7209/redirect-agent-user-id-in-training-simulator-conversation-to-user-id), updated 2026-07-21 — released target-agent correction, direct relevance, high confidence. An active targeted rule must not be classified as deactivated.
- [CONVI-7221](https://linear.app/cresta/issue/CONVI-7221/review-trainingsimualtor-conversation-evaluation-for-na), updated 2026-07-15 — released N/A handling, direct relevance, high confidence. N/A remains scoring/runtime state and must not drive this warning.
- [CONVI-7281](https://linear.app/cresta/issue/CONVI-7281/add-training-simulator-only-option-for-new-opera-rule-creation), updated 2026-07-27 — official sibling proposal, high relevance, medium-high confidence. Product-area applicability is not yet implemented and should remain distinct from activation.
- [Training Simulator Design](https://docs.google.com/document/d/1GCeE9XCAVcgetOhYWPvqZJ3hp3qeVhK5YMk4rBkWmd4/edit?tab=t.0), thread-confirmed 2026-05-06 — engineering design, broad relevance, medium confidence. It contains no warning-specific design; the ticket and incident are authoritative.

No dedicated warning design doc or implementation PR exists. Product owner is Krystal Truong; ticket assignee is Xuanyu Wang; Jack Jee owns relevant evaluation/identity context; Kurt Choi has recent Training Simulator frontend context.

## Conclusions

### Required behavior

- Show a table warning when any module criterion references a behavior that is not ACTIVE.
- Show criterion-level warning text only on each affected criterion card.
- ACTIVE criteria never warn, including targeted rules.
- INACTIVE and ARCHIVED behaviors warn.
- After behavior data has loaded, unresolved/deleted behavior references warn, matching Performance Config's fail-closed behavior.
- While behavior data is loading or unavailable, do not warn.
- Warnings are informational only and do not change save, evaluation, or scoring behavior.

### Recommended implementation shape

1. Add a small Training Simulator behavior-activation resolver near the evaluation-criteria code or shared Training Simulator utilities.
   - Input: `AutoQATriggers | undefined`.
   - Output: activation lookup or an explicit unknown state.
   - Normalize both full behavior resource names and bare behavior IDs.
   - Mark only `BehaviorBehaviorStatus.ACTIVE` as active.
   - Keep naming activation-specific so future CONVI-7281 applicability state is not conflated.

2. Update `ModulesTab.tsx`.
   - Call `useAutoQATriggers`.
   - Build the lookup once with `useMemo`.
   - Derive `hasDeactivatedBehavior` for each row by scanning `module.evaluationConfig?.criteria`.
   - Add that boolean to `ModuleRow` or derive it in the name cell.
   - Render `SharedTooltip` + orange `IconAlertCircle` beside the current truncated module title.
   - Suggested copy: “This module includes a deactivated behavior.”

3. Update `EvaluationCriteriaStep.tsx` and `CriterionListCard.tsx`.
   - Compute activation for each watched `behaviorResourceName`.
   - Pass an explicit `hasDeactivatedBehavior` prop to the card.
   - Render orange text: “This criterion is linked to a deactivated behavior.”
   - Preserve the saved `displayName` as a fallback and/or build the lookup with inactive/archived behaviors included, so the card does not regress to “Select a behavior.”

4. Add localized Training Simulator copy under `director-app-coaching`.

5. Add tests.
   - Activation resolver: full/bare IDs; ACTIVE, INACTIVE, ARCHIVED, absent ID, and undefined source data.
   - Modules table: aggregate icon only for affected module; tooltip copy; no warning while activation state is unknown.
   - Criterion card/step: warning only for affected criterion; saved label remains visible.
   - Explicitly avoid tests that infer deactivation from N/A or targeting because those states are outside this UI's input.

### Rejected/avoided approaches

- Proto/backend enrichment: unnecessary for these two frontend surfaces and adds rollout/cross-client cost.
- Importing the admin feature's Zustand activation store directly: creates cross-feature coupling and hidden initialization requirements. A small explicit resolver is safer.
- Using evaluation N/A/pending as a proxy: technically incorrect and contradicted by released N/A/targeting behavior.
- Blocking module save: not requested; the established Performance Config pattern warns only.
- Broadly consolidating the two `getTriggerOptions` implementations: useful cleanup but unnecessary scope expansion for this ticket.

## Risks and edge cases

- The current proto comment can cause policy-ID/behavior-ID confusion; do not join `behavior_id` directly to policies.
- `display_name` is stored on the module and can be stale after Opera renames; activation must use ID, not label.
- Loading all Opera triggers on the modules table adds a query, though React Query should cache/dedupe the same source used by the editor.
- Returning from an Opera edit may require the normal query refetch policy before the warning disappears; follow existing Performance Config behavior rather than adding bespoke polling.
- Archived behavior references must still be resolvable for warning/label display.
- Current local `director/main` is unsuitable for implementation: it was 1,109 commits behind before fetch and has an unrelated modification. Use a clean worktree from current `origin/main`.

## Validation plan

- Targeted unit/component tests for the helper, table, and criterion card.
- Typecheck and lint affected Director files.
- Manual checks with modules containing active-only, mixed active/inactive, archived, and missing behavior references.
- Confirm no visual warning during trigger loading/error.
- Confirm replacing/reactivating behavior clears the warning after data refresh.
- Confirm module save payload remains unchanged.

## Implementation completed

Implemented the plan in `/Users/xuanyu.wang/repos/director-convi-7280`:

- Added `behaviorActivation.ts` with a lookup keyed by both full behavior resource name and bare behavior ID. Only ACTIVE is treated as active; missing references fail closed after data is loaded, while undefined/loading/error state does not warn.
- Updated `ModulesTab.tsx` and `ModuleRow` to load current Opera triggers, derive aggregate module warning state, and render an accessible orange `IconAlertCircle` tooltip beside affected module names.
- Updated `EvaluationCriteriaStep.tsx` to derive per-criterion activation state, include inactive/archived behaviors in its lookup-only options, and fall back to the persisted criterion display name.
- Updated `CriterionListCard.tsx` to render the orange criterion warning text.
- Ran i18n extraction, producing the two keys in `director-app-coaching.json`.
- Added four focused test files covering the activation resolver, aggregate behavior state, module-name warning UI, criterion-card warning UI, and inactive behavior option lookup.

Validation results:

- Targeted Vitest: 4 files and 12 tests passed.
- `yarn tsc`: passed.
- `yarn workspace @cresta/director-app lint:precommit`: passed.
- Targeted `yarn biome check`: passed.
- `git diff --check`: passed.
- `yarn i18n:full-extract`: completed and generated locale entries; it emitted the repository's existing unrelated `GlobalAlerts.tsx` dynamic-key extraction warning.

## Next action

Manually verify the warnings against an inactive/archived Opera behavior, then review and commit the diff when requested.
