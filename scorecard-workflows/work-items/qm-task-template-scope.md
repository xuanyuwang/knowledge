# QM task template scope

**Status:** active  
**Primary domain:** `scorecard-workflows`  
**Primary subdomain:** `evaluation-and-scoring`  
**Official ticket:** none  
**Last updated:** 2026-10-08

## Objective and Impact

- **Objective:** Make one template-based QM task cover the same use cases as its scorecard template.
- **Customer/system impact:** A shared-template task must remain discoverable and usable for every applicable use case without mixing historical scorecards or unrelated task types.
- **Role:** designed

## Scope

**In scope:** QM task/template scope relationship, task discovery and evaluation, affected reporting and backfills.  
**Non-goals:** Independent task use-case subsets and changing group-calibration task scope.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/director`, `/Users/xuanyu.wang/repos/cresta-proto`
- **Worktrees:** Main checkouts; no dedicated source worktree for this design review.
- **Branches:** `main`; relevant paths checked against refreshed `origin/main` on 2026-10-08.
- **PRs/commits:** No implementation PR or commit.

## Current Understanding

The [decision](../decisions/2026-10-08-qm-task-inherits-template-usecase-scope.md) fixes the product invariant: a template-based QM task inherits its template's whole use-case scope for future evaluations. Current API/storage holds only one copied use case, and several queries use that value as a task filter. Normal UI and backfill creation is template-based, but the backend/schema do not guarantee every persisted QM row has a valid template link.

## Findings and Decisions

- One QM task may cover several use cases when its template does; task-owned audience, quota, schedule, and conversation filters remain separate.
- Existing scorecards retain their concrete use case and pinned template revision.
- The exact derived-versus-materialized task scope representation remains open.

## Blockers and Dependencies

- Audit relevant customer task rows for absent or stale template references before relying on the invariant for all existing data.
- Review task and scorecard backfills, report queries, and all-use-cases template behavior before enabling shared-template tasks.

## Validation and Rollout

Design capture only. Source evidence is in [the session note](../sessions/2026-10-08/codex-qm-task-template-usecase-relationship.md); no implementation or deployed-data validation has occurred.

## Next Actions

1. Select derived or synchronized task scope and enforce the template link in QM create/update.
2. Audit legacy rows and update discovery, evaluation, reporting, and backfills for A/B and all-use-cases templates.

## Timeline

- 2026-10-08 — Confirmed normal QM authoring is template-based, distinguished it from the unenforced storage shape, and recorded equal template/task scope as the product invariant. Evidence: [decision](../decisions/2026-10-08-qm-task-inherits-template-usecase-scope.md), [session](../sessions/2026-10-08/codex-qm-task-template-usecase-relationship.md).
