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

## Initial Knowledge Map

- end-to-end PG-to-CH data-flow architecture
- consistency and ordering invariants
- failure-mode taxonomy
- monitor/query catalog
- diagnosis ladder
- repair/reindex/backfill playbook
- validation and rollout standards
- historical incident and decision index

## Migration State

The domain is scaffolded. `pg-ch-scorecard-sync-investigation`, CONVI-5565, backfill, reindex, and mismatch folders remain source material until synthesized.

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `clickhouse-schema`, `cresta-proto`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
