# Reporting

## Purpose

Multi-level reporting for Training Simulator: session (assignment) stats today, plus designed lesson- and module-level diagnostics so coaches can judge content quality as well as assignment completion. CSV appears in design exploration but is not committed scope.

## Scope and Boundaries

**In scope**

- `RetrieveTrainingSimulatorTaskStats` API (per-task stats, per-agent entries, scores, pass rates) — **shipped**
- Aggregates: session overview, agent overview, completion stats (assigned/completed counts, average score, pass rate, duration)
- Filters: time_range, training lessons, direct-team-only, users, groups
- Consumed by Training Simulator session status view + dashboards
- **Designed / pending implementation scope:** lesson-level and module-level rollups and criterion pass fractions
- **Exploratory only:** CSV export (no authoritative commitment/spec/date found)

**Shared parent truth**

- Attempt identity and revision pins live on task runs; conversation outcomes now live in `training_simulator_conversation_scores`, while quiz outcomes live in `quiz_scores`: `evaluation`
- Task run / session model definition: `assignment-and-session`
- Content metadata surfaced (lesson title, focus criteria): `training-content`

**Out of scope**

- Generic analytics platform APIs, group/team hierarchy resolution, shared user filters: `analytics`
- General dashboards/product analytics infra
- Linking training completion to live production KPI uplift (separate customer FR)

## Semantics and Invariants

- Stats start from matching DirectorTask assignments and join task runs to their conversation/quiz subtype outcomes; the old score/pass/criterion columns on task runs are a migration fallback.
- An agent is scored only from **lesson-required modules** (task runs for non-required/stale modules excluded from completion & scoring, though returned in task-runs list).
- An agent **passes** only if they completed all required modules AND all modules passed (`AgentCompletionStatus.COMPLETE/INCOMPLETE`, `passed` on `AgentPerformanceEntry`).
- Lesson score is the simple average of required module scores; the latest module attempt is the official reporting score.
- Score normalization: overall scores are 0–1 in task stats proto (TaskStats `score`, AgentPerformanceEntry `score`); module eval scores are 0–100.
- N/A (`not_applicable`) values are excluded from aggregate scores; all-N/A denominator behavior must be explicit. Nil scores are skipped to avoid skewing average/rate.

## Architecture and Source Map

- **Frontend:** `director/.../features/training-simulator/TrainingSimulator.tsx` (Training Sessions tab), `TrainingSimulatorTabs.tsx`, `hooks/training-simulator/useTrainingSimulatorTaskStats.ts`, `agentScore.ts`
- **APIs:** `TrainingSimulatorService.RetrieveTrainingSimulatorTaskStats` (proto `stats.proto`, `training_simulator_service.proto`); response `tasks_stats[]` with `TaskStats` (task/lesson refs, focus_criteria, start/due times, score, not_applicable, pass_rate, agent_performance[], training_modules)
- **Backend/services:** `go-servers/apiserver/internal/trainingsimulator/action_retrieve_training_simulator_stats.go` (+ Nil handling fix #29784), `action_list_training_simulator_task_runs.go`; reporting stats build CONVI-7020 (#28636)
- **Configuration/flags:** `enableTrainingSimulator` / `enableTrainingSimulatorV2`; content-level aggregate APIs must use explicit roles and manageable-user authorization rather than trusting request filters.

## Operational Knowledge

- Large sessions/date ranges: ensure performant rollups (load test expectations); listing task runs at scale.
- Stale or unevaluated runs can distort metrics; filter by lesson-required modules and skip nil scores.
- Content-level aggregation must start from assignments (not runs) to retain never-started agents, and must use assignment-time revision snapshots once CONVI-7263 lands.
- Reporter answers "what do the numbers on the Training Simulator page mean" — trace each metric to its task-run/lesson/module source.

## Legacy Sources and Cases

- Training Simulator Design "Reporting"/"Reporting Stats API" (design proto for GetTrainingSimulatorStats) — superseded by merged `RetrieveTrainingSimulatorTaskStats`
- CONVI-7020 (reporting stats rpc build), #29745 (empty taskModules fix), #29784 (nil score panic), #29693 (removed COMPLETE guard, always compute overall score)

## Open Questions

- Confirm that current latest-attempt scoring applies unchanged to cross-session lesson/module rollups; define separate retry/first-pass/improvement metrics.
- Whether/when training data is projected to analytics stores (ClickHouse) for large dashboards.
- Lesson/module cohort definition, Insights vs Lesson Configuration placement, revision-safe history, randomized-scenario normalization, and whether CSV is committed — tracked in the deliverable.

## Active Work

- Requirements brief: [`../../deliverables/lesson-module-statistics-reporting.md`](../../deliverables/lesson-module-statistics-reporting.md)
- Backend design: [`../../deliverables/lesson-module-statistics-eng-design.md`](../../deliverables/lesson-module-statistics-eng-design.md)
- Frontend design: [`../../deliverables/lesson-module-statistics-fe-design.md`](../../deliverables/lesson-module-statistics-fe-design.md)
- Work item: [`../../work-items/lesson-module-statistics-reporting.md`](../../work-items/lesson-module-statistics-reporting.md)
- Design: [Figma node 13108:21741](https://www.figma.com/design/B5tJUlNnKbbjVfxH44nqNl/Training-Simulator--Coaching-Simulator-?node-id=13108-21741)
- Correctness prerequisite: [CONVI-7263 assignment revision snapshots](https://linear.app/cresta/issue/CONVI-7263/update-training-simulator-session-to-take-snapshot-of-revisions-from)
