# Submitted-scorecard lock GA override

**Date:** 2026-08-06
**Primary source repo:** `/Users/xuanyu.wang/repos/director`
**Primary branch/worktree:** `xwang/ga-submitted-scorecard-lock` at `/Users/xuanyu.wang/repos/director-ga-submitted-scorecard-lock`
**Coordinated backend branch/worktree:** `convi-ga-submitted-scorecard-lock` at `/Users/xuanyu.wang/repos/go-servers-ga-submitted-scorecard-lock`

## Goal

Release submitted-scorecard editor restrictions to all customers and retire `disableEditingOnSubmittedScorecards`.

## Findings

- Director's `useFeatureFlag` normally honors local storage before customer config and defaults absent values to false.
- Apiserver's `ScorecardPermissionEvaluator` separately reads the same Director legacy feature flag before enforcing submitted-scorecard editors.
- A frontend-only override would expose the editor configuration and proactive lock while leaving backend write enforcement disabled for customers without the flag.

## Implementation

- Director: removed the flag checks so editor controls and permission evaluation are unconditional.
- Go servers: removed the config lookup and obsolete flag-specific test plumbing so submitted-editor restrictions are always enforced.
- Config: used `yarn remove-flag:director` to remove the schema field and all canonical customer snapshot values. Derived `configv3` and legacy outputs remain sync-managed.

## Validation

- `yarn vitest src/hooks/useFeatureFlag.test.ts`
- `yarn lint:precommit` in `packages/director-app`
- `yarn tsc`
- `go test ./apiserver/internal/coaching/scorecards`
- `go test ./apiserver/internal/coaching -run TestEvaluateScorecardsPermissions`
- `go test ./apiserver/internal/coaching -run 'Test(EvaluateScorecardsPermissions|ResetScorecard|UpdateScorecard)'`
- `go vet ./apiserver/internal/coaching/scorecards`
- `bazel run //:gazelle`
- `yarn test:ci:configdb`
- `mage Lint apiserver/internal/coaching/scorecards` could not run because the local golangci-lint binary uses Go 1.24 while the repo targets Go 1.25.

## Output

- [director#21556](https://github.com/cresta/director/pull/21556)
- [go-servers#30861](https://github.com/cresta/go-servers/pull/30861)
- [config#151019](https://github.com/cresta/config/pull/151019)
