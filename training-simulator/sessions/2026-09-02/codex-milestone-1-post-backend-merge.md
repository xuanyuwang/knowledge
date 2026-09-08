# Milestone 1 status after backend merge

**Date:** 2026-09-02
**Primary source repo:** `director`
**Branch context:** current GitHub `main`, inspected read-only
**Related backend:** [go-servers#31780](https://github.com/cresta/go-servers/pull/31780)
**Related contract:** [cresta-proto#9716](https://github.com/cresta/cresta-proto/pull/9716)

## Verified state

- [go-servers#31780](https://github.com/cresta/go-servers/pull/31780) merged on 2026-09-02 as `f3766193554678e2795319a381963e45f46e55e6`.
- [cresta-proto#9716](https://github.com/cresta/cresta-proto/pull/9716) merged on 2026-08-31 as `4c8bc07d0fe6d4f0b53d4733e5ee7f11769653bd`.
- Linear still reports [CONVI-7583](https://linear.app/cresta/issue/CONVI-7583/complete-the-training-simulator-session-reporting-backend) as In Progress and [CONVI-7584](https://linear.app/cresta/issue/CONVI-7584/complete-the-focused-director-updates-for-training-simulator-session) as Backlog with a CONVI-7583 blocker. Those tracking fields lag the merged implementation.

## Remaining implementation

Current Director `main` confirms the two visible Milestone 1 gaps remain:

1. `SessionsDashboard` displays assigned, completed, and overdue unique-agent counts, but not the required incomplete unique-agent count (`assigned - complete in every visible assigned session`).
2. `AgentResult` does not carry `AgentPerformanceEntry.attemptCount`, and `AgentResultRow` renders only date, View, status, and score. The drawer therefore does not display authoritative attempt count.

When implementing these:

- Preserve current status/score rendering, unavailable aggregates, View routing, and the defensive assignment join.
- Rename `completedAt` to latest-activity semantics because it is the latest run timestamp and can exist for incomplete retries.
- Add focused derivation, filtering, rendering, accessibility, loading, and error coverage.

## Remaining contract and release work

- The merged `attempt_count` proto comment incorrectly says runs are counted only across required modules. The merged backend counts every assigned-agent task run returned for the session, including stale-module runs, before latest-attempt reduction. Correct the comment to prevent consumers from implementing the wrong semantics.
- Product/engineering still need explicit decisions for nonzero/applicable partial timeout classification and reporting persona/row scope if those remain Milestone 1 release requirements.
- Reconcile representative staging cases and verify the visible Director additions before release.
- Update Linear: mark CONVI-7583 complete, remove its blocker from CONVI-7584, and move CONVI-7584 into the active workflow when implementation begins.

## Sources

- GitHub PR metadata for go-servers#31780 and cresta-proto#9716.
- Current GitHub `cresta/director@main` files:
  - `training-sessions/dashboard/SessionsDashboard.tsx`
  - `training-sessions/dashboard/performanceOverview.ts`
  - `training-sessions/session-review-drawer/AgentResults.tsx`
  - `training-sessions/sessionAgentResults.ts`
- Current Linear records for CONVI-7582, CONVI-7583, and CONVI-7584.
- Merged `cresta/v1/trainingsimulator/stats.proto`.

Glean was unavailable because the local Glean plugin is not configured; the status was cross-checked directly against GitHub, Linear, and durable project records.
