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

## Subdomains

- [Template Authoring and Versioning](subdomains/template-authoring-and-versioning/README.md): template structure, builder behavior, revisions, duplication, and schema compatibility
- [Evaluation and Scoring](subdomains/evaluation-and-scoring/README.md): manual/AutoQM evaluation, option mapping, N/A, weights, outcomes, and score computation
- [Scorecard Lifecycle](subdomains/scorecard-lifecycle/README.md): creation, editing, submission, publishing, reversal, and concurrency/state transitions
- [Permissions and Visibility](subdomains/permissions-and-visibility/README.md): capability policy, audiences, submitted editing, and runtime visibility
- [Appeals](subdomains/appeals/README.md): request/resolve workflows, final-value interpretation, comments, and exports
- [Group Calibration](subdomains/group-calibration/README.md): answer keys, responses, completion, permissions, reporting, and exports
- [Process Scorecards and Generation](subdomains/process-scorecards-and-generation/README.md): process scorecards, existence rules, generation, repair, and backfill orchestration

## Current State

The first subdomain migration is complete. The subdomain references and [legacy source index](deliverables/legacy-source-index.md) are canonical navigation; historical folders retain detailed evidence and current work.

Work items, sessions, daily logs, and decisions remain at this parent domain. Future work items should record one optional primary subdomain.

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `director`, `cresta-proto`, `config`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `log/2026-07-15.md`
- `deliverables/legacy-source-index.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
