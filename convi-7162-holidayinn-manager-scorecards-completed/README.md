# CONVI-7162 - Holiday Inn Manager Scorecards Completed Counts

> Migrated navigation: [Analytics / Conversation Volume](../analytics/subdomains/conversation-volume/README.md). This folder remains detailed historical evidence.

**Created:** 2026-07-02
**Updated:** 2026-08-19

## Overview

This project tracks the investigation for Linear ticket CONVI-7162: Holiday Inn Club Vacations reports incorrect Manager Leaderboard `Scorecards Completed` daily counts for managers such as Cliff Hawker across `holidayinn-transfers-voice`, `holidayinn-voice`, and related Holiday Inn profiles.

## Current Finding

The root cause is confirmed for the ticket's primary `holidayinn-transfers-voice` Cliff Hawker example: an API semantic mismatch introduced by the CONVI-6968 Manager Leaderboard migration.

Current `director/main` no longer uses the old `RetrieveScorecardStats` response for the Manager `Scorecards completed` table/daily metric. It uses `RetrieveQAScoreStats` with:

- `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER`
- `scorecardReviewerAudience.users = manager resource names`
- `scorecardStatuses = [MANUALLY_SUBMITTED]`
- `scoreResource = QA_SCORE_RESOURCE_SCORECARD`

That QA path counts submitted scorecards, but its time range is still based on `scorecard_time` by default. For conversation scorecards, `scorecard_time` means conversation start time, not submission time. A scorecard submitted on Friday for a Monday conversation can therefore appear under Monday, and a scorecard submitted in the selected week for a conversation outside the week can be absent from that week's daily submitted-count view.

Concrete production evidence that grouping/filtering by `scorecard_submit_time` restores exact daily submission counts for the primary repro is in:

`deliverables/submit-time-exact-match-evidence.md`

Coaching Hub and QM Report (the ticket’s comparison UIs) already count by submit time (`submitted_at`); see:

`deliverables/coaching-hub-qm-report-submit-time-investigation.md`

The product changes are in the official release: the Director `2026-08-13` tag and Aug 12 prod-main deployment contain frontend PR #21534, and deployed go-servers revisions contain backend PR #30635. Config PR [#151886](https://github.com/cresta/config/pull/151886) enabled `filterByScorecardSubmitTime` broadly in production while preserving Schwab and Comcast's separate release flows.

Production verification passed on 2026-08-19. On the official `holidayinn-transfers-voice` `/director/` route, Cliff Hawker's Aug 17 Leaderboard cell now shows 2, and an Aug 17-only filter returns 2 in both the Leaderboard and the Ride Along Template breakdown. The exact customer-reported mismatch (Leaderboard 1 vs details 2) no longer reproduces. See `sessions/2026-08-19/codex-production-flag-retest.md`.

The correction also applies to historical reporting periods without a backfill. The Leaderboard queries the existing historical rows using `scorecard_submit_time`, so reopening an affected range since the issue began recalculates the daily and weekly counts under the submission day. One UI nuance remains: an individual breakdown row can display the conversation timestamp even though the row is included and counted under its submission day. See `sessions/2026-08-24/codex-historical-correction-question.md`.

## Key Evidence

- CONVI-7162 expects `Scorecards Completed` to match scorecard submissions per day.
- The earlier CONVI-6968 knowledge notes warned that QA APIs use `scorecard_time`, while the old Manager `RetrieveScorecardStats` path rewrote the time filter to `scorecard_submit_time`.
- `director` commit `2a0670e36a` changed Manager Leaderboard scorecard counts from old scorecard stats to QA APIs.
- Current `ManagerLeaderboardPage.tsx` builds the Manager scorecard request with submitter grouping and submitted-only status.
- Current `ManagerLeaderboard.tsx` displays `groupResult.totalScorecardCount` from `qaScoreResult.scores`.
- `go-servers` `RetrieveQAScoreStats` maps scorecard-table conversation-start time to `scorecard_time`.
- `RetrieveQAScoreStats` only supports conversation started/ended time-range targeting; it has no submit-time target.
- The old `RetrieveScorecardStats` ClickHouse query explicitly replaces `scorecard_time` with `scorecard_submit_time` and `agent_user_id` with `submitter_user_id`.

