# Training Simulator Lesson and Module Statistics — Engineering Design

> **Superseded requirement (2026-08-27):** Statements in this draft requiring persisted overall evaluation status/N/A or a distinct overall all-N/A reporting state are no longer current. Product accepted failure-equivalent reporting for the listed zero/false timeout, all-criteria-N/A, and failed cases. Criterion-level N/A remains excluded from scoring. See [the decision record](../decisions/2026-08-27-collapse-overall-zero-false-results.md).
>
> **Milestone 1 revalidation (2026-08-31):** The remaining visible session-reporting additions are the unique count of assigned agents with at least one incomplete visible session and per-agent drawer attempt count. The incomplete count is frontend-derivable. `AgentPerformanceEntry.attempt_count` is the only new response datum. Per-agent status plus score already exists and is not a missing pass-rate field. Treat contradictory Milestone 1 requirements later in this draft as historical.

Source: [Superhuman Docs](https://docs.superhuman.com/d/_dE0dCcz8Bub/_subF07cy)

Authors: xuanyu.wang@cresta.ai
Status: Draft for review from Jack Jee
Last reviewed / updated: Aug 24, 2026
Related: CONVI-7263

## Goal

Provide trustworthy lesson- and module-level Training Simulator statistics in Director:

* lesson session count, average score, pass rate, and per-module pass rates
* module active-lesson count, average score, pass fraction, and per-criterion pass fractions
* the small set of missing session-side-panel values described in Figma
* existing Assignee and Date Range filters
* batch reads without per-row requests
* explicit historical/data-quality warnings instead of plausible but incorrect rates

One correctness contract is required for reliable rollups: persist the conversation evaluator's overall status and overall N/A result. The reader must never guess whether an all-N/A-looking record is a genuine N/A result or an unfinished evaluation. Milestone 1 also establishes the official session-attempt rule: select the latest attempt first, then classify that attempt's subtype result. It must not fall back to an older completed result when a newer retry is unfinished.

## Non-goals

* CSV export (details to be determined)
* revision-exact historical reporting; this project uses current lesson/module definitions
* lesson/module-level quiz aggregates; Milestone 1 session completion still includes required quiz-module attempts because current session reporting and real staging data contain them

## Background

A DirectorTask can target multiple audience users. An agent session is (task, agent), not a separate task. Reporting must start from assignment facts, not task runs, so an assigned agent who never starts still counts.

The official latest module attempt is keyed by task + lesson + module + agent. Select the latest task run first; then classify the result attached to that run. Conversation results live in `director.training_simulator_conversation_scores`, while quiz results live in `director.quiz_scores`. Session completion must consider both subtypes while required quiz modules remain assignable.

Current result-persistence gaps:

1. **Overall conversation N/A is not persisted.** The evaluator returns an overall `not_applicable` value, but `director.training_simulator_conversation_scores` currently stores only score, passed, and per-criterion results. After the response is written, reporting cannot reliably tell whether a completed attempt was genuinely N/A. Add overall `not_applicable` to the existing conversation-score row and carry it through the `UpdateTrainingSimulatorTaskRun` write path.

2. **An unfinished evaluation can look like a completed all-N/A result.** While annotations are still pending, the evaluator represents unfinished criteria with the same per-criterion N/A-looking values used by a genuine N/A result. Director can persist the latest snapshot when its polling timeout expires, but the conversation-score row does not store whether evaluation was pending, in progress, or complete. Reporting could therefore count unfinished work as completed and N/A. Persist the evaluator's `evaluation_status` with overall N/A, score, passed, and criterion results in one transaction; only `EVALUATION_STATUS_COMPLETE` is a completed attempt.

Known historical limitations, not fixed by this project:

* evaluation can use current module data rather than the attempted revision
* current content reloads can change historical membership, criteria, weights, thresholds, and auto-fail semantics

The reporting APIs intentionally aggregate using the current lesson/module data available at request time. Keep these limitations visible in the design, but do not block this project on revision pinning or assignment snapshots.

Score scales differ across the current flow:

* **Evaluation and module configuration use 0–100.** A module passing score of `80` means 80%, and `EvaluateTrainingConversation` returns a score such as `82`.
* **Persistence uses 0–1.** Before calling `UpdateTrainingSimulatorTaskRun`, Director divides the evaluator score by 100, so `82` is stored in `director.training_simulator_conversation_scores.score` as `0.82`.
* **The statistics API also uses 0–1.** `average_score` and `pass_rate` return ratios such as `0.82`. The current passing score is not duplicated in the statistics response; Director reads the existing 0–100 value from the listed module's evaluation config.
* **Director displays percentages.** The frontend converts a statistics ratio such as `0.82` to `82%` exactly once, while formatting the module configuration's passing score as its existing 0–100 percentage value.

The aggregation loader must treat a persisted score as already normalized; it must not divide `0.82` by 100 again. Likewise, the frontend must not multiply an API score more than once. Tests should cover a concrete value such as evaluator `82` → persisted/API `0.82` → displayed `82%`, because a missed or repeated conversion would produce `0.82%` or `8,200%`.

## Overview

The API review considered three choices:

1. extend ListTrainingSimulatorTaskRuns
2. add statistics to the lesson/module list APIs
3. add dedicated lesson/module statistics RPCs.

The API boundary closed on 2026-08-28 with option 3 selected: dedicated lesson and module statistics RPCs. Avoiding a second frontend request was the main reason for option 2, but the review prioritized cleaner responsibility, convention, discoverability, and independent reporting behavior once cold-tab performance was not considered a major concern. The three options remain compared in [API design](#api-design) as decision history. See [the local decision record](../decisions/2026-08-28-dedicated-lesson-module-stats-apis.md).

Regardless of the public API, the backend starts from DirectorTask assignments, expands current lesson definitions into per-agent facts, selects the latest task run for each agent and required module, joins the selected run to its conversation or quiz result, and calculates results in memory. Lesson/module aggregates in this project remain conversation-only, but Milestone 1 session completion must normalize both result subtypes. All options share filters, training-result messages, user selection, data loading, and calculation code.

Director integrates these APIs into Lesson Configuration first. Existing Sessions assigned and Active Lessons placeholders become real values; lesson/module drawers show aggregate details. The hooks, view models, and drawers can later mount under a manager-visible Insights route without forking logic.

Data flow:

DirectorTask assignments → current lesson/module definitions → task runs → conversation/quiz results → latest-attempt-first classification → selected API contract → Director view models → tables/drawers

### User experience

Lesson table and drawer:

* populate Sessions assigned; keep lesson name as the authoring link
* show average score and pass rate
* show one row per required module with its pass-rate chip
* show a non-blocking warning for missing result status
* render empty/ambiguous rates as --, never 0%

Module table and drawer:

* populate Active Lessons from current content relationships
* show average score, the current passing score from module configuration, and pass rate with its passed/failed fraction
* show conversation-criterion pass fractions
* sort most-missed criteria by failed count, then display name
* exclude quiz modules from statistics in this project

Assignee and Date Range apply across Training Sessions, Lessons, and Modules. Content search remains local. Active Lessons is current metadata and does not change with selected-user filters.

## Detailed design

### Backend aggregation

Assignment facts are derived records created by expanding each DirectorTask audience into individual agent user IDs; `agent_user_id` is not a table column on the task:

* lesson: `(director_task_id, lesson_id, agent_user_id)`
* module: `(director_task_id, lesson_id, module_id, agent_user_id)`, expanded from the current lesson definition
* session: `(director_task_id, agent_user_id)`, covering all required modules assigned by that DirectorTask
* include ACTIVE and ARCHIVED; exclude DRAFT and DELETED, pending product confirmation
* use task active-window overlap for dates, pending product confirmation
* intersect expanded audience with the authorized/requested users

Latest attempt uses task-run `created_at`; run resource ID breaks ties. Selection happens before result-state classification. A newer pending or in-progress retry therefore remains the official latest attempt and makes the required module incomplete; reporting does not skip it to reuse an older completed result unless Product explicitly changes the policy. For the session side panel, count every matching conversation or quiz run for the session's required modules in the agent's `attempt_count`.

Read-only reporting dependency:

* EVALUATION_STATUS_PENDING
* EVALUATION_STATUS_IN_PROGRESS
* EVALUATION_STATUS_COMPLETE

The lesson/module reporting APIs are read-only. They must never create or update task runs, conversation scores, evaluation status, N/A, pass/fail, criterion results, or scores.

As a separate upstream prerequisite, extend the existing evaluation write path so future evaluations store `evaluation_status` and overall `not_applicable` on `director.training_simulator_conversation_scores`. Carry those values from `EvaluateTrainingConversationResponse` through `UpdateTrainingSimulatorTaskRun`; that existing command path—not either reporting RPC—writes them with score, passed, and criterion results. Do not put this data on `app.chats` and do not create another result table.

The reporting queries only read the persisted fields. For existing rows without status, preserve attempt/started facts, classify only unambiguous applicable completions, exclude ambiguous rows from score/pass/N/A denominators, and return `has_missing_result_status`.

### Statistics definitions

These definitions are authoritative for every API option.

Common terms:

| Term | Exact meaning |
|---|---|
| Matching assignment fact | A derived lesson or module assignment fact that matches the content, date, selected-user, authorization, and task-state filters. |
| Matching run | A conversation task run belonging to a matching module assignment fact. Quiz runs are excluded. |
| Latest run | The matching run with the greatest `created_at` for one `(director_task_id, lesson_id, module_id, agent_user_id)` fact; resource ID breaks timestamp ties. |
| Settled result | A latest conversation result whose persisted `evaluation_status` is `EVALUATION_STATUS_COMPLETE`. |
| Applicable result | A settled result whose persisted overall `not_applicable` is false. |
| Required modules | Modules in the lesson definition available when the reporting request runs. |

The common `Matching run`/`Settled result` terms above define the conversation-only lesson/module aggregates. Milestone 1 session reporting uses the following subtype-aware terms instead:

| Session term | Exact meaning |
|---|---|
| Matching session attempt | A conversation or quiz task run belonging to an assigned agent and one of the session's required modules. |
| Official session attempt | The matching session attempt with the greatest `created_at` for `(director_task_id, lesson_id, module_id, agent_user_id)`; resource ID breaks timestamp ties. Select this row before inspecting result state. |
| Settled conversation result | The official conversation attempt has persisted `EVALUATION_STATUS_COMPLETE`; an ambiguous legacy row without status is not authoritative. |
| Settled quiz result | The official quiz attempt has a submitted quiz result under the existing quiz contract. |
| Applicable session result | A settled conversation result whose overall `not_applicable` is false, or a settled quiz result whose score/pass semantics are defined by the existing session implementation. |

#### Session-level additions

The existing session design already covers the session totals, average score, pass rate, completion, assigned-agent count, due date, and score distribution. This project only needs to fill the following gaps.

| Missing statistic or side-panel value | Exact calculation or source |
|---|---|
| Agents who have not completed the assigned task | Count assigned agents whose session status is `INCOMPLETE`. An agent is incomplete when the official latest attempt for at least one required module does not have a settled conversation or quiz result, including agents with no runs. |
| One side-panel row per assigned agent | Group by the derived `(director_task_id, agent_user_id)` session assignment fact. Include agents who never started. |
| Passed / failed / incomplete / all-N/A | `PASSED` when all required modules are settled and every applicable module passed; `FAILED` when all required modules are settled and at least one applicable module failed; otherwise `INCOMPLETE`. A completed all-N/A result must be displayed as N/A rather than failed. Figma's All/Passed/Failed/Incomplete filters do not define where completed all-N/A belongs; Design must choose a first-class N/A filter/state or explicitly define another category before frontend implementation. |
| Attempt count | Count all matching conversation and quiz task runs for the agent and session before latest-attempt selection, across all required modules. |
| Pass/fail percentage shown for the agent | Use the agent's mean score across the latest applicable results for the required modules, displayed as a percentage. This is the score associated with the pass/fail status, not a pass rate across historical attempts. |
| Agent name | Use `AgentPerformanceEntry.agent_name` and `agent_display_name`. |
| Time | Use the most recent matching task-run `create_time` for the agent in the session. This is latest activity time, not necessarily completion time. Show “Not started” when the agent has no runs. |
| View link | Build the existing Conversation Review route from `TaskStats.task_name` and `AgentPerformanceEntry.agent_name`; no URL needs to be stored or returned. |

The incomplete-agent count can be derived from the complete `agent_performance` list, so `TaskStats` does not need another count field.

#### Lesson-level reporting

One lesson rollup groups matching lesson assignment facts across DirectorTasks.

| Statistic or breakdown | Exact calculation |
|---|---|
| `session_count` | Distinct matching DirectorTask IDs that assign the lesson. This is not the assigned-agent count. |
| `average_score` | First calculate each completed lesson fact's mean across applicable required-module scores, then average those lesson-fact scores. All-N/A facts are excluded so lessons with more modules are not overweighted. |
| `pass_rate` | Completed lesson facts that passed divided by completed lesson facts that passed or failed. A lesson fact passes when every applicable required module passed; it fails when at least one applicable required module failed. All-N/A and incomplete facts are excluded. |
| `modules[].pass_rate` | Apply the module pass-rate calculation below to each required module, restricted to this lesson's matching assignment facts. The lesson drawer uses this for each module's pass-rate chip. |

Created/last-edited metadata, module display names/descriptions, and current thresholds come from the existing lesson/module resources, not the statistics response.

#### Module-level reporting

One module rollup groups matching module assignment facts across lessons and DirectorTasks.

| Statistic or breakdown | Exact calculation |
|---|---|
| `active_lesson_count` | Distinct current `STATE_ACTIVE` lesson definitions whose latest revision contains the module. It does not change with date or selected-user filters. |
| `passed_count` | Applicable completed module facts whose persisted `passed` value is true. |
| `failed_count` | Applicable completed module facts whose persisted `passed` value is false. |
| `average_score` | Mean normalized score across applicable completed latest results. |
| `pass_rate` | `passed_count / (passed_count + failed_count)`. |
| `criteria[]` | Criterion results from the latest settled module result for each matching module fact. |

The module drawer displays `pass_rate` together with `passed_count / (passed_count + failed_count)`. Its current passing threshold and created/last-edited metadata come from the existing module resource.

#### Criterion breakdown within a module

| Statistic | Exact calculation |
|---|---|
| Criterion `passed_count` | Latest settled module results in which the criterion result is passed. |
| Criterion `failed_count` | Latest settled module results in which the criterion result is failed. |
| Criterion `pass_rate` | `passed_count / (passed_count + failed_count)`; criterion N/A results are excluded. |

Criterion rows group by stable behavior resource name. If no behavior resource exists, use `(module_id, criterion_id)` as the key. Display name comes from the recorded criterion result first, then the current module definition.

For `average_score` and `pass_rate`, return an absent optional value when the denominator is zero. `has_missing_result_status` is true when at least one matching legacy result cannot be classified because persisted evaluation status is missing, so score/pass values may be incomplete.

Required invariants:

* `passed_count + failed_count` is the module and criterion pass-rate denominator
* N/A and incomplete results never enter score or pass-rate denominators

### Changes to `RetrieveTrainingSimulatorTaskStats`

Keep the existing RPC and `TaskStats` resource as the session-level API. Most side-panel data already exists in `AgentPerformanceEntry`; the only new response value required for the side panel is the all-attempt count. The missing-result-status field remains a separate correctness warning for legacy data.

Required backend changes:

1. Start from every filtered and authorized active DirectorTask and its expanded audience. Return one `TaskStats` row for a session with no runs and one `AgentPerformanceEntry` for every assigned agent. Preserve the current Director zero-activity fallback as a defensive client behavior, but do not rely on it for API correctness.
2. Query all qualified runs directly with bounded database reads; do not use `ListTrainingSimulatorTaskRuns` or inherit its 1,000-row limit, because that would make attempt counts incomplete.
3. Calculate `attempt_count` from all matching conversation and quiz runs before reducing them to the official latest attempt per required module.
4. Keep `task_runs` for the existing details/time behavior. The frontend obtains the displayed time from the greatest task-run `create_time`.
5. After latest-attempt selection, classify conversation results from persisted `evaluation_status` and overall `not_applicable`; classify quiz results from their existing submitted score/pass contract. An unfinished or ambiguous legacy conversation result must not become passed or failed, and completed all-N/A must not become failed.
6. Preserve the existing agent name, display name, score, completion status, passed flag, and task/session names. Director can build the Conversation Review link from those resource names. The existing frontend field named `completedAt` should be renamed because it represents latest activity for incomplete and complete agents.
7. Enforce reporting authorization in the backend. Keep `AGENT` because agent-facing Assigned Training Sessions consumes this RPC for the assignee's progress, status, and score. Restrict agent-only callers to their own row, managers to manageable users, and administrators to their authorized customer/profile population before loading assignments and results; never trust Director request filters as authorization.

Additive proto example:

```protobuf
message AgentPerformanceEntry {
  // Existing fields 1-7 remain unchanged during migration.

  // All matching conversation and quiz runs for this agent and session across
  // required modules, counted before latest-attempt selection.
  int64 attempt_count = 8;

  // True when at least one latest module result for this agent cannot be
  // classified because persisted evaluation status is missing.
  bool has_missing_result_status = 9;
}
```

Populate the existing per-agent fields consistently:

| Agent-session result | `status` | `not_applicable` | `passed` | Legacy `score` |
|---|---|---:|---:|---:|
| Never started, partially complete, or latest result not settled | `INCOMPLETE` | `true` | `false` | `0` (not meaningful) |
| Complete and every required module is N/A | `COMPLETE` | `true` | `false` | `0` (not meaningful) |
| Complete and every applicable required module passed | `COMPLETE` | `false` | `true` | Mean of applicable module scores |
| Complete with at least one applicable required module failed | `COMPLETE` | `false` | `false` | Mean of applicable module scores |

This existing field combination is sufficient at the API compatibility boundary to distinguish incomplete, completed N/A, passed, and failed agent-session results. Director must stop interpreting every `COMPLETE && !passed` entry as failed; it must check `not_applicable` first. Its current `AttemptStatus` model has only passed/failed/incomplete, so Milestone 1 must add an explicit all-N/A presentation and resolve filter membership with Design. An entry with missing legacy evaluation status remains incomplete for reporting and sets `has_missing_result_status`; Director must surface that warning on the affected row and aggregate rather than silently rendering a plausible score or rate.

### Current-content aggregation and revision limitation

The reporting APIs use the current lesson and module data available when a request runs. They do not add assignment snapshots, pin new revisions, or change evaluation to reload the attempted revision.

This means lesson edits can change historical completion calculations, and module edits can make current criteria or thresholds differ from those used for an earlier attempt. These are important known limitations, but fixing them is outside this project's scope. Keep aggregation isolated so revision-aware reads can be added later without changing the public statistics API.

### Query and service plan

1. Validate profile, content names, and batch size.
2. Resolve the authorized/requested users. Pass the request's `stats_user_names`, `stats_virtual_group_names`, and `stats_team_group_names` directly into the shared backend user-filter parser. User Service still resolves membership, but the backend does not need a preliminary RPC to determine each group's type.
3. Load matching ACTIVE/ARCHIVED Training Simulator tasks.
4. Load current lesson/module definitions and expand them into assignment facts.
5. Load runs by customer/profile/task IDs and requested content IDs.
6. Batch-load conversation scores and exclude quiz task runs.
7. Normalize conversation rows to internal attemptResult values and aggregate.

Use bounded direct GORM queries; chunk large IN lists and return RESOURCE_EXHAUSTED rather than partial data. Measure the current task-scoped composite index before adding indexes. Keep DB loading, normalization, and pure aggregation separate.

### Frontend architecture

* add Director API methods and distinct React Query keys
* issue one batch request per active sub-tab, currently capped at 200 names
* map filterState.usersTeamsGroups.groupNames to `stats_virtual_group_names` and teamNames to `stats_team_group_names`; do not merge the group types or expand membership in Director
* thread filterState through TrainingSimulatorTabs → LessonConfiguration → LessonsTab/ModulesTab
* map protobufs into pure lesson/module view models
* index results by full resource name
* retain only selected resource name in drawer state so open drawers update after refetch
* reuse FullDrawer, DirectorTable, formatters, localization, and disclosure patterns

### Loading and prefetch

* Start the active tab’s content-list request immediately and start its batch stats request as soon as content names are available.
* Reuse cached content names so subsequent visits can begin list and stats work effectively in parallel.
* After the active tab becomes interactive, background-prefetch the inactive tab when its names and access gates are available.
* Use the same helpers to build selected-user filters and stable cache keys for normal requests, prefetch, and drill-through.
* Do not block authoring tables on reporting failure; keep content interactive and render reporting values as --.
* Measure cold first-tab, cold tab-switch, and cached tab-switch latency. Consider combining API requests only if cache and prefetch tuning miss the agreed target.

**Warning behavior:**

* has_missing_result_status: affected score/pass values render -- with a warning
* omitted requested entry: contract error plus client telemetry
* request failure: authoring table remains usable; reporting shows --

## API design

### Selected boundary

Option 3 is selected: lesson and module reporting each have an explicit batch API. The review accepted a second frontend request in exchange for cleaner responsibility, convention, discoverability, and independent reporting behavior. Director batches the loaded content names and uses caching/prefetch to mitigate cold tab switching; measured latency remains a release check.

No option changes the aggregation source: statistics are calculated by direct, filtered database reads over assignments, current lesson/module definitions, conversation attempts, and conversation-score rows. They are never calculated from a paginated API response.

All options reuse the same result messages. There is no shared `TrainingSimulatorStatsFilter`; each API request owns its reporting-population fields so lesson and module contracts can evolve and land independently.

```protobuf
// A module row inside the lesson drawer. Module metadata comes from the
// listed lesson/module resources.
message TrainingSimulatorModulePassRate {
  string training_module_name = 1;
  optional double pass_rate = 2;  // 0.0-1.0
}

message TrainingSimulatorCriterionStats {
  string behavior_name = 1;
  string criterion_id = 2;
  string display_name = 3;
  int64 passed_count = 4;
  int64 failed_count = 5;
  optional double pass_rate = 6;  // passed / (passed + failed)
}

message TrainingSimulatorLessonStats {
  string training_lesson_name = 1;
  int64 session_count = 2;
  optional double average_score = 3;  // 0.0-1.0
  optional double pass_rate = 4;      // 0.0-1.0
  repeated TrainingSimulatorModulePassRate modules = 5;
  // True when at least one matching legacy result cannot be classified because
  // it does not have a persisted evaluation status, so score/pass values may
  // be incomplete.
  bool has_missing_result_status = 6;
}

message TrainingSimulatorModuleStats {
  string training_module_name = 1;
  // Number of current STATE_ACTIVE lessons that contain this module. This is
  // current content metadata and does not change with user/date filters.
  int64 active_lesson_count = 2;
  optional double average_score = 3;  // 0.0-1.0
  int64 passed_count = 4;
  int64 failed_count = 5;
  optional double pass_rate = 6;      // passed / (passed + failed)
  repeated TrainingSimulatorCriterionStats criteria = 7;
  // Same legacy-data warning semantics as TrainingSimulatorLessonStats.
  bool has_missing_result_status = 8;
}
```

These are not new page filters. Training Simulator already renders one Assignee filter for agents, teams, and virtual groups plus Date Range. This project threads that existing state into Lesson Configuration. Each selected API request carries its own `stats_time_range`, `stats_user_names`, `stats_virtual_group_names`, and `stats_team_group_names`. Director maps `groupNames` to `stats_virtual_group_names` and `teamNames` to `stats_team_group_names`; the backend passes them to `UserFilterConditions.SelectedVirtualGroupNames` and `SelectedTeamGroupNames`. User Service still resolves membership, but no preliminary `GroupsByGroupType` call is needed. `direct_team_only` is omitted because the current page filter does not expose it; add it only if Product explicitly expands the page scope.

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
  cresta.v1.common.time.TimestampRange stats_time_range = 1;
  repeated string stats_user_names = 2;
  repeated string stats_virtual_group_names = 3;
  repeated string stats_team_group_names = 4;
  repeated string training_lesson_names = 5;
  repeated string training_module_names = 6;
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

1. Add `include_stats` plus reporting time/user/group fields directly to both list requests. `include_stats` requests statistics and preserves the existing fast path when false.
2. Keep the existing lesson/module resource messages unchanged.
3. Add a parallel statistics list to each response, aligned one-to-one and in the same order as the returned content page.
4. Run one assignment-rooted aggregation for the lessons or modules on that page; never issue one query per row.
5. Include all reporting fields in frontend query keys.
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

#### Why module IDs do not replace statistics filters

`training_module_names` selects content definitions, while `stats_user_names` and `stats_time_range` select the assignment/result facts counted for that content. For example, `training_module_names = [M1, M2]`, `stats_user_names = [Alice]`, and `stats_time_range = August` means: paginate the module query first; for a returned page `[M1, M2]`, load results once for only M1/M2, Alice, and August; then return `module_stats` with the same length and order as that page. Module IDs alone cannot express Alice or August.

This is the bounded page-level query: work is constrained to module IDs on the returned page rather than every module matching the pre-pagination query. An unbounded implementation would aggregate the tenant's entire matching module catalog even though the UI renders one page. Bounded scope still requires measured query and page-load validation; it is not a latency claim. The Assignee and Date Range controls are why the two statistics filters remain in the contract. They narrow the caller's authorized result set and never grant authorization.

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
- Consumers that do not need reporting still inherit the expanded generated contract, although they pay no query cost when `include_stats` is false.
- Reporting field semantics are duplicated across the lesson and module requests rather than enforced by one shared message.

### Option 3 — Add dedicated lesson and module statistics APIs (selected 2026-08-28)

#### High-level implementation plan

1. Add `RetrieveTrainingSimulatorLessonStats` and `RetrieveTrainingSimulatorModuleStats`.
2. Give both APIs a required profile, 1–200 content names, and direct reporting time/user/group fields.
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
  repeated string stats_virtual_group_names = 5;
  repeated string stats_team_group_names = 6;
}

message RetrieveTrainingSimulatorLessonStatsResponse {
  repeated TrainingSimulatorLessonStats lesson_stats = 1;
}

message RetrieveTrainingSimulatorModuleStatsRequest {
  string parent = 1;
  repeated string training_module_names = 2;
  cresta.v1.common.time.TimestampRange stats_time_range = 3;
  repeated string stats_user_names = 4;
  repeated string stats_virtual_group_names = 5;
  repeated string stats_team_group_names = 6;
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
- Valid content with no matching assignments returns zero session/pass-fraction counts and absent score/rate values. `active_lesson_count` remains current content metadata.
- Request filters narrow results but never grant authorization.
- Each request's `stats_user_names`, `stats_virtual_group_names`, and `stats_team_group_names` form one user selection, which is intersected with the caller's authorized users.
- A selected group with no members returns zero matching assignments; it never becomes an all-user query.
- The selected option must define measurable latency and failure behavior before implementation.

## Storage

Add evaluation_status and not_applicable to director.training_simulator_conversation_scores. Extend the conversation-attempt update contract so EvaluateTrainingConversationResponse can carry both values through UpdateTrainingSimulatorTaskRun, then persist them in the same transaction as score, passed, and criterion results. No app.chats change, new result table, assignment snapshot, or revision-pinning change is part of this project.

No new datastore, region, third-party service, blob storage, or raw PII category is introduced. Reads remain in Director PostgreSQL and existing user service lookups.

## Security & privacy

Initial access matches Lesson Configuration: QA_ADMIN, ADMIN, and SUPER_ADMIN. Do not copy the existing endpoint’s AGENT role.

Request filters are not authorization. Add manager roles only after defining and testing a manageable-user resolver that prevents arbitrary user queries outside the hierarchy. Do not broaden canConfigureLessons, which would expose authoring routes.

The design changes neither data region nor retention. Reuse existing authorization and audit/telemetry. Security review is required for a manager-visible surface.

## Monitoring

Record RPC latency/status; task, assignment, run, conversation-score, requested content, and agent counts; missing result status, orphan run/reference, and invariant counts; DB latency per loader stage.

Client telemetry records omitted requested entries and failures without breaking authoring tables.

## SLO

Initial PostgreSQL target: p95 under 2 seconds for 200 content names, 2,000 assignments, and 10,000 runs. No silent truncation; invariant violations remain zero.

Dependency failures return clear errors while Director authoring remains usable with reporting unavailable. Oversized user sets fail with RESOURCE_EXHAUSTED, never partial success.

## Testing plans

**Backend:**

* zero assignments and zero-run assignments
* latest-attempt ordering, pass/fail reversal, and incomplete latest attempt
* explicit N/A, mixed applicable/N-A, partial timeout, and ambiguous legacy cases
* current-content behavior after lesson/module edits
* conversation results and migration fallback paths
* selected-user authorization, task lifecycle/date scope, invariants, and query fixtures at supported safety bounds

**Frontend:**

* score scaling, absent optionals, response order/indexing, and missing-result-status warnings
* selected-user loading and no-user-selected behavior
* stable keys, filter refetch, and no per-row fan-out
* real count cells, preserved authoring links, drawers/drill-through, loading/error/zero states
* feature/role gates, accessibility/localization, and end-to-end selected-user reconciliation

## Technical debts

**Bounded debt:**

* historical calculations use current lesson/module definitions and are not revision-exact
* ambiguous legacy results remain excluded from authoritative denominators
* lesson/module quiz aggregates are excluded until the feature and its reporting semantics mature; session completion continues to include required quiz outcomes
* initial aggregation is PostgreSQL/in-memory; revisit only if measured bounds require it
* manager reporting needs a dedicated route or split report/author permissions

No new Hasura/Node service or warehouse dependency is introduced.

## Cost estimate

Net-new infrastructure cost should be negligible because existing PostgreSQL, user service, protobuf, Go service, and Director infrastructure are reused. Measure query load during staging. Engineering sizing is TBD until product closes lifecycle/date semantics and schema/security review closes the conversation result-status contract.

## Implementation milestones

Treat session, lesson, and module reporting as three independently reviewable and releasable product milestones. They are not completely independent internally. They share one official-attempt primitive:

> For each assigned agent and required module, select the latest task run first, then classify the result attached to that run as settled, applicable, N/A, ambiguous, or incomplete. Never skip a newer unfinished retry to reuse an older completed result.

```text
Assignments + attempts + persisted results
                    |
          official module result
                    |
       +------------+------------+
       |            |            |
       v            v            v
    Session       Lesson       Module
   reporting     reporting     reporting
```

This shared primitive expands assignments before reading runs, so agents who never started remain visible. It selects the latest run for each `(task, lesson, module, agent)` fact using `created_at` plus resource ID, then classifies the selected result. Milestone 1 normalizes both conversation and quiz subtypes because whole-session completion spans every required module. Conversation classification uses persisted evaluation status and overall N/A; ambiguous legacy results stay out of authoritative score/pass denominators.

### Milestone 1 — Complete session-level reporting

Use the existing `RetrieveTrainingSimulatorTaskStats` API and shipped Director session UI to establish and validate the shared reporting model. The current main branch already implements the Figma dashboard, session table, header stats, status filters, agent identity/date/View rows, and a client-side zero-activity fallback. Milestone 1 preserves those surfaces rather than rebuilding them.

Scope:

* persist `evaluation_status` and overall `not_applicable` on conversation-score rows
* select the latest task run first, then classify its conversation or quiz result; a newer unfinished retry does not fall back to an older completed score
* load qualified, authorized active assignments and all matching runs directly, without the task-run list's 1,000-row limit
* include sessions and assigned agents that have no runs
* return each agent's passed, failed, incomplete, or all-N/A state
* return attempt count, score, latest activity time, identity/link inputs, and missing-result-status warning
* narrow Director changes to attempt count, explicit all-N/A/filter behavior, visible missing-status warnings, latest-activity naming, and corrected aggregate semantics
* enforce the chosen reporting personas and self/manageable-user row scope in the backend

Exit criteria:

* manually reconcile real examples for never started, partially complete, passed, failed, completed all-N/A, retry with latest complete, retry with latest pending/in-progress, quiz-containing sessions, and ambiguous legacy results
* explain and verify the complete data path from DirectorTask assignment through task runs and the selected conversation/quiz subtype result to one displayed agent row
* verify that agents/managers cannot read rows outside their allowed scope
* exercise the direct query above the old 1,000-run boundary and at explicit safety limits; record stage counts and measured latency
* verify Figma-visible values plus the newly defined all-N/A filter behavior and warning presentation with Design

Read-only `cresta/walter-dev` staging aggregates support this scope: 68 active tasks include 12 with no runs; 97 assigned-agent facts include 39 with no runs; 317 task runs include 277 conversation and 40 quiz runs; 48 agent/module groups have retries, with a maximum of 53 attempts; 18 conversation results have all criteria N/A but are stored as score `0`/passed `false`, and 10 of those are current latest results. The profile does not exceed 1,000 total runs, so it validates the cap structurally but does not replace a boundary-scale query fixture.

Milestone 1 decision gates:

1. Confirm that a newer unfinished retry makes the module/session incomplete rather than preserving the previous completed result. Recommendation: preserve current latest-attempt-first behavior.
2. Decide where completed all-N/A appears in the drawer filters. Recommendation: add a first-class N/A state/filter rather than hiding it under All.
3. Confirm that required quiz modules remain valid session content. Existing code and staging data say yes; if Product excludes them, enforce that upstream instead of silently dropping quiz attempts from reporting.
4. Choose reporting personas and manager scope; do not copy the current `AGENT` role without backend row authorization.
5. Keep Milestone 1 on active assignments unless Product explicitly chooses to broaden the shipped session page to archived history.

This milestone proves the assignment/result model before adding new reporting levels.

### Milestone 2 — Add lesson-level reporting

Reuse the session milestone's assignment expansion, result normalization, and latest-attempt selection. Group the resulting official module results by lesson assignment.

Scope:

* assuming option 2, add `include_stats`, direct reporting fields, and lesson results to `ListTrainingLessons`
* calculate only Sessions assigned, average score, pass rate, and per-module pass-rate chips
* add lesson table values and the lesson drawer
* document and test the known current-lesson-definition behavior after content edits

The module pass-rate chips require module calculations restricted to one lesson. They do not require the complete module-reporting product or criterion breakdown.

Exit criteria:

* reconcile lesson results against the underlying sessions and agent-module results
* demonstrate with a worked example how required module results become one lesson score and pass/fail result
* verify authorization, loading/error/warning behavior, accessibility, telemetry, and measured query/page latency for the lesson surface

### Milestone 3 — Add module-level reporting

Reuse the same official module results, now grouped by module across lessons, assignments, and agents.

Scope:

* assuming option 2, add `include_stats`, direct reporting fields, and module results to `ListTrainingModules`
* calculate only Active Lessons, average score, passed/failed fraction and pass rate, and criterion pass fractions
* add module table values, the module drawer, and criterion breakdown
* connect the lesson drawer's module drill-through to the module reporting surface

Exit criteria:

* reconcile module results with the same underlying agent-module results used by session and lesson reporting
* verify that criterion N/A results do not enter pass-rate denominators
* verify that a lesson drawer's module pass-rate chip agrees with the module calculation for the same filters and lesson scope
* verify authorization, loading/error/warning behavior, accessibility, telemetry, and measured query/page latency for the module surface

### Suggested ticket boundaries

Keep the implementation breakdown small:

1. Persist explicit conversation evaluation status and overall N/A.
2. Session reporting backend correctness: latest-attempt-first conversation/quiz normalization, authorized assignment expansion, uncapped reads, zero-run rows, and attempt count.
3. Session reporting frontend delta: attempt count, all-N/A/filter behavior, warnings, and latest-activity semantics.
4. Lesson reporting backend and API.
5. Lesson reporting frontend.
6. Module reporting backend and API.
7. Module reporting frontend.

Authorization, tests, observability, performance measurement, staging reconciliation, feature flags, and rollout checks are acceptance criteria within each milestone rather than separate late-stage tickets. Do not begin customer rollout for a milestone until its reconciliation and correctness checks pass.

This is dependency ordering, not a committed calendar. The selected dedicated APIs determine tickets 4–7's transport wiring without changing the shared reporting primitive or the three-milestone structure.

## Design review notes

### API review decision

* Option 3 is selected: dedicated lesson and module statistics APIs.
* One batched statistics request per active tab is acceptable; no per-row requests.
* Use cached content names and inactive-tab prefetch, then measure cold and cached loading behavior.

### Open gates

* Product: first route, ACTIVE/ARCHIVED scope, date meaning, metric set, latest-attempt-first policy, required quiz handling, and all-N/A display/filter membership
* Backend/data: conversation-score evaluation_status/not_applicable migration, conversation/quiz result normalization for sessions, query plan, current-content behavior, and legacy fallback
* Security: session-reporting personas and enforced self/manageable-user row scope
* FE/BE: score scale, optional/empty behavior, warnings, response order, 200-name limit, and measurable cold/cached tab-switch targets
* Design/QA: drawer interactions, criteria, accessibility, cross-grain reconciliation, and rollout fixtures
