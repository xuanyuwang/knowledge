# Assignment and Session

## Purpose

How training lessons become per-agent work: the DirectorTask-based assignment model, audience expansion, the per-attempt `TrainingSimulatorTaskRun`, session/task status, and the link into Coaching Plan.

## Scope and Boundaries

**In scope**

- Director Task modeling for training (`DIRECTOR_TASK_TYPE_TRAINING_SIMULATOR`), audience/content/schedule configs
- Bulk vs individual assignment flows; audience expansion to concrete agent IDs; coaching-plan validation
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

- **The DirectorTask is the session/assignment, not the lesson.** A **session** is a DirectorTask of type `DIRECTOR_TASK_TYPE_TRAINING_SIMULATOR`; its content config references the **lesson** (`training_lesson_names`), so one lesson can be referenced by many per-agent tasks. The lesson itself is content in `training_lessons`, never a task.
- **Agents see and launch their assigned sessions from the Coaching Plan** (`coaching-workflow/agent-coaching/assigned-training-sessions/` "Assigned Training Sessions"; manager/coach surfaces in Training Simulator / Coaching Hub). The DirectorTask carries audience config (per-agent user resource names) + schedule config (`due_time`).
- Bulk assignment expands groups/teams (and optional individuals) into per-agent tasks, validates coaching plans per agent, and stores coaching-plan context in task metadata. Multi-plan linking supported.
- One agent execution of a lesson = session; each module attempt creates a `TrainingSimulatorTaskRun` (one conversation per run).
- Trainees must pass all modules within a lesson in order (module ordering enforced).
- Agent completion: an agent is COMPLETE only when all required modules are completed; PASSED requires completing all required modules **and** all passed.
- Training conversations (`conversation_source = TRAINING_SIMULATOR`) are excluded from agent progression/live analytics.

## Architecture and Source Map

- **Frontend:** `director/.../features/training-simulator/assign-training-session-modal`, `simulation/attemptStatus.ts`, `hooks/useTrainingSessions.ts`, `useCreateTrainingSimulatorTaskRun.ts`, `useUpdateTrainingSimulatorTaskRun.ts`, `useListTrainingSimulatorTaskRuns.ts`
- **APIs:** `TrainingSimulatorService` — `Create/Update/ListTrainingSimulatorTaskRun`, `ListTrainingSimulatorTaskRuns` (filter fields), `ListDirectorTasks` training filter (CONVI-7110); proto `training_simulator_task_run.proto`; DirectorTask extensions in `cresta-proto/cresta/v1/coaching/task.proto` (audience/content/schedule configs)
- **Backend/services:** `go-servers/apiserver/internal/trainingsimulator/` — `action_create/update/update_training_simulator_task_run.go`, `action_list_training_simulator_task_runs.go`; DirectorTask service (`task_audience_config.training_simulator_audience_config`, etc.)
- **Storage/data sources:** `director.training_simulator_task_runs` (conversation_name, director_task_name, lesson/module/scenario names, agent_name, evaluation_score/passed, criterion_results, timestamps)
- **Configuration/flags:** `enableTrainingSimulator`, role gates (CONVI-6991 `useTrainingSimulatorAccess`, CONVI-7241 agent-only filter), permission-based access (CONVI-7145)

## Operational Knowledge

- Task runs are created for each attempt; stale runs without evaluation score must not skew stats (nil-score guard in `RetrieveTrainingSimulatorTaskStats`, CONVI-7146).
- Status computation depends on lesson-required modules; changing lesson composition affects in-flight sessions.
- Audience expansion happens at assignment time for groups/teams.

## Legacy Sources and Cases

- Coaching Simulator Assignment Flow Design Doc (Kurt Choi) — full domain model, task schema, server implementation layout (`apiserver/internal/coaching/training/`)
- CONVI-6747 (page scaffold/filters), CONVI-7110 (ListDirectorTasks time_range + training filter), CONVI-7241 (agent-only filter, backend audience fix go-servers#29852), CONVI-7145 (permission access controls)

## Open Questions

- Behavior of session/task status when a lesson's modules change after assignment (snapshot vs refetch).
- Retry semantics across allowed-number-of-attempts at the task-run and session level.
