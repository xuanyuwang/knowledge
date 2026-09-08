# CONVI-7638: Honor training_lesson_names scope in RetrieveTrainingSimulatorModuleStats

**Status:** validating (PR open, awaiting review)
**Primary domain:** `training-simulator`
**Primary subdomain:** `reporting`
**Official ticket:** [CONVI-7638](https://linear.app/cresta/issue/CONVI-7638/honor-training-lesson-names-scope-in)
**Last updated:** 2026-09-07

## Objective and Impact

- **Objective:** Make the merged module-stats backend honor the published `training_lesson_names` (field 4) request scope: when non-empty, aggregate module statistics only over assignments for those lessons; when empty, ignore the field.
- **Customer/system impact:** Unblocks the Director lesson drawer, which lazily calls `RetrieveTrainingSimulatorModuleStats` with lesson scope to show per-lesson module breakdowns; closes open review follow-up B4 from the CONVI-7601 backend review.
- **Role:** implemented

## Scope

**In scope**

- Parse and validate `training_lesson_names` in `parseModuleStatsRequest` (empty entry, malformed name, cross-profile name → `InvalidArgument`), mirroring `training_module_names` validation.
- Narrow lesson discovery (`findLessonsContainingModules`) with `resource_id IN lessonIDs` when the scope is non-empty.
- Tests: parse validation, lesson-scoped narrowing, empty-list unscoped behavior, zero metrics for in-scope lessons without matching assignments.

**Non-goals**

- Scoping `active_lesson_count` by lesson (contract scopes assignments, not content metadata; deferred).
- Director frontend wiring (separate work).
- Proto changes (field 4 already published in cresta-proto).

## Source Context

- **Repos:** `go-servers`
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-convi-7638`
- **Branches:** `convi-7638-modules-in-lesson-scope` (off `origin/main` @ `bf2376e890`)
- **PRs/commits:** [go-servers#32039](https://github.com/cresta/go-servers/pull/32039), commit `0a01e7ee95`

## Current Understanding

The merged CONVI-7601 backend keys every assignment as `(task, lesson, module, agent)` and builds aggregation facts only from those keys, so honoring the lesson scope reduces to parsing/validating the field and filtering the latest-revision lesson query by `resource_id`. Latest-attempt selection, aggregation, ordering, and response construction need no changes; runs from out-of-scope lessons under the same task are excluded naturally because facts derive from narrowed assignment keys. `active_lesson_count` intentionally stays profile-global.

## Findings and Decisions

- The contract comment for field 4 scopes *assignments* ("narrow the scope of module statistics to assignments for those lessons"), so content metadata (`active_lesson_count`) is not narrowed.
- Two pre-existing tests carried placeholder `TrainingLessonNames` values that assumed the field was ignored; both were updated (parse subtest replaced with real lesson-parse coverage; `FiltersAndFacts` placeholder removed).

## Blockers and Dependencies

- None.

## Validation and Rollout

- `TestRetrieveTrainingSimulatorModuleStats` suite passes (9.6s); full `trainingsimulator` package passes (162s).
- `bazel run //:gazelle` no-op; `bazel build //apiserver/internal/trainingsimulator:trainingsimulator` passes (72s, cold cache).

## Next Actions

1. Address PR review on [go-servers#32039](https://github.com/cresta/go-servers/pull/32039) and merge.
2. Director lesson-drawer integration remains follow-up work.

## Timeline

- 2026-09-07 — Filled the Linear ticket, created worktree, implemented parse/validate + lesson scoping with tests; all validation green; opened [go-servers#32039](https://github.com/cresta/go-servers/pull/32039) (`0a01e7ee95`). Evidence: `sessions/2026-09-07/claude-convi-7638-lesson-scoped-module-stats.md`.
