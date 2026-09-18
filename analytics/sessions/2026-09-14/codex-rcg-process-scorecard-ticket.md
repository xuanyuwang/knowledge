# Session: RCG process-scorecard Performance Insights ticket

- Date: 2026-09-14
- Tool: Codex
- Source repo: `/Users/xuanyu.wang/repos/director`
- Branch/worktree context: read-only ticket follow-up; no product-code changes

## Request

Create a ticket for the RCG data-visibility issue discussed in Slack thread `p1788990592932079`, using `/Users/xuanyu.wang/Downloads/rcg.cresta.com.har` as evidence and keeping it separate from latency.

## Result

- Found the exact existing ticket [CONVI-7674](https://linear.app/cresta/issue/CONVI-7674/rcg-process-scorecards-show-no-data-in-performance-insights-after); did not create a duplicate.
- Verified the HAR has 20 `qaScoreStats:retrieve` calls, all HTTP 200 with `totalScorecardCount: 0`, the correct template/use case, no user/group restriction, and `TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT` on every request.
- Verified the Slack thread includes a renewed 2026-09-14 customer escalation.
- Renamed the ticket with an RCG prefix, assigned it to Xuanyu Wang, moved it to In Progress, and added `Customer Issue`, `Support`, `Royal Caribbean Group`, and `oncall-backlog` labels while preserving the relevant analytics labels.
- Linked CONVI-7667 and Director PR #22772, which implement the same root-cause fix.
- Kept the ticket limited to the no-data/process-scorecard regression; no latency content was added.
- Did not attach the raw HAR because it contains complete browser request metadata; the verified request/response facts are summarized in the ticket.

## Evidence

- Slack: https://crestalabs.slack.com/archives/C04NB5AMV0F/p1788990592932079
- HAR: `/Users/xuanyu.wang/Downloads/rcg.cresta.com.har`
- Prior investigation: `sessions/2026-09-09/claude-rcg-dtq-pi-visibility.md`
