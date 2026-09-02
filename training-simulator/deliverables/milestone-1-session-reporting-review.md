# Milestone 1 Review: Session-Level Reporting

> **Superseded requirement (2026-08-27):** The review's recommendation to persist overall evaluation status/N/A and display all-N/A separately is no longer current. Product accepted failure-equivalent reporting for the listed zero/false incomplete-timeout, completed all-criteria-N/A, and completed-failure cases. Criterion-level N/A remains excluded from scoring. See [the decision record](../decisions/2026-08-27-collapse-overall-zero-false-results.md).
>
> **Current frontend boundary (2026-08-31):** Figma and current Director confirm two visible deltas: show unique assigned agents with at least one incomplete visible session, and show authoritative per-agent attempt count in the drawer. The first is frontend-derivable; only `AgentPerformanceEntry.attempt_count` needs a new response field. The drawer already renders per-agent pass/fail status plus score. Treat contradictory scope-item recommendations below as historical review context.

**Review date:** 2026-08-24  
**Scope:** Milestone 1 only. Lesson/module reporting is excluded except where required to determine session completion.  
**Verdict:** Keep the milestone, but revise its contract before ticketing. The persistence, uncapped-read, all-assignee, attempt-count, and result-state work is justified. The proposal overstates the frontend work, under-specifies authorization and N/A presentation, and would regress session correctness if its conversation-only primitive excludes quiz results or if “latest settled result” falls back past a newer unfinished retry.

## Current system and data flow

```text
DirectorTask (one shared assignment)
  audience user IDs + current lesson reference + schedule
                    |
                    | one agent can create many module attempts
                    v
training_simulator_task_runs
  task + lesson/module revision IDs + conversation_score_id or quiz_score_id
                    |
              +-----+-----+
              |           |
              v           v
conversation_scores     quiz_scores
agent + score/pass +    submitter + score/submission
criterion results
              \           /
               \         /
                v       v
RetrieveTrainingSimulatorTaskStats
  ListDirectorTasks -> current lessons/modules -> ListTaskRuns (max 1,000)
  -> latest run per task/lesson/module/agent -> completion/score/pass
                    |
                    v
Director Training Sessions
  separately lists assignments, joins stats by task name, and supplies a
  zero-activity fallback before rendering dashboard/table/review drawer
```

The assignment is the `DirectorTask`; there is no separate per-agent session row. A displayed agent row is a projection of `(director_task, assigned_user)` plus that user's module attempts and results. Conversation attempts store identity on the task-run row and outcome/agent data on `training_simulator_conversation_scores`. Quiz attempts use the same task-run identity but obtain the agent and outcome from `quiz_scores`.

The current stats handler starts with active DirectorTasks, but then returns no task stats when the filtered run set is empty and groups output only from tasks represented in the run set. For tasks with runs, it expands the complete audience and can already return a never-started agent alongside an agent with runs. It reloads current lesson modules, obtains at most 1,000 task runs through `ListTrainingSimulatorTaskRuns`, selects the latest run per `(task, lesson, module, agent)`, and treats a conversation result as complete when `score` is non-null.

Director already implements the Figma session dashboard, session table, and review drawer. It lists assignments independently, joins task stats by `taskName`, and synthesizes incomplete rows when stats are absent. This masks the backend's zero-run omission in the current page but does not make the API complete or reusable.

## Evidence

### Figma

The Reporting frame shows:

- dashboard: total sessions, assigned agents, completed, overdue, average score, pass rate, and score distribution;
- session table: completion fraction, assigned agents, average score, pass rate, duration, and Review;
- session drawer: header score/pass rate/due date, lesson context, All/Passed/Failed/Incomplete filters, and one agent row with identity, date, View, attempt count, status, and score.

The current Director implementation already covers everything above except attempt count and trustworthy all-N/A/ambiguous-result presentation. Figma does not define where a completed all-N/A row belongs in the four filters, nor how a missing-result-status warning appears.

### Staging data

Read-only aggregate queries against `configv3/staging/cresta/walter-dev/config.yaml` (`voice-staging` / `walter-dev`) found:

