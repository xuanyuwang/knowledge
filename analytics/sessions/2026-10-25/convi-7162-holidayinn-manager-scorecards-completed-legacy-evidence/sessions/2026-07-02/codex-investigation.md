# Codex Investigation - CONVI-7162

**Date:** 2026-07-02  
**Ticket:** CONVI-7162  
**Source repo:** `/Users/xuanyu.wang/repos/director`  
**Branch/worktree:** `main` at `/Users/xuanyu.wang/repos/director`  
**Related source repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/cresta-proto`  
**Related knowledge project:** `/Users/xuanyu.wang/repos/knowledge/convi-6968-schwab-leaderboard-launch`

## Input

User asked to investigate only for:

`https://linear.app/cresta/issue/CONVI-7162/holiday-inn-club-vacations-manager-leaderboard-scorecards-completed`

The user also pointed to the prior CONVI-6968 knowledge project as the source of engineering design detail for the `Scorecards Completed` column.

## Linear Ticket Summary

CONVI-7162 reports that Holiday Inn Club Vacations Manager Leaderboard `Scorecards Completed` daily counts are incorrect or redistributed across days and instances.

Primary example:

- Manager: Cliff Hawker
- Prior week 2026-06-15 through 2026-06-21 expected 10 total, 2 per day.
- Leaderboard showed 2 Mon, 3 Tue, 1 Wed, 2 Thu, 0 Fri, total 8.
- Current week 2026-06-22 through 2026-06-24 expected 2 Monday and 2 Tuesday, but leaderboard showed 1 each day.

Prior occurrences noted in the ticket were resolved by backfill, but this recurrence is active week over week.

## Prior CONVI-6968 Context Reviewed

Important files:

- `/Users/xuanyu.wang/repos/knowledge/convi-6968-schwab-leaderboard-launch/README.md`
- `/Users/xuanyu.wang/repos/knowledge/convi-6968-schwab-leaderboard-launch/deliverables/api-decision-table.md`
- `/Users/xuanyu.wang/repos/knowledge/convi-6968-schwab-leaderboard-launch/deliverables/fe-engineering-work.md`
- `/Users/xuanyu.wang/repos/knowledge/auto-backfill-missing-scorecards/scorecard-count-api-mismatch.md`

Relevant prior findings:

- Old Manager `Scorecards completed` used `RetrieveScorecardStats`.
- `RetrieveScorecardStats` was changed in go-servers PR for CONVI-6968 to be submitter-attributed and submit-time-filtered.
- Its query intentionally rewrites:
  - `scorecard_time` -> `scorecard_submit_time`
  - `agent_user_id` -> `submitter_user_id`
- The QA APIs support submitter filtering/grouping but still use QA score time semantics.
- Prior docs explicitly call out that QA APIs using `scorecard_time` are not equivalent to submit-time metrics.

## Current Source Findings

### Current Manager aggregate uses QA stats, not old scorecard stats

Current file:

`/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/leaderboard/manager-leaderboard/ManagerLeaderboardPage.tsx`

Key current behavior:

- Lines 113-124 build `managerScorecardFiltersState` with `scorecardStatus = [MANUALLY_SUBMITTED]` and `scoreResource = QA_SCORE_RESOURCE_SCORECARD`.
- Lines 133-149 add `scorecardReviewerAudience.users = filterByManagerUsers.userNames` and group by `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER`.
- Lines 163-168 call `useQAScoreStats(...)` for both aggregate and time-range scorecard stats.

Current file:

`/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/leaderboard/manager-leaderboard/ManagerLeaderboard.tsx`

Key current behavior:

- Lines 192-204 iterate `scorecardStats.data?.qaScoreResult.scores`.
- The displayed Manager row field is `groupResult.totalScorecardCount`.

Current file:

`/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/leaderboard/leaderboard-by-metric/manager-leaderboard-by-metric/hooks/useLeaderboardByMetricDataForManagers.tsx`

Key current behavior:

- Lines 223-247 build daily Scorecards Completed metric cells from `scorecardStatsByTimeRange?.data?.qaScoreResult.scores`.
- Daily cell keys use `groupResult.groupedBy.intervalStart`.

### Current Manager drawer also uses QA conversations

Current file:

`/Users/xuanyu.wang/repos/director/packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useManagerScorecardTemplateBreakdown.ts`

Key current behavior:

- Lines 38-52 build submitted-only QA conversation filters.
- Lines 55-70 add `scorecardReviewerAudience` for the selected manager.
- Lines 72-90 fetch all QA conversations and group them by scorecard template.

### Commit that changed Manager from old stats/ListScorecards to QA APIs

`director` commit:

`2a0670e36afe16ac3a6cd4a69914cb339705a973 Use QA APIs for manager leaderboard scorecards`

