# Codex Session: Milestone 1 Current Gaps

## Context

- **Date:** 2026-08-31
- **Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
- **Branch/worktree context:** current Director main inspected read-only; `/Users/xuanyu.wang/repos/cresta-proto-convi-7583` and `/Users/xuanyu.wang/repos/go-servers-convi-7583` contain the contract and backend draft
- **Scope:** Reconcile the current Milestone 1 session-reporting gaps from the durable project records after the 2026-08-27 result-state decision.

## Current Conclusion

The verified **visible Director data gaps** are narrower than the previously recorded CONVI-7584 scope:

1. show the count of incomplete agents across the currently visible sessions; and
2. show each agent's authoritative attempt count in the session drawer.

The drawer already shows each agent's backend-derived Passed/Failed/Incomplete state and score, so “per-agent pass rate” is not a missing Milestone 1 datum in the current design. A historical passed-attempt rate would be a different, currently undefined metric and must not be added without choosing its denominator.

CONVI-7582 is not remaining implementation work: its premise was superseded, its three PRs were closed, and Linear already marks it Done. CONVI-7583 is In Progress; CONVI-7584 remains Backlog.

## Frontend Data Verification

| Proposed gap | Verification | Backend response impact |
|---|---|---|
| Incomplete count across agents/sessions | Missing as a displayed dashboard value. Figma places it on the unique “Assigned agents” card: an agent is incomplete when at least one visible assigned session is incomplete, equivalently `assigned unique agents - agents complete in every visible assigned session`. Director already has the required enriched rows and zero-run fallback. | No new protobuf field. The current stats RPC alone is incomplete because it omits assignments when no runs exist; CONVI-7583 must still become assignment-rooted so other consumers do not need Director's defensive join. |
| Attempt count per agent in a session drawer | Missing. The response's `task_runs` contains only the latest run per module, so retries cannot be counted accurately. | Requires `AgentPerformanceEntry.attempt_count`. This is already added by [cresta-proto#9716](https://github.com/cresta/cresta-proto/pull/9716), whose CI is green as of this verification. |
| Pass rate per agent in a session drawer | Not a confirmed missing datum. The existing row already renders backend-derived Passed/Failed plus the agent's score, matching the Figma drawer. | No new field. Existing `status`, `passed`, `score`, and latest per-module `task_runs` support the shipped row. A rate over historical attempts is not derivable, but that metric is not specified by the current design. |

## Remaining Gaps

1. **Backend correctness (CONVI-7583):** assignment-rooted reads, zero-run task/assignee coverage, bounded direct reads beyond the 1,000-run list cap, all-attempt counts, deterministic latest-attempt-first selection, conversation/quiz normalization, correct denominators/unavailable values, authorization, and telemetry.
2. **Focused Director work (CONVI-7584):** display the global incomplete count and per-agent `attempt_count`; preserve the existing status/score row, unavailable rendering, routing, and defensive assignment join; add focused tests. Rename the internal `completedAt` model to latest-activity semantics when touching the row.
3. **Contract decisions:** define a timed-out partial snapshot with applicable criteria or a nonzero score; choose allowed reporting personas and backend row scope.
4. **Release evidence:** representative staging reconciliation; unauthorized-scope checks; over-1,000-run and safety-bound tests; query count/latency telemetry; and Design verification of the two remaining visible deltas.
5. **Tracking hygiene:** remove the stale CONVI-7582 blocker from CONVI-7583 and update CONVI-7583/7584 descriptions to the accepted zero/false decision and verified frontend boundary.

## Evidence Reviewed

- `deliverables/milestone-1-session-reporting-review.md`
- `decisions/2026-08-27-collapse-overall-zero-false-results.md`
- `work-items/CONVI-7582.md`
- `work-items/CONVI-7583.md`
- `work-items/CONVI-7584.md`
- `sessions/2026-08-26/codex-milestone-1-remaining.md`
- `log/2026-08-27.md`
- `log/2026-08-28.md`
- Current Director session dashboard, result derivation, and drawer implementation under `packages/director-app/src/features/training-simulator/tabs/training-sessions/`
- Current `stats.proto` on main plus the CONVI-7583 contract branch
- Figma Reporting node `13108:21741`, including session drawer `13111:24963`
- Current Linear state and relations for CONVI-7582, CONVI-7583, and CONVI-7584

## Credential Use

No credentials were read or persisted. The investigation used local knowledge/code, Figma read tools, public Slack search, Linear, GitHub PR metadata, and read-only Git inspection.
