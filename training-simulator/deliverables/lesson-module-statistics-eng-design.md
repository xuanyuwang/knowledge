# Training Simulator Lesson and Module Statistics — Backend Design

> **Superseded requirement (2026-08-27):** Statements in this design requiring persisted overall evaluation status/N/A or a distinct overall all-N/A reporting state are no longer current. Product accepted failure-equivalent reporting for the listed zero/false timeout, all-criteria-N/A, and failed cases. Criterion-level N/A remains excluded from scoring. See [the decision record](../decisions/2026-08-27-collapse-overall-zero-false-results.md).

Authors: xuanyu.wang@cresta.ai
Status: Draft for review
Last updated: 2026-08-28
Related: [requirements brief](./lesson-module-statistics-reporting.md), [frontend design](./lesson-module-statistics-fe-design.md), CONVI-7263

## Design summary

The reporting path starts from DirectorTask assignments so never-started assignees remain in the denominator. It then joins task runs to the current normalized training-result tables and aggregates the latest attempt for each assigned agent, task, lesson, and module.

The public API boundary closed on 2026-08-28 with Option 3 selected: dedicated `RetrieveTrainingSimulatorLessonStats` and `RetrieveTrainingSimulatorModuleStats` batch RPCs. The Slack review determined that cleaner responsibility, convention, discoverability, and independent reporting behavior outweigh avoiding one frontend request when cold-tab performance is not a major concern. The alternatives remain below as decision history. See [the accepted decision](../decisions/2026-08-28-dedicated-lesson-module-stats-apis.md).

One correctness contract must land before reliable rollups: persist the conversation evaluator's overall status and overall N/A result. The evaluator already returns both values, but `director.training_simulator_conversation_scores` currently stores only score, passed, and criterion results. The reporting reader must not guess whether an all-N/A-looking record is a genuine N/A result or an unfinished evaluation.

Do not implement aggregation by calling `ListTrainingSimulatorTaskRuns`. Regardless of the public API selected, reporting queries DirectorTask assignments, task runs, and conversation-score rows directly and applies the statistics filters to the complete bounded dataset.

The earlier 2026-08-11 draft assumed score, pass, agent, and criterion data lived on `training_simulator_task_runs`. That is no longer true: conversation results live in `training_simulator_conversation_scores`, and the obsolete result/agent columns have been removed from the task-run table on the tracked `go-servers/origin/main` schema.

## Goals

- Return lesson-level assignment, completion, pass-rate, average-score, retry, and per-module summaries.
- Return module-level summaries and per-criterion results for conversation modules.
- Preserve zero-run assignments.
- Support the existing page-level date and assignee filters.
- Keep response time under two seconds at the expected beta/GA user-set size.
- Keep the existing session stats API unchanged in the first release.
- Make data-quality limitations explicit instead of producing plausible but semantically incorrect rates.

## Non-goals

- CSV export.
- ClickHouse projection or a new reporting warehouse.
- Production-KPI correlation.
- Changing the product's latest-attempt scoring policy.
- Historical revision-exact reporting. This project intentionally aggregates using the current lesson/module definitions available at read time.
- Quiz statistics. Quiz is not yet mature enough to define reliable reporting semantics and is excluded from this project.
- Returning per-agent rows; the existing session drawer owns that grain.

## Product assumptions to confirm

This design is implementable with the following assumptions. Product/design review must close them before the API is frozen:

1. Date range means DirectorTask active-window overlap: `created_at <= range.end` and `(due_time is null or due_time >= range.start)`.
2. Reporting includes `ACTIVE` and `ARCHIVED` Training Simulator tasks so archival does not erase history; `DRAFT` and `DELETED` tasks are excluded. Product must confirm that lifecycle contract.
3. The latest attempt is the official module result.
4. A lesson is complete only when every module in the current lesson definition has a completed latest attempt.
5. A lesson passes only when every required module passes.
6. Content rollups group by stable lesson/module resource ID and use current lesson/module definitions.
7. Only conversation modules are included. Quiz modules and quiz attempts are outside this project's reporting scope.

