# Evaluation

## Purpose

How an already-closed simulator conversation becomes a module outcome: project Opera moment annotations into criterion pass/fail/N/A results, compute a weighted 0–100 score and authoritative pass, then persist the normalized result on the task run.

## Scope and Boundaries

**In scope**

- `EvaluateTrainingConversation` module/task-run resolution and module-preview override
- Behavior annotation applicability and adherence mapping
- `PENDING` / `IN_PROGRESS` / `COMPLETE` evaluation status
- Criterion N/A, pass/fail, auto-fail, evidence, weights, and module threshold
- Director polling, timeout, score normalization, and verdict save/retry
- Outcome persistence on conversation task-run score records

**Shared parent truth**

- Criteria and pass configuration: `training-content`
- Conversation/task-run identity: `simulation-runtime`, `assignment-and-session`
- Aggregate interpretation: `reporting`

**Out of scope**

- Opera rule execution and whether a behavior uses deterministic or LLM-backed detection
- Generic AutoQA/scorecard scoring; the current Training Simulator evaluator does not call that engine
- Quiz scoring, which is a separate task-run subtype and execution path

## Semantics and Invariants

- The evaluator is stateless: each call reads the moment annotations available now and returns a snapshot. Director polls every two seconds; the tracked frontend has a 60-second guard.
- Normal evaluation resolves a module through the task run linked to the conversation. Module preview supplies the module directly and persists nothing.
- `SHOULD_DO_X` makes a behavior applicable. Applicable criteria pass only with at least one `DID_DO_X` and no `DID_NOT_DO_X`.
- No annotations means pending. Annotations without `SHOULD_DO_X` mean genuine N/A. Both use criterion `not_applicable=true`; overall response status disambiguates them only while the evaluation is live.
- Evidence includes `DID_DO_X` / `DID_NOT_DO_X` moment and message references; opportunity-only `SHOULD_DO_X` annotations are not shown as evidence.
- Score is `100 × passed applicable positive weight / total applicable positive weight`. Pending and N/A criteria are excluded.
- A failed applicable auto-fail criterion forces failure. Otherwise a positive threshold uses `score >= passing_score`; threshold zero means all applicable criteria must pass.
- Evaluation response and module threshold use 0–100. Persisted task-run and stats scores use 0–1. Director performs the conversion.
- Persisted `evaluation_passed` is authoritative. Do not recompute pass from score because that loses auto-fail and threshold-zero behavior.
- Evaluation calculation and task-run persistence are separate. A save failure can retry the write without replaying the conversation.

## Architecture and Source Map

- **Frontend:** `director/.../hooks/training-simulator/useEvaluateTrainingConversation.ts`; `training-conversation/useModuleEvaluationPipeline.ts`; `simulation/api/evaluationTimeout.ts`; `moduleResults.ts`
- **API:** `TrainingSimulatorService.EvaluateTrainingConversation`; `evaluation.proto`; task-run update RPC
- **Backend:** `go-servers/apiserver/internal/trainingsimulator/action_evaluate_training_conversation.go`; `action_update_training_simulator_task_run.go`
- **Upstream dependency:** Opera/moment annotation generation and `MomentService.ListMomentAnnotations`
- **Storage:** conversation score extension (`score`, `passed`, criterion-results JSON) linked from the base task run

## Current Correctness Gaps

- Overall response `not_applicable` is not persisted on the task run. All-N/A becomes normalized score `0` plus passed `false`, and current reporting treats the non-nil score as meaningful completion.
- Pending and genuine N/A share the same criterion encoding. A timeout-persisted partial snapshot permanently loses the distinction.
- The timeout path can persist score/pass calculated from only the annotations that arrived, including a partial pass.
- The evaluator reads the current module by stable name, not the module revision captured on the run.
- The proto requires a Training Simulator conversation source, but the handler does not fetch or enforce conversation source.
- The evaluator assumes upstream annotations preserve training-specific Opera policy applicability; post-hoc filtering cannot undo incorrectly generated annotations.

## Operational Knowledge

- A missing normal task run or missing module returns `NotFound`; moment-service errors fail the evaluation call.
- A failed evaluation poll currently asks the agent to repeat the conversation. A failed task-run update retains the verdict in frontend memory and offers retry-save.
- Module-preview polling and real-run polling share the timeout policy, but preview has no persistence path.
- Partial score/pass fields are numerically populated but contractually meaningful only at `COMPLETE`, except for the current timeout behavior.

## Legacy Sources and Cases

- Older design documents described direct LLM evaluation, AutoQA scorecard scoring, backend polling, and conversation-metadata persistence. Those are not the current handler behavior.
- CONVI-6926 (`EvaluateTrainingConversation`), CONVI-7146 (module preview), CONVI-7281 (Opera product-area applicability).

## Open Questions

- Explicit durable outcome enum/state that preserves pending, passed, failed, N/A, and technical failure.
- Server-owned completion/finalization instead of persisting a client-selected timeout snapshot.
- Assignment/run-time module revision resolution for historical scoring.
- Explicit source/tenant/entity validation at the evaluator boundary.
- Unified conversation/quiz module-outcome semantics.
