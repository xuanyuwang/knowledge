# CONVI-7601 quiz question_stats separation

Date: 2026-09-03
Source repo: `/Users/xuanyu.wang/repos/go-servers-milestone-3-module-reporting`
Branch: `codex/milestone-3-module-reporting`
Proto: cresta-proto [PR #9769](https://github.com/cresta/cresta-proto/pull/9769) (merged), bumped go.mod `v2.22.11` → `v2.22.30`

## Contract change (PR #9769)

`TrainingSimulatorModuleStats` gains:
- `TrainingModuleType module_type = 8` — `CONVERSATION` vs `QUIZ` (new enum in `training_module.proto`).
- `repeated TrainingSimulatorQuizQuestionStats question_stats = 9` — per-question `question_id`, `display_name`, `correct_result_count` (subset), `answered_result_count`. Preserves configured template order; empty for conversation modules.
- `criteria` is now documented empty for quiz modules.

PR #9769 also drops the positional order contract on `module_stats`/`lesson_stats` responses (resolves review B2) and relaxes `training_module_names`/`training_lesson_names` from `REQUIRED` to `OPTIONAL` (resolves review C1). PGV still has no rule on these fields.

## Implementation (go-servers)

`apiserver/internal/trainingsimulator/action_retrieve_training_simulator_module_stats.go`:

- `moduleStatsModuleMeta` carries `moduleType` and ordered `quizQuestions` (`question_id`, `display_name`, `ordinal`).
- `findModuleStatsModuleMetadata` derives `moduleType` from `TrainingModules.QuizTemplateID.Valid` and loads configured questions via one batched `findModuleStatsQuizQuestions` query (`quiz_template_id IN ?`, grouped by `(template_id, revision_id)`, sorted by `ordinal` then `question_id`). The `(template, revision)` key prevents a template's other revisions from cross-matching.
- `moduleLatestResult` carries `quizQuestionResults` (`question_id`, `correct`).
- `findLatestModuleAttemptResults` records the latest `quiz_score_id` per assignment key, then `attachModuleStatsQuizQuestionResults` loads `quiz_question_scores` (one batched `quiz_score_id IN ?` query) and attaches per-question results. `correct = score.Valid && score >= 1.0` (server-computed 0/1).
- `aggregateModuleStats` sets `stats.ModuleType` and, for quiz modules, builds `question_stats` via `buildModuleQuizQuestionStats`: aggregates answered/correct across scored latest attempts in configured order; questions no one answered still appear with zero counts; per-question results for `question_id`s not in the current template are dropped. Only scored latest attempts contribute, matching conversation criteria semantics.

Two new batched queries (template questions, per-question scores); no N+1. Conversation and quiz score queries remain separate (different tables/shapes, pre-existing).

## Tests

`action_retrieve_training_simulator_module_stats_test.go`:
- `seedQuizTemplate`/`seedQuizQuestion`/`seedQuizTaskRunWithQuestions` helpers (the `quiz_questions` FK requires the `quiz_templates` row).
- E2E test seeds two quiz questions, runs quiz attempts with per-question 0/1 scores, and asserts `module_type = QUIZ`, `question_stats` order/counts, empty `criteria`; conversation module asserts `module_type = CONVERSATION`, empty `question_stats`.
- New `TestRetrieveTrainingSimulatorModuleStats_QuizQuestionStatsOrderAndUnanswered`: seeds questions out of ID order to prove `ordinal` drives `question_stats` order, and a configured-but-never-answered question appears with zero counts.
- Removed the now-orphaned `seedQuizTaskRun` helper.

## Validation

- `go build`, `go vet`, `gofmt -s` clean.
- `go test -run TestRetrieveTrainingSimulatorModuleStats` PASS (8.6s).
- Full `./apiserver/internal/trainingsimulator/...` PASS (153s).
- `bazel run //:gazelle` — no BUILD.bazel changes.
- `bazel run //:gazelle-update-repos` — `deps.bzl` updated in sync with go.mod (only `com_github_cresta_cresta_proto_v2` version+sum).

## Notes / carry-over

- Review findings B1 (conversation score /100) and B5 (archived-lesson active count) were already fixed in the file before this session; tests now seed 0–1 conversation scores and `TestFindActiveLessonCounts_ExcludesArchivedLatestRevision` covers B5.
- Still open from the review: B3 (`criteria` sorting — failed-count desc, then display_name/criterion_id asc) and B4 (`training_lesson_names` narrowing ignored when non-empty). The PR #9769 contract change makes the empty-list and ordering concerns moot, but B3/B4 remain.
- Pre-existing lint in the test file (`seedTaskRun` unused customerID/profileID params; `seedModule` unused) left as-is — not introduced by this change.

## Follow-up fix: configured quiz passing score of zero is a valid threshold

Commit `204b77199a` on the same branch. Prompted by a review note on `quizScoreToLatestResult`.

### Decision

A configured `EvaluationConfig.passing_score` of `0` is a valid pass threshold: the quiz pass formula is `score >= passing_score/100`, so a threshold of `0` means every scored attempt passes. Previously `quizScoreToLatestResult` gated the derivation on `passingScore > 0`, which treated configured-`0` the same as a missing passing score and never marked any scored quiz attempt as passed.

### Distinguishing configured-0 from missing (evidence)

The `passing_score` field is `double passing_score = 2` in `training_module.proto` with only advisory `field_behavior = OPTIONAL` — it is **not** proto3 `optional`, so it has no field presence. `GetPassingScore()` returns `0` for both "unset" and "explicitly 0"; the value alone cannot distinguish them.

The only available signal that a passing score is configured is that the `EvaluationConfig` message itself is present. So the fix plumbs `hasPassingScore bool` (= `dbModule.EvaluationConfig.Message != nil`) through `moduleStatsModuleMeta` → `findLatestModuleAttemptResults` → `quizScoreToLatestResult`, and changes the guard from `if passingScore > 0` to `if hasPassingScore`.

Cross-checks supporting this distinguisher:
- `EvaluationConfig` is where `passing_score` lives; a module with no `EvaluationConfig` (Message nil) has no passing-score config → `hasPassingScore = false` → pass undefined.
- A quiz module's passing score is read from `EvaluationConfig.Message.GetPassingScore()` by the existing quiz-run creation path (`fetchModulePassingScores`, `user_enrichment.go`), confirming `EvaluationConfig` is the configured-passing-score carrier for quiz modules.
- The run-level API converter `ConvertQuizScoreDBToTaskRunAPI` already uses a `*float64` (`passingScore != nil`) to express "configured vs missing"; `hasPassingScore` is the module-stats equivalent given the proto's lack of field presence.

### Behavior

- Configured threshold > 0 (e.g. 80) → `passed = score >= 0.8` (unchanged).
- Configured threshold 0 (EvaluationConfig present) → `passed = score >= 0` → every scored attempt passes (was: never passed).
- No evaluation config (Message nil) → `passed` left unset (unchanged).
- `normalizedScore` unchanged — the score normalization path is not touched.

### Test

`TestRetrieveTrainingSimulatorModuleStats_QuizPassingScoreZero`: quiz module with `PassingScore: 0` (EvaluationConfig present), two scored attempts (0.5 and 0.0). Asserts `TotalAssignmentCount=2`, `ApplicableScoreCount=2`, `PassedAssignmentCount=2` (both pass, threshold 0), `AverageApplicableScore=0.25`. Existing quiz tests (thresholds 80/70) are unchanged because `hasPassingScore` is true and `passingScore > 0` there.

### Validation

`go build`, `go vet`, `gofmt -s` clean; module-stats suite PASS (8.7s). Pushed fast-forward `b936f59e87..204b77199a`.

### Why not also change `fetchModulePassingScores`?

`fetchModulePassingScores` (`user_enrichment.go`) still skips `ps == 0`. That convention backs the quiz-run *creation* gate (`CreateTrainingSimulatorTaskRun` rejects quiz creation when the module has no passing score in the map). Changing that convention is out of scope for this stats fix and would broaden the PR into creation-path semantics; it stays as a separate concern. The module-stats reader can now represent configured-0 because it only *reads* current config, not *enforces* creation eligibility.

### Why configured-0 reads as a threshold, not as unset

The quiz pass formula is the anchor and is already fixed in the codebase: `ConvertQuizScoreDBToTaskRunDBToAPI` computes `passed := score >= *passingScore/100`. Plugging `passing_score = 0` into that formula yields `score >= 0` → true for every valid quiz score (quiz scores are in [0,1]). So "0 threshold ⇒ everyone passes" is what the existing formula yields, not a new invention. The alternatives ("no one passes" needs threshold > 1; "N/A" needs *no* threshold configured) do not follow from the formula.

The value 0 alone is ambiguous: `passing_score` is `double` with no proto3 `optional`, so `GetPassingScore()` is 0 for both unset and explicit-0. The distinguisher is `EvaluationConfig.Message != nil` (the only presence signal): present config ⇒ threshold is configured (value 0 or left-default-0); absent config ⇒ missing ⇒ pass undefined.

Caveat (acknowledged): `Message != nil` can over-classify "present config with unset-within-config 0" as configured-0. This is safe because the flag only changes output in `quizScoreToLatestResult`, which is reached only for quiz modules that have at least one quiz run; and `CreateTrainingSimulatorTaskRun` blocks quiz creation unless `passing_score > 0` (`fetchModulePassingScores`). So any existing quiz run was created when `passing_score > 0`, meaning a current 0 is necessarily an edit down from a positive value — a configured change, not an initial unset. Quiz modules with 0 and no runs, or with no EvaluationConfig, never reach this path. So in every exercised case, "current 0 = configured threshold = everyone passes" is the faithful reading.

`fetchModulePassingScores`'s `ps == 0 → skip` is a creation-eligibility gate (future attempts), not a semantic claim about past attempts, so it is deliberately left unchanged to keep the PR scoped to the stats reader.
