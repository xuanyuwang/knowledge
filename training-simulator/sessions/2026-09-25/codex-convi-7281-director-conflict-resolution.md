# CONVI-7281 Director PR conflict resolution

**Date:** 2026-09-25
**Primary source repo:** `/Users/xuanyu.wang/repos/director-convi-7281`
**Branch:** `xw/convi-7281-add-training-simulator-only-option-for-new-opera-rule`
**PR:** [director#23164](https://github.com/cresta/director/pull/23164)

## Outcome

Merged current Director `main` into the published PR branch without rebasing or rewriting history. The only conflicts were additive test conflicts in `prepareOperaPolicy.test.ts` and `preparePolicy.test.ts`; the resolution retains both CONVI-7281 source-type coverage and upstream Deputy Agent action coverage.

Committed the merge as `8fd7d0ed5f` and pushed it over HTTPS. GitHub reports the PR head as that commit and `mergeable: MERGEABLE`; `mergeStateStatus: BLOCKED` reflects checks/review state rather than conflicts.

## Validation

- Focused Vitest suite: 3 files, 25 tests passed.
- Biome on both resolved files: passed.
- Typed ESLint on both resolved files: passed.
- `git diff --check`: passed before commit.
- Commit formatting and i18n extraction hooks: passed.

Full app TypeScript checking is currently blocked by a current-main generated-client mismatch in `coachingApi.ts` around `CoachingService.ListScorecards$1`. The broad precommit lint hook also hits the known unrelated Zendesk schema stack overflow, and broad i18next lint reports an unrelated literal in `IntegrationConfigHistoryDrawer.tsx`. The merge commit skipped only the two failing broad lint hooks after targeted validation passed.

## Credentials

Used GitHub CLI authentication over explicit HTTPS for fetch/push. No AWS, Okta, Azure, or SSH credentials were used.

## Second current-main refresh

Later on 2026-09-25, Director `main` advanced again and conflicted only in the `ReviewPolicySection.tsx` Finalize import block. Resolved it as a union: retained the Training Simulator `PolicySourceTypeSelect` and the upstream Deputy Agent workflow-automation compatibility helpers and logic. The selector remains rendered after the rule name.

Validation passed for 4 focused files / 35 tests, targeted typed ESLint, Biome, the Director app TypeScript build, and diff checks. The broad lint hook still encountered the unrelated Zendesk schema stack overflow, so only that hook was skipped; i18next lint, formatting, i18n extraction, and commit-message hooks passed. Pushed merge commit `5fcb473690`; GitHub reports the PR `MERGEABLE`, with fresh CI queued.

## Post-merge CI diagnosis

Build shard 2 failed two unit tests, reproduced locally:

- `useOperaStore.test.ts` still expects 19 draft-tracked properties. Adding `policySourceType` intentionally makes the count 20, so this assertion should be updated rather than ignoring the field.
- `utils.test.ts` correctly expects an AI Agent requirement `OperaPolicy` to remain non-editable. The frontend conversion currently normalizes every source other than Training Simulator to Coach Builder, so it overwrites `SOURCE_TYPE_AI_AGENT_REQUIREMENT`; `checkIfPolicyIsEditable` then reads the top-level Coach Builder value before `originPolicy`. Keep this test unchanged, preserve all specified source types during conversion, normalize only missing/unspecified values to Coach Builder, and add conversion coverage for AI Agent requirement source preservation.

No product-code or test changes were made during this diagnostic pass.
