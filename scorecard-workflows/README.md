# Scorecard Workflows Domain

## Purpose

The canonical knowledge home for scorecard and template business workflows from authoring and evaluation through submission, permissions, reversal, appeals, group calibration, and related review behavior.

## Scope and Boundaries

**In scope**

- template concepts, authoring, revisioning, schema evolution, and configuration
- scorecard creation, evaluation, scoring, editing, submission, publishing, and reversal
- manual, AutoQM, process, manager, and other scorecard-type behavior
- permissions, visibility, audiences, and capability evaluation
- appeals, appeal comments, group calibration, and review workflows
- generation/backfill behavior when the question is whether a scorecard should exist
- frontend/backend workflow contracts and business-rule invariants

**Out of scope**

- PostgreSQL-to-ClickHouse propagation, ordering, drift, repair, or reindex mechanics: `scorecard-data-sync`
- analytics-page display and aggregation semantics: `analytics`
- delivery mechanics for notifications: `notifications`

## Initial Knowledge Map

- scorecard and template concept map
- lifecycle/state-transition catalog
- workflow and scorecard-type matrix
- scoring and N/A semantics
- permission/visibility policy
- appeals and group-calibration flows
- schema compatibility and evolution
- generation/backfill decision rules
- frontend/backend contracts
- operational sharp edges and work-item history

## Migration State

The domain is scaffolded. `scorecard-template`, `group-calibration`, and related folders are seed material; no legacy canonical source has been replaced yet.

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `director`, `cresta-proto`, `config`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