## PG/CH Evidence

Read-only production queries on 2026-07-02 for `holidayinn/transfers-voice` found:

- Cliff Hawker user id: `9d654376ad4f1cdc`
- Template: `Ride Along Template`, template id `f00391f9-c9f8-4bd4-885a-3bcad260817c`, revision `9be08011`
- PG `director.scorecards` has exactly 2 submitted scorecards per ET weekday from 2026-06-15 through 2026-06-26 for Cliff as `submitter_user_id`.
- The 20 PG scorecards all exist in ClickHouse `holidayinn_transfers_voice.scorecard_d FINAL`: 20 distinct scorecards, 20 physical rows.
- CH grouped by `scorecard_submit_time` matches PG exactly: 2 per weekday.
- CH grouped by `scorecard_time` reproduces the customer-reported redistribution.

For the ticket's prior week, current Leaderboard semantics (`scorecard_time` in 2026-06-15 through 2026-06-21 ET, submitted status, Cliff submitter, Ride Along Template) return:

| Day ET | Count |
|---|---:|
| 2026-06-15 | 2 |
| 2026-06-16 | 3 |
| 2026-06-17 | 1 |
| 2026-06-18 | 2 |
| 2026-06-19 | 0 |

That matches the ticket's reported weekday pattern: 2 Mon, 3 Tue, 1 Wed, 2 Thu, 0 Fri.

The same 10 scorecards grouped by submit day are 2 each on 2026-06-15, 16, 17, 18, and 19.

For the current-week Monday/Tuesday submissions, the four scorecards submitted on 2026-06-22 and 2026-06-23 ET group by `scorecard_time` as:

| Scorecard Time Day ET | Count |
|---|---:|
| 2026-06-20 | 2 |
| 2026-06-22 | 1 |
| 2026-06-23 | 1 |

This explains why Monday and Tuesday showed only 1 each even though Cliff submitted 2 on each day: two Monday submissions were attached to Saturday conversations.

`holidayinn-voice` and `holidayinn-club-voice` had no Cliff Hawker submitted scorecards in the checked 2026-06-15 through 2026-06-28 ET window.

## Status

The fix is deployed, globally enabled, and verified on the official Holiday Inn production route. The exact Cliff Hawker Aug 17 scenario now returns 2 in both the Manager Leaderboard and its scorecard breakdown.

Chosen fix direction remains: keep the CONVI-6968 QA-backed Manager Leaderboard implementation, and add explicit submit-time filtering/grouping for Manager `Scorecards completed` aggregate and drawer.

**Product decision (2026-07-30, Tinglin Liu):** Restore scorecard **submit time** on Manager Leaderboard. Rationale: pre–CONVI-6968 Leaderboard used submit time; conversation start time was introduced by the CONVI-6968 QA API migration, not an intentional long-standing choice. See `decisions/2026-07-30-manager-leaderboard-submit-time.md`.

## Related Artifacts

- `deliverables/qa-time-range-column-mapping.md` — before/after visualization of how `filter_by_time_range` maps to ClickHouse columns
- `deliverables/staging-walter-dev-golden-sql-verification.md` — voice-staging run of adapted submit-time goldens
- `deliverables/submit-time-exact-match-evidence.md`
- `sessions/2026-07-02/codex-investigation.md`
- `sessions/2026-07-10/codex-implementation.md`
- `log/2026-07-02.md`
- `log/2026-07-03.md`
- `log/2026-07-10.md`
- `/Users/xuanyu.wang/repos/knowledge/convi-6968-schwab-leaderboard-launch`
