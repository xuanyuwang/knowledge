# Training Simulator Lesson and Module Statistics — Backend Design

Authors: xuanyu.wang@cresta.ai
Status: Draft for review
Last updated: 2026-08-13
Related: [requirements brief](./lesson-module-statistics-reporting.md), [frontend design](./lesson-module-statistics-fe-design.md), CONVI-7263

## Decision summary

Build two additive, batch-oriented read RPCs:

- `RetrieveTrainingSimulatorLessonStats`
- `RetrieveTrainingSimulatorModuleStats`

Both APIs start from DirectorTask assignments so never-started assignees remain in the denominator. They then join task runs to the current normalized outcome tables and aggregate the latest attempt for each assigned agent, task, lesson, and module.

Do not implement this by calling `ListTrainingSimulatorTaskRuns`. That endpoint has a hard 1,000-row limit and its public task-run shape hides important storage differences between conversation and quiz attempts.

The earlier 2026-08-11 draft assumed score, pass, agent, and criterion data lived only on `training_simulator_task_runs`. That is no longer true on `go-servers/main`: conversation outcomes now live in `training_simulator_conversation_scores`, quiz outcomes live in `quiz_scores`, and old task-run columns are only a migration fallback.

## Goals

- Return lesson-level assignment, completion, pass-rate, average-score, retry, and per-module summaries.
- Return module-level summaries and per-criterion results for conversation modules.
- Preserve zero-run assignments and historical content identity.
- Support the existing page-level date and assignee filters.
- Keep response time under two seconds at the expected beta/GA cohort size.
- Keep the existing session stats API unchanged in the first release.

## Non-goals

- CSV export.
- ClickHouse projection or a new reporting warehouse.
- Production-KPI correlation.
- Changing the product's latest-attempt scoring policy.
- Repairing historical assignments that predate immutable assignment snapshots.
- Returning per-agent rows; the existing session drawer owns that grain.

## Product assumptions to confirm

This design is implementable with the following assumptions. Product/design review must close them before the API is frozen:

1. Date range means DirectorTask active-window overlap: `created_at <= range.end` and `(due_time is null or due_time >= range.start)`.
2. Reporting includes active Training Simulator tasks; draft tasks are excluded. Whether completed/archived tasks should also be included needs confirmation because task status currently has no reporting-specific definition.
3. The latest attempt is the official module outcome.
4. A lesson is complete only when every module in the assignment snapshot has a completed latest attempt.
5. A lesson passes only when every required module passes.
6. Content rollups group by stable lesson/module resource ID across revisions and disclose mixed revisions.
7. Quiz attempts participate in lesson/module outcome metrics but do not produce criterion rows.

## Current authoritative implementation

Validated against:

- `go-servers/main` at `01a2c4ecbf` (2026-08-12)
- `cresta-proto/main` at `df2b436d03`
- `director/main` at `4a0962688d`

### Existing reporting path

`RetrieveTrainingSimulatorTaskStats` lists DirectorTasks, calls `ListTrainingSimulatorTaskRuns`, reloads current lesson/module revisions, and aggregates per task in memory.

Known constraints that the new path must not copy:

- It returns no task row when no task run exists.
- It does not consume `direct_team_only`.
- It relies on the task-run list endpoint's 1,000-row cap.
- It reloads current content, so later edits can change historical completion semantics.
- It assumes the first run identifies the task's lesson.
- Its score comments/tests are inconsistent across historical code paths; new APIs must define one scale explicitly.
- The current aggregate code reads `evaluation_score`/`evaluation_passed`; quiz runs expose `quiz_score` instead and therefore need a unified outcome adapter.

### Storage read model

| Table/source | Relevant fields | Role |
|---|---|---|
| `director.tasks` | task ID/type/status, content config, audience config, `created_at`, due time | Assignment denominator and date scope |
| `director.training_simulator_task_runs` | task/lesson/module IDs and revision IDs, `conversation_score_id`, `quiz_score_id`, timestamps | Attempt identity and revision pin |
| `director.training_simulator_conversation_scores` | agent, score, passed, criterion results | Conversation outcome |
| `director.quiz_scores` | submitter, score, submitted time | Quiz outcome |
| `director.training_lessons` | stable ID, revision ID, module membership, title | Historical required-module set and display fallback |
| `director.training_modules` | stable ID, revision ID, evaluation config, display name, quiz reference | Historical pass threshold and criterion metadata |
| User service | requested users/groups/direct team | Cohort resolution |

