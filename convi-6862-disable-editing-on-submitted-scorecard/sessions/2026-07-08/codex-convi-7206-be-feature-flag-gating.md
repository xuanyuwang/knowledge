# CONVI-7206 — Backend feature flag gating

**Date:** 2026-07-08
**Ticket:** [CONVI-7206](https://linear.app/cresta/issue/CONVI-7206/bug-febe-mismatch-submitted-scorecard-lock-gated-by-feature-flag-on-fe)

## Decision

Gate apiserver submitted-scorecard editor enforcement behind the same per-customer Director config flag as the frontend: `disableEditingOnSubmittedScorecards` (read from `legacy_public_config.featureFlags` via config service).

When the flag is off, backend falls back to legacy `HasScorecardEditPermission` behavior for:
- `UpdateScorecard`
- `ResetScorecard`
- `EvaluateScorecardsPermissions`

## PRs (draft)

1. [go-servers#29774](https://github.com/cresta/go-servers/pull/29774) — apiserver gating via config service

## Notes

- Director FE flag already exists in `config` CustomerConfig (`featureFlags.disableEditingOnSubmittedScorecards`); no new config PR needed.
- **Do not use flagd** for this flag — it must be per-customer/profile via config service, not a cluster env var.
- [flux-deployments#301532](https://github.com/cresta/flux-deployments/pull/301532) was created initially but is **incorrect** and should be closed.
- Implementation: `ScorecardPermissionEvaluator` reads `legacy_public_config.featureFlags.disableEditingOnSubmittedScorecards` internally from the scorecard profile config; no public evaluator feature-flag argument and no shared `ProfileConfig` helper.
- **PR compromise (documented in go-servers#29774):** reuse the Director flag on the backend instead of a separate typed `Features` field, because FE/BE are same-team and must roll out together; one config knob is simpler than paired FE+BE flags.

## CI follow-up

- Failed job: `Run apiserver coverage (coaching)` in GitHub Actions run `28964474026`, job `85944483866`.
- Root cause: `TestResetScorecard` exercised `hasSubmittedScorecardEditPermission`, which now calls `ScorecardPermissionEvaluator.disableEditingOnSubmittedScorecardsEnabled`; the reset suite did not mock `config.Client.GetProfileConfig`, so the gomock fatal left shard 5 running until the one-hour timeout.
- Fix: add the existing `mockConfigClientGetProfileConfigWithDirectorFeatureFlags` setup to `TestResetScorecardSuite.BeforeTest` with `disableEditingOnSubmittedScorecards: false`, preserving the suite's legacy reset behavior.
- Verification: `go test ./apiserver/internal/coaching/ -run 'TestResetScorecard|TestEvaluateScorecardsPermissions' -count=1` and `go test ./apiserver/internal/coaching/scorecards/... -run 'TestScorecardPermissionEvaluator' -count=1` passed locally.
- PR state: amended one-commit branch to `e96201964e` and force-pushed; fresh CI run started.

## CI follow-up 2

- Failed job: `Run apiserver coverage (heavy-a)` in GitHub Actions run `28968988165`, job `85959922837`.
- Root cause assessment: unrelated to CONVI-7206 changes. The failing target was `//apiserver/internal/internaljob:internaljob_test` shard 1; the PR changed only coaching scorecard permission code/tests.
- Failure detail: `TestStartChunkedProcessing...` triggered `ServiceImpl.publishProgressUpdate`, which attempted `NotificationServiceClient.CreateNotification`; the internaljob test mock had no `CreateNotification` expectation, causing a testify mock panic.
- Action: attempted direct job rerun, but GitHub rejected it while `Run apiserver coverage (coaching)` was still running. A one-shot background helper waited for run `28968988165` to complete, then successfully requested `gh run rerun 28968988165 --failed`.
- Current rerun state: `Run apiserver coverage (heavy-a)` and `Run apiserver coverage (coaching)` are pending again; all other rerun checks observed so far are passing. Codeowner approval is still pending.

## CI follow-up 3

- Failed job: `Run apiserver coverage (coaching)` rerun in GitHub Actions run `28968988165`, job `85975330416`.
- Root cause: same evaluator-internal flag lookup pattern, but through `UpdateScorecard`. `TestUpdateScorecardSuite.BeforeTest` did not mock `config.Client.GetProfileConfig`, so gomock fatals during `ServiceImpl.hasSubmittedScorecardEditPermission` and shard 5 timed out.
- Fix: add `mockConfigClientGetProfileConfigWithDirectorFeatureFlags` setup to `TestUpdateScorecardSuite.BeforeTest` with `disableEditingOnSubmittedScorecards: false`, matching reset-suite behavior and preserving legacy update-scorecard behavior when the feature flag is off.
- Verification: `go test ./apiserver/internal/coaching/ -run 'TestUpdateScorecard|TestResetScorecard|TestEvaluateScorecardsPermissions' -count=1` and `go test ./apiserver/internal/coaching/scorecards/... -run 'TestScorecardPermissionEvaluator' -count=1` passed locally.
- PR state: amended one-commit branch to `140f1d8399` and force-pushed; fresh CI run `28984508000+` started.
