# CONVI-7435: Bswift PI vs QM scorecard discrepancies

**Status:** active
**Primary domain:** `analytics`
**Primary subdomain:** `performance-insights`
**Official ticket:** [CONVI-7435](https://linear.app/cresta/issue/CONVI-7435/bswift-performance-insights-and-qm-report-scorecard-discrepancies)
**Last updated:** 2026-08-05
**Investigation report:** [deliverables/convi-7435-bswift-pi-qm-investigation.md](../deliverables/convi-7435-bswift-pi-qm-investigation.md)

## Objective and Impact

- **Objective:** Diagnose two Bswift reporting discrepancies that undermine trust in Cresta as the source of truth for agent performance: (1) weekly date-range mismatch between Performance Insights and QM Report; (2) PI showing 40% for a conversation whose scorecard UI shows Performance Score N/A.
- **Customer/system impact:** High — Bswift / voice-care; Zendesk #22972; agent Breanna Marshall.
- **Role:** diagnosed (assigned on Linear; Slack ping from Tinglin for issue #2)

## Scope

**In scope**

- Issue #2 (priority for this session): conversation `#712933057605` / UUID `019fb6c4-e4df-72c7-a23c-6ea3a86ad964` — PI 40% vs conversation scorecard N/A / 0 criteria scored
- Issue #1 context: weekly grouping expanding PI into Jun 28–30 vs QM honoring Jul 1–31 (12 vs 13 scorecards)
- Date/score semantics for Manual Heartbeat Quality Scorecard under namespace `voice-care`

**Non-goals**

- Immediate product fix without confirmed root cause
- Unrelated PI/QM filter differences outside this ticket

## Source Context

- **Repos:** `go-servers`, `director`
- **Worktrees:** main checkouts initially; dedicated worktree TBD if fix needed
- **Branches:** TBD
- **PRs/commits:** none yet
- **Slack:** https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785959396310119
- **Zendesk:** https://crestasupport.zendesk.com/tickets/22972

## Current Understanding

**Issue #2 — confirmed:** Manual Heartbeat for `#712933057605` **was reset/deleted**. CH `system.mutations` shows successful deletes of Manual template rows (`019ce797-…`) at this conversation’s `scorecard_time` (`2026-07-30 21:25:48`) for scorecard IDs `019fb852-53fe-…` (2026-07-31) and `019fc8e3-197d-…` (2026-08-03). That matches ResetScorecard’s PG-first + async CH delete path: closed conversation can show N/A immediately while PI still reads stale CH until the mutation lands. After catch-up, the conversation disappears from Manual PI filter (current state).

**Issue #1:** PI Weekly expands calendar weeks beyond the selected Jul 1–31 (Jun 28–30 / through Aug 1); QM honors the selected range and submitted Manual evals (12). Product/UX alignment still open.

## Findings and Decisions

- CH mutations prove Manual scorecards existed for this conversation and were deleted (not a template-filter leak of Non-Performance).
- Screenshot `72%=(76+100+40)/3` is consistent with a ~40% Manual score still visible in CH before/during async delete lag.
- Path: PI = CH; closed conversation = PG; reset deletes PG scores immediately, CH via `DeleteScoresOnShard` / `DeleteScorecardOnShard`.

## Blockers and Dependencies

- Optional: confirm who reset / whether score was exactly 40% (PG gone; CH rows purged by mutation).

## Validation and Rollout

-

## Next Actions

1. Optionally check apiserver/audit logs for ResetScorecard on those scorecard IDs
2. Confirm with Tinglin expected weekly-boundary semantics for issue #1 / Support guidance
3. Update Linear + Slack with reset + CH lag explanation for issue #2

## Timeline

- 2026-08-05 — Slack triage; CH/PG investigation; reset theory confirmed via `system.mutations`. Evidence: `sessions/2026-08-05/claude-convi-7435-bswift-pi-qm.md`, `log/2026-08-05.md`
