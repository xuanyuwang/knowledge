# CONVI-6476: Leaderboard broken - Hands Raised and associated metrics are all N/A

**Created:** 2026-03-24
**Updated:** 2026-03-24
**Status:** Fixed (PR #26519)
**Ticket:** https://linear.app/cresta/issue/CONVI-6476

## Overview

All three live assist columns on the Leaderboard "Agent leaderboard" table are N/A:
- Hands Raised
- Raises Answered %
- Live assist (received)

The `retrieve_live_assist_stats` API returns an empty response while other APIs (retrieve_hint_stats, retrieve_agent_stats) return data with the same filters.

## Root Cause

**Args misalignment in `liveAssistStatsClickHouseQuery` when ext tables are used.**

### The Bug

File: `insights-server/internal/analyticsimpl/retrieve_live_assist_stats_clickhouse.go:25`

```go
c.args = append(c.args, c.arg)
```

This line is meant to duplicate the positional arg for the duplicated `OR manager_user_id IN (?)` condition. It works correctly with inline `IN (?)` args (where `c.arg` is a `[]string`), but with ext tables:

1. `c.arg = nil` (ext table uses `IN (SELECT user_id FROM agent_filter)`, no `?` placeholder)
2. `c.args = append(nil, nil)` creates `[nil]`
3. In `concatConditionsAndArgs`:
   - `c.arg = nil` is skipped (guarded by `if conditionAndArg.arg != nil`)
   - `c.args = [nil]` has len > 0, so **`nil` IS appended to the positional args list**
4. This extra `nil` shifts all subsequent `?` bindings
5. The usecase filter (`usecase_id IN (?)`) receives `nil` instead of `["care-voice"]`
6. `usecase_id IN (NULL)` matches nothing -> 0 rows -> empty response

### Why It Started Happening

Three changes combined to trigger this:

| Change | Date | Effect |
|--------|------|--------|
| CONVI-6372 (#26178) | Mar 11 | Added ext table support for user filter |
| CONVI-6316 (#26250) | Mar 12 | Always pass FinalUsers via ext table when `ENABLE_EXT_TABLE_FOR_USER_FILTER=true` (previously `ShouldQueryAllUsers=true` skipped the user filter) |
| `ENABLE_EXT_TABLE_FOR_USER_FILTER` | Recently | Enabled on all environments |

Before CONVI-6316, `ShouldQueryAllUsers=true` (ACL disabled + no user/group filter) meant no user filter was added to the ClickHouse query, so the condition modification code in `liveAssistStatsClickHouseQuery` never ran. After CONVI-6316 + ext table enabled, users are always passed via ext table, the modification runs, and the nil arg causes misalignment.

### Why Only Live Assist Stats

The `c.args = append(c.args, c.arg)` pattern is unique to `liveAssistStatsClickHouseQuery`. It's the only query builder that modifies `conditionAndArg` structs before calling `concatConditionsAndArgs`. Other APIs (agent stats, hint stats, etc.) don't do this modification, so they're unaffected.

The `scorecardStatsClickhouseQuery` does a similar `agent_user_id -> creator_user_id` rename, but it operates on the final SQL string AFTER `concatConditionsAndArgs`, so it doesn't have this issue.

## Fix

Guard the arg duplication against nil:

```go
// Before (broken with ext tables)
c.args = append(c.args, c.arg)

// After (safe)
if c.arg != nil {
    c.args = append(c.args, c.arg)
}
```

## Query Construction Walkthrough

This traces `liveAssistStatsClickHouseQuery` line by line using the **ext table + usecase filter** case.

### Input

```
conditionsAndArgs[actionAnnotationTable] = [
  {condition: "conversation_start_time >= ?",                          arg: "2021-01-01"},
  {condition: "conversation_start_time < ?",                           arg: "2021-01-10"},
  {condition: "agent_user_id IN (SELECT user_id FROM agent_filter)",   arg: nil},
  {condition: "usecase_id IN (?)",                                     arg: ["care-voice"]},
]
groupByKeys = ["truncated_time", "agent_user_id"]
```

### Step-by-step (lines 17-31)

**Line 18:** `groupBy := concatKeys(groupByKeys)`
> `"truncated_time, agent_user_id"`

**Line 19:** `aaConditions := conditionsAndArgs[actionAnnotationTable]`
> Grabs the 4-element slice.

**Lines 20-30: Loop over conditions**

| i | condition | contains `agent_user_id`? | what happens |
|---|-----------|--------------------------|--------------|
| 0 | `conversation_start_time >= ?` | no | skipped |
| 1 | `conversation_start_time < ?` | no | skipped |
| 2 | `agent_user_id IN (SELECT user_id FROM agent_filter)` | **yes** | see below |
| 3 | `usecase_id IN (?)` | no | skipped |

**i=2 in detail:**

1. **Line 23:** `cm = "manager_user_id IN (SELECT user_id FROM agent_filter)"` (replaced `agent_user_id` → `manager_user_id`)
2. **Line 24:** `c.condition = "agent_user_id IN (SELECT user_id FROM agent_filter) OR manager_user_id IN (SELECT user_id FROM agent_filter)"`
3. **Line 25-27 (the fix):** `if c.arg != nil` → `c.arg` is **nil** → **skip** the append. No arg duplication needed because the ext table subquery has no `?` placeholder.
4. **Line 28:** Write back modified condition.

**Line 31:** `aaCondition, aaArgs := concatConditionsAndArgs(aaConditions)`

`concatConditionsAndArgs` walks each element:

| i | condition wrapped in `()` | arg | args field | effect on aaArgs |
|---|--------------------------|-----|------------|------------------|
| 0 | `(conversation_start_time >= ?)` | `"2021-01-01"` (non-nil → append) | nil | `["2021-01-01"]` |
| 1 | `(conversation_start_time < ?)` | `"2021-01-10"` (non-nil → append) | nil | `["2021-01-01", "2021-01-10"]` |
| 2 | `(agent_user_id IN (...) OR manager_user_id IN (...))` | nil → skip | nil (len=0) → skip | unchanged |
| 3 | `(usecase_id IN (?))` | `["care-voice"]` (non-nil → append) | nil | `["2021-01-01", "2021-01-10", ["care-voice"]]` |

**Result:** `aaArgs = ["2021-01-01", "2021-01-10", ["care-voice"]]` — 3 args, no nil. The `?` placeholders in the final SQL match 1:1.

### What broke before the fix

Without `if c.arg != nil` at i=2:
1. `c.args = append(nil, nil)` → `c.args = [nil]`
2. `concatConditionsAndArgs`: `c.arg` nil → skipped, but `len(c.args) > 0` → appended `nil`
3. **aaArgs = ["2021-01-01", "2021-01-10", nil, ["care-voice"]]** — 4 args
4. The `nil` consumed the first `?` for the usecase filter → `usecase_id IN (NULL)` → 0 rows

### Inline args path (no ext table)

When ext table is disabled, i=2 looks like:
- `c.condition = "agent_user_id IN (?)"`, `c.arg = ["user1", "user2"]`
- After modification: `c.condition = "agent_user_id IN (?) OR manager_user_id IN (?)"` — two `?` placeholders
- `c.arg != nil` → `c.args = append(nil, ["user1", "user2"])` → `c.args = [["user1", "user2"]]`
- `concatConditionsAndArgs`: appends `c.arg` (original) + `c.args[0]` (duplicate) → two copies of user IDs for the two `?` placeholders

## Testing Ext Table Queries in ClickHouse Client

The `agent_filter` ext table only exists in-memory during the Go client's binary protocol request. To manually test the query in a ClickHouse client, prepend a CTE using `arrayJoin`:

```sql
WITH agent_filter AS (
    SELECT arrayJoin(['user1', 'user2', 'user3']) AS user_id
),
raised_hands_and_whispers AS (
    -- ... rest of the query unchanged, keeps IN (SELECT user_id FROM agent_filter) intact
)
```

## Verification

ClickHouse has data for Brinks care-voice in the date range:
- 4249 AGENT_RAISE_HAND events across 541 conversations from 88 agents
- 4456 MANAGER_WHISPER events across 456 conversations from 86 agents
- Usecase `care-voice` has 8703 events

The query returns data when run directly without the arg misalignment.