## Current authoritative implementation

Validated against tracked refs:

- `go-servers/origin/main` at `9099ace9140a` (2026-08-13)
- `cresta-proto/origin/main` at `d175ba9c8a13` (2026-08-10)
- `director/origin/main` at `18370ed8fd7d` (2026-08-14)

### Existing reporting path

`RetrieveTrainingSimulatorTaskStats` lists DirectorTasks, calls `ListTrainingSimulatorTaskRuns`, reloads current lesson/module revisions, and aggregates per task in memory.

Known constraints that the new path must not copy:

- It returns no task row when no task run exists.
- It does not consume `direct_team_only`.
- It reloads current content, so later edits can change historical completion semantics.
- It assumes the first run identifies the task's lesson.
- Its score comments/tests are inconsistent across historical code paths; new APIs must define one scale explicitly.
- The public task-run shape exposes result fields, but the storage rows point to conversation-score records; reporting therefore needs an explicit internal result adapter rather than treating the base run as authoritative.
- Overall N/A is not persisted. A zero/false result with N/A-looking criteria can represent a genuine all-N/A training result or a partial snapshot persisted at timeout.
- Evaluation resolves the current module by stable name rather than the attempted module revision, so historical criteria, weights, threshold, and auto-fail semantics can drift.

### Storage read model

| Table/source | Relevant fields | Role |
|---|---|---|
| `director.tasks` | task ID/type/status, content config, audience config, `created_at`, due time | Assignment denominator and date scope |
| `director.training_simulator_task_runs` | task/lesson/module IDs, `conversation_score_id`, timestamps | Conversation attempt identity |
| `director.training_simulator_conversation_scores` | agent, score, passed, criterion results; proposed evaluation status and overall N/A | Conversation training result |
| `director.training_lessons` | stable ID, current module membership, title | Current required-module set and display metadata |
| `director.training_modules` | stable ID, current evaluation config and display name | Current criterion and threshold metadata |
| User service | requested users/groups/direct team | User selection |

The base task-run table no longer carries score, pass, criteria, agent, or conversation-result columns on the tracked schema. The new reader loads result identity and values through `conversation_score_id`; it does not add a pre-migration fallback path.

`training_simulator_conversation_scores` does not persist the evaluator's overall status or overall N/A value. Add those fields before treating `not_applicable_count` or `completed_count` as reliable. This belongs on the existing Training Simulator conversation-score row, not `app.chats` and not a new table.

## API design

### Selected boundary

Option 3 is selected: lesson and module reporting each have an explicit batch API. Option 2's only decisive advantage was avoiding a second frontend request, especially on cold tab switching. The review accepted that cost in exchange for conventional, discoverable contracts and independent authorization, latency, errors, monitoring, caching, and evolution. Director will batch all loaded content names and use cache/prefetch rather than issue per-row requests.

No option changes the aggregation source: statistics are calculated by direct, filtered database reads over assignments, current lesson/module definitions, conversation attempts, and conversation-score rows. They are never calculated from a paginated API response.

The comparison below preserves the shared-filter proposal as design history. The selected contract follows the later reviewer direction: each dedicated request owns its time/user fields directly; group/team filters and a shared `TrainingSimulatorStatsFilter` are not part of the current PR scope.

