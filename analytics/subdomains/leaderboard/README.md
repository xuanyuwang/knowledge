# Leaderboard

## Purpose

Own Agent, Team, and Manager Leaderboard semantics, including ranking/grouping, summary cards, page-specific filters, metric sources, and FE presentation.

## Current Semantics

- Agent Leaderboard commonly groups non-QA metrics by agent and group; QA score groups by agent.
- Team Leaderboard groups by group and requires correct descendant-team expansion.
- Manager Leaderboard uses manager-oriented scorecard, coaching-session, comment, and Live Assist APIs and does not share every agent-filter rule.
- Summary cards may reuse response-level aggregates from the same grouped requests as the table.
- Ranking semantics such as tiers/quintiles should be referenced from their metric subdomain rather than redefined here.

## Source Map

- Director `features/insights/leaderboard/agent-leaderboard/`
- Director `features/insights/leaderboard/team-leaderboard/`
- Director `features/insights/leaderboard/manager-leaderboard/`
- Analytics hooks and `insights-server/internal/analyticsimpl/`

## Legacy Sources and Cases

- `convi-6260-team-leaderboard/`
- `convi-6494-raises-answered-zero/`
- `convi-6968-schwab-leaderboard-launch/`
- `agent-quintiles-support/` (ranking cross-link)

## Open Questions

- Finish a metric-by-tab API/source/filter matrix.
- Document sorting, ties, missing values, minimum-volume gates, and export parity.
