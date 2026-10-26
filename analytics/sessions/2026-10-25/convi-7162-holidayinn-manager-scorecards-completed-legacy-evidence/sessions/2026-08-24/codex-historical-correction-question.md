# CONVI-7162 historical correction question

**Date:** 2026-08-24 (America/Toronto)
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** read-only review of merged commits and rollout worktree; no source changes
**Linear comment:** `7a5665e7-2ce7-4979-9811-fd781fa0e76c`

## Customer question

Does the released fix also correct manager scorecard placement for all affected Holiday Inn locations and historical reporting periods since the issue began?

## Findings

- The change is query-time, not a data migration. `RetrieveQAScoreStats` and `RetrieveQAConversations` receive `TimeRangeFilterTarget=SUBMIT_TIME` and filter/group existing ClickHouse rows by `scorecard_submit_time`.
- Historical scorecard rows already contain `scorecard_submit_time`; the primary June repro had all expected rows and exact submit timestamps. No backfill or recompute is required for those rows.
- Reopening an affected historical date range recalculates Manager Leaderboard counts by submission day, so the June-to-rollout reporting periods self-correct within the available reporting window.
- The rollout config added `filterByScorecardSubmitTime: true` to every relevant mapping in the Holiday Inn production customer history file. The production Aug 17 verification confirms the official route consumes the enabled behavior.
- Both the aggregate/daily heatmap and the Manager scorecard breakdown are filtered by submit time.

## Important UI nuance

The scorecard breakdown still displays each item's conversation timestamp. In the verified Aug 17-only result, both submitted scorecards were correctly included and counted on Aug 17, while one row visibly showed an Aug 16 conversation timestamp. Therefore:

- historical daily/weekly reporting counts are corrected by submit day;
- scorecards are not physically rewritten or moved;
- a detail-row date label can still differ because it describes the conversation, not the submission.

If the customer requires the visible detail-row timestamp itself to show submit time, that is separate UI work from CONVI-7162.

## Recommended reply

Yes—for the Manager Leaderboard reporting counts. The fix applies when historical ranges are queried, so scorecards since the issue began are recalculated under their submission day across Holiday Inn's enabled profiles/use cases; no backfill is required. We verified this by filtering Aug 17 only: both Cliff Hawker scorecards appeared in the total and breakdown. One nuance is that an individual breakdown row can still display the conversation timestamp, even though the scorecard is counted under the correct submission day. If the customer needs that displayed row timestamp changed to submission time too, that would be separate UI work.

## Credential hygiene

- No credentials were read or used.
- A fresh historical production UI check was attempted, but the customer session had expired before any customer data loaded.