```protobuf
message TrainingSimulatorStatsFilter {
  cresta.v1.common.time.TimestampRange time_range = 1;

  repeated string user_names = 2 [
    (google.api.resource_reference).type = "cresta.v1.user.User"
  ];
  repeated string virtual_group_names = 3 [
    (google.api.resource_reference).type = "cresta.v1.user.Group"
  ];
  repeated string team_group_names = 4 [
    (google.api.resource_reference).type = "cresta.v1.user.Group"
  ];
}

// Defined once and reused by every option.
message TrainingSimulatorResultSummary {
  int64 assigned_count = 1;
  int64 started_count = 2;
  int64 completed_count = 3;
  int64 incomplete_count = 4;
  int64 not_applicable_count = 5;
  int64 passed_count = 6;
  int64 failed_count = 7;
  optional double average_score = 8;  // 0.0-1.0
  optional double pass_rate = 9;      // 0.0-1.0
  int64 total_attempt_count = 10;
  int64 retried_agent_count = 11;
}

message TrainingSimulatorModuleResultSummary {
  string training_module_name = 1;
  string training_module_display_name = 2;
  TrainingSimulatorResultSummary results = 3;
}

message TrainingSimulatorCriterionStats {
  string behavior_name = 1;
  string criterion_id = 2;
  string display_name = 3;
  int64 passed_count = 4;
  int64 failed_count = 5;
  int64 not_applicable_count = 6;
  optional double pass_rate = 7;  // passed / (passed + failed)
}

message TrainingSimulatorLessonStats {
  string training_lesson_name = 1;
  int64 session_count = 2;
  TrainingSimulatorResultSummary results = 3;
  repeated TrainingSimulatorModuleResultSummary modules = 4;
  // True when at least one matching legacy result cannot be classified because
  // it does not have a persisted evaluation status. Assignment and attempt
  // counts remain valid, but score/pass/N-A results may be incomplete.
  bool has_missing_result_status = 5;
}

message TrainingSimulatorModuleStats {
  string training_module_name = 1;
  // Number of current STATE_ACTIVE lessons that contain this module. This is
  // current content metadata and does not change with user/date filters.
  int64 active_lesson_count = 2;
  TrainingSimulatorResultSummary results = 3;
  repeated TrainingSimulatorCriterionStats criteria = 4;
  // Same legacy-data warning semantics as TrainingSimulatorLessonStats.
  bool has_missing_result_status = 5;
}
```

These are not new page filters. Training Simulator already renders one Assignee filter for agents, teams, and virtual groups plus Date Range. This project threads that existing state into Lesson Configuration. Director maps `groupNames` to `virtual_group_names` and `teamNames` to `team_group_names`; the backend passes them to `UserFilterConditions.SelectedVirtualGroupNames` and `SelectedTeamGroupNames`. User Service still resolves membership, but no preliminary `GroupsByGroupType` call is needed. `direct_team_only` is omitted because the current page filter does not expose it; add it only if Product explicitly expands the page scope.

### Option 1 — Extend `ListTrainingSimulatorTaskRuns`

#### High-level implementation plan

1. Add optional statistics options to the existing request.
2. Keep the existing task-run response unchanged when the options are absent.
3. When requested, run the shared assignment-rooted aggregation directly against the database. Do not calculate statistics from the returned task-run list.
4. Return lesson and/or module statistics beside the task runs.
5. Define separate authorization for the aggregate fields because the existing task-run API permits `AGENT`.
6. Document that task-run filters control the returned run list, while `stats.filter` controls the aggregate dataset.

#### Proto example

```protobuf
message TrainingSimulatorStatsOptions {
  TrainingSimulatorStatsFilter filter = 1;
  repeated string training_lesson_names = 2;
  repeated string training_module_names = 3;
}

// Add to the existing request. Fields 1–6 remain unchanged.
message ListTrainingSimulatorTaskRunsRequest {
  // Existing fields omitted from this illustration.
  TrainingSimulatorStatsOptions stats = 7;
}

// Add to the existing response. Field 1 remains unchanged.
message ListTrainingSimulatorTaskRunsResponse {
  repeated TrainingSimulatorTaskRun training_simulator_task_runs = 1;
  repeated TrainingSimulatorLessonStats lesson_stats = 2;
  repeated TrainingSimulatorModuleStats module_stats = 3;
}
```

#### Pros

- One request can return attempt rows plus lesson and module statistics.
- Reuses an existing service method and client entry point.
- Works well if a screen genuinely needs task runs and aggregate statistics together.
- The API is Training Simulator-specific, so the risk of unrelated consumers driving uncontrolled expansion is lower than for a general Analytics API.

#### Cons

