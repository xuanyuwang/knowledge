# Reporting

## Purpose

Session-level reporting for Training Simulator: aggregated stats per training task, per-agent performance, completion/pass-rate rollups, and the surfaces that consume them (Training Simulator page, Coaching Plan, dashboards).

## Scope and Boundaries

**In scope**

- `RetrieveTrainingSimulatorTaskStats` API (per-task stats, per-agent entries, scores, pass rates)
- Aggregates: session overview, agent overview, completion stats (assigned/completed counts, average score, pass rate, duration)
- Filters: time_range, training lessons, direct-team-only, users, groups
- Consumed by Training Simulator session status view + dashboards

**Shared parent truth**

- Scoring/pass data originates in evaluation on task runs: `evaluation`
- Task run / session model definition: `assignment-and-session`
- Content metadata surfaced (lesson title, focus criteria): `training-content`

**Out of scope**

- Generic analytics platform APIs, group/team hierarchy resolution, shared user filters: `analytics`
- General dashboards/product analytics infra

## Semantics and Invariants

- Stats are computed from `TrainingSimulatorTaskRun`s scoped to the matching director tasks/lessons.
- An agent is scored only from **lesson-required modules** (task runs for non-required/stale modules excluded from completion & scoring, though returned in task-runs list).
- An agent **passes** only if they completed all required modules AND all modules passed (`AgentCompletionStatus.COMPLETE/INCOMPLETE`, `passed` on `AgentPerformanceEntry`).
- Score normalization: overall scores are 0–1 in task stats proto (TaskStats `score`, AgentPerformanceEntry `score`); module eval scores are 0–100.
- N/A (`not_applicable`) marks make score/pass not meaningful and must be handled in rollups; nil scores are skipped to avoid skewing average/rate (stats nil-score guard).

## Architecture and Source Map

- **Frontend:** `director/.../features/training-simulator/TrainingSimulator.tsx` (Training Sessions tab), `TrainingSimulatorTabs.tsx`, `hooks/training-simulator/useTrainingSimulatorTaskStats.ts`, `agentScore.ts`
- **APIs:** `TrainingSimulatorService.RetrieveTrainingSimulatorTaskStats` (proto `stats.proto`, `training_simulator_service.proto`); response `tasks_stats[]` with `TaskStats` (task/lesson refs, focus_criteria, start/due times, score, not_applicable, pass_rate, agent_performance[], training_modules)
- **Backend/services:** `go-servers/apiserver/internal/trainingsimulator/action_retrieve_training_simulator_stats.go` (+ Nil handling fix #29784), `action_list_training_simulator_task_runs.go`; reporting stats build CONVI-7020 (#28636)
- **Configuration/flags:** `enableTrainingSimulator`; role/permission gating on stats access (agents → their own; managers → team)

## Operational Knowledge

- Large sessions/date ranges: ensure performant rollups (load test expectations); listing task runs at scale.
- Stale or unevaluated runs can distort metrics; filter by lesson-required modules and skip nil scores.
- Reporter answers "what do the numbers on the Training Simulator page mean" — trace each metric to its task-run/lesson/module source.

## Legacy Sources and Cases

- Training Simulator Design "Reporting"/"Reporting Stats API" (design proto for GetTrainingSimulatorStats) — superseded by merged `RetrieveTrainingSimulatorTaskStats`
- CONVI-7020 (reporting stats rpc build), #29745 (empty taskModules fix), #29784 (nil score panic), #29693 (removed COMPLETE guard, always compute overall score)

## Open Questions

- Session completion %, avg score, pass rate definitions across attempts (latest vs best) for agents with multiple task runs.
- Whether/when training data is projected to analytics stores (ClickHouse) for large dashboards.
