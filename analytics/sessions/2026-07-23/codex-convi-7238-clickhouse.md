# CONVI-7238 ClickHouse reproduction

**Date:** 2026-07-23
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree:** `main` at `/Users/xuanyu.wang/repos/go-servers`
**Status:** root cause refined

## Investigation question

How many scorecards should `RetrieveQAScoreStats` count for Manager Leaderboard, and why is an expected manually evaluated Chat Quality Form scorecard omitted?

## Inputs

- Customer: United
- Manager: Emma Alatan
- Manager user ID: `1b3b751ed82f8272`
- Initially supplied scorecard template ID: `0199e3b1-232c-76bb-9f71-f63d1da2527b` (`1.2 - Engaging the Customer (Auto Scored)`)
- Chat Quality Form template ID present in matching data: `019e6964-1a14-72ea-b11d-54c2b7951e10`
- Use case: `chat-reservations`
- Request time range: `[2026-07-01 04:00:00 UTC, 2026-07-24 03:59:59.999 UTC]`
- Request grouping: scorecard submitter and daily time range
- Request score resource: scorecard-level (`scorecard_d`)
- Ticket: [CONVI-7238](https://linear.app/cresta/issue/CONVI-7238)
- Slack context: [thread](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1784381433567329)

## Starting hypotheses

1. The expected scorecard has `MANUALLY_SUBMITTED` status but is excluded by submitter/reviewer attribution.
2. The query filters on a different user column than the UI semantics imply.
3. Template revision identity differs from the logical template ID supplied by the UI.
4. Time attribution, deletion/state flags, customer/virtual-group scope, or `FINALIZED`/submitted-state predicates remove the row.
5. Multiple criterion rows exist, but distinct-scorecard aggregation or joins eliminate the scorecard.

## Work log

- Read the analytics domain, QA Score, Leaderboard, and CONVI-6968 migration context.
- Traced `RetrieveQAScoreStats` from request filters to `scorecard_d`.
- Confirmed the supplied FE request does not filter by scorecard template; template-specific inspection is a diagnostic query, not part of the production aggregate.
- Confirmed `MANUALLY_SUBMITTED` is implemented as `scorecard_submit_time <> 0`, not `manually_scored = true`.
- Confirmed the time range is applied to `scorecard_time` (for conversation scorecards, conversation start time), not submission time.
- Confirmed `includeNaScored: false` adds `score >= 0`, so a submitted scorecard with an unavailable/negative aggregate score is omitted.
- Confirmed submitter filtering and grouping use `submitter_user_id`.
- Confirmed latest-version dedup selects the maximum `scorecard_last_update_time` per scorecard and applies submission status after dedup.
- Located the approved connection helper at `/Users/xuanyu.wang/repos/xuanyu-scripts/connect-clickhouse.zsh` and used its AWS Secrets Manager path without printing credentials.
- Executed the production-shaped query against `united_east_us_east_1.scorecard_d`.
- The exact FE request returns seven distinct scorecards for Emma. All seven are from template `0199e3b1-232c-76bb-9f71-f63d1da2527b`.
- Daily grouping returns three rows in ascending time order: July 14 = 3, July 17 = 3, July 20 = 1.
- All seven latest target-template rows satisfy the request's time, use-case, submitter, submitted-status, and non-negative-score predicates.
- Queried PostgreSQL template history and confirmed `0199e3b1...@0d37d9b0` is titled `1.2 - Engaging the Customer (Auto Scored)`.
- Found two Chat Quality Form template IDs in PostgreSQL; matching ClickHouse rows use `019e6964-1a14-72ea-b11d-54c2b7951e10`.
- Ran a latest-row breakdown before applying `includeNaScored: false`: Emma has 44 submitted/manually scored Chat Quality Form scorecards and 7 submitted/manually scored Engaging the Customer scorecards.
- All 44 Chat Quality Form rows have aggregate `score < 0`; all 7 Engaging rows have `score >= 0`.
- Therefore, both aggregate and detail APIs consistently return only the seven Engaging rows because both requests set `includeNaScored: false`.
- Also found a separate frontend risk in `ManagerLeaderboard.tsx`: repeated per-day rows are assigned rather than accumulated. This does not explain the template-specific omission from both APIs.

## Evidence

- Prior CONVI-6968 knowledge records that `QAAttribute.users/groups` filter `agent_user_id`, while `scorecard_reviewer_audience` filters `submitter_user_id`.
- The target Chat Quality Form is reported to be manual-only, so auto-score exclusion cannot by itself explain the missing scorecard.
- `retrieve_qa_score_stats_clickhouse.go` selects `scorecard_d` when `scoreResource` is scorecard-level and counts `COUNT(DISTINCT scorecard_id)`.
- `common_clickhouse.go` maps the request to `scorecard_time`, `usecase_id`, `submitter_user_id`, `scorecard_submit_time <> 0`, and `score >= 0`.
- The production-shaped ClickHouse result is:
  - `2026-07-14 04:00:00` — 3
  - `2026-07-17 04:00:00` — 3
  - `2026-07-20 04:00:00` — 1
  - Expected period total — 7
- `ManagerLeaderboard.tsx` assigns rather than accumulates `groupResult.totalScorecardCount` for repeated manager rows.

## Conclusion

Scenario 3 is correct. There are more than seven manually evaluated scorecards: 51 total under the supplied scope. The APIs return seven because `includeNaScored: false` excludes the 44 Chat Quality Form scorecards whose aggregate score is N/A/negative. The seven returned records correctly belong to Engaging the Customer; the initially supplied template ID was misidentified.

## Score correctness case study

Conversation `84608747-b451fdca-43d8-448f-b35b-9d5bf8f02ac0` has two Postgres scorecards for Chat Quality Form revision `b678cc4d`:

- `019f3e8e-4e65-711a-a663-b05be3ce3fb6`
- `019f3e97-a65f-74a9-8b70-f7c93f377833`

Both Postgres scorecard rows are submitted by Emma, manually scored, and have `score = NULL`.

The template revision has one sentence criterion with weight 1 and eight labeled-radio scoring criteria with weight 0. The sentence criterion has text but does not produce a numeric percentage. The scored radio criteria produce valid percentages, but their total scoring weight remains zero. `ComputeScores` calls `calculationSummary.computePercentage`, which returns `scored=false` when total weight is zero, so Postgres correctly stores the overall score as NULL/N/A.

ClickHouse score representation:

- `scorecard_d` stores Postgres NULL score as `-1`.
- For `019f3e8e...`, ClickHouse is fully aligned with Postgres: submitted, Emma as submitter, manually scored, and score `-1`.
- For `019f3e97...`, criterion-level `score_d` is current and aligned with Postgres at update `2026-07-07 22:02:16.192882`, including submission metadata and score `-1`.
- However, scorecard-level `scorecard_d` is stale at update `2026-07-07 22:01:21.807790`: it still shows draft submission time, empty submitter, and score `-1`. The submitted scorecard-level update is absent on all three replicas, so this is not replica lag.

This case therefore shows two independent facts:

1. N/A overall score is correct under the configured zero-weight scoring model.
2. One scorecard has a real Postgres-to-ClickHouse `scorecard_d` projection gap, which would exclude it from scorecard-level submitted queries even if `includeNaScored` were enabled.

## Next steps

1. Confirm product semantics: should `Scorecards evaluated` include submitted scorecards with N/A aggregate score?
2. If yes, set `includeNaScored: true` for Manager aggregate and detail requests and validate 51.
3. Investigate/reindex the missing submitted `scorecard_d` version for `019f3e97-a65f-74a9-8b70-f7c93f377833` and determine how many of the 44 Chat Quality Form scorecards have similar projection gaps.
4. Separately test/fix daily-row accumulation in the Manager table.
