# Lazy lesson-scoped module statistics

Date: 2026-09-01
Source repo: `/Users/xuanyu.wang/repos/cresta-proto`
Worktrees:
- `/Users/xuanyu.wang/repos/cresta-proto-milestone-2`
- `/Users/xuanyu.wang/repos/cresta-proto-milestone-3-module-reporting`
Branches:
- `codex/milestone-2-lesson-reporting`
- `codex/milestone-3-module-reporting`
PRs:
- [cresta-proto#9677](https://github.com/cresta/cresta-proto/pull/9677)
- [cresta-proto#9676](https://github.com/cresta/cresta-proto/pull/9676)

## Problem

`RetrieveTrainingSimulatorLessonStats` currently returns
`TrainingSimulatorModuleStats` for every module of every requested lesson. A
large lesson batch can therefore force module/criterion aggregation and response
materialization even when the user never opens a lesson side panel.

## Proposed direction

- Remove nested `modules` from `TrainingSimulatorLessonStats`; keep the lesson
  API focused on lesson-row statistics.
- Add optional repeated `training_lesson_names` scope to
  `RetrieveTrainingSimulatorModuleStatsRequest`.
- When the lesson drawer opens, Director sends the selected lesson name, that
  lesson's ordered module names, and the current time range in one batched module
  statistics request.
- When lesson scope is absent, the module reporting page continues aggregating
  each requested module across all eligible lesson assignments.

## Contract semantics

- Nonempty lesson names restrict assignment-derived aggregation to DirectorTasks
  assigned any of those lessons; when empty, the field is ignored.
- Requested module names preserve request/response ordering and should be
  validated under the request parent.
- `time_range` continues filtering assignment-window overlap after lesson scope.
- Defer whether `active_lesson_count` and other metadata are global or scoped to
  the named lessons until frontend consumption and implementation are finalized.
- Director should fetch on drawer open and cache by lesson name, ordered module
  names/revisions, and date range. Prefetch remains optional and measurable.

## Contract cleanup while unshipped

- Remove `TrainingSimulatorLessonStats.modules = 6`.
- Renumber the remaining lesson-statistics fields in source order:
  `applicable_score_count = 4`, `passed_assignment_count = 5`, and
  `total_assignment_count = 6`.
- Keep the published module request's `time_range = 3` and add
  `training_lesson_names = 4`.

## Local implementation

- Updated both local proto worktrees with the cumulative contract shape.
- Added optional repeated `training_lesson_names = 4` to the module request
  while preserving published `time_range = 3`.
- Removed nested lesson module stats and renumbered the remaining unreleased
  lesson fields contiguously in source order.
- Scoped Buf lint/build, targeted Bazel build, and `git diff --check` pass in
  both worktrees.
- See Publication below for the final branch state.

## Wire-compatibility review

- [cresta-proto#9676](https://github.com/cresta/cresta-proto/pull/9676)
  merged as `35ce589882` on 2026-09-01 at 18:03 UTC.
- Generated sources were committed as `95a1b5ee54` and released in tag
  `v2.22.6` at 18:39 UTC.
- Therefore `RetrieveTrainingSimulatorModuleStatsRequest.time_range = 3` is a
  published wire contract and must retain field 3. The new
  `training_lesson_names` field must use a new tag, recommended field 4.
- Organization-wide GitHub code search found only the proto and generated
  module-stats declarations, not application consumers. This reduces observed
  rollout risk but does not undo publication or make tag reuse safe.
- `TrainingSimulatorLessonStats` remains introduced only by open
  [cresta-proto#9677](https://github.com/cresta/cresta-proto/pull/9677);
  organization-wide search found no default-branch declaration or consumer.
  Its remaining fields may be renumbered contiguously, and removed field 6/name
  `modules` do not need reservation before merge.

## Publication

- The lazy-response and lesson-scope contract was published on the lesson branch
  through commit `150890b5e0`.
- The compatibility correction preserving module-request `time_range = 3` and
  assigning `training_lesson_names = 4` was committed as `85240b0b08` and
  pushed to `origin/codex/milestone-2-lesson-reporting`.
