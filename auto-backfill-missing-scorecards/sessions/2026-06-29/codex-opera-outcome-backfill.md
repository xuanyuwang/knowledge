# Codex Session: Opera Outcome Scorecard Backfill

Date: 2026-06-29

Source repo: `/Users/xuanyu.wang/repos/go-servers`

Branch/worktree: main checkout at `/Users/xuanyu.wang/repos/go-servers`

## User Question

Investigate whether a one-time cron job/script can backfill a scorecard template over the last 90 days when that template uses outcomes related to Opera rules. The user suspects simply enabling the feature flag globally for the backfill may not be enough and is considering a one-time script that enables the feature flag only in the cron job/script execution.

Questions:

1. Is it possible to write a one-time cron job?
2. How complex is it?

## Investigation Plan

- Search internal docs for Time Machine, feature Time Machine, Opera, Opera rules, and outcomes.
- Search `go-servers` for scorecard backfill, Time Machine, scorecard template evaluation, feature flag wiring, and Opera/outcome code paths.
- Determine whether existing cron/script patterns support one-time or bounded backfill execution.
- Summarize feasibility, complexity, risks, and recommended implementation shape.

## Findings

- Internal docs distinguish two Time Machine paths:
  - Performance Config Time Machine backfills scorecards over a date range, but only where Opera has already generated behavior annotations.
  - Opera Time Machine backfills Opera rule evaluation on historical conversations, and can generate scorecards/performance scores when behaviors are attached to the template at backfill time. The UI path is documented as capped at 500 conversations per rule.
- `go-servers` already has a scorecard-only backfill:
  - `apiserver/internal/coaching/action_backfill_scorecard_templates.go`
  - `temporal/ingestion/backfillscorecards/*`
  - This path reads existing moment annotations via `AutoQA.GetConversationAutoScoringContext` and then calls `AutoQA.CalculateScoredItems`. It does not itself run Opera policy/rule evaluation.
- `go-servers` also has an Opera annotations + scorecards chained path:
  - `apiserver/internal/policy/action_backfill_policies.go` creates `JOB_TYPE_DOWNSTREAM_BACKFILL_JOB`.
  - The downstream job includes `BackfillAnnotationsPayload{EnableOpera: true, TriggeredByOperaTimeMachine: true}` and optionally `BackfillScorecardsPayload`.
  - `temporal/ingestion/downstreambackfill` finds conversations by `StartJobFromConversationsFilter`, then runs child annotation and scorecard jobs.
  - `temporal/ingestion/backfillsteps/step_backfill_annotations.go` maps the child payload to `BackfillOperaPoliciesPayload`.
  - `apiserver/internal/internaljob/jobhandler/backfill_opera_policies_handler.go` schedules the Opera policy backfill workflow.
- LLM/outcome wiring:
  - `config/src/CustomerConfig.ts` describes `enableLLMEvaluationMoment` as a Director/Opera UI feature flag for LLM Evaluation block components.
  - Backend policy search emits `LlmEvaluationConfig` when saved policy/moment config contains LLM evaluation definitions; I did not find a backend check for the exact frontend `enableLLMEvaluationMoment` flag in the execution path.
  - Orchestrator runs the LLM evaluator path when `LlmEvaluationConfig` has definitions and the worker has `LLMEvaluatorNode`, then persists `LLM_EVALUATION` moments and feeds positive outcomes into policy evaluation.
  - User clarified the relevant Opera rule evaluates the whole transcript. This maps to `LlmEvaluationConfig.Definition.ConversationSegment=FULL_CONVERSATION`; backend `selectMessages` returns all conversation messages for that segment.
  - Policy engine code treats `evaluation_outcome` in `llm_evaluation_payload.llm_output` as the resolved outcome for LLM-eval behavior adherence.
- Job scheduling:
  - `DownstreamBackfillHandler` implements `PubliclySchedulableJobHandler` and supports cron schedule payload timestamp shifting.
  - For a true one-time execution, creating a one-off internal job is simpler than adding a recurring cron schedule. A temporary cron/task-runner wrapper is possible, but it should self-disable or be removed immediately after the run.
