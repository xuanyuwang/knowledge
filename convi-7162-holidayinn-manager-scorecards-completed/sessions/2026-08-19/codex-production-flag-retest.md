# CONVI-7162 production flag retest

**Date:** 2026-08-19 (America/Toronto)
**Surface:** `https://holidayinn-transfers-voice.cresta.com/director/insights/leaderboard/managers`
**Use case:** `transfers-voice`
**Manager:** Cliff Hawker
**Metric:** Manager Leaderboard `Scorecards completed`

## Objective

Reproduce the exact post-release bug reported in Linear comment `f8ffbd2d`: for 2026-08-17, Cliff Hawker's daily Leaderboard cell showed 1 while the scorecard details showed 2 evaluations. Verify the result after the global `filterByScorecardSubmitTime` rollout.

## Exact reproduction

The authenticated customer production session was opened on the official `/director/` route without a query-parameter override.

For the screenshot's original 2026-08-14 through 2026-08-18 range, the Manager `Scorecards completed` heatmap now shows:

| Day | Count |
|---|---:|
| 2026-08-14 | 2 |
| 2026-08-15 | 0 (`N/A`) |
| 2026-08-16 | 0 (`N/A`) |
| 2026-08-17 | 2 |
| 2026-08-18 | 2 |
| Total | 6 |

This directly fixes the reported 2026-08-17 cell, which previously showed 1.

To remove later submissions from the comparison, the range was narrowed to exactly 2026-08-17. The result was:

- Cliff Hawker Leaderboard total: 2
- Cliff Hawker daily 08/17 cell: 2
- Scorecard template breakdown: `2 scorecards across 1 template`
- Ride Along Template: 2 scorecards

The two returned scorecard records display conversation timestamps of 2026-08-16 8:29 PM and 2026-08-17 2:35 PM. Their inclusion in the 2026-08-17-only result confirms that the filter/count uses scorecard submit time rather than the displayed conversation time.

## Conclusion

PASS. The exact bug cannot be reproduced after the global flag rollout. The official Holiday Inn production Manager Leaderboard now attributes both Cliff Hawker scorecards to the 2026-08-17 submit day, and the aggregate and breakdown both return 2.

## Credential hygiene

- Used only the customer production session that the user had already authenticated in Chrome.
- No CLI, cloud, database, or stored credentials were read or used.