`training_simulator_task_runs.score`, `.passed`, `.criterion_results`, `.agent_user_id`, and conversation fields remain only for pre-migration fallback rows. The reader must support both paths until the migration fallback is officially removed.

## API contract

Add messages to `cresta/v1/trainingsimulator/stats.proto` and RPCs to `training_simulator_service.proto`.

```protobuf
message TrainingSimulatorOutcomeSummary {
  int64 assigned_count = 1;
  int64 started_count = 2;
  int64 completed_count = 3;
  int64 incomplete_count = 4;
  int64 not_applicable_count = 5;
  int64 passed_count = 6;
  int64 failed_count = 7;
  optional double average_score = 8; // 0.0–1.0
  optional double pass_rate = 9;     // 0.0–1.0
  int64 total_attempt_count = 10;
  int64 retried_agent_count = 11;
}

message TrainingSimulatorCriterionStats {
  string criterion_key = 1;
  string behavior_name = 2;
  string display_name = 3;
  int64 passed_count = 4;
  int64 failed_count = 5;
  int64 not_applicable_count = 6;
  optional double pass_rate = 7;
}

message TrainingSimulatorModuleOutcomeSummary {
  string training_module_name = 1;
  string training_module_display_name = 2;
  TrainingSimulatorOutcomeSummary outcomes = 3;
}

message TrainingSimulatorLessonStats {
  string training_lesson_name = 1;
  string training_lesson_title = 2;
  int64 session_count = 3;
  TrainingSimulatorOutcomeSummary outcomes = 4;
  repeated TrainingSimulatorModuleOutcomeSummary modules = 5;
  uint32 revision_count = 6;
  bool mixed_revisions = 7;
  bool historical_snapshot_missing = 8;
}

message TrainingSimulatorModuleStats {
  string training_module_name = 1;
  string training_module_display_name = 2;
  int64 active_lesson_count = 3;
  optional double current_passing_score = 4; // 0.0–1.0
  TrainingSimulatorOutcomeSummary outcomes = 5;
  repeated TrainingSimulatorCriterionStats criteria = 6;
  uint32 revision_count = 7;
  bool mixed_revisions = 8;
  bool historical_snapshot_missing = 9;
}
```

Use `optional` for values with an empty denominator. Do not use `-1` sentinels in the new contract.

Requests carry:

- required profile `parent`
- required 1–200 lesson or module resource names
- optional `time_range`
- optional `user_names`, `group_names`, and `direct_team_only`

The 200-name limit matches Director's current `ListTrainingLessons`/`ListTrainingModules` front-end load. If those screens move to server pagination, lower the stats batch size to the visible page size.

Responses contain one entry per valid requested name that exists in the profile, ordered like the request. Return `INVALID_ARGUMENT` for malformed or cross-profile names. A valid content item with no assignments returns a zero-valued outcome entry, not an omitted row.

### Roles and authorization

Do not copy the existing stats RPC's `AGENT` role onto content-level aggregate APIs. These endpoints expose cohort statistics and are not required by the agent experience.

Initial roles should match the users who can reach the selected reporting surface:

- `QA_ADMIN`, `ADMIN`, `SUPER_ADMIN` if the first UI ships only in Lesson Configuration.
- Add `MANAGER` and `MANAGER_2ND` only when the reporting surface is available to managers and the backend has an explicit manageable-user scope.

Request filters are not authorization. Before manager enablement, define and test the permitted-user resolver so a manager cannot request arbitrary user names outside their hierarchy.

## Assignment snapshots

Accurate never-started and historical completion counts require assignment-time content. Extend both the public coaching proto and persisted DB proto:

```protobuf
message TrainingSimulatorModuleSnapshot {
  string training_module_name = 1;
  string training_module_revision_id = 2;
}

message TrainingSimulatorLessonSnapshot {
  string training_lesson_name = 1;
  string training_lesson_revision_id = 2;
  repeated TrainingSimulatorModuleSnapshot training_modules = 3;
}

message TrainingSimulatorContentConfig {
  repeated string training_lesson_names = 1; // legacy compatibility
  repeated TrainingSimulatorLessonSnapshot training_lesson_snapshots = 2;
}
```

