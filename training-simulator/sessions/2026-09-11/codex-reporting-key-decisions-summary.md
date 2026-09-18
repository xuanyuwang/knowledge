# Training Simulator reporting key-decisions summary

Date: 2026-09-11
Tool: Codex
Source repo: `/Users/xuanyu.wang/repos/knowledge`
Branch/worktree context: main checkout, branch `main`

## Request

Summarize the key decisions for the Training Simulator reporting project from the durable project record.

## Sources reviewed

- `training-simulator/README.md`
- `training-simulator/subdomains/reporting/README.md`
- `training-simulator/work-items/lesson-module-statistics-reporting.md`
- all four records under `training-simulator/decisions/`
- `training-simulator/deliverables/lesson-module-statistics-reporting.md`
- `training-simulator/deliverables/lesson-module-statistics-eng-design.md`
- `training-simulator/deliverables/lesson-module-statistics-fe-design.md`
- `training-simulator/work-items/CONVI-7656.md`
- recent project logs for 2026-09-08 and 2026-09-09

## Synthesis

- Reporting is a three-grain model: shipped session/assignment reporting plus lesson- and module-centric diagnostics across assignments.
- Lesson and module reporting use dedicated batch RPCs. Content-list APIs remain content-only; Director batches loaded resource names and uses caching/prefetch, never per-row requests.
- Aggregation is assignment-rooted so never-started assignees remain in denominators. It reads current lesson/module definitions and direct PostgreSQL result facts; ClickHouse, revision-exact historical reporting, and new indexes are deferred unless scale evidence requires them.
- Only ACTIVE DirectorTasks count in v1 lesson and module reports. Archiving removes an assignment from these statistics; history-preserving reporting is deferred.
- The assignment time filter means overlap with the DirectorTask create-to-due window. Attempt/result timestamps do not drive inclusion.
- The newest attempt for each required module is authoritative. A newer unscored retry makes that module, and therefore its lesson assignment, incomplete; there is no fallback to an older successful result.
- Lesson completion requires evaluated latest attempts for every required module. Lesson pass requires every required module to pass; lesson score is the simple average of required-module scores.
- Overall zero-score/false outcomes for failure, all-criteria-N/A, and the recorded timeout case are intentionally collapsed for reporting. Criterion-level N/A remains distinct and is excluded from applicable score/pass denominators.
- Conversation and quiz modules have explicit type-specific detail. `TrainingModuleType` is authoritative; conversation modules expose criterion statistics, quiz modules expose correct/answered question statistics in configured question order.
- Content-level stats begin with admin/QA-admin authorization. Request filters narrow data but never grant access; manager/assignee filtering requires an explicit manageable-user model and product surface.
- Valid content with no matching assignments returns zero/default statistics. CSV export, revision-safe history, production-KPI linkage, richer cohort filters, and performance-driven index changes remain outside the committed v1 scope.

## Validation status

As of 2026-09-09, both dedicated RPCs passed planned black-box staging scenarios against independent read-only PostgreSQL calculations, including zero/never-started assignments, conversation and quiz behavior, configured question order, lesson scoping, assignment-window filtering, completeness, and newest unscored retakes.
