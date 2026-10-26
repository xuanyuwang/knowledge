# Scorecard Lifecycle

## Purpose

Own valid scorecard states and transitions from creation through editing, submission, publishing, reversal, and repair.

## Semantics and Invariants

- Scorecard type and state constrain available transitions; normal, process, calibration, appeal, and other types are not interchangeable.
- Submission and publishing are distinct lifecycle events.
- Reversal must establish which record/state is authoritative and how downstream consumers interpret it.
- A read immediately following creation may require primary/write-database consistency; replica lag must not surface as a false `NOT_FOUND`.
- Lifecycle transition policy and PG-to-ClickHouse projection are separate concerns.

## Architecture and Source Map

- **Frontend:** scorecard editor, autosave, submit/publish/reset/reverse controls
- **APIs:** create, update, submit, publish, reset, and reversal actions
- **Backend/storage:** coaching service and PostgreSQL scorecard state; async downstream projection

## Operational Knowledge

- Reconstruct the exact state transition and scorecard type before diagnosing a failed action.
- For create-then-update races, verify whether the existence read was routed to a replica.

## Legacy Sources and Cases

- `scorecard-workflows/sessions/2026-10-25/scorecard-template-legacy-evidence/deliverables/scorecard-lifecycle.md`
- [nrg-option-value-display-mismatch](../../work-items/nrg-option-value-display-mismatch.md)
- [process-scorecard-create-autosave-race](../../work-items/process-scorecard-create-autosave-race.md)

## Open Questions

- Publish a state-transition matrix per scorecard type.
- Define idempotency and concurrency guarantees for every mutation.

See the [October 25 historical synthesis](../../deliverables/inactive-legacy-project-consolidation-2026-10-25.md) for consolidated cases, later superseding decisions, and their evidence limits.
