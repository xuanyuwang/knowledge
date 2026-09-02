# CONVI-7583 Backend Implementation Plan

**Date:** 2026-08-31  
**Status:** Ready for implementation after proto field-number review  
**Primary repos:** `cresta-proto`, `go-servers`  
**Dependent frontend ticket:** CONVI-7584

## Outcome

Make `RetrieveTrainingSimulatorTaskStats` an assignment-rooted, authorization-safe session-reporting API that returns every qualified active session and assigned agent, counts all attempts, selects one deterministic official attempt per required module, handles conversation and quiz outcomes consistently, and never silently truncates at 1,000 task runs.

CONVI-7582 persistence fields are not prerequisites. Under the accepted 2026-08-27 decision, a persisted conversation outcome with both score and pass present is settled; zero/false is failure-equivalent whether it originated from timeout, all-criteria-N/A, or an applicable failure. A row with either score or pass absent remains incomplete and is excluded from score/pass calculations.

## Exact data contract

### New response data

Add exactly one public data field:

```protobuf
message AgentPerformanceEntry {
  // Existing fields 1-7 remain unchanged.

  // Number of conversation and quiz task runs for this assigned agent
  // within the session. Counted before official-attempt selection.
  int64 attempt_count = 8 [(google.api.field_behavior) = OPTIONAL];
}
```

`attempt_count` is per `(director_task, assigned_agent)` session row. It counts every conversation or quiz task run whose task and subtype-derived agent match the assignment, including incomplete retries and stale/non-required modules, before reducing retries to the official attempt.

### Existing response data to populate correctly

No new fields are required for these values:

| Output | Required behavior |
|---|---|
| `tasks_stats[]` | One entry per qualified active DirectorTask, including tasks with no task runs. Preserve task-list order. |
| `agent_performance[]` | One entry per user in each matched task's full stored audience, including never-started users. |
| `task_runs[]` | Latest task run for each module that has a run for the agent, including stale modules. Exclude older attempts and preserve enough enrichment for existing consumers. |
| `status` | `COMPLETE` only when every current required module has a settled official conversation or quiz result; otherwise `INCOMPLETE`. |
| `passed` | True only when the agent is complete and every required module passed. Zero/false conversation outcomes are failure-equivalent. |
| `score` | Mean of official required-module scores for a complete agent, normalized to the existing 0–1 contract. Use `0` when unavailable. |
| `not_applicable` | True when the agent score is unavailable and, at task level, when no authorized assigned agent completed. This remains the availability bit; do not use negative score sentinels. |
| `TaskStats.score` | Mean of completed agent-session scores, 0–1. |
| `TaskStats.pass_rate` | Passed completed agents divided by completed agents; `0` plus `not_applicable = true` when no agent completed. |
| task/lesson metadata | Preserve names, titles, focus criteria, modules, start time, and due time. |

### Backend source data that must be loaded

| Source | Fields needed | Purpose |
|---|---|---|
| Active Training Simulator `DirectorTask` | task name/ID, display name, create time, due time, lesson names, audience user names | Establish qualified sessions and assignment facts before reading attempts. |
| Current `TrainingLesson` definitions | lesson name/title, required module names/order, focus criteria, module resources | Define the modules used for completion, score, and pass calculations. This retains the known current-content limitation. |
| Authorized user population | user resource name/ID and display name | Intersect requested users/groups/direct-team selection with manageable users before returning rows. |
| `training_simulator_task_runs` | resource ID, task ID, lesson ID, module ID, conversation score ID, quiz score ID, created time, revision IDs needed by the existing task-run response | Count attempts and select the official attempt by `(created_at, resource_id)`. |
| `training_simulator_conversation_scores` | score ID, agent user ID, score, passed, and existing task-run enrichment fields | Identify the agent and classify/score conversation attempts. |
| `quiz_scores` plus module threshold | score ID, submitter user ID, score, submitted time, quiz identity; current module passing score | Identify the agent and classify/score quiz attempts. Do not load quiz question-result rows unless required to preserve the existing returned task-run payload. |

## Result classification

Select the official attempt before inspecting its result:

1. Group matching runs by `(task, lesson, required module, subtype-derived agent)`.
2. Choose greatest `created_at`; break equal timestamps by greatest task-run resource ID.
3. Classify that chosen attempt.

| Official attempt | Settled? | Score/pass behavior |
|---|---:|---|
| No run | No | Module and session incomplete. |
| Conversation with score present | Yes | Use normalized 0–1 score and persisted passed value; missing passed remains false. |
| Conversation missing score | No | Module and session incomplete. |
| Quiz with submitted time | Yes | Use the submitted score and derived pass value; missing pass remains false. |
| Quiz missing submission | No | Module and session incomplete. |

A newer incomplete retry supersedes an older completed attempt and temporarily makes the module/session incomplete. No older-result fallback is allowed.

## Proto decision

A proto change is required:

1. Add `AgentPerformanceEntry.attempt_count = 8` in `stats.proto`, following the existing message's `OPTIONAL` field-behavior convention.
2. Retain the existing `AGENT` role on `RetrieveTrainingSimulatorTaskStats`. The agent-facing Assigned Training Sessions flow calls this RPC for the assignee's progress, status, and score. Backend authorization must restrict agent-only callers to their own row while managers receive only manageable-user rows.
3. Clarify existing score and availability comments if necessary, without renumbering or changing fields 1–7.

No result-state enum, warning flag, latest-activity field, schema column, table, or migration is required. Existing `status`, `passed`, `score`, and `not_applicable` encode incomplete, passed, failure-equivalent, and unavailable aggregate states. Director can continue deriving latest activity from the returned official task runs until CONVI-7584 decides whether a dedicated convenience field is worthwhile.

## Authorization contract

