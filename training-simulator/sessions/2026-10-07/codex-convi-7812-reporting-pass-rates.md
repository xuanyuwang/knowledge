# CONVI-7812 reporting pass rates

**Date:** 2026-10-07
**Source repo:** `/Users/xuanyu.wang/repos/director`
**Branch/worktree:** `xw/convi-7812-reporting-pass-rates` at `/Users/xuanyu.wang/repos/director-convi-7812`

## Inputs

- [Linear issue](https://linear.app/cresta/issue/CONVI-7812/update-reporting-pass-rates-for-changing-rubrics)
- [Original Slack thread](https://crestalabs.slack.com/archives/C0AEE1T2U1X/p1791394182355259?thread_ts=1790098405.109829&cid=C0AEE1T2U1X)
- Director module reporting view model, section component, and focused tests at Director `origin/main` `b8af819a83`.
- Go module-stats handler at `origin/main` `5a88a70feaf9`.

## Findings

- Product's example has C1+C2 for earlier attempts and C1+C3 for later attempts. Overall pass rate uses assigned participants; each criterion uses its own count.
- Backend `aggregateModuleStats` seeds current configured criteria, then adds any IDs found only in stored latest-attempt criterion results. It increments `applicableResultCount` only for non-N/A results and emits passed counts per criterion.
- `toModuleStatsView` already displays an overall percentage and fraction, while each criterion row displayed only `(passed/applicable)`.
- The module view model is shared by the direct module drawer and lesson module drilldown.

## Change

- Criterion rows now display `percentage (passed/applicable)` using the existing percentage formatter. Empty denominators retain the placeholder.
- A mixed-rubric test asserts overall `1/2`, shared C1 `2/2`, removed C2 `1/1`, and added C3 `0/1`.

## Validation

- `yarn install --immutable`: completed with existing peer-dependency warnings.
- Focused Vitest: 13 passed with `NODE_OPTIONS=--localstorage-file=/tmp/convi-7812-vitest-localstorage.json`. The first run failed before test execution on Node v26 because local storage lacked a backing file.
- Biome changed-file check: passed.
- Director app TypeScript check: passed.
- `git diff --check`: passed in both the Director worktree and `knowledge`.

## Next

- User then requested a PR. Committed `a08f82fafe` with the Director commit convention, pushed the branch, and created draft [director#23579](https://github.com/cresta/director/pull/23579) using the unchanged repository template outside its description. Pre-commit checks passed; CI was queued at creation.
- Recorded Before on Walter dev and After on `localhost:3100` with `yarn devx` connected to Walter dev, with `enableTrainingSimulatorReporting` enabled through Local Override. The verified clips show the same `Ask the weather` module's `Hello World - Tinglin` criterion as `(1/2)` Before and `50% (1/2)` After. Initial clips were replaced because frame inspection found a Zoom overlay and loading screens.
- Uploaded both verified clips to the PR's Before and After fields, selected `smoke-UI`, added the navigation path, feature flag, and Walter dev environment link. The latest `qa-metadata-check` passed after final clip replacement.
- Marked [director#23579](https://github.com/cresta/director/pull/23579) ready for review. Code owner approval remains outstanding.
- Follow-up: changed the PR's two video attachments from titled Markdown links to bare URLs on separate lines. GitHub's full-body Markdown renderer produced two `<video>` elements labeled `convi-7812-before-final.mov` and `convi-7812-after-final.mov`; the rerun `qa-metadata-check` and `task-check` passed. Code owner check remains pending.

## Percentage-only follow-up

- The ticket wording calls for pass rates as percentages. Updated the shared Director view model so lesson, module, and criterion pass-rate displays omit `x/y`; quiz question correct/answered counts remain because they are not pass rates.
- Committed `9e39225baa` and pushed the existing worktree branch. Focused Vitest passed (24 tests), Biome passed, Director TypeScript passed, and commit hooks passed.
- Recorded a clean full-screen After clip on `localhost:3100` with `yarn devx` connected to Walter dev. The `Ask the weather` drawer shows `Pass rate: 50%` and `Hello World - Tinglin 50%`, without fractions. Frames at 0.5 and 7 seconds were checked.
- Uploaded the clip to GitHub and replaced only the old After attachment URL; retained the Before clip. Updated the PR summary and After caption to percentage-only wording. GitHub renders both bare attachment URLs as inline videos, and `qa-metadata-check` passed on the new commit.
- At final status check, the PR review was approved and codeowner check passed. All reported checks passed or skipped except `Build / E2E Tests`, which remained pending.
- The final E2E check completed. GitHub reports `mergeStateStatus=CLEAN`, `reviewDecision=APPROVED`, and no remaining non-successful, non-skipped checks on head `9e39225baa`.
