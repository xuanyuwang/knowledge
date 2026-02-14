# Agent Stats "Active Days" Fix — FULL OUTER JOIN

**Created**: 2026-02-09
**Updated**: 2026-02-11
**PR**: https://github.com/cresta/go-servers/pull/25613
**Notion Doc**: https://www.notion.so/cresta/3024a587b06180cbace3faa4fd6c8b14
**Linear (stale labels bug)**: https://linear.app/cresta/issue/CONVI-6242
**Related Project**: [convi-6192-conversation-source-config](../convi-6192-conversation-source-config/)

## Overview

After enabling `exclude_ingestion_pipeline_from_agent_stats` for Alaska Air, agents who only had source 8 (INGESTION_PIPELINE) conversations on certain days showed **"N/A"** instead of **"0"** in the Agent Leaderboard's "Active Days" metric.

## Root Cause

The `RetrieveAgentStats` ClickHouse query uses two CTEs joined together:

- `convs` — from `conversation_d`, filtered by `conversation_source in (0)` (after flag)
- `convs_with_aa` — from `conversation_with_labels_d`, no source filter (table lacks the column)

With LEFT JOIN, when `convs` has no rows for a given day (agent only had source 8 conversations), the entire row disappears — causing "N/A".

## Fix

Three changes in `retrieve_agent_stats_clickhouse.go`:

1. **SQL: LEFT JOIN → FULL OUTER JOIN** — preserves rows from both CTEs
2. **SQL: COALESCE** — handles NULLs from the outer join (`COALESCE(convs.total_agent_count, 0)`)
3. **Go: Guard condition** — `if useCrestaConversationWithAACount && row.totalAgentCount > 0` prevents source-8-only rows (where `total=0, active=1`) from incorrectly showing as "1"

### Why simpler approaches don't work

| Approach | Problem |
|----------|---------|
| RIGHT JOIN | Drops rows where agent has conversations but no AA (regression for "0" days) |
| Three-CTE with semi-join | Correct but complex; FULL OUTER JOIN + guard achieves the same result |
| Filter `convs_with_aa` by source | `conversation_with_labels_d` has no `conversation_source` column |

## Files Changed

| File | Change |
|------|--------|
| `retrieve_agent_stats_clickhouse.go` | FULL OUTER JOIN + COALESCE in SQL; `totalAgentCount > 0` guard in Go |
| `retrieve_agent_stats_test.go` | New test case for source-8-only agent; if-else → switch (lint fix) |
| 5 testdata SQL files | Updated expected SQL to match FULL OUTER JOIN + COALESCE |

## Production Verification (Alaska Air)

Tested agents `76515806a32dcf91` and `a86442ed872e344b`:

- Agent `a86442ed872e344b` on 2026-01-26: `total=0, active=1` in FULL OUTER JOIN (was missing entirely with LEFT JOIN)
- The Go guard correctly maps this to display "0" instead of "1"

## CSM Questions Investigated (2026-02-09)

Two questions from the CSM team about today's data:

1. **Agent `a86442ed872e344b` shows 0 despite having chats**: 25 source-0 conversations exist, but only 2 rows in `conversation_with_labels_d` (both `has_agent_assistance=false`). Root cause: `cron-label-conversations` runs every 30 minutes — data hadn't been processed yet.

2. **Agent `76515806a32dcf91` shows 0 with no chats today**: Actually has 28 conversations with AA=true. Query correctly returns `total=1, active=1`. CSM may have had a timezone or UI caching issue.

## CSM Question Investigated (2026-02-11): Corinne Harmon Shows 0

**Report**: Corinne Harmon on 2/10 shows 0 on Leaderboard (chat use case), but all chats were processed in Real Time. First chat at 6:33pm MT.

**Agent**: `256f70253da263fe`, customer: Alaska Air

### Root Cause: Stale `conversation_with_labels_d` After Conversation Re-assignment

17 conversations were created starting 01:33 UTC on 2/11 (6:33pm MT on 2/10). Timeline:

1. **01:33 UTC**: Conversations created with original agents in `reservations-chat` use case
2. **02:00 UTC**: `cron-label-conversations` runs → writes to `conversation_with_labels_d` with original agents + `reservations-chat`
3. **03:01 UTC**: Conversations re-assigned to Corinne Harmon + reclassified as `chat` → `conversation_d` updated
4. `conversation_with_labels_d` **never re-labeled** → still has original agents + `reservations-chat`

**Result**: When Leaderboard queries `usecase_id='chat'` for Corinne:

| CTE | Table | usecase filter | agent filter | Result |
|-----|-------|---------------|-------------|--------|
| `convs` | `conversation_d` | chat ✓ | Corinne ✓ | 17 rows → total=1 |
| `convs_with_aa` | `conversation_with_labels_d` | chat ✗ (has `reservations-chat`) | Corinne ✗ (has other agents) | 0 rows → active=0 |

With `useCrestaConversationWithAACount=true`: `agentCount = activeAgentCount = 0` → all metrics = 0.

### Data Evidence