- Risk:
  - A scorecard-only backfill is insufficient if the missing historical data is the Opera/LLM outcome annotations.
  - A backfill-only feature override is not trivial unless the needed switch is already represented in the backfill payload. The existing payload can force `EnableOpera`, skip conversation outcome/reason, include entity/sentiment, etc.; it does not carry a generic "pretend config flag is enabled" override for frontend flags.
  - If the saved policy/template config already contains LLM Evaluation moments, the backend likely does not need `enableLLMEvaluationMoment`; it needs the backfill workers and model/LLM services deployed and configured.
  - Existing UI/PR evidence notes parallel scorecard backfills can create ES/CH load pressure and race/data issues, so run only one backfill at a time and start with a small dry-run/limited window.

## Visibility Follow-up

User raised a concern that backfilling Opera annotations while `enableLLMEvaluationMoment` is off may still create customer-visible moment annotations.

Findings:

- `enableLLMEvaluationMoment` is defined in `/Users/xuanyu.wang/repos/config/src/CustomerConfig.ts` as a Director/UI flag for LLM Evaluation block components. Usages found in Director are in Opera authoring/editing flows, not the Closed Conversations read path.
- Closed Conversations annotation loading in `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/conversations/hooks/annotations/useConversationMomentAnnotations.ts` explicitly queries `MomentType.LLM_EVALUATION` and separately queries unspecified-adherence LLM evaluation annotations, then merges LLM output onto adherence annotations.
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/conversations/hooks/annotations/extractBehaviorAnnotations.ts` extracts evidence and explanations from `MomentType.LLM_EVALUATION` payloads for conversation annotation rendering.
- Backend `ListMomentAnnotations` in `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/moment/action_list_moment_annotations.go` filters by customer/profile/conversation/type/adherence/etc. and excludes archived/deleted behaviors, but does not check `enableLLMEvaluationMoment` or policy role visibility.
- Director applies policy role visibility after loading annotations via `/Users/xuanyu.wang/repos/director/packages/director-app/src/hooks/useAnnotationsFilteredByCurrentUserRole.ts`; behavior annotations are rendered only if their moment template belongs to a triggered evaluated policy visible to the current user. This depends on `policyConfig.rolesWithVisibility`, not `enableLLMEvaluationMoment`.
- Internal docs for Opera Time Machine state that running Time Machine from Opera makes the rule appear on older conversations in Closed Conversations and, if attached to a Performance Template, updates scorecards and Performance Insights. Docs for Finalize Opera Rule state Platform Visibility controls whether annotations/behaviors are hidden in Closed Conversations, scorecards, and template criteria surfaces.

Updated risk assessment:

- The user's concern is valid. `enableLLMEvaluationMoment` should not be treated as a read-side visibility/privacy boundary.
- Persisting backfilled LLM evaluation annotations is likely customer-visible when the associated Opera rule/policy is visible to that user's role.
- If the rule is restricted through Platform Visibility / `rolesWithVisibility`, Director should hide the rendered behavior annotations for users without those roles, but the backend annotation list endpoint itself does not enforce that filtering. That means Platform Visibility may be adequate for product UI, but it is not the same as preventing the data from being returned by the annotations API.
- Safer options are: make the rule Cresta-only / restricted before backfill and verify all customer-facing surfaces; add read-side/backend gating for LLM evaluation annotations; or build a backfill mode that computes scorecards without persisting customer-visible annotations.

## Preliminary Answer

Yes, a one-time job is possible. The safest implementation is probably not a new raw cron, but a one-off `DOWNSTREAM_BACKFILL_JOB` using the existing Opera annotation backfill plus scorecard backfill steps. Complexity is low if only scorecards need recalculation from existing annotations; medium if Opera annotations/outcomes must be generated first; medium-high if a true backfill-only config/feature override has to be added through proto, job payload, policy backfill preparation, and orchestrator input construction.
