# Process Scorecards and Generation

## Purpose

Own whether process and generated scorecards should exist, how they are created, and how missing records are safely repaired or backfilled.

## Semantics and Invariants

- “Should a scorecard exist?” is a workflow/configuration question; “has it projected to ClickHouse?” is a data-sync question.
- Generation eligibility depends on template/use-case configuration, conversation context, required annotations, and workflow state.
- Opera annotation backfill may need to run before scorecard generation when AutoQM outcomes are inputs.
- A UI feature flag is not necessarily a backend execution or customer-visibility guard.
- One-off downstream backfill is preferable to a recurring cron when the requirement is finite historical repair.

## Architecture and Source Map

- **Frontend:** process scorecard creation/editing and Performance Config backfill controls
- **Backend/jobs:** coaching generation, downstream backfill, Opera/policy backfill, and repair flows
- **Storage:** PostgreSQL scorecards/configuration and upstream annotations
- **Boundary:** projection/reindex mechanics live in `scorecard-data-sync`

## Operational Knowledge

- Prove eligibility and upstream annotation availability before attempting regeneration.
- Protect customer-visible annotations with explicit read-side/platform visibility policy.
- After generation, use the Scorecard Data Sync domain to diagnose PG/CH drift.

## Legacy Sources and Cases

- `auto-backfill-missing-scorecards/`
- `scorecard-template/deliverables/empty-scorecards-workflow-and-api-analysis.md`
- `convi-6841-process-scorecard-update-race/`

## Open Questions

- Publish an executable eligibility decision tree.
- Define idempotency, throttling, monitoring, and rollback for every backfill mode.