- Resolve the caller's authorized user population before loading/returning assignment rows, using the shared user-filter/ACL path and the existing auth/config dependencies on `ServiceImpl`: self only for agent-only callers, manageable users for managers, and the permitted customer/profile population for administrators.
- Apply `user_names`, `group_names`, and `direct_team_only` as selection inside that authorized population. The current handler ignores `direct_team_only`; implementation must honor it.
- Pass requested users to `ListDirectorTasks` as task selectors. Aggregate every matched task over its full stored audience; do not project the audience down to the users that caused the task to match.
- Treat collection/count authorization as query/filter injection, not post-fetch rejection.
- Validate every requested user, group, lesson, task-derived lesson, and parent resource against the request customer/profile before use.
- Server principals may retain service-to-service behavior, but user principals must never receive rows outside their manageable scope.

## Data-loading design

Keep four stages separate:

1. `resolveStatsUsers`: validate the request, resolve groups/direct-team behavior, apply ACL/manageable-user scope, and produce an ordered allowed-user set plus display metadata.
2. `loadSessionAssignments`: list active Training Simulator tasks, filter lessons, load current lesson definitions, and expand ordered `(task, agent)` facts even when no runs exist.
3. `loadSessionAttempts`: reuse the task-run List parsing and enrichment through an internal method accepting a page size. Pass `ceiling + 1` for reporting so overflow returns `ResourceExhausted` rather than partial data. Start with the existing composite task/lesson/module index and add an index only if `EXPLAIN ANALYZE` shows it is needed.
4. `aggregateSessionStats`: pure deterministic code that counts matching runs, selects official attempts, classifies results, and builds ordered protobufs.

The reporting row ceiling should be explicit and testable: cap one request at 100,000 task runs and request 100,001 through the shared internal List method as an overflow probe.

## Implementation sequence

### 1. Protobuf contract

- Edit `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/trainingsimulator/stats.proto` to add `attempt_count`.
- Leave `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/trainingsimulator/training_simulator_service.proto` authorization unchanged; `AGENT` is required by the agent-facing Assigned Training Sessions consumer.
- Run formatter, Buf lint/build, targeted Bazel proto build, Gazelle if dependencies change, and API lint where available. Do not commit generated code.

### 2. Backend model and loader refactor

- Refactor `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/trainingsimulator/action_retrieve_training_simulator_stats.go` so assignment loading, attempt loading, classification, and aggregation are separate.
- Add small same-package files if needed, for example `training_simulator_stats_loader.go` and `training_simulator_stats_aggregation.go`; avoid a new package unless a real boundary appears.
- Extract `ListTrainingSimulatorTaskRuns` into a shared internal method accepting `pageSize`.
- Call it from `fetchTaskRunsForTasks` with 100,001 rows and reuse the existing conversion/enrichment path.

### 3. Authorization and request filtering

- Integrate the shared manageable-user/ACL filter before the DirectorTask call.
- Implement `direct_team_only`.
- Ensure task audience expansion consistently uses the matched task's full stored audience.
- Add request validation for cross-customer/profile names and preserve empty authorized populations as a successful empty response.

### 4. Pure aggregation

- Build assignment facts first, then attach matching attempts.
- Count attempts before reduction.
- Select by `(created_at, resource_id)` deterministically.
- Normalize conversation and quiz outcomes to one internal result type.
- Count all session attempts and return the latest run per module; use only current required modules for completion, score, and pass calculations.
- Return score `0` with `not_applicable = true` for unavailable agent/task aggregates; remove the `-1` sentinel.
- Preserve stable task, agent, and required-module ordering.

### 5. Telemetry

- Rely on the framework envelope for total RPC latency/availability.
- Add one structured, non-PII summary per request with counts and stage durations for qualified tasks, assignment facts, task-run rows, conversation outcomes, quiz outcomes, incomplete/malformed official results, and aggregation duration.
- Record safety-bound rejection distinctly. Do not log user names, group names, task names, or score payloads.

### 6. Tests and validation

Add or correct tests for:

- zero tasks; zero-run task; never-started agent alongside started agents;
- all-attempt count across conversation and quiz modules, including stale modules;
- latest completed retry, latest incomplete retry, and equal-time resource-ID tie break;
- passed, failed/failure-equivalent zero/false, missing-score, missing-pass, submitted quiz, incomplete quiz, and mixed conversation/quiz sessions;
- normalized 0–1 scoring for both subtypes; update existing fixtures that incorrectly seed conversation scores as 0–100;
- correct agent and task score/pass denominators and unavailable values;
- requested users/groups as task selectors, full-audience aggregation for matched tasks, `direct_team_only`, manager manageable-user filtering, admin/root access, agent self-only behavior, attempts to request another agent, and cross-tenant resource names;
- 1,001+ task runs proving removal of the old truncation; exactly-at-bound and over-bound behavior proving explicit `ResourceExhausted` rather than partial output;
- stable task/agent/module ordering and no older-result fallback.

Run focused Go tests, `gofmt -w -s`, Gazelle, targeted Bazel/Go build and tests, and lint. Reconcile representative staging sessions and capture query plans/counts/latency before requesting review.

## PR structure

1. `cresta-proto`: additive attempt-count field only; retain the existing RPC role annotation.
2. `go-servers`: loader/authorization/aggregation implementation and tests, based on the generated proto dependency.

CONVI-7584 consumes `attempt_count` and handles the focused Director presentation changes after the backend contract is available.

## Explicit non-goals

- Persisting evaluation status or overall N/A.
- New database schema or indexes without measured query evidence.
- Assignment-time content/audience snapshots.
- Archived-session reporting; preserve the current active-task scope.
- Lesson/module rollup APIs.
- CSV export or rebuilding the existing Training Sessions UI.
