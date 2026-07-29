# CONVI-7384: Coaching Hub Performance and Scorecards mismatch

**Status:** diagnosing — runtime instrumentation pending reproduction
**Primary domain:** `analytics`
**Primary subdomain:** `qa-score`
**Official ticket:** [CONVI-7384](https://linear.app/cresta/issue/CONVI-7384/performance-score-and-scorecard-counts-do-not-match-in-coaching-hub)
**Last updated:** 2026-07-28

## Objective and Impact

- **Objective:** Explain and reconcile the 6-scorecard weekly Performance result, 4-scorecard monthly Performance result, and 5-scorecard Scorecards-tab result for RCG agent Irina Aguilera.
- **Customer/system impact:** Coaching Hub presents different QA scores and evaluation counts for apparently equivalent June filters.

## Source Context

- **Repos:** `go-servers`, `director`, `cresta-proto`
- **Worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7384`
- **Branch:** `convi-7384-coaching-scorecard-time-parity`
- **Runtime data:** RCG production ClickHouse `rcg_us_east_1`

## Current Understanding

The three observed results select three different scorecard populations:

- Weekly Performance filters `scorecard_time` from May 31 04:00Z through July 5 04:00Z. It includes four June-scored rows and two July 2-scored rows: six scorecards averaging 91.6667%.
- Monthly Performance for the June calendar window filters `scorecard_time` from June 1 04:00Z through July 1 04:00Z. It includes four scorecards averaging 93.75%.
- The Scorecards tab calls `ListScorecards` with `startSubmitTime`/`endSubmitTime` for June. It includes one May 28-scored scorecard submitted June 1 plus the four June-scored scorecards: five scorecards averaging 95%.

This exactly reproduces every supplied count and percentage. Backend instrumentation now records request boundaries, query time columns, and returned buckets so an end-to-end rerun can prove the actual monthly request boundaries before a fix is selected.

## Hypotheses

1. **Frequency/date-boundary mismatch:** weekly selection expands to Sunday week boundaries and includes July 2 data, while monthly uses calendar-month boundaries.
2. **Time-basis mismatch:** Performance uses `scorecard_time`, while the Scorecards tab uses `scorecard_submit_time`.
3. **Template revision mismatch:** the June 1 scorecard is omitted because it references revision `f9839486`.
4. **ClickHouse projection/dedup loss:** one or more scorecards are missing or stale in analytics storage.
5. **Monthly truncation defect:** `DATE_TRUNC('month', ...)` drops otherwise qualifying June rows.

Production rows strongly confirm hypotheses 1 and 2 and explain the result without 3–5. The instrumented reproduction remains required to capture request-boundary evidence in the debug session log.

## Validation and Rollout

- Confirmed the Coaching REST path maps to `CoachingService.ListScorecards` in `coaching_service.proto`.
- Queried latest RCG ClickHouse `scorecard_d` rows for the agent, use case, and template.
- Reconstructed the three populations and exact averages from production timestamps and scores.
- Added temporary instrumentation to `RetrieveQAScoreStats`; do not remove it until post-fix verification or explicit user confirmation.

## Next Actions

1. Reproduce WEEKLY and MONTHLY requests against the instrumented backend.
2. Evaluate all hypotheses from `/Users/xuanyu.wang/repos/.cursor/debug-407263.log`.
3. Choose intended cross-tab semantics before implementing a fix.

## Timeline

- 2026-07-28 — Retrieved Linear/Slack context, confirmed `ListScorecards`, queried production ClickHouse, exactly reconstructed all three results, and added request/query/result instrumentation. Evidence: `sessions/2026-07-28/codex-convi-7384-time-semantics.md`.
