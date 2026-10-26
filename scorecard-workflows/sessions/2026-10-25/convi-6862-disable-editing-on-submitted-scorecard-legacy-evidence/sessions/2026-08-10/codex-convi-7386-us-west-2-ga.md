# CONVI-7386 us-west-2 submitted-scorecard lock GA

**Date:** 2026-08-10
**Source repo:** `/Users/xuanyu.wang/repos/config`
**Branch/worktree:** `convi-7386-us-west-2-enable-submitted-scorecard-lock` at `/Users/xuanyu.wang/repos/config-convi-7386-us-west-2`

## Goal

Enable `disableEditingOnSubmittedScorecards` for the 44 supplied production profiles in us-west-2, following the batch rollout used by config PR #151175.

## Implementation

- Added the flag with value `true` to profile-level and use-case-level legacy frontend config copies.
- Updated 44 profiles across 10 customer history files.
- The resulting diff contains 114 additions and no deletions.
- Opened [config#151187](https://github.com/cresta/config/pull/151187).

## Validation

- ConfigDB frontend schema validation passed as part of `yarn test:ci`.
- `yarn yaml-validate:ci` passed.
- `git diff --check` passed.
- Full `yarn test:ci` reached `ajv-validate:v3-frontend` and failed on unrelated pre-existing `configv3` app-config schema violations on current `master`.

## Direction change

The earlier flag-removal PRs were closed without merge:

- [director#21556](https://github.com/cresta/director/pull/21556)
- [go-servers#30861](https://github.com/cresta/go-servers/pull/30861)
- [config#151019](https://github.com/cresta/config/pull/151019)

The active GA mechanism remains staged configuration rollout rather than removing the runtime flag.
