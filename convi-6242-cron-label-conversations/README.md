# CONVI-6242: cron-label-conversations Stale Data & Backfill

**Created:** 2026-02-18
**Updated:** 2026-02-18
**Linear:** https://linear.app/cresta/issue/CONVI-6242/cron-label-conversations-writes-stale-agentusecase-to-conversation
**Status:** In Progress

## Overview

`cron-label-conversations` labels conversations in `conversation_with_labels_d` (ClickHouse) while they are still open. When conversations are later re-assigned to a different agent or reclassified to a different use case, the labels become stale — the cron never revisits previously labeled conversations.

This causes the Agent Leaderboard "Active Days" metric to show **0 instead of 1** for affected agents.

### Customer Impact

- **Alaska Air** (2026-02-10): Agent Corinne Harmon had 17 chat conversations re-assigned at 03:01 UTC. The cron had already labeled them at 02:00 UTC with the original agents and `usecase_id=reservations-chat`. Leaderboard showed "0" instead of "1" for Active Days.

## Root Cause

1. `cron-label-conversations` (`go-servers/cron/sync-users/internal/conversation-agent-assistance/task.go`) filters by `chats.created_at` and processes conversations as soon as they're created, including still-open ones
2. `setTimeRange` starts from `max(conversation_start_time)` in the labels table — forward-only, never revisits
3. When `conversation_d` is later updated (agent re-assignment, usecase reclassification), `conversation_with_labels_d` retains the stale values

## ClickHouse Table Constraint

The `conversation_with_labels` table uses `ReplicatedReplacingMergeTree` with:

```sql
ORDER BY (toStartOfHour(conversation_end_time), agent_user_id, conversation_id, customer_id, profile_id)
```

Since `agent_user_id` and `toStartOfHour(conversation_end_time)` are in the ORDER BY, re-writing a conversation with changed agent/end_time creates a **duplicate row** instead of replacing. Any fix must **delete existing rows before writing**.

## Fix Options

### Option A: Lookback Window + Delete-Before-Write

Change `setTimeRange` to start from `max(conversation_start_time) - 2 hours`. Re-processes recent conversations, catching re-assignments within the window. Delete existing rows for re-processed `conversation_id`s before writing.

### Option B: Filter by `ended_at` + Delete-Before-Write (Recommended)

Change `findConversations` to filter by `chats.ended_at` instead of `chats.created_at`. Only label closed conversations — by which point re-assignments are typically complete. Still needs delete-before-write because `ended_at` itself can change.

### Option C: Change ORDER BY (Long-term)

Remove mutable columns from ORDER BY: `(conversation_id, customer_id, profile_id)`. Makes ReplacingMergeTree deduplicate on `conversation_id` alone — no delete-before-write needed. Requires schema migration across all customer databases.

**Recommended:** Option B, optionally combined with Option C long-term.

## PRs

| PR | Description |
|----|-------------|
| [go-servers#25706](https://github.com/cresta/go-servers/pull/25706) | Filter by `ended_at` to prevent stale labels |
| [clickhouse-schema#172](https://github.com/cresta/clickhouse-schema/pull/172) | Change `conversation_with_labels` ORDER BY to `conversation_start_time` |

## Backfill Plan

See **[backfill-plan.md](backfill-plan.md)** for the full detailed plan.

### Summary

1. **Deletion is mandatory** — the ClickHouse ORDER BY includes mutable columns (`agent_user_id`, `toStartOfHour(conversation_end_time)`), so re-inserting changed conversations creates duplicates, not replacements
2. **Execution order**: DELETE existing 2026 rows → wait for mutation → re-run cron with time range env vars → verify
3. **Cron supports backfill natively** via env vars:
   - `LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_START_AT_RANGE_START` / `_END` for time range
   - `FILTER_CUSTOMER_IN_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE` for customer filter
4. **K8s job pattern**: same as `backfill-scorecards/` — create job from cronjob template, set env vars, apply
5. **For large customers**: use day-by-day sequential approach (learned from backfill-scorecards CVS/Oportun)

### Files to Change

- `go-servers/cron/sync-users/internal/conversation-agent-assistance/task.go` — main cron logic

## Related

- **CONVI-6242** (Linear) — the tracking issue
- **PR #25613** — Fixed a different Active Days bug (N/A instead of 0) caused by LEFT JOIN → FULL OUTER JOIN (separate issue)

## Log History

| Date | Summary |
|------|---------|
| 2026-02-18 | Project created, documented issue details, root cause, fix options, and backfill plan |
