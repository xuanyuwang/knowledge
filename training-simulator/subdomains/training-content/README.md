# Training Content

## Purpose

Training content configuration: how supervisors/training leads create and edit the structured content that drives simulations — lessons, modules, scenarios, evaluation criteria, and quizzes — and how it maps to backing virtual agents and scorecard templates.

## Scope and Boundaries

**In scope**

- `TrainingScenario`: customer situation backed by a Customer AI virtual agent (context, visitor objective, initial message, VA name + revision)
- `TrainingModule`: exactly one content type—an ordered scenario pool plus `EvaluationConfig`, or one pinned quiz-template revision
- `TrainingLesson`: ordered module composition + lesson focus criteria + use case + state (ACTIVE/ARCHIVED)
- `EvaluationConfig` / `EvaluationCriterion`: criteria, weights, `passing_score`, `allowed_number_attempts`, `maximum_conversation_turns`, `auto_fail`
- QuizTemplate: versioned quiz content (each save = new revision), with question snapshots and outcomes in `director.quiz_scores` / `director.quiz_question_scores`
- Content lifecycle: create/update/archive; scenario/module/lesson edits are append-only versioned; in-use sessions require assignment-time revision snapshots, but the current DirectorTask lesson-name contract leaves that as an active correctness gap (CONVI-7263)

**Shared parent truth**

- Assignment/session modeling, task runs, statuses: `assignment-and-session`
- VA proto shape, voice-agent pipeline construction: `simulation-runtime`
- How criteria become scores/outcomes: `evaluation`

**Out of scope**

- Generic scorecard template authoring/versioning mechanics: `scorecard-workflows/template-authoring-and-versioning`

## Semantics and Invariants

- A module carries exactly one content type: one-or-more scenarios or one quiz. The backend rejects both/neither; scenario modules require evaluation criteria in Director.
- For a scenario module, the FE (`pickRandomScenario` in `simulation/lessonUtils.ts`) picks **one scenario at random** from the pool when the agent starts; the pool is not run as a set.
- **Passing a scenario module requires passing a single scenario attempt**: one attempt = one conversation with one randomly-chosen scenario (a `TrainingSimulatorTaskRun`), scored against the module's `EvaluationConfig`. A passing score (or all-applicable-criteria pass when no `passing_score` is set) passes the module. You do **not** need to pass every scenario in the pool. `allowed_number_attempts` expresses the configured limit, but the current conversation pipeline treats enforcement as a future hook; each actual retry picks a possibly new random scenario.
- A module is the atomic training unit; a lesson is an ordered list of existing modules.
- Scenarios map to VA configs: changing a VA-affecting field or scenario state rematerializes VA revisions; unchanged VA inputs carry the prior revision forward (multi-call handling covers >1 scenario flows).
- Training VAs are `SINGLE_PROMPT_SUB_VA` with purpose `training_simulator`; they must be excluded from general VA lists.
- Evaluation criteria define both whether a criterion is met (`behavior_id` → moment) and its weight/auto_fail semantics.

## Architecture and Source Map

- **Frontend:** `director/packages/director-app/src/features/training-simulator/create-lesson`, `create-module`, `lessonConfigurationTab.ts`; `src/hooks/training-simulator/useCreateTrainingLesson.ts`, `useCreateTrainingModule.ts`, `useListTrainingModules.ts` etc.
- **APIs:** `TrainingSimulatorService` — `BatchCreate/BatchUpdateTrainingScenarios`, `CreateTrainingModule`, `ListTrainingModules`, `UpdateTrainingModule`, `CreateTrainingLesson`, `ListTrainingLessons`, `UpdateTrainingLesson` (protos in `cresta-proto/cresta/v1/trainingsimulator/training_simulator_service.proto`; entity protos `training_lesson.proto`, `training_module.proto`, `training_scenario.proto`, `quiz_template.proto`)
- **Backend/services:** `go-servers/apiserver/internal/trainingsimulator/` — `action_batch_create_training_scenarios.go`, `action_create_training_module.go`, `action_create_training_lesson.go`, `action_list_*.go`, `action_update_*.go`; `converter/` (goverter, `module.go` module factory)
- **VA creation:** `apiserver/internal/trainingsimulator/action_batch_create_training_scenarios.go` → `batchCreateAIAgents` creates `SINGLE_PROMPT_SUB_VA` revisions; `go-servers/bot-server/internal/virtualagent/validate.go` allows training-simulator batches to skip base umbrella VA via `isTrainingSimulatorBatch`
- **Storage/data sources:** append-only `director.training_lessons`, `director.training_modules`, and `director.training_scenarios`; `director.quiz_templates`/`quiz_questions`; attempt outcomes in `director.quiz_scores`/`quiz_question_scores`
- **Configuration/flags:** `enableTrainingSimulator` (LA gating); role-based gating (admin / QA_ADMIN / supervisor) as interim, migrating to permission-based access (CONVI-7145)

## Operational Knowledge

- `go generate`/converter coupling: new proto fields must have matching DB source or generation fails (e.g., `TrainingModule.QuizTemplateName`).
- Module preview flow (`EvaluateTrainingConversation` with `preview_training_module_name`) persists nothing and skips task-run lookup.
- Scenario edits rematerialize VA revisions — cost/time concerns with many scenarios (multi-call batching fix).

## Legacy Sources and Cases

- Design docs: Training Simulator Design (Jack Jee, gdrive/coda), Coaching/Training Simulator PRD, Training Simulator Design P1 Update (Quizzes + Upload Media)
- CONVI-6747 (FE scaffold), CONVI-6774 (create scenario API), CONVI-7010 (module creation/update >1 scenario), CONVI-7023 (usecase filtering), CONVI-7047 (module revisions), CONVI-7049

## Open Questions

- Align quiz pass semantics and the module threshold across task-run creation, session stats, and future lesson/module reporting; question weight/auto-fail controls are not yet persisted by the current contract.
- How/when training content is projected to analytics store for reporting scale (current JSONB arrays noted as tech debt; ClickHouse normalization contemplated).
