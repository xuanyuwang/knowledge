# Leaderboard

## Purpose

Own Agent, Team, and Manager Leaderboard semantics, including ranking/grouping, summary cards, page-specific filters, metric sources, and FE presentation.

## Current Semantics

- Agent Leaderboard commonly groups non-QA metrics by agent and group; QA score groups by agent.
- Team Leaderboard groups by group and requires correct descendant-team expansion.
- Manager Leaderboard uses manager-oriented scorecard, coaching-session, comment, and Live Assist APIs and does not share every agent-filter rule.
- Summary cards may reuse response-level aggregates from the same grouped requests as the table.
- Ranking semantics such as tiers/quintiles should be referenced from their metric subdomain rather than redefined here.
- Scorecard template drawers must inherit page conversation filters that affect scorecard membership, including voicemail exclusion. Drawer requests may narrow to the selected agent or manager, but must not silently drop shared filters such as `voicemailMoment`.

## Source Map

- Director `features/insights/leaderboard/agent-leaderboard/`
- Director `features/insights/leaderboard/team-leaderboard/`
- Director `features/insights/leaderboard/manager-leaderboard/`
- Analytics hooks and `insights-server/internal/analyticsimpl/`

## Consolidated Historical Cases

- [CONVI-6260](../../work-items/CONVI-6260.md): parent-team selection requires descendant row expansion independently of user membership filtering; the canonical investigation is in Insights User Filter.
- [CONVI-6494](../../work-items/CONVI-6494.md): correlating raise and whisper events at manager-ID grain split the evidence because raise events lacked that ID. Agent and manager metrics need appropriate aggregation grains. The saved Brinks comparison supports the proposed split, while the last May audit still recorded fixture-only partial implementation.

## Legacy Sources and Cases

- [CONVI-6260](../../work-items/CONVI-6260.md)
- [CONVI-6494](../../work-items/CONVI-6494.md)
- [CONVI-6968](../../work-items/CONVI-6968.md)
- [agent-quintiles-and-directionality](../../work-items/agent-quintiles-and-directionality.md) (ranking cross-link)

## Open Questions

- Finish a metric-by-tab API/source/filter matrix.
- Document sorting, ties, missing values, minimum-volume gates, and export parity.

See the [October 25 historical synthesis](../../deliverables/inactive-legacy-project-consolidation-2026-10-25.md) for consolidated cases, later superseding decisions, and their evidence limits.
