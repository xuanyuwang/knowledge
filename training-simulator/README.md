# Training Simulator Domain

## Purpose

Canonical engineering knowledge home for **Training Simulator**: Cresta's agentic-AI practice environment where agents run scenario-based role-play against AI-simulated customers instead of manual 1:1 role-play, and are graded automatically against the same rules (Opera / AutoQA scorecard criteria) used for live quality review.

Training Simulator is a **paid add-on to QM & Coach** that launched July 9, 2026 as a limited beta (Mutual of Omaha, Snap Finance), expanding to more beta customers early Q3 2026, targeting GA late Q3 2026.

## Scope and Boundaries

**In scope**

- Training content model and configuration: lessons, modules, scenarios, evaluation criteria (LLM or Opera), quizzes
- Assignment and session model: bulk/individual assignment of lessons to agents, modeled as Director Tasks with a training task type, per-agent task runs
- Simulation runtime: Customer AI virtual agents, voice-agent/LiveKit pipeline, role mapping, conversation creation with `TRAINING_SIMULATOR` source
- Evaluation: Opera basic/composite moment detection, optional LLM evaluation, module/session scoring, pass/fail, auto-fail, N/A handling
- Reporting: session-level stats, per-agent results, completion/pass-rate rollups, Training Simulator page
- Frontend and backend contracts and invariants for the above

**Out of scope**

- Generic QA/Coaching Plan scorecard evaluation semantics when not about simulator attempts: `scorecard-workflows`
- Analytics-platform display and aggregation mechanics (shared analytics APIs, grouping, user filters): `analytics`
- PG→ClickHouse projection of training data if/when that exists: `scorecard-data-sync`
- Notification delivery mechanics for training task notifications: `notifications`

## Current State

- **Launched Jul 9, 2026** as Limited Availability paid add-on (`enableTrainingSimulator` feature flag for beta access gating).
- Backend lives in `go-servers/apiserver/internal/trainingsimulator/` (service impl + action handlers). Proto surface in `cresta-proto/cresta/v1/trainingsimulator/`.
- FE lives in `director/packages/director-app/src/features/training-simulator/` (Training Simulator page with Training Sessions + Lesson Configuration tabs) plus `hooks/training-simulator/`.
- Bounded by a **minimum vertical slice**: creation → assignment → simulation → evaluation → reporting, reusing Director / Coaching infrastructure (DirectorTask, Coaching Plan, Voice-agent, Opera/AutoQA).

## Subdomains

- [Training Content](subdomains/training-content/README.md) — lessons, modules, scenarios, evaluation config (criteria/passing/attempts), quiz templates
- [Assignment and Session](subdomains/assignment-and-session/README.md) — DirectorTask modeling, audience expansion, task runs, statuses (NOT_STARTED/IN_PROGRESS/PASSED/FAILED/OVERDUE)
- [Simulation Runtime](subdomains/simulation-runtime/README.md) — Customer AI virtual agents, voice-agent/LiveKit pipelping, GoWalter role/channel mapping, simulator conversation creation
- [Evaluation](subdomains/evaluation/README.md) — Opera/LLM evaluation, moment annotations, scoring, pass/fail and auto-fail, N/A and evidence
- [Reporting](subdomains/reporting/README.md) — session/agent/task stats APIs, CompletionStats/pass-rate/rollups, dashboards

## Key Semantics and Invariants

- **Lesson** = ordered collection of modules with shared training focus; treated as a single **Session** when assigned to an agent.
- **Module** = atomic training unit; binds scenario pool, optional quiz, and evaluation criteria + scorecard template; one scenario chosen at random per run.
- **Scenario** = a Customer AI virtual-agent configuration (a customer situation); changes to a scenario rematerialize VA revisions.
- **Session** = one agent's execution of a lesson; aggregates module scores/outcomes into session result.
- One **conversation = one module attempt**; each attempt is a `TrainingSimulatorTaskRun`.
- Training conversations use `ConversationSource.TRAINING_SIMULATOR` and must **not** be included in agent progression/live analytics.
- Customers should be able to self-serve lesson/module/config via UI; RBAC separates supervisors (create/assign/view results) from agents (see/run only their own sessions).

## Architecture Overview