- The API name remains task-run-oriented even when assignment-based aggregates become equally important.
- Two filter scopes are easy to misunderstand: one controls returned runs and one controls statistics.
- Returned runs and aggregate statistics have different completeness rules.
- Correct zero-run statistics still require the same new assignment-rooted database query.
- Aggregate authorization is stricter than the existing `AGENT` task-run authorization.
- Adding more result levels and flags can make the request difficult to read even if the endpoint remains page-scoped.

### Option 2 — Add statistics to `ListTrainingLessons` and `ListTrainingModules`

#### High-level implementation plan

1. Add `include_stats`, `stats_time_range`, and `stats_user_names` directly to both list requests. `include_stats` requests statistics and preserves the existing list-only path when false.
2. Keep the existing lesson/module resource messages unchanged.
3. Add a parallel statistics list to each response, aligned one-to-one and in the same order as the returned content page.
4. Run one assignment-rooted aggregation for the lessons or modules on that page; never issue one query per row.
5. Include the statistics filter in frontend query keys.
6. Decide whether a statistics failure fails the entire list request or returns content with an explicit reporting warning. The simpler initial contract is to fail the request when `include_stats` is true.

#### Proto example

```protobuf
// Add to ListTrainingLessonsRequest. Existing fields 1–7 remain unchanged.
message ListTrainingLessonsRequest {
  // Existing fields omitted from this illustration.
  bool include_stats = 8;
  cresta.v1.common.time.TimestampRange stats_time_range = 9;
  repeated string stats_user_names = 10;
}

// Add field 3; fields 1–2 remain unchanged.
message ListTrainingLessonsResponse {
  repeated TrainingLesson training_lessons = 1;
  string next_page_token = 2;
  // Same length and order as training_lessons when include_stats is true.
  repeated TrainingSimulatorLessonStats lesson_stats = 3;
}

// Add to ListTrainingModulesRequest. Existing fields 1–6 remain unchanged.
message ListTrainingModulesRequest {
  // Existing fields omitted from this illustration.
  bool include_stats = 7;
  cresta.v1.common.time.TimestampRange stats_time_range = 8;
  repeated string stats_user_names = 9;
}

// Add field 3; fields 1–2 remain unchanged.
message ListTrainingModulesResponse {
  repeated TrainingModule training_modules = 1;
  string next_page_token = 2;
  // Same length and order as training_modules when include_stats is true.
  repeated TrainingSimulatorModuleStats module_stats = 3;
}
```

#### Module selection vs statistics selection

Module-list fields and statistics fields filter different datasets. `training_module_names` selects module definitions; `stats_user_names` and `stats_time_range` select the assignment/result facts counted for those modules. Obtaining module IDs first does not communicate the selected assignees or date range to the backend.

For example:

```text
training_module_names = [M1, M2]
stats_user_names = [Alice]
stats_time_range = August
```

The server first applies the ordinary module filters, ordering, and page size. If the returned page is `[M1, M2]`, it then performs one statistics load constrained to those page IDs, Alice, and August, aggregates the results, and returns `module_stats` aligned with `[M1,M2]`. Pagination therefore happens before statistics loading.

“Bounded” means the statistics load is limited to module IDs on the returned page and the authorized result scope. “Unbounded” would load results for every module matching the pre-pagination content query, so work could grow with the tenant's entire module catalog even though the response displays only one page. Bounded does not itself guarantee low latency; representative query measurement remains required.

If Product intentionally defines module reporting as always all-time and all-authorized-users, the two statistics filters could be removed. They remain required for the accepted UI because Lesson Configuration exposes Assignee and Date Range filters. Request filters narrow authorized results; they never grant access.

#### Pros

- Each tab receives content and its statistics in one request.
- Lesson statistics align naturally with lesson rows; module statistics align naturally with module rows.
- Existing server pagination bounds aggregation to the visible content page.
- Director does not need a second request or a resource-name join.
- Reviewers currently prefer this option because it reuses APIs without mixing lesson and module statistics into a task-run contract.
- The APIs are expected to remain scoped to the Training Simulator page, reducing the chance of broad cross-product complexity.

