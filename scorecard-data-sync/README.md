# Scorecard Data Sync Domain

## Purpose

The canonical knowledge home for keeping PostgreSQL scorecard state and its ClickHouse analytical projection consistent, observable, diagnosable, and repairable.

## Scope and Boundaries

**In scope**

- source-of-truth and projection contracts
- write, async propagation, ordering, and idempotency behavior
- race conditions, lost/stale updates, and mismatch taxonomies
- ClickHouse schema/materialization details relevant to correctness
- sync monitoring, comparison queries, thresholds, and alert interpretation
- reindex and backfill mechanics
- repair workflows, rollout controls, and post-fix validation
- acceptable residual mismatches and documented limitations

**Out of scope**

- scorecard business lifecycle or whether a scorecard should be generated: `scorecard-workflows`
- Performance Insights/Leaderboard presentation semantics: `analytics`
- notification delivery: `notifications`

## Subdomains

No subdomains are defined currently. Projection architecture, monitoring, diagnosis, and repair remain tightly coupled and are maintained as one cohesive domain.

## Initial Knowledge Map

- [Architecture and invariants](deliverables/architecture-and-invariants.md)
- [Failure modes and case index](deliverables/failure-modes-and-case-index.md)
- [Monitoring and diagnosis](deliverables/monitoring-and-diagnosis.md)
- [Repair and backfill playbook](deliverables/repair-and-backfill-playbook.md)
- [Legacy source index](deliverables/legacy-source-index.md)
- [CONVI-7186 completed work item](work-items/CONVI-7186.md)

## Current State

The first migration is complete. The domain artifacts above are now the canonical synthesis for PG/CH scorecard projection. Historical ticket folders remain as evidence and execution archives, with migration pointers back here.

Two originally proposed sources were reclassified after review:

- `convi-6841-process-scorecard-update-race` is a PostgreSQL read-after-write workflow issue, not a PG/CH projection failure.
- `hilton-coaching-discrepancy` concerns coaching analytics/session semantics, not scorecard data synchronization.

On 2026-07-22, a production Slack ambiguity was confirmed: the displayed fraction counts only missing ClickHouse rows, while reindex workflows include both missing and timestamp-stale scorecards. A zero-missing line can therefore create a valid stale-row repair workflow; see `sessions/2026-07-22/codex-slack-missing-vs-reindex.md`.

## Reading Order

1. Start with architecture and invariants.
2. Use the failure-mode index to classify a symptom.
3. Follow the monitoring/diagnosis ladder to isolate the mismatch.
4. Choose the narrowest safe repair from the repair/backfill playbook.
5. Use the legacy source index only when historical detail or execution artifacts are needed.

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `clickhouse-schema`, `cresta-proto`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `log/2026-07-15.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