This commit changed:

- Manager aggregate from `ScorecardStatsGroupResult.averageScorecardCompletedPerUser` to `QAScoreStatsResponse.qaScoreResult.scores[].totalScorecardCount`.
- Manager drawer from `ListScorecards` with submit-time filters to `RetrieveQAConversations`.

This is part of merged PR commit:

`6ec3409229 CONVI-6968 Leaderboard scorecard breakdown (#19346)`

### QA stats time semantics

Current file:

`/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/util-hooks/useQAScoreStatsRequestParams.ts`

Key current behavior:

- Lines 32-44 build the request with `filterByTimeRange` from `filtersState.submitDateRange`.
- Line 42 passes `conversationTimeRangeField: filtersState.dateRangeTarget`.
- The Manager code does not set a submit-time target because no such target exists.

Current proto:

`/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/analytics/analytics_service.proto`

Key current behavior:

- `RetrieveQAScoreStatsRequest.filter_by_time_range` is `[start, end)`.
- `conversation_time_range_field` defaults to `TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_STARTED_AT`.
- `TargetFieldForTimeRange` only supports conversation started or conversation ended, not scorecard submitted time.

Current backend:

`/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go`

Key current behavior:

- `scorecardTable` maps `conversationStartTimeColumn` to `scorecardTimeColumn`.
- Therefore default QA scorecard-table time filtering/grouping uses `scorecard_time`.
- `scorecardReviewerAudience` filters `submitter_user_id`.
- `MANUALLY_SUBMITTED` status filters `scorecard_submit_time <> 0`, but does not make the time range use `scorecard_submit_time`.

Current backend:

`/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`

Key current behavior:

- `RetrieveQAScoreStats` uses `scorecardTable` when `scoreResource = QA_SCORE_RESOURCE_SCORECARD`.
- It builds common time conditions with `parseCommonConditionsForQAAttribute(... WithTimeRangeType(req.ConversationTimeRangeField))`.
- It groups time using `parseClickhouseGroupByForQAAttribute(... WithTimeRangeType(req.ConversationTimeRangeField))`.
- It counts `COUNT(DISTINCT scorecard_id) AS total_scorecard_count`.

### Old scorecard stats path had the desired submit-time semantics

Current backend:

`/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_scorecard_stats_clickhouse.go`

Key current behavior:

- Lines 35-53 build the scorecard stats query by replacing:
  - `scorecard_time` with `scorecard_submit_time`
  - `agent_user_id` with `submitter_user_id`
- This path was intentionally changed by go-servers commit:
  - `d8dca2b39a CONVI-6968: attribute scorecard stats to submitter (#28383)`

## Interpretation

The customer expectation in CONVI-7162 is submit-time daily completion counts:

> Cliff completed exactly 2 scorecards per day, so the leaderboard should reflect 2 per day.

The current Manager Leaderboard implementation instead answers:

> How many manually submitted scorecards, submitted by this manager, have scorecard_time in the selected day/week?

For conversation scorecards, `scorecard_time` corresponds to conversation time rather than scorecard submission time. That naturally creates the reported behavior:

- Scorecards submitted Friday for earlier conversations can appear on earlier days.
- Scorecards submitted in the selected window for older conversations can be missing from that window.
- Daily distribution can show 3 on Tuesday and 1 on Wednesday even if the manager submitted 2 each day.
- Totals can be lower than submitted counts for the selected week.

## PG/CH Validation

After the source-code pass, the user approved running read-only PG/CH queries for more evidence.

### User and scope

Resolved Cliff Hawker in the likely Holiday Inn profiles:

| Profile | User ID | Username | Active State | Roles |
|---|---|---|---:|---|
| `transfers-voice` | `9d654376ad4f1cdc` | `chawker@holidayinnclub.com` | 1 | `{2,8,14,15}` |
| `voice` | `9d654376ad4f1cdc` | `chawker@holidayinnclub.com` | 2 | `{2,8,14,15}` |
| `club-voice` | `9d654376ad4f1cdc` | `chawker@holidayinnclub.com` | 2 | `{2,8,14,15}` |

`holidayinn-voice` and `holidayinn-club-voice` had no scorecards submitted by this user in the checked 2026-06-15 through 2026-06-28 ET window. The concrete reproduction is `holidayinn/transfers-voice`.

### PG submitted-time source of truth

PG query against `director.scorecards` for:

- customer `holidayinn`
- profile `transfers-voice`
- `submitter_user_id = '9d654376ad4f1cdc'`
- `submitted_at >= 2026-06-15 00:00 America/New_York`
- `submitted_at < 2026-06-29 00:00 America/New_York`

Result grouped by ET submit day and template:

