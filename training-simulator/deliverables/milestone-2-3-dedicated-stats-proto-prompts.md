# Prompts to update the Milestone 2 and 3 proto PRs

> Superseded for CONVI-7600: use
> [convi-7600-dedicated-lesson-stats-proto-prompt.md](./convi-7600-dedicated-lesson-stats-proto-prompt.md),
> which reflects the final rewritten #9676 contract at `97873e3108`.

These prompts implement the accepted [dedicated statistics API decision](../decisions/2026-08-28-dedicated-lesson-module-stats-apis.md). Run them separately in the named worktrees. Milestone 3 remains the base because Milestone 2 reuses `TrainingSimulatorModuleStats`.

## Milestone 3 — update module statistics PR #9676

```text
Update cresta-proto PR #9676 (CONVI-7601) in /Users/xuanyu.wang/repos/cresta-proto-milestone-3-module-reporting on branch codex/milestone-3-module-reporting to implement the new dedicated module-statistics API direction.

Decision context:
- We are no longer extending ListTrainingModules with reporting behavior.
- The original reason for that shape was to avoid a second frontend call/cold tab-switch request. Slack review concluded that this performance concern is not material enough to outweigh API responsibility, convention, and discoverability. Decision: /Users/xuanyu.wang/repos/knowledge/training-simulator/decisions/2026-08-28-dedicated-lesson-module-stats-apis.md
- Keep this PR narrowly scoped to the handwritten protobuf contract; do not generate clients and do not add backend or Director implementation.

Required contract change:
1. Preserve module_stats.proto and the reviewed TrainingSimulatorModuleStats / TrainingSimulatorCriterionStats field semantics, counts, ordering, comments, and resource references unless compilation or API lint requires a narrowly justified correction.
2. Restore ListTrainingModulesRequest and ListTrainingModulesResponse to their pre-PR content-list shape:
   - remove include_stats, stats_time_range, and stats_user_names;
   - remove ListTrainingModulesResponse.module_stats;
   - remove the AIP-132 response-unknown-fields suppression that existed only for module_stats.
3. Add dedicated messages, preferably in training_simulator_service.proto unless repository convention clearly requires another file:
   - RetrieveTrainingSimulatorModuleStatsRequest
     - required parent profile resource name;
     - required repeated training_module_names, preserving request order, bounded to 1–200 items, with TrainingModule resource references and validation consistent with repository conventions;
     - optional stats_time_range;
     - optional repeated stats_user_names with User resource references;
     - do not add group/team filters or a shared TrainingSimulatorStatsFilter.
   - RetrieveTrainingSimulatorModuleStatsResponse
     - repeated TrainingSimulatorModuleStats module_stats;
     - document that entries have the same length and order as training_module_names and that a valid requested module with no matching assignments returns a zero-valued stats entry.
4. Add TrainingSimulatorService.RetrieveTrainingSimulatorModuleStats with a read-only HTTP GET binding consistent with RetrieveTrainingSimulatorTaskStats and a dedicated path under the profile (for example /v1/{parent=customers/*/profiles/*}/trainingSimulatorModuleStats).
5. Give the dedicated reporting RPC the reporting/admin role surface (ADMIN, SUPER_ADMIN, QA_ADMIN); do not inherit AGENT/MANAGER access from ListTrainingModules without explicit evidence.
6. Keep the BUILD.bazel source entry for module_stats.proto. Run Gazelle only if required by repository workflow and retain only an actual generated BUILD diff.

Correctness and review constraints:
- The API is one batch request for the loaded module page, never one request per row.
- training_module_names select content; stats_time_range/stats_user_names select the assignment/result facts counted for that content.
- Request filters narrow authorized data and never grant authorization.
- Do not reintroduce persisted evaluation-status/N/A fields, generated code, database changes, backend logic, frontend logic, or Milestone 2 lesson messages.
- Inspect the three-dot diff against origin/main and ensure it contains only the intended module reporting contract.

Validation:
- format changed proto files with repository tooling;
- run buf lint/build for the Training Simulator package;
- run bazel build //cresta/v1/trainingsimulator:all (or the repository's current equivalent);
- run git diff --check;
- report exact changed files, validation results, commit SHA, and any API-lint suppression still required. Do not claim PR/CI state you did not directly verify.
```

