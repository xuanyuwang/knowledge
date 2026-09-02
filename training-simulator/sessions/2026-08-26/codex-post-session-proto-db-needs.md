# Proto and DB needs after session reporting

Date: 2026-08-26
Source repos: `/Users/xuanyu.wang/repos/cresta-proto`, `/Users/xuanyu.wang/repos/go-servers`
Branch/worktree context: main checkouts; read-only inspection

## Question

After Milestone 1 session reporting is in progress, identify which additional protobuf and database changes are actually required for lesson and module reporting.

## Finding

Assuming Milestone 1 lands explicit conversation `evaluation_status` and overall `not_applicable`, Milestones 2 and 3 need additive reporting protobufs but no currently justified database schema changes. An offline reviewer subsequently requested that the APIs omit the proposed shared filter; the implemented contract therefore places reporting fields directly on each list request.

### Milestone 2 — lesson reporting protobuf

- Add `include_stats` plus time range and separate user, virtual-group, and team-group fields directly to `ListTrainingLessonsRequest`.
- Add `TrainingSimulatorModulePassRate` and `TrainingSimulatorLessonStats`.
- Under API option 2, add fields 8–12 to `ListTrainingLessonsRequest` and `lesson_stats = 3` to `ListTrainingLessonsResponse`.
- Keep `TrainingLesson` unchanged.

### Milestone 3 — module reporting protobuf

- Add the same reporting semantics directly to `ListTrainingModulesRequest`, without a shared filter message.
- Add `TrainingSimulatorCriterionStats` and `TrainingSimulatorModuleStats`.
- Under API option 2, add fields 7–11 to `ListTrainingModulesRequest` and `module_stats = 3` to `ListTrainingModulesResponse`.
- Keep `TrainingModule` unchanged.

The lesson milestone may introduce all four result messages together if one proto review/rollout is operationally easier, but implementation and product rollout can remain milestone-specific.

## Why no additional DB schema is planned

- Assignment/task, lesson/module identity, agent identity, attempt time, score, pass state, and subtype references already exist.
- Conversation `criterion_results` JSONB already persists criterion ID, behavior resource name, display name, passed/N/A state, weight, and auto-fail result, which is enough for the Milestone 3 criterion breakdown.
- Current lesson/module definitions already supply membership, lifecycle state, display metadata, and thresholds.
- `active_lesson_count` is derived from current active lesson definitions rather than a new persisted aggregate.
- Reporting can calculate aggregates from direct bounded reads; no summary table or materialized rollup is currently justified.

## Conditional changes, not current requirements

- Add an index only if representative `EXPLAIN (ANALYZE, BUFFERS)` results show the existing task-scoped composite index is insufficient.
- Historical assignment snapshots or revision-pinned reporting would require persistence changes, but they are explicitly outside this project's scope.
- A materialized aggregate/warehouse table would be a later scale response, not part of the initial PostgreSQL implementation.
- Do not refactor the existing session request merely for symmetry; it can retain its current contract unless Milestone 1 independently needs separate virtual/team group fields.

## Parallel protobuf development

There is no existing reusable Training Simulator filter message: `RetrieveTrainingSimulatorTaskStatsRequest` stores its filters inline, includes session-specific fields, and combines group names; the lesson/module list requests currently contain content-list filters rather than reporting-population filters.

Following reviewer direction, each list request adds `include_stats` plus directly prefixed `stats_time_range`, `stats_user_names`, `stats_virtual_group_names`, and `stats_team_group_names`. `include_stats` is required so an empty population filter can still request unfiltered statistics while old callers continue to avoid reporting cost. Lesson and module result messages live in separate proto files, so their contract work can proceed independently. The duplicated request-field semantics are an accepted tradeoff.

Do not always populate statistics or infer the request from ordinary content-list fields: that would add reporting latency and authorization behavior to existing authoring callers.

## Implemented local proto change

- `lesson_stats.proto`: lesson statistics and lesson-scoped module pass rates.
- `module_stats.proto`: module and criterion statistics.
- `training_simulator_service.proto`: independent direct reporting fields on both list requests and ordered parallel statistics arrays on both responses.
- `BUILD.bazel`: regenerated with Gazelle; generated API/client code remains unmodified.

Validation passed with scoped `buf lint`, scoped `buf build`, repository proto import/HTTP path linters, Gazelle, and `bazel build //cresta/v1/trainingsimulator:all`.

## Evidence inspected

- `cresta-proto/cresta/v1/trainingsimulator/stats.proto`
- `cresta-proto/cresta/v1/trainingsimulator/training_simulator_service.proto`
- `cresta-proto/cresta/v1/trainingsimulator/evaluation.proto`
- `cresta-proto/cresta/v1/trainingsimulator/training_lesson.proto`
- `cresta-proto/cresta/v1/trainingsimulator/training_module.proto`
- `go-servers/apiserver/sql-schema/director/director-schema.sql`
- `deliverables/superhuman-api-design-update-draft.md`