```
supervisor FE --> TrainingSimulatorService (go-servers apiserver)
  |-- trainingScenarios / trainingModules / trainingLessons  (CRUD, JSONB config)
  |-- trainingSimulatorTaskRuns                              (a conversation ↔ task/lesson/module attempt)
  |-- evaluateTraining                                       (Opera + optional LLM scoring)
  |-- trainingSimulatorTaskStats                             (Reporting)
agent FE ---> LiveKit room join ---> voice-agent (python) ---> Pipecat pipeline
                 |                     |--VirtualAgentService.GetVirtualAgentRevision (Customer AI VA)
                 |                     `--CrestaVALLMService (turn LLM); TTS/STT vendors
                 `--> GoWalter (Go) transcription/persistence: creates app.chat conversation,
                      maps [AGENT, VISITOR] channels to roles (CHANNEL_SWAP for training)
Storage: director.training_lessons/modules/scenarios(+quiz), app.chat conversations w/ metadata,
         moment annotations, director.training_simulator_task_runs, AutoQA AutoScoredItems.
```

## Operational Knowledge

- **SLOs (design):** assignment visibility ~real time (99.5%), runtime stable VA connection (99.5%), module eval ≤1min (99%), session results within a few minutes (99%).
- **Evaluation latency:** `EvaluateTrainingConversation` returns immediately with a status; FE must poll. Annotations take time to become ready (60s poll timeout in backend).
- **N/A handling:** conversations that time out / have no scoring signal can be marked `not_applicable`; score and `passed` are then not meaningful (affects reporting rollups).
- **Stale runs:** `RetrieveTrainingSimulatorTaskStats` must exclude stale/nil-score runs and constrain to lesson-required modules or metrics skew (CONVI-7146/nil-panic fixes).
- **Roles:** training VAs are `SINGLE_PROMPT_SUB_VA` with purpose `training_simulator`; must be filtered out of any existing VA lists (CONVI items).
- **go generate coupling:** proto changes ripple into go-servers converters (`mage RegenerateProto` / `go generate`); converter fields like `QuizTemplateName` have caused `go generate` failures when DB source field lags.

## Current Objective

Maintain a canonical, implementation-accurate engineering map of Training Simulator as it moves toward GA, and use it as the home for ongoing CONVI training work items (permission hardening CONVI-7145, quiz P1, Synthetic Customers, GA readiness).

## Key Findings (seeded from Glean docs, 2026-08-09)

- Training Simulator reuses DirectorTask + Coaching Plan + Voice-agent + Opera/AutoQA rather than new infrastructure.
- Bulk assignment expands audiences into per-agent DirectorTasks with coaching-plan context in task metadata.
- Voice-agent plays the **customer** (Customer AI simulator); the human plays the **agent** — a role/channel swap vs normal calls, done in GoWalter.
- Evaluation reuses the existing Opera DAG (keyword, entity, sentiment, emotion → composite moments) plus optional LLM criteria, then AutoQA scoring against a scorecard template; weighted score (0–100), auto-fail gating, `passing_score` threshold.
- Design-doc protos are **not final**; the merged `cresta-proto` service surface differs (batch scenario CRUD, `EvaluateTrainingConversation` with polling, `RetrieveTrainingSimulatorTaskStats`, `TrainingSimulatorTaskRun`).

## Source Context

- **Primary repo:** `go-servers`
  - `apiserver/internal/trainingsimulator/` — service impl + action handlers + converter
  - `bot-server/internal/virtualagent/` — VA validation for training simulator batches
  - `voice-integration/gowalter/` — role/channel mapping, conversation creation
  - `apiserver/sql-schema/` — `director.training_*` tables, `training_simulator_task_runs`
- **Related repos:**
  - `cresta-proto` — `cresta/v1/trainingsimulator/` (service, entity, stats, evaluation, quiz protos); `cresta/v1/coaching/task.proto` (DirectorTask extensions); conversation `TRAINING_SIMULATOR` source
  - `director` — `packages/director-app/src/features/training-simulator`, `src/hooks/training-simulator`
  - `python-ai-services` — `voice-agent/src/processors/gowalter.py` (channel → role)

## Related Artifacts

- `project.yaml`
- `log/2026-08-09.md`
- `sessions/2026-08-09/claude-training-simulator-domain-setup.md`
- `subdomains/<name>/README.md`
- `work-items/` as CONVI tickets are tracked
- `decisions/`, `deliverables/` as content is synthesized