| Evidence | Count |
|---|---:|
| Training Simulator tasks | 71 |
| Active tasks | 68 |
| Active tasks with no runs | 12 |
| Active tasks with runs | 56 |
| Assigned-agent facts on active tasks | 97 |
| Assigned-agent facts with no run | 39 |
| Task runs | 317 |
| Conversation runs | 277 |
| Quiz runs | 40 |
| Agent/module groups with retries | 48 |
| Maximum attempts for one agent/module | 53 |
| Conversation scores with null score and null pass | 110 |
| Conversation scores with score and pass | 167 |
| Rows whose recorded criteria are all N/A but are stored as score `0`, passed `false` | 18 |
| Latest conversation results whose recorded criteria are all N/A and appear failed | 10 |
| Latest conversation results with no score/criteria | 13 |

This data directly validates zero-run coverage, attempt count, explicit evaluation status, and overall N/A persistence. It also disproves the assumption that session reporting can safely exclude quiz runs: quiz attempts are 40 of 317 observed runs. The 1,000-row cap is not hit by this profile's current total, so staging validates the truncation risk structurally rather than empirically.

## Scope-item review

| Proposed Milestone 1 item | Review | Required correction |
|---|---|---|
| Persist `evaluation_status` and overall `not_applicable` | **Keep; release blocker.** Current timeout persistence writes score `0`, pass `false`, and N/A-looking criteria without the evaluator status, while genuine all-N/A rows have the same stored shape. | Persist both values atomically with score/pass/criteria. Define legacy fallback narrowly: null score/pass with no criteria is incomplete; score/pass with all criteria N/A is still ambiguous without explicit overall status/N/A. |
| Load assignments and runs directly without the 1,000-row task-run-list cap | **Keep, but separate correctness from performance.** The current handler inherits the hard cap. | Root the read in filtered, authorized assignments; batch direct task-run/result reads; fail explicitly at safety bounds; measure query stages. Do not claim staging already demonstrates >1,000-run behavior. |
| Include sessions and assigned agents with no runs | **Keep for API correctness; current UI impact is partially masked.** Director already joins the independent assignment list and renders zero-activity fallback rows. | Return one `TaskStats` per qualified task and one agent entry per assigned user so API consumers and reconciliation do not depend on a second client-side source. Preserve Director's defensive fallback. |
| Return passed, failed, incomplete, and all-N/A | **Keep, but the current proto/UI model is insufficiently specified.** Existing fields can encode complete all-N/A, but Director maps every `COMPLETE && !passed` to failed and `AttemptStatus` has only three states. | Define all-N/A as a first-class display state or explicitly map it to a named filter category. Figma's four filters do not answer this. Do not count all-N/A as failed or incomplete; exclude it from score/pass denominators. |
| Return attempt count | **Keep.** Current `task_runs` contains only latest-per-module runs, so Director cannot derive historical attempt count. Retries are common in staging. | Count all matching runs before latest selection. Add `attempt_count`; do not return all historical runs merely to support the count. Decide whether quiz attempts count—session semantics say yes for required quiz modules. |
| Return score, latest time, identity/link inputs | **Mostly already implemented.** Score, agent identity, latest time from returned runs, task/agent resource names, and route construction exist. | Treat these as preservation/verification work, not net-new feature scope. Rename the frontend's `completedAt`; it is latest activity time, including incomplete attempts. |
| Return missing-result-status warning | **Keep, but define granularity and UI.** | Per-agent warning is useful for row truth; also define task-level aggregation for the table/dashboard and a concrete Director presentation. A silent boolean that never reaches the dashboard is insufficient. |
| Update the Director session side panel | **Narrow.** The current main branch already implements the Figma drawer, status filters, agent identity/date/View, and header stats. | Limit net-new UI to attempt count, all-N/A state/filter behavior, missing-status warning, and any corrected aggregate semantics. Add accessibility/loading/error tests around those changes. |
| Verify authorization | **Keep and make concrete.** The RPC allows `AGENT`; the handler trusts requested user/group filters, and `ListDirectorTasks` does not scope rows to the requester's manageable users. | Choose the reporting personas. If the surface is manager/admin-only, remove `AGENT`; otherwise enforce self/manageable-user row scope in the backend. Never rely on Director filters for authorization. |
| Shared “latest settled result” primitive | **Rewrite before implementation.** The phrase is ambiguous and can mean falling back to an older completed result when a newer retry is pending. Current code chooses the latest attempt first and then tests whether it is scored. | Contract should be: select the latest attempt by `(created_at, resource_id)` for each assignment/module/agent, then classify that attempt as settled/applicable/N/A/incomplete. Do not skip a newer unfinished retry unless Product explicitly chooses last-completed semantics. |
| Conversation-only result primitive | **Reject for session reporting.** Current session stats include quiz attempts; staging has 40 quiz runs. | Use one result union that can classify required conversation and quiz modules, or explicitly remove quizzes from assignable session content before shipping. Conversation-only aggregation cannot truthfully determine whole-session completion. |

