# Approval-Gated Scorecard Auto-Heal Design

## Context

- **Date/timezone:** 2026-09-20, America/Toronto
- **Primary domain:** `scorecard-data-sync`
- **Source repo:** `/Users/xuanyu.wang/repos/go-servers`
- **Source context:** local `main` at `b2bc3849854d124ed659058f8a8de6813262ddc6`, read-only; 149 commits behind `origin/main` when inspected
- **Knowledge checkout:** `/Users/xuanyu.wang/repos/knowledge`, main checkout
- **Question:** Can every detected repair become a Temporal workflow, with over-threshold repairs paused until manager approval?

## Answer

Yes, Temporal supports a durable human approval wait. The safe implementation is not to create all 5,000-scorecard reindex workflows and pause them. Create one profile-level repair-request workflow, wait there when approval is required, and fan out bounded execution jobs only after approval and candidate revalidation.

## Current Behavior

- `scorecard-sync-monitor` returns before job creation when candidates exceed `SCORECARD_SYNC_MONITOR_MAX_AUTO_HEAL_SCORECARDS`.
- Accepted candidate sets are split into jobs of at most 5,000 names, and each `InternalJobService.CreateJob` immediately schedules the existing `ReindexScorecardsWorkflow`.
- The generic Temporal job wrapper changes a scheduled job to `RUNNING` before invoking its workflow body.
- The reindex job handler uses a 24-hour workflow execution timeout.
- The public job state enum has `PENDING` and `RUNNING`, but no `AWAITING_APPROVAL` state.
- The internal job service already has parent/child chunking with `max_concurrent_chunks`; this is a good execution primitive after approval.
- The repository has a manual-review precedent in retraining: notify, then wait on a Temporal signal before deployment.

## Recommended State Machine

```text
DETECTED
  -> AUTO_APPROVED -> REVALIDATING -> DISPATCHING -> VERIFYING -> SUCCEEDED/PARTIAL/FAILED
  -> AWAITING_APPROVAL
       -> APPROVED -> REVALIDATING -> DISPATCHING -> VERIFYING -> SUCCEEDED/PARTIAL/FAILED
       -> REJECTED
       -> EXPIRED
       -> CANCELLED

REVALIDATING
  -> AWAITING_APPROVAL when the fresh candidate set exceeds the approved envelope
```

## Request Contents

Persist enough information to make approval meaningful without putting an incident-sized list into Temporal history:

- customer/profile and monitor window
- observed candidate count and missing rate
- missing, stale-submit-time, stale-update-time, process, and conversation counts
- policy version and auto-approval reason
- candidate fingerprint or external snapshot reference
- requester/source, created time, expiry time
- approval decision, actor, time, reason, approved maximum count
- child job names and rolled-up progress after dispatch
- exact-ID post-repair convergence result

## Approval Contract

- Expose an authenticated application endpoint or UI action; do not use the Temporal UI as the manager interface.
- Check that the actor has the intended manager/admin scope for the customer/profile.
- Make approve/reject idempotent and reject decisions after expiry or terminal state.
- Bind approval to an envelope: candidate count, query window, and policy version. Material expansion requires reapproval.
- Support rejection, cancellation, and a bounded expiry such as 24-72 hours. An indefinite wait is undesirable; use a Temporal timer/selector and a workflow timeout longer than the business expiry.
- Preserve an immutable audit trail and expose the waiting state in Slack/job UI.

## Capacity and Safety

- Keep execution chunks at 5,000 scorecards.
- Use the existing chunked-job `max_concurrent_chunks` control plus a regional candidate/concurrency budget.
- Deduplicate by customer/profile plus monitor window or active repair key so daily runs coalesce instead of producing duplicate approvals.
- Recompute or revalidate candidates after approval. A delayed snapshot may contain already-healed scorecards or may have grown beyond the approved scope.
- Gate on candidate count, missing rate, regional ClickHouse health/backlog, and outstanding repair load. Missing rate alone is not a capacity measure.
- Perform exact-ID convergence verification after child completion. Successful Temporal closure only proves application-path execution.

## Alternatives Considered

### Pause every reindex workflow

Technically feasible with a signal wait, but not recommended. It creates one dormant workflow per chunk, marks jobs `RUNNING` under the current wrapper, collides with the 24-hour timeout, retains candidate payloads, and creates a large duplicate-control problem.

### Create PENDING jobs without Temporal executions

Possible only with a new scheduling contract because `CreateJob` currently creates and schedules the job together. This could work, but still needs an approval entity/API and atomic transition from approved request to bounded execution. A parent repair request is clearer.

### Store only a Slack approval message

Insufficient as the source of truth. Slack can be the notification/action surface, but the durable request, authorization, idempotency, expiry, and audit state must live in the application/workflow system.

## Suggested Delivery Slices

1. Introduce a repair-request payload/state and create it only for above-threshold detections; keep current below-threshold execution unchanged.
2. Add approve/reject API, RBAC, audit fields, notification, timeout, and deterministic deduplication.
3. On approval, revalidate candidates and use parent/child chunking with bounded concurrency.
4. Move below-threshold cases through the same request orchestrator as auto-approved requests, once the gated path is stable.
5. Add exact convergence telemetry and regional circuit breaking before allowing incident-scale approval execution.

## Validation Needed

- Temporal tests for signal/update before wait, duplicated approval, approval versus expiry race, cancellation, and replay determinism.
- API tests for unauthorized approvers and cross-profile access.
- Deduplication tests across repeated daily monitor runs.
- Revalidation tests for candidate shrink, growth, and approval-envelope breach.
- Chunking tests for maximum concurrency and partial child failure rollup.
- Load test with incident-scale counts that proves only one waiting parent is created before approval.
- End-to-end test that notification links to the correct request and records the approving actor.

## Credentials

No AWS, Okta, Azure, or SSH credential was used.
