# Codex Session: Coda Active Days Refresh

**Date:** 2026-06-29
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree:** `main` at `/Users/xuanyu.wang/repos/go-servers`

## Objective

Refresh the Coda page `Agent Leaderboard - Active Days Behavior Guide` based on the plan document and all related PRs by Xingwei, then record the latest behavior in this consolidated knowledge project.

## Inputs

- Coda guide: `https://coda.io/d/_dXbEJBQXiGs/Agent-Leaderboard-Active-Days-Behavior-Guide_sueK_Nu_`
- Slack plan doc: `https://cresta.enterprise.slack.com/files/U0896UC4MQF/F0B8PREKRRQ/conversation_with_labels_update.md`
- Legacy knowledge: `/Users/xuanyu.wang/repos/knowledge/agent-stats-active-days-fix`
- Legacy knowledge: `/Users/xuanyu.wang/repos/knowledge/convi-6242-cron-label-conversations`

## Investigation Notes

- Read the Slack plan artifact through Glean. The original plan proposed flipping `conversation_with_labels_d` from conversation-span overlap to message-timestamp matching, adding message columns, and filtering agent messages by batch message time.
- Read historical context from:
  - `/Users/xuanyu.wang/repos/knowledge/agent-stats-active-days-fix`
  - `/Users/xuanyu.wang/repos/knowledge/convi-6242-cron-label-conversations`
- Queried PR metadata through GitHub CLI and inspected `origin/main` source for:
  - `cron/sync-users/internal/conversation-agent-assistance/task.go`
  - `insights-server/internal/analyticsimpl/retrieve_agent_stats_clickhouse.go`
  - `insights-server/internal/analyticsimpl/retrieve_conversation_stats_clickhouse.go`
  - `shared/clickhouse/conversations/conversation.go`
- Confirmed the current cron:
  - processes closed conversations by `chats.ended_at`
  - fetches Normal and Ingestion Pipeline conversations
  - excludes dev users through the conversation agent
  - fetches agent messages with `speaker_role = 'agent'`
  - scopes message lookup by `conversation_id` only, not by message timestamp
  - chunks conversation IDs in batches of 10,000 for Postgres parameter safety
  - fetches online events by event `startTime` in batch range plus a 3-hour buffer
  - writes one true row per matched agent message
  - writes fallback rows for conversations with no agent messages, using start/end anchor matching
- Confirmed the current Agent Stats read path:
  - uses `FULL OUTER JOIN` between `convs` and `convs_with_aa`
  - uses `message_time` for new message rows and `conversation_start_time` for legacy/fallback rows
  - keeps the `totalAgentCount > 0` guard before replacing total count with Agent Assist active count
- Confirmed Conversation Stats differs from Active Days: conversation-level Agent Assist filters are scoped by conversation time, not message time, because that API answers whether a conversation had Agent Assist.

## PR Evidence

| PR | Repository | Evidence |
|---|---|---|
| #25706 | `go-servers` | Earlier stale-label fix: process closed conversations by `ended_at` instead of `created_at`. |
| #8844 | `cresta-proto` | Adds message fields to `ConversationWithLabels`. |
| #213 | `clickhouse-schema` | Adds `message_id`, `message_agent_user_id`, and `message_time`; extends ordering with `message_id`. |
| #28531 | `go-servers` | Switches cron write path to per-message labeling. |
| #28706 | `go-servers` | Updates read paths for mixed legacy and message-level rows; Active Days uses `message_time` for new rows. |
| #29030 | `go-servers` | Adds fallback conversation-level rows for conversations with no agent messages. |
| #29058 | `go-servers` | Overturns the plan's message-time filter assumption; message lookup is by `conversation_id` only. |
| #29084 | `go-servers` | Chunks agent-message lookup to avoid Postgres 65,535-parameter limit. |

## Coda Update

- Updated `https://coda.io/d/_dXbEJBQXiGs/Agent-Leaderboard-Active-Days-Behavior-Guide_sueK_Nu_`.
- Saved the replacement body as `deliverables/active-days-behavior-guide-2026-06.md`.
- Browser verification after replacement:
  - old opening text `This document describes how the Agent Leaderboard determines...` count: 0
  - new opening text `This guide explains how Agent Leaderboard decides...` count: 1
  - `Message lookup is scoped by conversation_id only` count: 1
  - `Conversations With No Agent Messages` count: present
  - `GLOBAL LEFT ANTI JOIN` count: 1
  - `New Agent Assist labels are message-granular` count: 1
