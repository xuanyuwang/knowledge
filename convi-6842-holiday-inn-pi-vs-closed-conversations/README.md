# CONVI-6842: Holiday Inn PI vs Closed Conversations mismatch

**Status**: Director PR in corrective cleanup  
**Ticket**: https://linear.app/cresta/issue/CONVI-6842  
**Customer**: Holiday Inn Voice and Transfer  
**Worktree**: `/Users/xuanyu.wang/repos/director-convi-6842`  
**Branch**: `xwang/convi-6842-conversation-volume`  
**PR**: https://github.com/cresta/director/pull/20153

## Problem

Holiday Inn reported that Performance Insights can show more conversations than Closed Conversations for the same date/profile/filter set, which should not happen if both surfaces are reflecting the same underlying conversation set.

Reported example from the Linear ticket:

- Performance Insights with no template: 611 conversations
- Closed Conversations with the same date/profile filter: 611 conversations
- Performance Insights with template `1. Agent Intro - MKTINTRO`: 634 conversations
- Performance Insights "All criteria" row: 592 conversations scored

The unexpected behavior is the template-filtered PI volume increasing above the no-template and Closed Conversations counts.

## Current Working Model

For the no-template path, the mismatch is not primarily a frontend state bug. It comes from the fact that the page can issue two different analytics APIs with different source tables:

- `RetrieveConversationStats` counts conversations from `message_d` joined with `conversation_d`
- `RetrieveQAScoreStats` counts conversations from `score_d` / scorecard rows

These are not the same population, even when the filters look similar at the UI level.

## Key Findings For "No template"

- `Conversation volume` in the no-template path is backed by `RetrieveConversationStats`, not by the QA score stats query.
- `RetrieveConversationStats` requires a matching `(conversation_id, agent_user_id)` pair in `message_d`, then joins that pair with `conversation_d`.
- `RetrieveQAScoreStats` can count a conversation from `score_d` even if the same conversation has no rows at all in `message_d`.
- On current production data for Holiday Inn `transfers-voice` on April 15, 2026:
  - `RetrieveConversationStats`: `7253`
  - `RetrieveQAScoreStats` with `includeNaScored=false`: `7256`
  - `RetrieveQAScoreStats` with `includeNaScored=true`: `8284`
- The large jump from `7253` to `8284` is driven by `1031` conversations that exist in QA/score tables but have no rows in `message_d`.
- Those `1031` extra conversations are mostly near-zero-duration conversations:
  - `876` have `conversation_duration_secs <= 0`
  - `931` have `conversation_duration_secs <= 1`
  - `986` have `conversation_duration_secs <= 5`
- The smaller residual delta between `RetrieveConversationStats` and `RetrieveQAScoreStats(includeNaScored=false)` is `3` conversations, and those also have no rows in `message_d`.

## Current Assets

- Linear branch name exists and a dedicated worktree has been created.
- Knowledge project structure is in place for logs, sessions, and deliverables.

## Interpretation

If the product intends `Conversation volume` to mean "closed conversations with actual message/transcript rows", the `RetrieveConversationStats` number is the defensible one.

If the product intends `Conversation volume` to mean "all conversations represented in QA scorecards, including N/A / no-message scorecarded conversations", then the `RetrieveQAScoreStats(includeNaScored=true)` number is the defensible one.

The important point is that these are two different business definitions. The UI label currently hides that distinction.

## Current Implementation Direction

The accepted product decision is to keep Closed Conversations strict and make the Performance Insights conversation-volume card scorecard-centric. The Director PR should stay minimal: replace the no-template conversation-volume data source from `RetrieveConversationStats` to `RetrieveQAScoreStats`, while preserving the existing chart layout, legend behavior, export behavior, and selected-template behavior.

## Log History

| Date | Summary |
|------|---------|
| 2026-07-03 | Knowledge wrap-up; PR #20153 in review after staging validation. |
| 2026-07-02 | Reviewed staging before/after screenshots; confirmed volume delta matches QA source swap. |
| 2026-06-29 | Reduced Director PR to minimal `ConversationCountChart.tsx` source swap; pushed to PR #20153. |

## Related Artifacts

- `log/2026-06-29.md`
- `log/2026-07-02.md`
- `log/2026-07-03.md`
- `sessions/2026-06-29/codex-pr-diff-cleanup.md`
- `sessions/2026-07-02/codex-pr-comment-walter-dev.md`