| Submitted Day ET | Template | PG Submitted Count |
|---|---|---:|
| 2026-06-15 | Ride Along Template | 2 |
| 2026-06-16 | Ride Along Template | 2 |
| 2026-06-17 | Ride Along Template | 2 |
| 2026-06-18 | Ride Along Template | 2 |
| 2026-06-19 | Ride Along Template | 2 |
| 2026-06-22 | Ride Along Template | 2 |
| 2026-06-23 | Ride Along Template | 2 |
| 2026-06-24 | Ride Along Template | 2 |
| 2026-06-25 | Ride Along Template | 2 |
| 2026-06-26 | Ride Along Template | 2 |

Template details:

- title `Ride Along Template`
- template id `f00391f9-c9f8-4bd4-885a-3bcad260817c`
- revision `9be08011`

### CH existence and timestamp comparison

Queried `holidayinn_transfers_voice.scorecard_d FINAL` for the exact 20 PG `resource_id` values.

Result:

| Metric | Value |
|---|---:|
| distinct scorecards present | 20 |
| physical rows | 20 |

Therefore, the primary sample is not a missing-row/backfill problem.

For each exact scorecard, CH `scorecard_submit_time` matches PG submitted time, while `scorecard_time` often points to the earlier conversation time:

| Submitted ET | Scorecard Time ET | Scorecard ID |
|---|---|---|
| 2026-06-15 18:42 | 2026-06-14 15:49 | `019ecd72-28c5-7253-a649-6c7456dc8c32` |
| 2026-06-15 18:45 | 2026-06-14 15:17 | `019ecd75-4f91-77fd-9bb0-472e36a90a1a` |
| 2026-06-16 16:58 | 2026-06-15 18:28 | `019ed239-3d51-7683-9a8a-f4efcfeb4b83` |
| 2026-06-16 17:01 | 2026-06-15 17:22 | `019ed23c-4c5a-702a-a5d3-08376e8197ab` |
| 2026-06-17 18:16 | 2026-06-16 17:27 | `019ed7a7-1717-731a-bede-c86025bb6354` |
| 2026-06-17 18:18 | 2026-06-16 17:11 | `019ed7a9-aed4-76ed-8877-3472a360b931` |
| 2026-06-18 17:57 | 2026-06-18 17:40 | `019edcbc-141c-7356-bf6d-e6e02914c516` |
| 2026-06-18 18:39 | 2026-06-16 17:09 | `019edce2-b51f-715b-b194-634f158015dd` |
| 2026-06-19 16:55 | 2026-06-17 18:56 | `019ee1a8-5dd3-751b-8d4b-af8aba77b436` |
| 2026-06-19 16:59 | 2026-06-18 17:53 | `019ee1ac-f352-753b-a288-975033d0dbc3` |
| 2026-06-22 18:20 | 2026-06-20 16:44 | `019ef16a-c0e2-75e2-8bc6-5b22e0b32508` |
| 2026-06-22 18:22 | 2026-06-20 16:27 | `019ef16c-6f33-722a-b12c-58100edd3cd6` |
| 2026-06-23 18:48 | 2026-06-23 13:23 | `019ef6aa-414e-772f-9302-e7d6aea2a255` |
| 2026-06-23 18:52 | 2026-06-22 18:38 | `019ef6ae-b448-75f5-9bc2-47b296ad5cf0` |
| 2026-06-24 18:21 | 2026-06-24 17:10 | `019efbb8-d4aa-754c-9abe-2c6028318b90` |
| 2026-06-24 18:27 | 2026-06-24 14:52 | `019efbbd-64ca-76f3-8a03-8f8b75e70689` |
| 2026-06-25 17:42 | 2026-06-23 17:37 | `019f00ba-2b20-748b-9d33-a86d64a3c811` |
| 2026-06-25 17:46 | 2026-06-25 15:17 | `019f00bc-bcf8-73de-ab74-cfa207db68ef` |
| 2026-06-26 11:56 | 2026-06-25 17:39 | `019f04a3-afd6-7431-a32a-57a900f8a858` |
| 2026-06-26 12:11 | 2026-06-25 18:09 | `019f04b0-fd98-7648-b045-0b0167864e2a` |

### Aggregates

CH grouped by `scorecard_submit_time` for the same Cliff/Ride Along scorecards matches PG exactly:

| Submit Day ET | Count |
|---|---:|
| 2026-06-15 | 2 |
| 2026-06-16 | 2 |
| 2026-06-17 | 2 |
| 2026-06-18 | 2 |
| 2026-06-19 | 2 |
| 2026-06-22 | 2 |
| 2026-06-23 | 2 |
| 2026-06-24 | 2 |
| 2026-06-25 | 2 |
| 2026-06-26 | 2 |

The same set grouped by `scorecard_time` is redistributed:

| Scorecard Time Day ET | Count |
|---|---:|
| 2026-06-14 | 2 |
| 2026-06-15 | 2 |
| 2026-06-16 | 3 |
| 2026-06-17 | 1 |
| 2026-06-18 | 2 |
| 2026-06-20 | 2 |
| 2026-06-22 | 1 |
| 2026-06-23 | 2 |
| 2026-06-24 | 2 |
| 2026-06-25 | 3 |

Current Leaderboard-emulation query for 2026-06-15 through 2026-06-21 ET using `scorecard_time` range, Cliff submitter, Ride Along template, and submitted status:

| Leaderboard Day ET | Count |
|---|---:|
| 2026-06-15 | 2 |
| 2026-06-16 | 3 |
| 2026-06-17 | 1 |
| 2026-06-18 | 2 |
| 2026-06-20 | 2 |

For weekday display, this is exactly the ticket's reported pattern: Mon 2, Tue 3, Wed 1, Thu 2, Fri 0. The two scorecards submitted on Friday 2026-06-19 are counted under Wednesday and Thursday by `scorecard_time`.

For the current-week Monday/Tuesday submissions only (`scorecard_submit_time` 2026-06-22 through 2026-06-23 ET), grouping by `scorecard_time` gives:

| Scorecard Time Day ET | Count |
|---|---:|
| 2026-06-20 | 2 |
| 2026-06-22 | 1 |
| 2026-06-23 | 1 |

This explains the ticket's "only 1 each day" for Monday and Tuesday despite 2 submitted each day.

## Data-Sync Alternative

The ticket references prior backfill-resolved Holiday Inn issues, so missing ClickHouse rows were a possible secondary cause before validation.

For the primary Cliff Hawker sample, data sync is ruled out:

- All 20 exact PG scorecards exist in `scorecard_d FINAL`.
- CH `scorecard_submit_time` matches PG `submitted_at`.
- The observed mismatch is fully explained by `scorecard_time` grouping.

Other affected managers/profiles could still have independent sync problems, but the reported Cliff Hawker reproduction does not require a backfill.

## Recommended Validation

Completed for the primary Cliff Hawker `holidayinn/transfers-voice` sample.

Optional broader validation:

1. Ask Support for additional affected manager names and exact profiles.
2. Run the same PG submit-time vs CH scorecard-time comparison.
3. If any sample has PG rows absent from CH, handle that separately as sync/backfill.

## Fix Direction

For the ticket's customer expectation, restore submit-time semantics for Manager `Scorecards completed`.

Options:

1. Revert Manager aggregate to `RetrieveScorecardStats` and Manager drawer to `ListScorecards` using `submitterUserNames + startSubmitTime/endSubmitTime`.
2. Add an explicit submit-time range target to QA scorecard APIs, then keep the QA implementation but request submit-time filtering/grouping for this Manager metric.

Option 1 is narrower. Option 2 is cleaner if the team wants Manager scorecard drawer/filter parity inside QA APIs long term, but it requires proto/backend work.

## Commands / Artifacts Reviewed

- `mcp__linear.get_issue` for CONVI-7162.
- `rg` and `nl -ba` over `director`, `go-servers`, and `cresta-proto`.
- `git show` for director commit `2a0670e36a`.
- `git show` for go-servers commit `d8dca2b39a`.
- Prior knowledge docs under CONVI-6968 and auto-backfill scorecard investigations.

## Continuation Check

Resumed the investigation after handoff and re-read the saved CONVI-7162 session, README, daily log, and the CONVI-6968 Manager tab API analysis.

Additional confirmation from live repo state:

- `director/main` still builds Manager `Scorecards completed` through `useQAScoreStats` with `QA_ATTRIBUTE_STRUCTURE_BY_SCORECARD_SUBMITTER` and `QA_ATTRIBUTE_STRUCTURE_BY_SCORECARD_SUBMITTER_AND_TIME`.
- `ManagerLeaderboard.tsx` still displays `groupResult.totalScorecardCount` from `qaScoreResult.scores`.
- The Manager template breakdown drawer still uses `RetrieveQAConversations` through `useRetrieveAllQAConversations` with selected manager in `scorecardReviewerAudience`.
- `go-servers/main` still maps the scorecard table's conversation-start time column to `scorecard_time`; submitted-only status only filters `scorecard_submit_time <> 0`.

No additional PG/CH query was needed because the existing read-only evidence already proves the primary Cliff Hawker sample:

- PG submitted counts match CH grouped by `scorecard_submit_time`.
- All exact PG scorecards exist in CH.
- CH grouped by `scorecard_time` reproduces the ticket's daily counts.

Conclusion remains unchanged: the active bug is semantic, not sync/backfill, for the primary reproduction.
