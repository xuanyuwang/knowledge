# QM tasks inherit their template's use-case scope

**Date:** 2026-10-08  
**Status:** Decided product invariant; implementation pending  
**Scope:** Template-based QM tasks only

## Decision

A QM task is based on one scorecard template. Its effective use-case scope is the same as that template's scope: `QM task scope = scorecard template scope`. The task does not have an independently chosen use-case subset. Multiple QM tasks may share a template while owning different audiences, quotas, schedules, and conversation filters.

The scope used for future task discovery and evaluations follows the applicable current template scope. A scorecard already evaluated under a prior template revision keeps its concrete use case and pinned revision; this decision does not rewrite historical evaluations. An empty template use-case list retains the existing meaning of all use cases.

This applies to QM tasks. Group-calibration and other Director task types retain their own scope contracts.

## Basis

- The normal Director QM setup path requires a scorecard template and stores its name in the task content config. The QM task-config backfill also creates tasks from templates.
- The current `DirectorTask` API and `director.tasks` table have a singular use case; Director copies the template's first use case. Task listing, report discovery, and evaluation-period lookup filter on that copied value. It cannot represent a task spanning a template's A and B use cases.
- The API/database do not yet require every QM-typed row to have a valid template reference. Existing rows need a missing/stale-reference audit before all reads can assume the invariant.

Detailed source trace: [`../sessions/2026-10-08/codex-qm-task-template-usecase-relationship.md`](../sessions/2026-10-08/codex-qm-task-template-usecase-relationship.md).

## Implementation boundary

Enforce the template relationship and scope rule server-side. Choose whether task scope is derived from the template or stored as a synchronized projection; update task queries, evaluation paths, and backfills consistently. The API/storage shape, legacy-row handling, and template lifecycle behavior remain implementation decisions.
