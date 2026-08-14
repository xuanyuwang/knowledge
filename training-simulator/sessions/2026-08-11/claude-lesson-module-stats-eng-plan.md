# Session Note - 2026-08-11 - Claude - Lesson/module stats engineering design

**Started:** 2026-08-11
**Tool:** Claude Code
**Project:** `training-simulator`
**Goal:** Write a BE engineering design plan for lesson/module-level statistics reporting following the Cresta Eng Design Doc template

## Source Context

- **Primary repo:** `knowledge` (durable write) + read `go-servers`, `cresta-proto`
- **Repo path:** `/Users/xuanyu.wang/repos/knowledge`
- **Worktree path:** main checkout
- **Branch:** main (knowledge)

## Inputs Reviewed

- Project README, project.yaml, subdomain READMEs
- Existing requirements brief: `deliverables/lesson-module-statistics-reporting.md`
- Current session stats proto: `stats.proto`, `training_simulator_service.proto`
- Current BE implementation: `action_retrieve_training_simulator_stats.go`, `action_list_training_simulator_task_runs.go`
- DB schema: `director.training_simulator_task_runs`, `director.training_lessons`, `director.training_modules`, `director.tasks`
- GORM model: `TrainingSimulatorTaskRuns` (all columns including revision IDs, score, passed, criterion_results JSONB)
- Evaluation proto: `CriterionEvaluationResult`, `EvaluationConfig`
- Module proto: `TrainingModule`, `EvaluationCriterion`
- Cresta Eng Design Doc template (from Glean search → GitHub `eng-design-docs` repo)

## Actions Summary

- Read all knowledge project artifacts for training-simulator domain
- Read current proto surface (`stats.proto`, `training_simulator_service.proto`, `evaluation.proto`, `training_module.proto`, `training_simulator_task_run.proto`)
- Read current BE implementation (`action_retrieve_training_simulator_stats.go`, `action_list_training_simulator_task_runs.go`, `module.go`)
- Read DB schema and GORM model for `training_simulator_task_runs`
- Fetched Cresta Eng Design Doc Template from Glean
- Wrote comprehensive engineering design document with all template sections filled

## Findings

- Current session stats has several issues (1,000-row cap, loses zero-run tasks, ignores `direct_team_only`, reloads latest content revisions) — the new lesson/module implementation must not replicate these.
- DB already stores revision IDs (`training_lesson_revision_id`, `training_module_revision_id`) on task runs, enabling correct historical aggregation.
- Criterion results are stored as JSONB in the task runs table — can be parsed in Go for criterion-level breakdowns.
- Two new DB indexes needed for date-ranged content aggregation.
- No new tables needed — existing `training_simulator_task_runs` has all required columns.
- CONVI-7263 (assignment snapshots) is a pre-requisite for accurate historical adoption reporting but the new APIs can work with legacy data (flagged as `historical_snapshot_missing`).

## Decisions Made

- Add two separate RPCs (`RetrieveTrainingSimulatorLessonStats`, `RetrieveTrainingSimulatorModuleStats`) rather than extending `TaskStats`.
- Use direct DB queries instead of `ListTrainingSimulatorTaskRuns` RPC (avoid 1,000-row cap).
- Aggregate in Go memory rather than pushing to SQL (acceptable for v1 with 100-name request limit).
- Use `optional` for rates/scores with empty denominators instead of sentinel values.
- Normalize scores to 0.0–1.0 scale (consistent with existing `TaskStats`).
- Keep time-range semantics matching current session stats (DirectorTask window overlap).

## Deliverable

- `deliverables/lesson-module-statistics-eng-design.md` — full engineering design doc per Cresta template

## Links

- Requirements brief: `deliverables/lesson-module-statistics-reporting.md`
- Engineering design: `deliverables/lesson-module-statistics-eng-design.md`
- CONVI-7263: https://linear.app/cresta/issue/CONVI-7263/update-training-simulator-session-to-take-snapshot-of-revisions-from
- Template source: https://github.com/cresta/eng-design-docs/blob/main/Cresta%20Eng%20Design%20Doc%20Template.md
