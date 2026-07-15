# CONVI-6842 PM Agreement Wrap-Up

**Date:** 2026-07-06
**Source repo:** `/Users/xuanyu.wang/repos/director`
**Branch/worktree:** merged via `xwang/convi-6842-conversation-volume` at `/Users/xuanyu.wang/repos/director-convi-6842`

## Objective

Complete post-agreement wrap-up for CONVI-6842 after PM sign-off with Krystal on 2026-07-03.

## PM-Agreed Semantics

| Surface | Definition | Holiday Inn Apr 15 example |
|---|---|---|
| PI Conversation volume (no template or selected template) | Distinct conversations with a scorecard, including all-N/A scorecards | 639 |
| PI Performance progression `# of convo` | Scorecarded conversations with at least one non-N/A score | 592 |
| Closed Conversations | Scorecarded conversations with non-empty message/transcript rows | 611 |

## Actions

1. Confirmed director#20153 merged (`4c67e9c`, 2026-07-06T15:36:30Z).
2. Updated Coda conversation-count guide with summary table, rewritten section (b), new section (e), and revised FAQs.
3. Added Linear resolution comment and marked CONVI-6842 Done.
4. Wrote `agent-stats-analytics-behaviors/deliverables/conversation-count-behavior-guide-2026-07.md`.
5. Updated stale `insights-user-filter/fe-group-by-usage-patterns.md` reference.

## References

- Slack thread: https://crestalabs.slack.com/archives/D07UWJ1U1CH/p1782487572741609
- Coda: https://coda.io/d/_doM9F-e3jUe/What-does-Conversation-Count-mean-across-Insights-pages_suvFVqLd
- PR: https://github.com/cresta/director/pull/20153