## Milestone 2 — update lesson statistics PR #9677

```text
Update cresta-proto PR #9677 (CONVI-7600) in /Users/xuanyu.wang/repos/cresta-proto-milestone-2 on branch codex/milestone-2-lesson-reporting to implement the new dedicated lesson-statistics API direction.

Dependency and decision context:
- PR #9677 is stacked on module-statistics PR #9676 because TrainingSimulatorLessonStats.modules reuses TrainingSimulatorModuleStats. First ensure the worktree is based on the updated #9676 head; preserve the lesson-only three-dot diff.
- We are no longer extending ListTrainingLessons with reporting behavior. The extra batched frontend request/cold-tab concern does not outweigh cleaner API responsibility, convention, discoverability, and independent reporting behavior. Decision: /Users/xuanyu.wang/repos/knowledge/training-simulator/decisions/2026-08-28-dedicated-lesson-module-stats-apis.md
- Keep this PR narrowly scoped to the handwritten protobuf contract; do not generate clients and do not add backend or Director implementation.

Required contract change:
1. Preserve lesson_stats.proto and the reviewed TrainingSimulatorLessonStats field semantics, optional score/rate presence, ordered module entries, comments, and reuse of TrainingSimulatorModuleStats unless compilation or API lint requires a narrowly justified correction.
2. Restore ListTrainingLessonsRequest and ListTrainingLessonsResponse to their pre-PR content-list shape:
   - remove include_stats, stats_time_range, and stats_user_names;
   - remove ListTrainingLessonsResponse.lesson_stats;
   - remove the AIP-132 response-unknown-fields suppression that existed only for lesson_stats.
3. Add dedicated messages, preferably in training_simulator_service.proto unless repository convention clearly requires another file:
   - RetrieveTrainingSimulatorLessonStatsRequest
     - required parent profile resource name;
     - required repeated training_lesson_names, preserving request order, bounded to 1–200 items, with TrainingLesson resource references and validation consistent with repository conventions;
     - optional stats_time_range;
     - optional repeated stats_user_names with User resource references;
     - do not add group/team filters or a shared TrainingSimulatorStatsFilter.
   - RetrieveTrainingSimulatorLessonStatsResponse
     - repeated TrainingSimulatorLessonStats lesson_stats;
     - document that entries have the same length and order as training_lesson_names and that a valid requested lesson with no matching assignments returns a zero-valued stats entry.
4. Add TrainingSimulatorService.RetrieveTrainingSimulatorLessonStats with a read-only HTTP GET binding consistent with RetrieveTrainingSimulatorTaskStats and a dedicated path under the profile (for example /v1/{parent=customers/*/profiles/*}/trainingSimulatorLessonStats).
5. Give the dedicated reporting RPC the reporting/admin role surface (ADMIN, SUPER_ADMIN, QA_ADMIN); do not inherit broader ListTrainingLessons content access without explicit evidence.
6. Keep lesson_stats.proto and module_stats.proto imports/source dependencies exactly as required. Do not duplicate the shared module statistics messages in this PR.

Correctness and review constraints:
- The API is one batch request for the loaded lesson page, never one request per row.
- training_lesson_names select content; stats_time_range/stats_user_names select the assignment/result facts counted for that content.
- Request filters narrow authorized data and never grant authorization.
- Do not add module RPC changes that belong in #9676, persisted evaluation-status/N/A fields, generated code, database changes, backend logic, or frontend logic.
- Inspect the three-dot diff against the updated #9676 base and ensure it contains only the intended lesson reporting contract.

Validation:
- format changed proto files with repository tooling;
- run buf lint/build for the Training Simulator package;
- run bazel build //cresta/v1/trainingsimulator:all (or the repository's current equivalent);
- run git diff --check;
- report exact changed files, validation results, commit SHA, updated base SHA, and any API-lint suppression still required. Do not claim PR/CI state you did not directly verify.
```
