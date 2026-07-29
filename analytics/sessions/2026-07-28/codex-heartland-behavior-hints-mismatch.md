# Heartland Behavior Hints adherence mismatch

**Date:** 2026-07-28
**Source repo:** `/Users/xuanyu.wang/repos/director`
**Branch/worktree:** `main` in `/Users/xuanyu.wang/repos/director`
**Issue:** https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785201046765479

## Report

- Customer: Heartland
- User: Jamie Rimbach
- Behavior: Offering Scheduling Flexibility and Availability
- Performance Insights adherence: 79%
- Assistance Insights adherence: approximately double
- Aggregate symptom: Behavior Hints can exceed 100%

## Investigation plan

1. Trace the Director components, API hooks, request fields, and transformations used by both surfaces.
2. Locate the backend/query semantics behind the relevant request.
3. Replay an equivalent read-only query against Heartland production ClickHouse.
4. Compare source data with each UI calculation and identify the root cause.

## Evidence and findings

- Performance Insights and Assistance Insights do not query the same metric:
  - Performance progression uses `RetrieveQAScoreStats` and displays conversation/scorecard-level QA criterion scores.
  - Assistance Behavioral Hints uses `RetrieveHintStats` and presents a hint follow rate conditioned on a hint being fired.
  - Therefore the two percentages are not inherently expected to match, even after fixing the invalid Assistance aggregation.
- Director's Assistance Insights Behavioral Hints surface calls `RetrieveHintStats` through `useHintStats`.
  - The behavior breakdown groups by policy and calculates `totalHintFollowedCount / totalHintSentCount` in `useGetEngagementTableData.tsx`.
  - The agent/policy leaderboard requests agent + policy grouping with no frequency and calculates the same ratio in `useSpecificHintLeaderboardForAgent.tsx`.
  - The overall Behavioral Hints card also directly divides the API totals in `AssistanceInsightsCarouselMenu.tsx`.
  - Some other Assistance table utilities cap the ratio at 1, but the reported surfaces do not. Capping would hide the invalid percentage while still producing the wrong answer.
- The API source is `insights-server/internal/analyticsimpl/retrieve_hint_stats_clickhouse.go`.
  - Behavioral `hint_sent_count` is `COUNT(DISTINCT adherence_action_annotation_id)`.
  - Behavioral `hint_followed_count` is `COUNT(DISTINCT moment_annotation_id)`.
  - These are different units. One hint action can be associated with multiple positive moment annotations, so followed can exceed sent.
- Heartland production was resolved to `heartland_us_west_2` on `us-west-2-prod`; all validation was read-only.
- Identity resolution:
  - Jamie Rimbach user ID: `a96b2c90c7e1c183`.
  - Behavior: `Offer Scheduling Flexibility and Availability`, ID `019e5cc1-ef3b-748a-a6df-2c8008c28d2f`.
  - Policy: `Offer Scheduling Flexibility and Availability`, ID `019e5ccb-be79-7357-a54f-1f3ac848b221`.
- Exact weekly reproduction for `[2026-07-20, 2026-07-27)` with the API's behavioral-hint filters:
  - 134 distinct sent hint actions.
  - 215 distinct positive moment annotations, yielding the Assistance API/UI value `215 / 134 = 160.4%`.
  - 106 distinct sent actions with at least one positive annotation, yielding `106 / 134 = 79.1%`, numerically matching the reported Performance value. This does not make the metrics equivalent because Performance is sourced from QA scorecards.
- Raw data is internally consistent. Distribution of positive moments per sent action over `[2026-07-20, 2026-07-28)`:
  - 37 actions had 0 positive moments.
  - 56 had 1; 39 had 2; 16 had 3; 10 had 4; 3 had 5; 3 had 6; 1 had 9.
  - The overage is therefore not duplicate ClickHouse rows; it is multiple valid moment annotations being counted as multiple followed hints.
- The same policy affects multiple Heartland agents. In the same recent range, at least eight additional user/policy rows had moment-based API ratios above 100%, while action-level ratios stayed at or below 100%.
- Root cause of the mathematically invalid Assistance value is primarily backend aggregation/API semantics, not a frontend arithmetic bug. The broader cross-page difference also includes an intentional API/population mismatch: QA criterion score versus post-hint follow rate.
- The query additionally combines sent branches with `MAX` and followed branches with `SUM`. That can create other mixed-source inflation risks, although the reproduced Heartland behavioral-hint case is explained directly by the action-ID versus moment-ID grain mismatch.

## Recommended fix

1. Confirm the intended product definition is “fraction of sent hints followed,” as the Assistance tooltip and denominator imply. If so, count behavioral followed hints as `COUNT(DISTINCT adherence_action_annotation_id)` under the positive-adherence predicate, aligned with the sent unit.
2. Add ClickHouse query and RPC tests where one hint action has multiple positive moment annotations; assert followed count is one and never exceeds sent.
3. Add a frontend defensive treatment for malformed API data only as a safeguard/telemetry signal. Do not use `Math.min(rate, 1)` as the primary fix because it would turn 160% into 100% rather than the correct 79%.

## Documentation promotion

- Added canonical API reference: `subdomains/shared-analytics-platform/retrieve-hint-stats.md`.
- Added Assistance Insights subdomain: `subdomains/assistance-insights/README.md`.
- Promoted the request/response contract, Director call shapes, backend flow, group aggregation, source tables, hint-type semantics, current mixed-grain defect, replay queries, and test gaps from this investigation into durable domain documentation.
