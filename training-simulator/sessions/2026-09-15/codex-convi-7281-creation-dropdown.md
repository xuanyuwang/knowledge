# CONVI-7281 Opera creation and dropdown refresh

**Date:** 2026-09-15
**Primary source repo:** `/Users/xuanyu.wang/repos/director`
**Branch/worktree context:** read-only inspection of `director/origin/main` at `0c5c9adb38f8bdd28f5f42ecdd565c4515e9c7db` (2026-09-14); related contract check of `cresta-proto/origin/main` at `ad3c0a73d1c5a79a431905f622c538bd51c94c99`

## Request

Locate current Opera rule creation and identify the established dropdown pattern for CONVI-7281 before implementation planning.

## Live ticket evidence

- CONVI-7281 is Todo and explicitly proposes an “applicable product areas” field in step 4, Finalize.
- The choices are production conversations, Training Simulator, or both.
- The ticket references the Opera 3.0 settings design, has one comment asking Phoebe Wang to add design, and is related to COA-2918.
- No customer needs or releases are attached.

## Current creation path

1. `packages/director-app/src/features/opera/rules-overview/OverviewPage.tsx` navigates from Add New Rule to `ABS_ROUTES.OPERA.RULE_CREATE`.
2. `packages/director-app/src/routes/routes.ts` defines that relative route as `rule/create` under Opera.
3. `packages/director-app/src/features/opera/CoachBuilderRouter.tsx` maps the route to `CreatePolicy`.
4. `packages/director-app/src/features/opera/create-policy/CreatePolicy.tsx` loads Opera system/behavior data and renders `CreateAndEditPolicy variant="create"`.
5. `packages/director-app/src/features/opera/policy-editor/CreateAndEditPolicy.tsx` owns the Define, Configure, Analyze, and Finalize tabs.
6. Finalize renders `ReviewPolicySection` or `ReviewPolicySectionWithPerformanceConfig` depending on `enablePerformanceConfigSelection`.

Implication: the ticket's field belongs in the Finalize grid, but must be present in both Finalize variants. Prefer a shared Opera-specific component over duplicating selector logic.

## Dropdown precedents

### Closest semantic/visual precedent

`components/data-management/RegularMetadataDrawer/components/ApplicableAreasSelect.tsx` already renders a field named “Applicable product areas” with `ListSelect`:

- multi-select value;
- `showSelectAll`;
- `hideSearchBar`;
- popover width matching the target;
- custom option rows with descriptions;
- “All areas” and “No areas” display text.

Do not import it directly into Opera. It is coupled to React Hook Form's `applicableAreas`, `MomentAppType`, Data Management translations, and metadata-specific options. Reuse the interaction shape with an Opera policy product-area enum.

### Closest Opera state-flow precedent

The Finalize `selectedUsecases` flow is controlled from `OperaStore`, updates via `updateState`, and calls `emitDraftSave`. That is the right lifecycle pattern for applicability. The `UsecasesPicker` UI itself is a specialized chip/`LevelsSelectionPopover` design and is heavier than needed for two product-area choices.

## State and contract touchpoints

- Add the selection to `OperaStore` and its explicit create/reset/edit hydration paths.
- Round-trip it through `OperaPolicy`, `getOperaPolicyFromState`, `prepareToPolicy`, and `prepareToOperaPolicy`.
- Because store state participates in dirty/draft serialization unless ignored, adding the field naturally makes changes draft-visible; draft round-trip still needs a focused test.
- Current Director and public policy proto contain no product-area applicability field. Generated client support is a prerequisite.

## Conclusion

The current placement and UI pattern are clear: add an Opera-specific `ListSelect` row to Finalize, shared by both Finalize implementations. Use the Data Management selector as visual behavior prior art and the Opera use-case selector as state/draft lifecycle prior art. This remains more than a frontend-only change because the selection has no current policy contract and must affect runtime policy selection.
