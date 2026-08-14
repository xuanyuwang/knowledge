# Evaluation

## Purpose

How a simulator conversation (one module attempt) is scored: Opera moment detection, optional LLM evaluation, AutoQA scorecard scoring, module pass/fail with auto-fail gating, and how results land on `TrainingSimulatorTaskRun` / conversation metadata.

## Scope and Boundaries

**In scope**

- Basic moment detection (keyword, entity, sentiment, emotion) and composite moments via Opera rules/DAG
- Optional LLM-based criteria (OpenAI-styled response: outcome, confidence, evidence)
- AutoQA scoring against ScorecardTemplate, weighted score (0–100), `passing_score` threshold, auto-fail criteria
- `EvaluateTrainingConversation` RPC: validation, polling for annotations, scoring, persistence, preview path
- Evaluation status (PENDING/IN_PROGRESS/COMPLETE), criterion results, N/A handling, evidence mapping `CriterionEvidence`
- Module completion → evaluation trigger

**Shared parent truth**

- Criteria defined on the module: `training-content`
- Task run that receives the score: `assignment-and-session`
- Rollups consumed by reporting: `reporting`

**Out of scope**

- General AutoQA/Opera engine internals and scorecard template business rules: `scorecard-workflows/evaluation-and-scoring`
- Analytics display of scores: `analytics`

## Semantics and Invariants

- Module completion event triggers the Evaluate API → evaluates the conversation matching module + session.
- Evaluation is asynchronous: annotations take time to be ready; the RPC returns immediately with current `EvaluationStatus`; FE **polls** (backend polls moment annotations up to a 60s timeout).
- Weighted overall score is computed (0–100) by grouping annotations by behavior; **auto-fail** on any auto-fail criterion forces module failure; pass/fail determined by `passing_score` or all-criteria threshold.
- `CriterionEvaluationResult`: criterion_id, behavior_name, passed, weight, auto_failed, `CriterionEvidence` (moment annotation + message + adherence type), not_applicable.
- N/A: when the score is not applicable (e.g., timeout / "NA" scores), `score`/`passed` are not meaningful; design notes "NA scores due to timeouts" handling.
- Score/passed are persisted to the task run (`evaluation_score` 0.0–1.0, `evaluation_passed`, criterion_results) and to conversation metadata.
- Opera rule product-area applicability must be enforced upstream when orchestrator selects policies for a conversation. The Training Simulator evaluator only consumes the resulting moment annotations, so filtering criteria after evaluation would be too late (CONVI-7281).

## Architecture and Source Map

- **Frontend:** `director/.../hooks/training-simulator/useEvaluateTrainingConversation.ts`, `useModuleResults.ts`/`moduleResults.ts` (feedback tab, per-module results/polling)
- **APIs:** `TrainingSimulatorService.EvaluateTrainingConversation` (proto `training_simulator_service.proto`; evaluation proto `evaluation.proto`); request takes `conversation_name` (must be TRAINING_SIMULATOR source) and optional `preview_training_module_name` (module-preview path, persists nothing); response includes score, passed, criterion_results, status, not_applicable
- **Backend/services:** `go-servers/apiserver/internal/trainingsimulator/action_evaluate_training_conversation.go` — validates conversation, fetches module, polls for moment annotations (60s), fetches annotations w/ pagination, groups by behavior, computes weighted score + auto-fail, persists to conversation metadata/task run; `action_create_training_simulator_task_run.go`/`action_update_training_simulator_task_run.go` store results
- **Dependencies:** Opera (moment evaluation), AutoQA/Xai (AutoScoredItems), LLM providers; moment annotation store; scorecard templates
- **Storage/data sources:** behavior/moment annotations, AutoScoredItems, task-run criterion_results JSONB, conversation metadata

## Operational Knowledge

- If annotations are not ready, do not block; return PENDING/IN_PROGRESS and let FE poll (60s cap).
- Hide `SHOULD_DO_X` evidence (CONVI evidence filter, #29650); stale unevaluated runs must not affect stats.
- Cost: LLM tokens for evaluation (external LLM dependency); monitor evaluation latency vs SLO (99% module results ≤1min).

## Legacy Sources and Cases

- Training Simulator Design "Evaluation flow" Lucidchart + gdrive/coda sections (Opera DAG, LLM eval, AutoQA scoring)
- CONVI-6926 (EvaluateTrainingConversation rpc), CONVI-7146 (module preview)

## Open Questions

- Quiz scoring interaction with conversation criterion scoring (module score composition with quiz).
- When "NA" applies precisely (timeout vs all-criteria-N/A) and its effect on module/session pass.
