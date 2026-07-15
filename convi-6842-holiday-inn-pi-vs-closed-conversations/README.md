# CONVI-6842: Holiday Inn PI vs Closed Conversations mismatch

> Migrated navigation: [Analytics / Conversation Volume](../analytics/subdomains/conversation-volume/README.md). This folder remains detailed historical evidence.

**Status**: Resolved
**Ticket**: https://linear.app/cresta/issue/CONVI-6842  
**Customer**: Holiday Inn Voice and Transfer  
**PR**: https://github.com/cresta/director/pull/20153 (merged 2026-07-06, commit `4c67e9c`)
**Coda**: https://coda.io/d/_doM9F-e3jUe/What-does-Conversation-Count-mean-across-Insights-pages_suvFVqLd

## Problem

Holiday Inn reported that Performance Insights can show more conversations than Closed Conversations for the same date/profile/filter set, which should not happen if both surfaces are reflecting the same underlying conversation set.

Reported example from the Linear ticket:

- Performance Insights with no template: 611 conversations
- Closed Conversations with the same date/profile filter: 611 conversations
- Performance Insights with template `1. Agent Intro - MKTINTRO`: 634 conversations
- Performance Insights "All criteria" row: 592 conversations scored

The unexpected behavior was the template-filtered PI volume increasing above the no-template count because no-template and selected-template paths used different analytics APIs.

## Resolution

PM agreement with Krystal (2026-07-03) and fix merged via director#20153.

| Surface | Definition | Holiday Inn Apr 15 example |
|---|---|---|
| PI Conversation volume (no template or selected template) | Distinct conversations with a scorecard, including all-N/A scorecards | 639 |
| PI Performance progression `# of convo` | Scorecarded conversations with at least one non-N/A score | 592 |
| Closed Conversations | Scorecarded conversations with non-empty message/transcript rows | 611 |

PI and Closed Conversations are **intentionally allowed to differ**. The fix aligns PI internal consistency (no-template vs selected-template), not cross-page parity.

## Root Cause

No-template PI Conversation volume used `RetrieveConversationStats` (`message_d` joined with `conversation_d`). Selected-template volume used `RetrieveQAScoreStats` (`score_d` with `includeNaScored: true`). Same UI label, different populations.

## Fix

Minimal Director change in `ConversationCountChart.tsx`: no-template volume now uses `useQAScoreStats` with `includeNaScored: true` via `filtersWithNAValues` from `PerformanceConversations`. Closed Conversations behavior unchanged.

## Key Findings (pre-fix investigation)

- On production data for Holiday Inn `transfers-voice` on April 15, 2026:
  - `RetrieveConversationStats`: `7253`
  - `RetrieveQAScoreStats` with `includeNaScored=false`: `7256`
  - `RetrieveQAScoreStats` with `includeNaScored=true`: `8284`
- `1031` conversations existed in QA/score tables but had no rows in `message_d`.

## Log History

| Date | Summary |
|------|---------|
| 2026-07-06 | PM agreement wrap-up: PR merged, Coda + Linear updated, knowledge consolidated. |
| 2026-07-03 | Knowledge wrap-up; PR #20153 in review after staging validation. |
| 2026-07-02 | Reviewed staging before/after screenshots; confirmed volume delta matches QA source swap. |
| 2026-06-29 | Reduced Director PR to minimal `ConversationCountChart.tsx` source swap; pushed to PR #20153. |

## Related Artifacts

- `log/2026-07-06.md`
- `log/2026-07-03.md`
- `log/2026-07-02.md`
- `log/2026-06-29.md`
- `sessions/2026-07-06/codex-pm-agreement-wrap-up.md`
- `sessions/2026-06-29/codex-pr-diff-cleanup.md`
- `sessions/2026-07-02/codex-pr-comment-walter-dev.md`
- `/Users/xuanyu.wang/repos/knowledge/agent-stats-analytics-behaviors/deliverables/conversation-count-behavior-guide-2026-07.md`