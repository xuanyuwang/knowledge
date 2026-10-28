# CONVI-7472: QM Report Adjusted appeals chart inflation

**Status:** Done in Linear, verified October 28, 2026. [Director PR #24302](https://github.com/cresta/director/pull/24302) merged as `97bdbe691989e15585a58f682ece90db33de8469`. Deployment/customer recheck unverified.
**Primary domain:** analytics
**Primary subdomain:** none
**Official ticket:** [CONVI-7472](https://linear.app/cresta/issue/CONVI-7472/qm-report-appeals-adjusted-count-inflated-to-total-submitted-count-gcc)
**Last updated:** 2026-10-28

## Objective and Impact

Correct the Adjusted chart series and preserve investigation and verification evidence. Golden Customer Care (`golden-cc`), Heather Strauss, Zendesk #22447; customer waiting since July 14. Role: diagnosed the production mismatch, implemented and tested the frontend fix, and closed the bug after merge.

## Source Context

- Primary implementation repo: `/Users/xuanyu.wang/repos/director`. New worktree `/Users/xuanyu.wang/repos/director-convi-7472`, branch `xw/convi-7472-adjusted-appeals`, from freshly fetched origin/main `76f6b847418f7f287e9b598279348c1a7394c6e1`.
- Related backend: go-servers `retrieve_appeal_stats.go` / `collectAppealStats` (references from ticket only).
- Evidence: ticket description and three indexed comments via Glean, October 27 live API/browser validation, current source confirmation, and local regression tests. [PR #24302](https://github.com/cresta/director/pull/24302) merged as `97bdbe691989e15585a58f682ece90db33de8469`; deployment not verified.

## Live Validation — October 27, 2026

Completed the first follow-up in the user's Chrome session. Full July time-series API bucket returns 10 resolved and 3 adjusted conversations; chart displays 10 adjusted. Table has three adjusted rows, and criteria overview is 3/12 = 25%. July 1–22 Daily returns totalAppealStats 7 resolved / 1 adjusted, with table and criteria overview matching. The ticket's 6/1 example is historical, not the current full-month expected value. Browser evidence confirms chart inflation after the API supplies a correct distinct adjusted count. Current source confirms this mapping defect; the local fix now uses adjustedData. See [live check](../sessions/2026-10-27/codex-convi-7472-live-api-check.md). Original full-July Monthly view restored; no product or external-ticket changes.

## Current Understanding (ticket evidence recorded October 26)

The current description and October 23 comment identify an Appeals Over Time chart series wiring defect. July 14: chart shows 4 adjusted, actual 1. July 22: chart shows 6 adjusted, actual 1. Table rows show one adjusted appeal; By Criteria shows 1 of 8 resolved criteria, rounded to 13% (a different denominator).

The ticket reports `QAReportAppealTrendsCard.tsx` constructs `adjustedSeries` with `data: resolvedData`, while computing unused `adjustedData` from `adjustedConversationCount`. The earlier September 4 comment suspected artifact double-counting analogous to CONVI-7075 / go-servers #28912; the current description explicitly rules that out based on correct table/overview data from `collectAppealStats`. October 27 live time-bucket validation independently confirms the distinct adjusted count returned by the API.

## Completion and operational follow-up

The engineering bug is complete: API response verified, series reference corrected, three regression cases added, PR merged, and Linear set to Done at the user's request. Linear status was read back after reload on October 28.

Deployment and a post-deployment GCC check were not performed in this chat. When checking, compare the chart with the current API response; October 27 full-July values were 10 resolved / 3 adjusted, and July 1–22 values were 7 / 1. The historical 6 / 1 screenshot is not a full-month invariant. Zendesk/customer communication was not sent.

The requested adjusted-appeals / all-evaluations error ratio is a separate feature, excluded from this bug.

## Validation and Rollout

All 59 QM report tests (11 files), full Director app TypeScript, changed-file ESLint and Biome, and diff whitespace checks pass. Three new cases failed against the original mapping before the one-line fix. Setup used existing GitHub Packages authentication without recording secrets; no AWS/Okta/Azure/SSH credentials accessed. Original head `3620322dbc0584cb9ae35af630f965fb509b0a25` merged through PR #24302 as `97bdbe691989e15585a58f682ece90db33de8469`. GitHub merge verified October 28: mergedAt `2026-10-28T03:08:18Z` (October 27, 23:08:18 Toronto). Deployment remains unverified. Linear Done confirmed live on October 28. See [implementation record](../sessions/2026-10-27/codex-convi-7472-fix.md).

## Timeline

- 2026-10-26 — Summarized the ticket and comments; separated the superseded backend hypothesis from the reported frontend defect and pending API validation. See [session](../sessions/2026-10-26/codex-convi-7472-ticket-summary.md).

- 2026-10-27 — Live API/report validation completed; chart displays resolved count as adjusted despite distinct API counts. See live-check session.

- 2026-10-27 — Implemented the one-line chart fix and regression tests in the requested fresh worktree; local verification passed. PR/deployment pending.

## Construction report

[Interactive construction report](../deliverables/convi-7472-construction-report.html) walks through the original mapping, failing regression cases, and final one-line fix. Includes captured local source, scenario matrix, and deployment limits. [Report session](../sessions/2026-10-27/codex-convi-7472-construction-report.md).

- 2026-10-27 — Created draft [Director PR #24302](https://github.com/cresta/director/pull/24302) after fresh 59-test/typecheck validation and passing commit hooks. Worktree clean; merge/deploy/live validation remain.

## Closure — October 28, 2026

Confirmed GitHub merge and changed Linear from In Progress to Done; verified after reload. [Closure session](../sessions/2026-10-28/codex-convi-7472-close.md) and [status screenshot](../sessions/2026-10-28/convi-7472-done.jpg). Related documents committed at the user's request; unrelated work excluded.
