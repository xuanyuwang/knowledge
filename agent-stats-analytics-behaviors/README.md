# Agent Stats Analytics Behaviors

> Migrated navigation: [Analytics / Shared Analytics Platform](../analytics/subdomains/shared-analytics-platform/README.md). This folder remains detailed historical evidence.

**Created:** 2026-06-29
**Updated:** 2026-07-08

## Overview

Durable context center for agent stats and related analytics behavior across Performance, Leaderboard, coaching, and QM APIs.

This project consolidates older focused investigations:

- `/Users/xuanyu.wang/repos/knowledge/agent-stats-active-days-fix`
- `/Users/xuanyu.wang/repos/knowledge/convi-6242-cron-label-conversations`
- `/Users/xuanyu.wang/repos/knowledge/convi-6842-holiday-inn-pi-vs-closed-conversations`

## Current Objective

Maintain durable conversation-count and agent-stats behavior guides aligned with PM decisions and merged code.

## Key Findings

- Coda page `Agent Leaderboard - Active Days Behavior Guide` was refreshed on 2026-06-29 from current PR/source evidence.
- The June 2026 behavior is message-granular: new Agent Assist label rows represent matched agent messages, with `message_id`, `message_agent_user_id`, and `message_time`.
- The original Slack plan assumption that `findAgentMessages` should filter by batch message time was overturned by PR #29058. The final implementation scopes messages by `conversation_id` only because batches are based on `conversation_end_time` and multi-day conversations can contain valid messages before the batch start.
- Conversations with no agent messages now get fallback rows with `message_id = ''`; those rows use conversation start/end anchor matching and can be true or false.
- `RetrieveAgentStats` attributes new rows by `message_time` and fallback/legacy rows by `conversation_start_time`; the source-exclusion guard still prevents ingestion-only Agent Assist evidence from becoming `1` when ingestion is excluded.
- Historical Active Days issue #1: source-8-only days were dropped by a `LEFT JOIN`; later fixed by preserving rows and guarding the displayed active count.
- Historical Active Days issue #2: `conversation_with_labels_d` could become stale when cron labeled open conversations before later agent/usecase reassignment; the mitigation path moved labeling toward final conversation state and required duplicate cleanup/backfill awareness.
- CONVI-7143 investigation: Hilton HGV Club conversation `019ed272-c005-7c27-b060-54d7ff273d5e` is missing a powered-by-Agent-Assist label even though its agent message falls within an online Agent Assist interval; likely cause is message filtering by batch time while conversations are batched by end time.
- CONVI-7143 data source is confirmed from Director source and local dev TanStack Query Devtools: Assistance Insights' powered-by-Agent-Assist chart uses two `RetrieveConversationStats` queries, and the powered count adds `agentAssistFilters=ON`.
- CONVI-7143 latest `go-servers` `origin/main` already has the correctness fix from PR #29058: `findAgentMessages` scopes messages by `conversation_id` only and no longer filters by batch `platform_timestamp`; PR #29084 chunks IDs to avoid Postgres parameter limits.
- CONVI-7143 was repaired on 2026-07-02 by running one-off Hilton label backfill job `backfill-labels-hilton-convi-7143-1783017928`; ClickHouse and UI both now show `10 total / 10 powered` for the target Gaylynn Bryant Jun 16 HGV Club Assistance Insights slice.
- CONVI-7143 follow-up for Linda Nesmith Jun 29 / Jul 6 is not stale pre-#29058 label data. The missing conversations are real-time source but their agent messages fall in short gaps between Agent Assist online-session intervals, so current labeler semantics produce no `conversation_with_labels_d` rows. Closed Conversations Source=batch is not a valid way to identify conversations that are not powered by Agent Assist.
- CONVI-6842 resolution: PI Conversation volume (no template and selected template) is scorecard-centric via `RetrieveQAScoreStats` with `includeNaScored: true` after director#20153. Closed Conversations stays strict/non-empty via `RetrieveConversationStats`. PI and Closed Conversations are intentionally allowed to differ.
- Coda page `What does Conversation Count mean across Insights pages?` was refreshed on 2026-07-06 with PM-agreed semantics and a new Closed Conversations section.

## Status

Active.

## Source Context

- **Primary repo:** `go-servers`
- **Repo path:** `/Users/xuanyu.wang/repos/go-servers`
- **Active worktree:** `/Users/xuanyu.wang/repos/go-servers`
- **Branch:** `main`

## Log History

| Date | Summary |
|------|---------|
| 2026-07-08 | Investigated CONVI-7143 Hilton follow-up; Linda Jun 29 / Jul 6 misses are online-event coverage gaps, not stale pre-#29058 data. |
| 2026-07-06 | CONVI-6842 wrap-up: director#20153 merged; refreshed Coda conversation-count guide; added deliverable. |
| 2026-07-03 | Knowledge wrap-up; no new investigation. CONVI-7143 repair remains verified. |
| 2026-07-02 | Ran the Hilton label backfill for CONVI-7143 and verified ClickHouse/UI now show `10 total / 10 powered`. |
| 2026-07-02 | Confirmed latest `go-servers` `origin/main` already removed the message timestamp predicate from `findAgentMessages` and chunks conversation ID queries. |
| 2026-06-30 | Investigated CONVI-7143, confirmed the Assistance Insights data source from source/local TanStack, and identified the missing Hilton powered-by-Agent-Assist conversation plus likely labeler batching cause. |
| 2026-06-30 | Removed leftover comment-export blocks after the Coda guide Summary section. |
| 2026-06-29 | Created consolidation project, reviewed Slack plan and Xingwei PR chain, updated Coda guide, and saved replacement guide draft. |

## Related Artifacts

- `project.yaml`
- `log/2026-06-29.md`
- `log/2026-06-30.md`
- `log/2026-07-02.md`
- `log/2026-07-03.md`
- `sessions/2026-06-29/codex-coda-active-days-refresh.md`
- `sessions/2026-06-30/codex-coda-cleanup.md`
- `sessions/2026-06-30/codex-convi-7143-target-aa-missing-conversation.md`
- `sessions/2026-07-02/codex-convi-7143-message-query-performance.md`
- `sessions/2026-07-06/codex-convi-6842-conversation-count-coda-refresh.md`
- `sessions/2026-07-08/codex-convi-7143-hilton-followup.md`
- `deliverables/active-days-behavior-guide-2026-06.md`
- `deliverables/conversation-count-behavior-guide-2026-07.md`