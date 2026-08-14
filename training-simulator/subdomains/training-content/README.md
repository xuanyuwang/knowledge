# Training Content

## Purpose

Training content configuration: how supervisors/training leads create and edit the structured content that drives simulations — lessons, modules, scenarios, evaluation criteria, and quizzes — and how it maps to backing virtual agents and scorecard templates.

## Scope and Boundaries

**In scope**

- `TrainingScenario`: customer situation backed by a Customer AI virtual agent (context, visitor objective, initial message, VA name + revision)
- `TrainingModule`: ordered scenario pool + `EvaluationConfig` + optional quiz template reference
- `TrainingLesson`: ordered module composition + lesson focus criteria + use case + state (ACTIVE/ARCHIVED)
- `EvaluationConfig` / `EvaluationCriterion`: criteria, weights, `passing_score`, `allowed_number_attempts`, `maximum_conversation_turns`, `auto_fail`
- QuizTemplate (P1: versioned quiz, each save = new revision; stored JSONB; responses in `training.quiz_responses`)
- Content lifecycle: create/update/archive; module/lesson edits are append-only versioned (CONVI-7047), contents editable at any time, in-use sessions use snapshot

**Shared parent truth**

- Assignment/session modeling, task runs, statuses: `assignment-and-session`
- VA proto shape, voice-agent pipeline construction: `simulation-runtime`
- How criteria become scores/outcomes: `evaluation`

**Out of scope**

- Generic scorecard template authoring/versioning mechanics: `scorecard-workflows/template-authoring-and-versioning`

## Semantics and Invariants

- The FE (`pickRandomScenario` in `simulation/lessonUtils.ts`) picks **one scenario at random** from the module's pool when the agent starts a module; the pool is not run as a set.
- **Passing a module requires passing a single scenario attempt**: one attempt = one conversation with one randomly-chosen scenario (a `TrainingSimulatorTaskRun`), scored against the module's `EvaluationConfig`. A passing score (or all-applicable-criteria pass when no `passing_score` is set) passes the module. You do **not** need to pass every scenario in the pool; a failed attempt can be retried up to `allowed_number_attempts`, each retry picking a (possibly new) random scenario.
- A module is the atomic training unit; a lesson is an ordered list of modules.
- Scenarios map to VA configs: changing a scenario triggers `BatchCreateVirtualAgentRevision` to rematerialize VA revisions (multi-call batching for >N scenarios, CONVI-7049/CONVI-7010 handling >1 scenario flows).
- Training VAs are `SINGLE_PROMPT_SUB_VA` with purpose `training_simulator`; they must be excluded from general VA lists.
- Evaluation criteria define both whether a criterion is met (`behavior_id` → moment) and its weight/auto_fail semantics.

## Architecture and Source Map

- **Frontend:** `director/packages/director-app/src/features/training-simulator/create-lesson`, `create-module`, `lessonConfigurationTab.ts`; `src/hooks/training-simulator/useCreateTrainingLesson.ts`, `useCreateTrainingModule.ts`, `useListTrainingModules.ts` etc.
- **APIs:** `TrainingSimulatorService` — `BatchCreate/BatchUpdateTrainingScenarios`, `CreateTrainingModule`, `ListTrainingModules`, `UpdateTrainingModule`, `CreateTrainingLesson`, `ListTrainingLessons`, `UpdateTrainingLesson` (protos in `cresta-proto/cresta/v1/trainingsimulator/training_simulator_service.proto`; entity protos `training_lesson.proto`, `training_module.proto`, `training_scenario.proto`, `quiz_template.proto`)
- **Backend/services:** `go-servers/apiserver/internal/trainingsimulator/` — `action_batch_create_training_scenarios.go`, `action_create_training_module.go`, `action_create_training_lesson.go`, `action_list_*.go`, `action_update_*.go`; `converter/` (goverter, `module.go` module factory)
- **VA creation:** `apiserver/internal/trainingsimulator/action_batch_create_training_scenarios.go` → `batchCreateAIAgents` creates `SINGLE_PROMPT_SUB_VA` revisions; `go-servers/bot-server/internal/virtualagent/validate.go` allows training-simulator batches to skip base umbrella VA via `isTrainingSimulatorBatch`
- **Storage/data sources:** `director.training_lessons` (training_module_ids, focus criteria), `director.training_modules` (evaluation JSONB, scenario refs, quiz fields), `director.training_scenarios`; quiz revisions/`training.quiz_responses`
- **Configuration/flags:** `enableTrainingSimulator` (LA gating); role-based gating (admin / QA_ADMIN / supervisor) as interim, migrating to permission-based access (CONVI-7145)

## Operational Knowledge

- `go generate`/converter coupling: new proto fields must have matching DB source or generation fails (e.g., `TrainingModule.QuizTemplateName`).
- Module preview flow (`EvaluateTrainingConversation` with `preview_training_module_name`) persists nothing and skips task-run lookup.
- Scenario edits rematerialize VA revisions — cost/time concerns with many scenarios (multi-call batching fix).

## Legacy Sources and Cases

- Design docs: Training Simulator Design (Jack Jee, gdrive/coda), Coaching/Training Simulator PRD, Training Simulator Design P1 Update (Quizzes + Upload Media)
- CONVI-6747 (FE scaffold), CONVI-6774 (create scenario API), CONVI-7010 (module creation/update >1 scenario), CONVI-7023 (usecase filtering), CONVI-7047 (module revisions), CONVI-7049

## Open Questions

- Quiz reward/impact on module score and interaction with conversation scoring (P1 partially defined in design).
- How/when training content is projected to analytics store for reporting scale (current JSONB arrays noted as tech debt; ClickHouse normalization contemplated).
