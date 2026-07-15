# Codex Session: CONVI-6842 Conversation Count Coda Refresh

**Date:** 2026-07-06
**Source repo:** `/Users/xuanyu.wang/repos/director`
**Branch/worktree:** merged `xwang/convi-6842-conversation-volume`

## Objective

Document PM-agreed conversation-count semantics after director#20153 merge, following the Active Days Coda refresh pattern.

## Inputs

- Slack PM agreement: https://crestalabs.slack.com/archives/D07UWJ1U1CH/p1782487572741609
- Coda guide: https://coda.io/d/_doM9F-e3jUe/What-does-Conversation-Count-mean-across-Insights-pages_suvFVqLd
- Ticket project: `/Users/xuanyu.wang/repos/knowledge/convi-6842-holiday-inn-pi-vs-closed-conversations`
- PR: https://github.com/cresta/director/pull/20153

## Coda Updates

- Added summary block with semantics table and director#20153 reference.
- Rewrote section (b) to use `score_d` / `RetrieveQAScoreStats` with N/A scorecards included.
- Removed contradictory aspirational bullet from section (a).
- Added section (e) for Closed Conversations.
- Revised FAQs #2–#5 for post-fix behavior.

## Knowledge Outputs

- `deliverables/conversation-count-behavior-guide-2026-07.md`
- Updated `convi-6842-holiday-inn-pi-vs-closed-conversations` project status to resolved.
- Updated `insights-user-filter/fe-group-by-usage-patterns.md`.

## Verification

- Linear CONVI-6842 marked Done with resolution comment.
- Coda `content_modify` operations succeeded.