## Corrected Milestone 1 boundary

### Backend and contract

1. Persist conversation `evaluation_status` and overall `not_applicable` through the existing update transaction.
2. Define one official-attempt function: latest attempt first, deterministic resource-ID tie break second, result-state classification third.
3. Support both conversation and quiz attempt subtypes when evaluating required session modules.
4. Start from authorized active assignments and expanded audience facts; return tasks/agents with zero runs.
5. Read all qualified attempts/results directly in bounded batches; calculate all-run `attempt_count` before reducing to the official attempt.
6. Preserve current task/agent fields and add `attempt_count` plus an explicit missing-status warning. Define optional/absent score and pass-rate behavior rather than relying only on scalar `0` plus `not_applicable`.
7. Make authorization persona and row scope explicit; add stage-level counts/latency telemetry.

### Director

1. Preserve the shipped dashboard/table/drawer and the zero-activity defensive join.
2. Add attempt count to the agent row.
3. Add a defined complete-all-N/A presentation and decide its filter membership with Design.
4. Surface ambiguous legacy-result warnings at the row and affected aggregate level.
5. Rename latest activity semantics and keep View routing based on task/agent resource names.

### Exit criteria

- Reconcile real examples for never started, partial, passed, failed, completed all-N/A, retry with latest complete, retry with latest pending/in-progress, quiz-containing sessions, and ambiguous legacy results.
- Confirm the same agent/session answer from assignment audience through official attempts to the rendered row.
- Verify unauthorized agents/managers cannot obtain rows outside their allowed scope.
- Exercise the direct query above the old 1,000-run boundary and at explicit safety limits; record query-stage latency and counts.
- Verify Figma-visible values plus the unresolved all-N/A and warning states with Design.

## Decision gates before ticketing

1. Does a newer unfinished retry temporarily make the module/session incomplete, or does the previous completed result remain official? Recommendation: preserve current latest-attempt-first semantics.
2. Where does completed all-N/A appear in the drawer filters? Recommendation: add an N/A filter/state rather than hiding it under All.
3. Are quiz modules valid required content for this milestone? Existing code and staging say yes; if Product says no, enforce that upstream rather than silently excluding quiz attempts from reporting.
4. Which personas may call session reporting, and what manager scope is allowed?
5. Are archived assignments intentionally excluded from this milestone? The current API and current page use active tasks only; do not broaden lifecycle scope incidentally.

## Source references

- Figma Reporting frame: `Training Simulator (Coaching Simulator)`, node `13108:21741`, visually inspected 2026-08-24.
- Current stats contract: `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/trainingsimulator/stats.proto`.
- Current stats handler: `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/trainingsimulator/action_retrieve_training_simulator_stats.go`.
- Current run/result persistence: `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/trainingsimulator/action_update_training_simulator_task_run.go` and `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/training-simulator/training-conversation/useModuleEvaluationPipeline.ts`.
- Current Director session reporting: `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/training-simulator/tabs/training-sessions/`.
- Staging aggregate evidence: read-only customer app DB queries for `cresta/walter-dev`; no user-level data was retained.
