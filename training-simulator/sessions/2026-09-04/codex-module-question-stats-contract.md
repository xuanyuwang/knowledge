# Module question statistics contract

## Context

- Source repo: `/Users/xuanyu.wang/repos/cresta-proto`
- Branch: `xuanyu/reporting-patch`
- Pull request: [cresta-proto#9769](https://github.com/cresta/cresta-proto/pull/9769)
- Commits: `3e2f65a4e6c` (`add quiz question module stats`), `b55f81da8b` (`share training module type enum`), and `be942ddbf0` (`document quiz question stats order`), pushed to `origin/xuanyu/reporting-patch`
- Trigger: review follow-up identified that quiz modules need question-level statistics rather than evaluation-criterion statistics.

## Decision and implementation

Updated `cresta/v1/trainingsimulator/module_stats.proto` additively:

- Added `TrainingSimulatorQuizQuestionStats` with stable `question_id`, `display_name`, `correct_result_count`, and `answered_result_count`.
- Added shared top-level `TrainingModuleType` in `training_module.proto`, with `CONVERSATION` and `QUIZ` values.
- Added `module_type = 8` so clients select the appropriate detailed breakdown explicitly instead of inferring the module kind from empty repeated fields.
- Added `question_stats = 9` alongside the existing `criteria = 6` field.
- Defined conversation modules to populate `criteria` and quiz modules to populate `question_stats`.
- Defined question correctness as `correct_result_count / answered_result_count`; unanswered questions are excluded rather than treated as criterion-style N/A results.
- Defined question-stat ordering to preserve the configured question order from the quiz template, per review comment `r3937716427`.

This supersedes the earlier reporting-plan statement that quiz detail statistics were excluded. Backend work must now load per-question results, populate `module_type`, and populate only the type-appropriate detail field.

## Validation

- `buf lint --path cresta/v1/trainingsimulator/module_stats.proto` — passed.
- `buf build --path cresta/v1/trainingsimulator/module_stats.proto -o /tmp/cresta-proto-pr-9769.binpb` — passed.
- `mage -v apiLint` — passed after starting the existing Colima runtime.
- `bazel build //cresta/v1/trainingsimulator:all` — passed.
- `buf breaking --against '.git#ref=origin/main' --path cresta/v1/trainingsimulator/module_stats.proto` — passed.
- `git diff --check` — passed.
- Fetched `refs/pull/9769/head` after the initial push and verified it equaled commit `3e2f65a4e6c`; the shared-enum follow-up was pushed as `b55f81da8b`.
- `bazel run //:gazelle` after adding the cross-file import — passed with no build-file changes.

## Follow-up

- Update the CONVI-7601 backend to populate `module_type` and question-level aggregates from the selected latest quiz results.
- Update Director to branch on `module_type` and render `question_stats` for quiz modules.

## Web-client export audit

- `resource/cresta-web-client/scripts/export_symbols_whitelist.json` contains no `cresta/v1/trainingsimulator` entry, including for the existing module-statistics messages and RPC types.
- The whitelist controls only symbols re-exported from the broad `@cresta/web-client/v1` barrel.
- `resource/cresta-web-client/package.json` exposes `./v1/*`, and the generated `cresta/v1/trainingsimulator/index.ts` re-exports every generated file and symbol in that package.
- Director consistently imports Training Simulator contracts from `@cresta/web-client/v1/trainingsimulator`, so `TrainingModuleType`, `TrainingSimulatorQuizQuestionStats`, and the updated message fields will be available after generation without changing the whitelist.
- Proto fields are members of their generated message types and are not independent whitelist symbols.
- CodeRabbit comment `r3937293109` concerns stale checked-in generated output under `gen/web-client-lite`, not the export whitelist. The repository instructions explicitly prohibit generated-code changes in proto PRs; CI/release generation updates `gen/cresta-web-client`, and `resource/cresta-web-client/scripts/prepare.sh` derives `gen/web-client-lite` from it via `rsync`. No generated or whitelist file should be changed in PR #9769.