#### Cons

- User/date filters now invalidate otherwise stable content-list caches.
- Reporting latency becomes part of authoring-list latency when statistics are requested.
- Reporting failure can block content loading unless partial-success behavior is designed.
- Existing content-list authorization may be broader than authorization for statistics across users.
- Parallel content/statistics arrays require a strict same-length, same-order contract.
- Consumers that do not need reporting still inherit the expanded generated contract, although they pay no reporting-query cost when `include_stats` is false.

### Option 3 — Add dedicated lesson and module statistics APIs

#### High-level implementation plan

1. Add `RetrieveTrainingSimulatorLessonStats` and `RetrieveTrainingSimulatorModuleStats`.
2. Give both APIs a required profile, 1–200 content names, and direct time/user reporting fields.
3. Implement both with the same assignment loader, conversation-result loader, and calculation library.
4. Preserve request order and return a zero-valued row for valid content with no matching assignments.
5. Authorize and monitor reporting independently from task-run and content-list APIs.
6. In Director, request one batch of statistics after the visible content page supplies its resource names; use caching and inactive-tab prefetch.

#### Proto example

```protobuf
message RetrieveTrainingSimulatorLessonStatsRequest {
  string parent = 1;
  repeated string training_lesson_names = 2;
  cresta.v1.common.time.TimestampRange stats_time_range = 3;
  repeated string stats_user_names = 4;
}

message RetrieveTrainingSimulatorLessonStatsResponse {
  repeated TrainingSimulatorLessonStats lesson_stats = 1;
}

message RetrieveTrainingSimulatorModuleStatsRequest {
  string parent = 1;
  repeated string training_module_names = 2;
  cresta.v1.common.time.TimestampRange stats_time_range = 3;
  repeated string stats_user_names = 4;
}

message RetrieveTrainingSimulatorModuleStatsResponse {
  repeated TrainingSimulatorModuleStats module_stats = 1;
}

service TrainingSimulatorService {
  rpc RetrieveTrainingSimulatorLessonStats(
      RetrieveTrainingSimulatorLessonStatsRequest)
      returns (RetrieveTrainingSimulatorLessonStatsResponse);

  rpc RetrieveTrainingSimulatorModuleStats(
      RetrieveTrainingSimulatorModuleStatsRequest)
      returns (RetrieveTrainingSimulatorModuleStatsResponse);
}
```

#### Pros

- Provides the cleanest API boundaries and names.
- Reporting authorization, latency, errors, monitoring, and evolution are independent from content authoring and task-run listing.
- Zero-run assignment semantics are first-class.
- Reporting failure does not prevent content lists from loading.
- Lesson and module contracts can evolve independently while sharing backend implementation.
- Because the APIs are page-specific, they are unlikely to accumulate the broad, unrelated use cases seen in Analytics APIs.

#### Cons

- Adds two service methods, generated clients, frontend hooks, and query keys.
- A cold tab normally loads content names before requesting statistics.
- Director must join statistics to content rows by resource name.
- Separate content and statistics requests can observe slightly different database snapshots.
- More API surface is added even though the expected consumer is primarily one page.

### Common behavior for all options

- Reporting is read-only; no reporting handler modifies task runs, scores, result status, or evaluation data.
- The backend applies the statistics filter to direct database reads and includes assignees with no runs.
- Quiz statistics remain outside this project.
- Valid content with no matching assignments returns zero counts and absent score/rate values.
- Request filters narrow results but never grant authorization.
- `user_names`, `virtual_group_names`, and `team_group_names` form one user selection, which is intersected with the caller's authorized users.
- A selected group with no members returns zero matching assignments; it never becomes an all-user query.
- The selected option must define measurable latency and failure behavior before implementation.

## Current-content aggregation and revision limitation

The reporting APIs use the lesson and module data currently available when the request runs. They do not add assignment snapshots, pin new revisions, or change evaluation to reload the attempted revision.

This creates two known historical limitations:

