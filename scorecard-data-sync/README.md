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
- [Alaska Air orphaned scorecard report](deliverables/alaska-air-orphaned-scorecard-report.md)
- [Legacy source index](deliverables/legacy-source-index.md)
- [CONVI-7186 completed work item](work-items/CONVI-7186.md)

## Current State

The first migration is complete. The domain artifacts above are now the canonical synthesis for PG/CH scorecard projection. Historical ticket folders remain as evidence and execution archives, with migration pointers back here.

Two originally proposed sources were reclassified after review:

- `convi-6841-process-scorecard-update-race` is a PostgreSQL read-after-write workflow issue, not a PG/CH projection failure.
- `hilton-coaching-discrepancy` concerns coaching analytics/session semantics, not scorecard data synchronization.
- [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378) (SCAN Consent to Call) was ruled out after PG/CH alignment; canonical evidence lives in `analytics`.

On 2026-07-22, a production Slack ambiguity was confirmed: the displayed fraction counts only missing ClickHouse rows, while reindex workflows include both missing and timestamp-stale scorecards. A zero-missing line can therefore create a valid stale-row repair workflow; see `sessions/2026-07-22/codex-slack-missing-vs-reindex.md`.

On 2026-07-29, an Alaska Air scorecard was confirmed to remain in ClickHouse after an exact score-row deletion mutation completed. The surviving rows carry update times 15 seconds before the mutation, and no scorecard deletion mutation from the incident exists. Current evidence strongly supports an in-flight async writer inserting stale score and scorecard rows after reset deleted PostgreSQL state and created the ClickHouse mutation. The orphaned projection was backed up and removed on 2026-07-30, with zero rows verified across distributed views and all raw replicas. Runtime probes remain active in `go-servers-alaska-scorecard-orphan`; see `work-items/alaska-air-orphaned-scorecard.md`.

On 2026-07-30, the large `us-east-1` missing-rate spike was traced to delayed ClickHouse Distributed-table fan-out during regional ClickHouse/Keeper instability. Producers successfully inserted rows into `scorecard_d`, but the monitor queried local shard tables before queued files were delivered. Acorns' exact 2,183-row alert was reproduced from query logs and later part arrivals; see `sessions/2026-07-30/codex-us-east-1-missing-spike.md`.

On 2026-08-01, [INSI-4251](https://linear.app/cresta/issue/INSI-4251) showed that the regional failure had not fully recovered and was affecting Performance Insights. Guitar Center and Home Care Delivered retained large PG/CH gaps while customer Distributed queues held scorecard data. A distributed score-deletion DDL had also remained incomplete on all nine hosts since July 30, leaving stale all-N/A Guitar Center rows and blocking re-score cleanup. See `work-items/INSI-4251.md` and `sessions/2026-08-01/codex-performance-insights-missing-data.md`.

On 2026-08-02, the incident was resolved and the regional root cause confirmed: unbatched Distributed inserts produced approximately 6.36 million small spool files while the 16-thread sender pool was saturated across 3,328 Distributed tables. Enabling batched sends and split-on-failure, increasing the pool to 48, and rolling the Conversations nodes drained the queue and restored affected tenants. The canonical case index gathers the retrospective, investigation records, work item, logs, production changes, and interactive Canvas at `cases/2026-08-us-east-distributed-backlog/README.md`.

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
