# CONVI-7281 Director frontend implementation

**Date:** 2026-09-22
**Primary source repo:** `/Users/xuanyu.wang/repos/director-convi-7281`
**Branch:** `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`
**Base:** Director `origin/main` at `2ab017e85f`

## Outcome

Implemented the Opera applicability selector and persistence lifecycle without changing Director APIs. Current Director already pins `@cresta/web-client` `2.28.16`, which exposes `SOURCE_TYPE_TRAINING_SIMULATOR`.

## Changes

- Added a shared Finalize `ListSelect`, rendered by both `ReviewPolicySection` variants and gated by `enableTrainingSimulator`.
- Added the two exclusive choices “Quality Management and Training Simulator” and “Training Simulator only.”
- Added `policySourceType` to `OperaStore` and `sourceType` to `OperaPolicy`.
- Defaulted new and reset editors to `SOURCE_TYPE_COACH_BUILDER`.
- Hydrated Training Simulator policies on edit and normalized unspecified/legacy values to Coach Builder.
- Carried the source through store-to-policy and policy-to-API mapping, which also makes normal draft serialization preserve it.
- Added `SOURCE_TYPE_TRAINING_SIMULATOR` to the Opera overview query so saved rules remain visible and editable.
- Added focused tests for API hydration/normalization, API save mapping, and store-to-policy mapping.

## Backend clarification

The earlier compatibility warning applied to runtime PR head `00993a5289`, where module-derived IDs were installed as a non-empty `policy_override` and therefore replaced ordinary active-policy loading. Runtime PR head `99d196e76f` removed that resolver/override approach. The current path propagates conversation source into policy search; ordinary traffic excludes Training Simulator-only policies and Training Simulator traffic includes all active policy sources. There is therefore no backend compatibility blocker for this frontend implementation.

## Validation

- `yarn test src/features/opera/utils/preparePolicy.test.ts src/features/opera/utils/prepareOperaPolicy.test.ts src/features/opera/policy-editor/useCreatePolicyToSaveFunction.test.ts` — 3 files, 23 tests passed.
- `yarn tsc` in `packages/director-app` — passed.
- `yarn lint:precommit` — passed for changed Director packages.
- `git diff --check` — passed.

No commit, push, or pull request was created.