- Editing a lesson's module membership can change historical lesson completion calculations.
- Editing module criteria, weights, thresholds, or auto-fail settings can make current metadata differ from the configuration used for an earlier attempt.

These issues are important, but fixing them is outside this project's scope. The implementation should keep the aggregation logic isolated so revision-aware reads can be added later without changing the public statistics API.

## Persisted training-result prerequisite

Extend `director.training_simulator_conversation_scores`; do not add this data to `app.chats` and do not create another result table. Persist the evaluator's existing status and overall `not_applicable` value alongside score, passed, and criterion results.

The write contract must carry these values from `EvaluateTrainingConversationResponse` through the existing `UpdateTrainingSimulatorTaskRun` flow. Add `evaluation_status` and overall `not_applicable` to the conversation-attempt update shape (or introduce an equally explicit conversation-result update input), then update the converter and transaction to persist all evaluation fields together.

The evaluator already produces these statuses:

```text
EVALUATION_STATUS_PENDING
EVALUATION_STATUS_IN_PROGRESS
EVALUATION_STATUS_COMPLETE
```

Together, persisted `evaluation_status`, `not_applicable`, and `passed` distinguish pending, partial, completed N/A, completed passed, and completed failed attempts. Write them in the same transaction as score and criterion results. Do not infer overall N/A from “all criterion results are N/A”: unfinished criteria currently use the same per-criterion encoding.

For legacy rows without this status:

- preserve started/attempt facts;
- permit applicable completed classification only where the historical record is unambiguous;
- exclude ambiguous all-N/A-looking rows from pass-rate and average-score denominators; and
- set `has_missing_result_status = true` so Director can disclose incomplete historical data.

Do not recompute conversation pass as `score >= threshold`; auto-fail and zero-threshold rules make persisted pass reliable.

## User selection and aggregation semantics

### Assignment facts

- Lesson fact: `(director_task_id, lesson_id, assigned_agent_id)`.
- Module fact: `(director_task_id, lesson_id, module_id, assigned_agent_id)` expanded from the current lesson definition.
- A task containing multiple lessons creates facts for each lesson.
- Intersect the expanded task audience with the authorized/requested user set.
- `session_count` is the distinct matching DirectorTask count, not assigned-agent count.
- `active_lesson_count` is the distinct count of current `STATE_ACTIVE` lesson definitions whose latest revision contains the module. It is a content-relationship count and does not change with date/assignee filters.

### Unified attempt result

Create an internal `attemptResult` independent of the public task-run proto:

```go
type attemptResult struct {
    TaskRunID       string
    DirectorTaskID  string
    LessonID        string
    ModuleID        string
    AgentUserID     string
    Score           *float64 // normalized 0.0–1.0
    Passed          *bool
    EvaluationStatus trainingsimulatorpb.EvaluationStatus
    NotApplicable    *bool
    Criteria        []*trainingsimulatorpb.CriterionEvaluationResult
    AttemptTime     time.Time
}
```

Mapping rules:

- Conversation row: read agent, score, passed, criterion results, evaluation status, and overall N/A from `training_simulator_conversation_scores`.
- Do not fall back to removed task-run result columns.
- Exclude quiz task runs from this project.
- A run is completed only when `evaluation_status` is `EVALUATION_STATUS_COMPLETE`.

### Latest attempt

For each assignment fact, select the conversation run with the greatest task-run `created_at`; use task-run resource ID as the deterministic tie-breaker.

The latest run is the official result even when an earlier run passed. Count every run in `total_attempt_count`.

### Module training results

- `assigned_count`: module assignment facts.
- `started_count`: facts with at least one run.
- `completed_count`: facts whose latest run has a complete training result.
- `not_applicable_count`: results with complete evaluation status and persisted overall `not_applicable = true`. Never infer this from criterion rows alone.
- `passed_count`/`failed_count`: applicable completed latest results.
- `average_score`: mean normalized score across applicable completed latest results.
- `pass_rate`: `passed_count / (passed_count + failed_count)`.
- `retried_agent_count`: distinct agents with more than one run for the same `(task, lesson, module)` fact.

