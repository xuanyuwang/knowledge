# CONVI-6494: Leaderboard "Raises Answered %" always 0%

**Created:** 2026-03-24
**Updated:** 2026-05-07
**Status:** Root cause still verified; current uncommitted worktree change only updates the SQL fixture, not the committed production implementation
**Ticket:** https://linear.app/cresta/issue/CONVI-6494

## Overview

The "Raises Answered %" column on the Leaderboard Agent table is always 0% for all customers. Pre-existing bug in `liveAssistStatsClickHouseQuery` since CONVI-2891.

## Root Cause

The `raised_hands_and_whispers` CTE groups by `manager_user_id`:

```sql
GROUP BY 1,2,3,4  -- conversation_start_time, conversation_id, agent_user_id, manager_user_id
```

But `manager_user_id` is only populated on whisper events (action_type=12), never on raise hand events (action_type=7). This splits them into different groups, making `has_raised_hand = 1 AND has_whisper = 1` always false.

| action_type | manager_user_id |
|-------------|-----------------|
| 7 (raise hand) | always empty (29,104/29,104 on Brinks) |
| 12 (whisper) | always populated (30,819/30,819 on Brinks) |

## Fix

Split into two CTEs — one for agent stats (no `manager_user_id` in GROUP BY) and one for manager stats (no `agent_user_id` in GROUP BY):

- `agent_raised_hands_and_whispers`: GROUP BY conversation_start_time, conversation_id, agent_user_id
- `manager_raised_hands_and_whispers`: GROUP BY conversation_start_time, conversation_id, manager_user_id

Verified on Brinks care-voice March 2026:
- Before fix: raised_hand_answered_count = 0
- After fix: raised_hand_answered_count = 2193 (~90% of 2442 raised hands)

## Current Uncommitted State (2026-05-07)

The active worktree `/Users/xuanyu.wang/repos/go-servers-convi-6494` has one uncommitted change:

- `insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveLiveAssistStats_ExtTable_WithUsecaseFilter_request.sql`

That diff rewrites the fixture from one `raised_hands_and_whispers` CTE into separate `agent_raised_hands_and_whispers` and `manager_raised_hands_and_whispers` CTEs. No corresponding committed production-query change was part of this wrap-up pass, so the branch should be treated as partially staged work.

## Log History

| Date | Summary |
|------|---------|
| 2026-03-24 | Discovered while verifying CONVI-6476. Root cause identified, fix verified on CH. |
| 2026-05-07 | Wrapped up the reopened worktree state: recorded that the only dirty change is the SQL fixture update, and that production implementation still needs to be carried through intentionally |
