# Assignment and Session

## Purpose

How training lessons become agent work: one DirectorTask-based assignment can target multiple explicit users, while each agent's session state is derived from that shared task plus their per-module `TrainingSimulatorTaskRun` attempts.

## Scope and Boundaries

**In scope**

- Director Task modeling for training (`DIRECTOR_TASK_TYPE_TRAINING_SIMULATOR`), audience/content/schedule configs
- Single-agent and multi-agent assignment flows; explicit audience membership and mutation
- `TrainingSimulatorTaskRun` (one conversation = one module attempt) linking conversation → task → lesson → module → scenario → agent, plus evaluation results
- Session/task statuses and completion semantics (NOT_STARTED / IN_PROGRESS / PASSED / FAILED / OVERDUE)
- Task listing/filters (by conversation, task, agent, module, lesson) and time-range filter (CONVI-7110)

**Shared parent truth**

- Content entities (lesson/module/scenario) being assigned: `training-content`
- Runtime that produces the conversations/task runs: `simulation-runtime`
- How scores/pass are populated on task runs: `evaluation`

**Out of scope**

- General DirectorTask/Coaching Plan abstractions (reused, not redefined); QA leaderboard/scorecards assignment semantics: `analytics`, `scorecard-workflows`
- Notification delivery for training task notifications: `notifications`

## Semantics and Invariants

- **The DirectorTask is the shared assignment/supervisor session, not the lesson.** It has type `DIRECTOR_TASK_TYPE_TRAINING_SIMULATOR`, an explicit list of audience user names, lesson content config, and due time. The lesson remains reusable content.
- **One DirectorTask can target multiple users.** An agent session is the `(task, agent)` projection joined with that agent's attempts; it is not a separate per-agent DirectorTask.
- **Agents see and launch their assigned sessions from the agent Coaching surface** (`coaching-workflow/agent-coaching/assigned-training-sessions/`); manager/coach surfaces live in Training Simulator / Coaching Hub.
- Current Director creates one lesson per task even though the proto field is repeated and backend contracts can round-trip multiple names; runtime reads the first lesson, so one lesson per assignment is the current product invariant.
- Each agent/module attempt creates a `TrainingSimulatorTaskRun`; retries create additional runs under the same task/module/agent identity.
- Director locks later modules behind the first not-yet-passed module; passed modules remain sticky/unlocked.
- Agent completion: an agent is COMPLETE only when all required modules are completed; PASSED requires completing all required modules **and** all passed.
- Training conversations (`conversation_source = TRAINING_SIMULATOR`) are excluded from agent progression/live analytics.

## Architecture and Source Map

- **Frontend:** `director/.../features/training-simulator/assign-training-session-modal`, `simulation/attemptStatus.ts`, `hooks/useTrainingSessions.ts`, `useCreateTrainingSimulatorTaskRun.ts`, `useUpdateTrainingSimulatorTaskRun.ts`, `useListTrainingSimulatorTaskRuns.ts`
- **APIs:** `TrainingSimulatorService` — `Create/Update/ListTrainingSimulatorTaskRun`, `ListTrainingSimulatorTaskRuns` (filter fields), `ListDirectorTasks` training filter (CONVI-7110); proto `training_simulator_task_run.proto`; DirectorTask extensions in `cresta-proto/cresta/v1/coaching/task.proto` (audience/content/schedule configs)
- **Backend/services:** `go-servers/apiserver/internal/trainingsimulator/` — `action_create/update/update_training_simulator_task_run.go`, `action_list_training_simulator_task_runs.go`; DirectorTask service (`task_audience_config.training_simulator_audience_config`, etc.)
- **Storage/data sources:** `director.training_simulator_task_runs` (conversation_name, director_task_name, lesson/module/scenario names, agent_name, evaluation_score/passed, criterion_results, timestamps)
- **Configuration/flags:** `enableTrainingSimulator`, role gates (CONVI-6991 `useTrainingSimulatorAccess`, CONVI-7241 agent-only filter), permission-based access (CONVI-7145)

## Operational Knowledge

- Task runs are created for each attempt; latest-attempt selection and nil-score guards prevent stale/unevaluated runs from skewing stats.
- Status computation depends on lesson-required modules; changing lesson composition affects in-flight sessions.
- Training task audience is stored as explicit user names. Removing one user updates the shared task; removing the final user archives it.
- Task persistence precedes best-effort notification delivery, so an assignment can exist even if notification creation fails.
- The current stats handler omits an assignment when it has zero runs; agent UI compensates by joining task rows with optional stats, but aggregate reporting remains a correctness gap.

## Legacy Sources and Cases

- Coaching Simulator Assignment Flow Design Doc (Kurt Choi) — full domain model, task schema, server implementation layout (`apiserver/internal/coaching/training/`)
- CONVI-6747 (page scaffold/filters), CONVI-7110 (ListDirectorTasks time_range + training filter), CONVI-7241 (agent-only filter, backend audience fix go-servers#29852), CONVI-7145 (permission access controls)

## Open Questions

- Assignment-time lesson/module revision snapshots and the behavior of in-flight tasks after content edits (CONVI-7263).
- Audience-history semantics for add/remove after attempts and historical reporting cohorts.
- Server-side retry enforcement for `allowed_number_attempts` and its interaction with admin/overdue overrides.
