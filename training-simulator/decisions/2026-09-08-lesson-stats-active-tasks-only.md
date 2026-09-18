# Lesson stats scope ACTIVE-only DirectorTasks

Date: 2026-09-08
Status: Accepted; reverses the ACTIVE+ARCHIVED task-scope clause in the eng design and work item for lesson stats

## Decision

`RetrieveTrainingSimulatorLessonStats` selects only `ACTIVE` Training Simulator `DirectorTask`s. `ARCHIVED` tasks are excluded, alongside `DRAFT` and `DELETED`. This matches `RetrieveTrainingSimulatorModuleStats`, which already filtered `findActiveModuleStatsDirectorTasks` to `task_status = ACTIVE`.

## Why the direction changed

The eng design (`deliverables/lesson-module-statistics-eng-design.md`) and the work item (`work-items/lesson-module-statistics-reporting.md`) previously required `ACTIVE` and `ARCHIVED` assignments so "archival does not erase history." During review of go-servers PR #32038 (discussion r3961582621), the reviewer flagged that lesson stats included archived tasks while module stats did not, and the two stats surfaces must stay consistent. Product intent for the v1 reporting slice is in-flight assignments only; reporting on archived assignments is not a launch requirement. Keeping the two RPCs on the same task-scope avoids divergent denominators between the Lesson and Module tabs.

## Tradeoff accepted

Archiving a task removes its assignments from lesson stats. Historical reporting that must survive archival is deferred until product commits to a history-preserving surface. The lesson-content intersection (`expandLessonAssignmentKeys`) already restricts to current `ACTIVE` lesson definitions, so this decision only narrows the task-scope, not the lesson-content scope.

## Implementation impact

- `action_retrieve_training_simulator_lesson_stats.go`: `findLessonStatsDirectorTasks` renamed to `findActiveLessonStatsDirectorTasks` and changed from `task_status IN (ACTIVE, ARCHIVED)` to `task_status = ACTIVE`; doc comment updated to state DRAFT/DELETED/ARCHIVED exclusion and parity with module stats.
- Tests: `TestFindActiveLessonStatsDirectorTasks_StatusAndTimeWindow` no longer expects `task-archived`; `TestRetrieveTrainingSimulatorLessonStats_EndToEnd` seeds `task-b` as ACTIVE (multi-task aggregation still validated without relying on archived-task inclusion).

## Supersedes

- The "Reporting includes `ACTIVE` and `ARCHIVED` Training Simulator tasks" clause in `deliverables/lesson-module-statistics-eng-design.md`.
- The "Reporting must include `ACTIVE` and `ARCHIVED` assignments" line in `work-items/lesson-module-statistics-reporting.md`.
- The "ACTIVE+ARCHIVED task windowing" / "ARCHIVED inclusion per plan" notes in the 2026-09-04 session and work-item changelog, which described the superseded implementation choice.