New assignment writes populate both fields. Task-run creation must use snapshot revision IDs instead of looking up the latest lesson/module revision. Today `lookupLessonAndModuleRevisions` explicitly chooses latest, so CONVI-7263 must change that behavior as well as the stored task content.

Legacy tasks fall back to current lesson membership and set `historical_snapshot_missing = true`. Do not present mixed legacy/current results as revision-exact.

## Cohort and aggregation semantics

### Assignment facts

- Lesson fact: `(director_task_id, lesson_id, assigned_agent_id)`.
- Module fact: `(director_task_id, lesson_id, module_id, assigned_agent_id)` expanded from the assignment snapshot.
- A task containing multiple lessons creates facts for each lesson.
- Intersect the expanded task audience with the authorized/requested user set.
- `session_count` is the distinct matching DirectorTask count, not assigned-agent count.
- `active_lesson_count` is the distinct count of current `STATE_ACTIVE` lesson definitions whose latest revision contains the module. It is a content-relationship count and does not change with date/assignee filters.

### Unified attempt outcome

Create an internal `attemptOutcome` independent of the public task-run proto:

```go
type attemptOutcome struct {
    TaskRunID       string
    DirectorTaskID  string
    LessonID        string
    LessonRevision  string
    ModuleID        string
    ModuleRevision  string
    AgentUserID     string
    Score           *float64 // normalized 0.0–1.0
    Passed          *bool
    NotApplicable   bool
    Criteria        []*trainingsimulatorpb.CriterionEvaluationResult
    AttemptTime     time.Time
}
```

Mapping rules:

- Conversation row: read agent/score/pass/criteria from `training_simulator_conversation_scores`; fall back to old task-run columns only when `conversation_score_id` is null.
- Quiz row: read agent from `quiz_scores.submitter_user_id`, score from `quiz_scores.score`, and completion time from `submitted_at`.
- Proposed quiz pass rule: `quiz_score >= module-revision passing_score / 100`. This rule must be confirmed and shared with session stats; quiz storage currently has no persisted pass boolean.
- Quiz rows have no criterion results.
- A run is completed only when the subtype outcome is complete: conversation score/pass populated, or quiz submitted with score.

### Latest attempt

For each assignment fact, select the completed or incomplete run with the greatest semantic attempt time:

- quiz: `submitted_at`, falling back to task-run `created_at`
- conversation: task-run `created_at`
- deterministic tie-breaker: task-run resource ID

The latest run is the official result even when an earlier run passed. Count every run in `total_attempt_count`.

### Module outcomes

- `assigned_count`: module assignment facts.
- `started_count`: facts with at least one run.
- `completed_count`: facts whose latest run has a complete subtype outcome.
- `not_applicable_count`: completed conversation outcomes whose criterion results are all N/A; quizzes are never N/A in v1.
- `passed_count`/`failed_count`: applicable completed latest outcomes.
- `average_score`: mean normalized score across applicable completed latest outcomes.
- `pass_rate`: `passed_count / (passed_count + failed_count)`.
- `retried_agent_count`: distinct agents with more than one run for the same `(task, lesson, module)` fact.

### Lesson outcomes

For each lesson assignment fact:

- started if any required module has a run
- completed if every snapshotted module has a completed latest outcome
- all-N/A if completed and every required module is N/A
- passed if completed, applicable, and every required module passed
- score is the mean of applicable required-module scores

Aggregate lesson average score as the mean of per-assignment lesson scores so lessons with more modules do not gain extra weight.

### Invariants

```text
incomplete_count = assigned_count - completed_count
completed_count = passed_count + failed_count + not_applicable_count
started_count <= assigned_count
passed_count + failed_count is the pass-rate denominator
```

### Criterion keys

Prefer stable behavior resource name as the rollup key. For a criterion without a behavior, use `module_revision_id + ":" + criterion_id`; never group bare criterion IDs across module revisions. Display name comes from the recorded result first, then the historical module revision.

## Query plan

Implement one shared loader and aggregator used by both RPC handlers.

1. Parse and validate the profile and requested resource names.
2. Resolve the authorized/requested agent set.
3. Load in-scope Training Simulator DirectorTasks with the established active-window overlap predicate.
4. Expand assignment snapshots into lesson/module assignment facts.
5. Query task runs by `(customer_id, profile_id, director_task_id IN ...)` and requested lesson/module IDs.
6. Batch-load referenced conversation scores and quiz scores.
7. Batch-load exact lesson/module revisions referenced by snapshots and runs, plus latest revisions for current display metadata.
8. Normalize rows into `attemptOutcome` values and aggregate in memory.