```
conversation_d (updated 03:01 UTC):
  agent=256f70253da263fe (Corinne), usecase=chat

conversation_with_labels_d (labeled 02:00 UTC, never re-labeled):
  agent=9f87ef86708a74fc (different), usecase=reservations-chat
```

All 17 "chat" conversations show this same mismatch pattern.

### This Is a Different Bug from PR #25613

| Aspect | Feb 9 Bug (PR #25613) | Feb 11 Bug (this) |
|--------|----------------------|-------------------|
| Symptom | N/A instead of 0 | 0 instead of 1 |
| Root cause | LEFT JOIN drops rows when convs has no data | conversation_with_labels_d has stale agent/usecase |
| Trigger | exclude_ingestion_pipeline flag + source 8 only | Conversation re-assignment after cron labeling |
| Fix | FULL OUTER JOIN + COALESCE + Go guard | `cron-label-conversations` needs to re-label on updates |

### Cron Job Analysis

**Code**: `go-servers/cron/sync-users/internal/conversation-agent-assistance/task.go`

**Current behavior**:
- Time range: `[max(conversation_start_time) from conversation_with_labels_d, now)` — only new conversations, never revisits
- Filters by `chats.created_at` — processes conversations as soon as they're created, including still-open ones
- Does NOT filter by closed conversations (`ended_at` is not checked)
- This is how the 17 conversations got labeled at 02:00 UTC (while still open) with the original agents — before re-assignment at 03:01 UTC

**ClickHouse table schema**:
```sql
ENGINE = ReplicatedReplacingMergeTree(..., update_time)
PRIMARY KEY (toStartOfHour(conversation_end_time), agent_user_id)
ORDER BY (toStartOfHour(conversation_end_time), agent_user_id, conversation_id, customer_id, profile_id)
```

**All fix options need delete-before-write** because `agent_user_id` and `toStartOfHour(conversation_end_time)` are in the ORDER BY. When these change after re-assignment, re-writing creates a duplicate instead of replacing.

### Proposed Fix Options

#### Option A: Lookback Window + Delete-Before-Write

Change `setTimeRange` to start from `max(conversation_start_time) - 2 hours`. Re-processes recent conversations, catching re-assignments.

- Simple change, covers ~1.5 hour delay observed in Corinne's case
- Modest increase in processing load (~4x per conversation in the window)
- Won't catch re-assignments after the window

#### Option B: Filter by `ended_at` + Delete-Before-Write (Recommended)

Change `findConversations` to filter by `chats.ended_at` instead of `chats.created_at`. Only label closed conversations — by which point re-assignments are typically complete. Adjust `setTimeRange` to derive start from `max(conversation_end_time)` or `max(update_time)`.

- Addresses root cause directly — labels final state, not intermediate
- Long-running conversations labeled later (only after close) — acceptable for daily metric
- Still needs delete-before-write because `ended_at` itself can change

#### Option C: Change ORDER BY (Long-term)

Remove mutable columns from ORDER BY: `(conversation_id, customer_id, profile_id)`. Makes ReplacingMergeTree deduplicate on `conversation_id` alone — no delete-before-write needed.

**Performance**: The current PRIMARY KEY `(toStartOfHour(conversation_end_time), agent_user_id)` does NOT benefit the agent stats query. That query (`retrieve_agent_stats_clickhouse.go`) filters and groups by `conversation_start_time`, not `conversation_end_time`. Neither column in the current key matches the query's access pattern. Removing them has zero impact on agent stats.

- Requires schema migration across all customer databases
- Eliminates the duplicate row problem entirely

## Key Learnings

- `conversation_with_labels_d` contains conversations from ALL sources (0, 7, 8) despite the SQL comment claiming "only source 0"
- `cron-label-conversations` runs every 30 minutes, causing up to 30-min delay for AA tagging
- Agent Assist tagging is based on login heartbeat overlap, not direct feature usage — leading to false positives (browser extension logins) and false negatives (heartbeat gaps, cron delay)
- **NEW**: `conversation_with_labels_d` does NOT update when `conversation_d` is updated (agent re-assignment, usecase reclassification). The cron labels once and never re-labels.
- **NEW**: `conversation_with_labels_d` now has a `usecase_id` column, and the agent stats query filters by it. Mismatched usecase_ids between the two tables cause incorrect results.

## Notion Documentation

Created a non-engineer-facing guide: [Agent Leaderboard – 'Active Days' Behavior Guide](https://www.notion.so/cresta/3024a587b06180cbace3faa4fd6c8b14)

Covers:
- Where Active Days appears (widget vs table)
- Display values with UI tooltips (N/A, 0, 1)
- What is Agent Assist (login detection logic)
- Known limitations (false positives, false negatives, cron delay)
- Scenario breakdowns with the `exclude_ingestion_pipeline_from_agent_stats` flag
- Real-world Alaska Air example

## Log History

| Date | Summary |
|------|---------|
| 2026-02-09 | Full investigation, fix, PR, Notion doc, production verification, CSM investigation |
| 2026-02-11 | New bug: stale `conversation_with_labels_d` after conversation re-assignment (Corinne Harmon case) |