### Lesson training results

For each lesson assignment fact:

- started if any required module has a run
- completed if every module in the current lesson definition has a completed latest result
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

Prefer stable behavior resource name as the rollup key. For a criterion without a behavior, use `module_id + ":" + criterion_id`. Display name comes from the recorded result first, then the current module definition.

## Query plan

Implement one shared loader and aggregator used by the selected public API handlers.

1. Parse and validate the profile and requested resource names.
2. Resolve the authorized/requested agent set. Populate the shared user-filter parser with `SelectedUserNames`, `SelectedVirtualGroupNames`, and `SelectedTeamGroupNames` directly. User Service still resolves group membership, but no preliminary group-type lookup is needed.
3. Load in-scope `ACTIVE` and `ARCHIVED` Training Simulator DirectorTasks with the established active-window overlap predicate; exclude `DRAFT`/`DELETED`.
4. Load current lesson/module definitions and expand them into lesson/module assignment facts.
5. Query task runs by `(customer_id, profile_id, director_task_id IN ...)` and requested lesson/module IDs.
6. Batch-load referenced conversation scores; exclude quiz task runs.
7. Normalize rows into `attemptResult` values and aggregate in memory.

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
  selected API handler changes
  stats_assignment_loader.go
  stats_attempt_loader.go
  stats_aggregator.go
  *_test.go

go-servers/apiserver/sql-schema/director/director-schema.sql
  evaluation_status and not_applicable on training_simulator_conversation_scores
```

Keep DB loading, result normalization, and pure aggregation separate. The aggregator should accept plain structs and have table-driven unit tests without database setup.

## Failure behavior

- Invalid parent/content/user/group name: `INVALID_ARGUMENT`.
- Requested content not in parent: `INVALID_ARGUMENT` for cross-profile; zero row or `NOT_FOUND` for absent same-profile content (choose once and document; zero row is preferred for batch stability).
- Current lesson/module definition is missing: return a contract error for the affected requested item and emit telemetry rather than inventing historical membership.
- Orphaned conversation-score reference: count as started/incomplete, emit a counter, and continue.
- Missing/ambiguous legacy result status: exclude from score/pass/N/A denominators, set the response warning, and emit a counter.
- Run not attributable to an assignment fact: exclude from official training results, count it in an orphan telemetry metric.
- Filtered user or run set exceeds a safety bound: `RESOURCE_EXHAUSTED`, never partial success.

## Observability and SLO

Record per RPC:

- latency and status
- task, assignment-fact, task-run, and conversation-score counts
- requested content and agent counts
- missing/ambiguous result-status counts
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
- explicit all-N/A, mixed N/A/applicable, partial-timeout persistence, ambiguous legacy N/A, and empty denominator
- conversation results and missing-result-status behavior
- criteria with the same ID in different modules
- selected-user intersection and cross-profile rejection
- count invariants

Integration tests:

- RPC handler through PostgreSQL fixtures for conversation results
- active/archived inclusion, draft/deleted exclusion, and task-window overlap
- current lesson membership after the lesson is edited, documented as current-data behavior
- role authorization and manager scope before manager roles are enabled
- query plan/latency fixture at the supported task/run safety bounds

## Rollout

1. Approve task lifecycle, date, N/A, and authorization semantics.
2. Land the explicit persisted result/evaluation state and stop persisting partial snapshots as settled results.
3. Build the shared normalized conversation-attempt loader and aggregations using current lesson/module definitions.
4. Add the selected API contract behind `enableTrainingSimulatorV2` (or a dedicated reporting flag if independent rollout is needed).
5. Compare staging data against manually calculated user selections, including missing-status cases.
6. Integrate Director and gradually enable customers with latency, warning, and invariant dashboards.

## Review gates

- Product: reporting surface, active/archived scope, date meaning, and all-N/A display.
- Security: initial roles and manageable-user scope.
- Data: conversation-score status/N/A migration and fallback policy plus a representative query plan.
- FE/BE: score scale, empty-value behavior, response ordering, and 200-name batch limit.