Query directly with GORM. Do not use page-token helpers internally unless the loop consumes every page. Chunk large `IN` lists (for example 500 IDs per query) and enforce a bounded task/run count; return `RESOURCE_EXHAUSTED` with telemetry rather than silently truncating.

The existing composite index on task runs already starts with customer, profile, and task ID:

```sql
(customer_id, profile_id, director_task_id, training_lesson_id, training_module_id)
```

Do not add the earlier proposed lesson-only/module-only indexes before measuring the task-scoped plan. Add indexes only after `EXPLAIN (ANALYZE, BUFFERS)` on representative data. The score tables are loaded by their primary IDs, so their primary keys cover the batch join.

## Backend file plan

```text
cresta-proto/cresta/v1/trainingsimulator/
  stats.proto
  training_simulator_service.proto

go-servers/apiserver/internal/trainingsimulator/
  action_retrieve_training_simulator_lesson_stats.go
  action_retrieve_training_simulator_module_stats.go
  stats_assignment_loader.go
  stats_attempt_loader.go
  stats_aggregator.go
  *_test.go

go-servers/apiserver/sql-schema/protos/qa/task.proto
  persisted assignment snapshots
```

Keep DB loading, outcome normalization, and pure aggregation separate. The aggregator should accept plain structs and have table-driven unit tests without database setup.

## Failure behavior

- Invalid parent/content/user/group name: `INVALID_ARGUMENT`.
- Requested content not in parent: `INVALID_ARGUMENT` for cross-profile; zero row or `NOT_FOUND` for absent same-profile content (choose once and document; zero row is preferred for batch stability).
- Snapshot references a deleted revision: include the assignment, mark `historical_snapshot_missing`, and omit unavailable display metadata rather than dropping the denominator.
- Orphaned subtype score reference: count as started/incomplete, emit a counter, and continue.
- Run not attributable to an assignment fact: exclude from official outcomes, count it in an orphan telemetry metric.
- Cohort exceeds safety bound: `RESOURCE_EXHAUSTED`, never partial success.

## Observability and SLO

Record per RPC:

- latency and status
- task, assignment-fact, task-run, conversation-score, and quiz-score counts
- requested content and agent counts
- legacy-snapshot and mixed-revision counts
- orphaned run/subtype-reference counts
- DB query latency by loader stage

Targets for the initial PostgreSQL path:

- p95 under 2 seconds for 200 content names, 2,000 assignments, and 10,000 task runs
- no silent truncation
- metric invariant violations equal zero

## Test plan

Unit tests:

- zero assignments and zero-run assignments
- one/multiple tasks and lessons
- latest attempt wins with deterministic tie break
- pass then fail, fail then pass, incomplete latest attempt
- all-N/A, mixed N/A/applicable, empty denominator
- mixed lesson/module revisions
- legacy task without snapshot
- conversation, quiz, and pre-migration fallback outcomes
- quiz threshold boundary
- criteria with same ID across revisions
- cohort intersection and cross-profile rejection
- count invariants

Integration tests:

- RPC handler through PostgreSQL fixtures for all four outcome storage paths
- task active-window overlap
- exact snapshot revision membership after the current lesson is edited
- role authorization and manager scope before manager roles are enabled
- query plan/latency fixture above the old 1,000-row cap

## Rollout

1. Proto and snapshot contract.
2. Snapshot write/read path and task-run revision pinning (CONVI-7263).
3. Shared normalized attempt loader and tests, including quiz semantics.
4. Lesson/module RPCs behind `enableTrainingSimulatorV2` (or a dedicated reporting flag if independent rollout is needed).
5. Staging data comparison against manually calculated cohorts.
6. FE integration.
7. Gradual customer enablement with latency and invariant dashboards.

## Review gates

- Product: reporting surface, task statuses, date meaning, quiz pass semantics, all-N/A display.
- Security: initial roles and manageable-user scope.
- Data: snapshot migration/fallback policy and representative query plan.
- FE/BE: score scale, empty-value behavior, response ordering, and 200-name batch limit.
