# CONVI-6242: cron-label-conversations Stale Labels Fix

**Created:** 2026-02-12
**Updated:** 2026-02-12
**Ticket:** [CONVI-6242](https://linear.app/cresta/issue/CONVI-6242)

## Problem

`cron-label-conversations` labels conversations while still open (filters by `created_at`). When a conversation is re-assigned to a different agent after labeling, `conversation_with_labels_d` retains the stale `agent_user_id` and `usecase_id` from the first assignment. This causes Agent Leaderboard "Active Days" to show 0 instead of 1 (observed for Corinne Harmon / Alaska Air).

## Root Cause

The cron filters by `chats.created_at`, so it processes conversations that are still in progress. If the conversation is later re-assigned, the already-written row in ClickHouse has the wrong `agent_user_id`/`usecase_id`.

## Fix: Filter by `ended_at` instead of `created_at`

Only label **closed** conversations where agent/usecase are stable. Since a conversation can only end once, `conversation_end_time` and `agent_user_id` are final at that point. Each conversation is written exactly once — no duplicates, no stale data.

**Files:** `go-servers/cron/sync-users/internal/conversation-agent-assistance/task.go`

### `findConversations` (line ~311)

```go
// Before:
Where("chats.created_at >= ? AND chats.created_at < ?", startTime, endTime).

// After:
Where("chats.ended_at >= ? AND chats.ended_at < ?", startTime, endTime).
Where("chats.ended_at IS NOT NULL").
```

### `setTimeRange` watermark (line ~270)

The watermark query must match the new filter column — use `conversation_end_time` as the high watermark since that's what `ended_at` maps to in ClickHouse.

```go
// Before:
"SELECT max(conversation_start_time) AS start_time FROM conversation_with_labels_d"

// After:
"SELECT max(conversation_end_time) AS start_time FROM conversation_with_labels_d"
```

### `findEventTimeRange` — use `EndTime` for upper bound

The event fetch window upper bound was `maxConvStartTime + 3h`. With `ended_at` filtering, a batch can contain long-running conversations whose `EndTime` is much later than `StartTime`. Online events between `maxConvStartTime + 3h` and the conversation's `EndTime` would be missed.

```go
// Before: upper bound based on max StartTime
if i == 0 || c.StartTime.After(maxConvStartTime) {
    maxConvStartTime = c.StartTime
}

// After: upper bound based on max EndTime
if i == 0 || c.EndTime.After(maxConvEndTime) {
    maxConvEndTime = c.EndTime
}
```

### Comment/error message updates

- Line ~42: `create_time` → `end_time`
- Line ~273: error message `"max convo start time"` → `"max convo end time"`
- Line ~289: `"find conversations whose create_time"` → `"find closed conversations whose ended_at"`

## Why ClickHouse Schema Migration Is NOT Needed

We initially planned to change the ORDER BY from `toStartOfHour(conversation_end_time)` to `toStartOfHour(conversation_start_time)` to prevent duplicate rows via ReplacingMergeTree deduplication.

**This is unnecessary** because:

1. The `ended_at` filter ensures the cron only processes **closed** conversations
2. A conversation can only end once → `conversation_end_time` and `agent_user_id` are final
3. Each conversation is written exactly once → no duplicates to deduplicate
4. Re-runs/backfills produce identical ORDER BY tuples → ReplacingMergeTree handles them correctly

The ORDER BY change to `conversation_start_time` could still be a **performance optimization** (the agent stats query filters by `conversation_start_time`, so a matching PRIMARY KEY would benefit reads), but it's not needed for correctness and can be evaluated separately.

clickhouse-schema [PR #172](https://github.com/cresta/clickhouse-schema/pull/172) was closed with this reasoning.

## Test Changes

**File:** `go-servers/cron/sync-users/internal/conversation-agent-assistance/task_test.go`

- `TestSetTimeRange` "recent" subtest: Split `mockTime` into `mockStartTime`/`mockEndTime`, assertion now compares to `mockEndTime` (the new watermark)
- `TestSetTimeRange` "far" subtest: Updated debug queries to `max(conversation_end_time)`
- `TestFindConversations`: No changes needed — `TestChatTemplate.EndedAt = 2020-01-02` (Valid=true) passes both `ended_at >= zero_time` and `IS NOT NULL`

## Pull Requests

| Repo | PR | Status | Description |
|------|----|--------|-------------|
| go-servers | [#25706](https://github.com/cresta/go-servers/pull/25706) | Merged | Filter by `ended_at`, update watermark, fix event window |
| clickhouse-schema | [#172](https://github.com/cresta/clickhouse-schema/pull/172) | Closed | ORDER BY change — not needed for correctness |

## Implementation Status

| Component | Status |
|-----------|--------|
| `task.go` — `ended_at` filter + watermark + event window fix | Done — go-servers [#25706](https://github.com/cresta/go-servers/pull/25706) |
| `task_test.go` — test updates | Done — go-servers [#25706](https://github.com/cresta/go-servers/pull/25706) |
| ClickHouse schema migration | Not needed |
