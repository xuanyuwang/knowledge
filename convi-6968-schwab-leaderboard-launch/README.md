# CONVI-6968 - Schwab Leaderboard Launch Support

**Created:** 2026-06-01  
**Updated:** 2026-06-03

## Overview

This project tracks the investigation and implementation planning for the Schwab leaderboard launch work on the Insights Leaderboard page.

The current focus is the data path and frontend implementation shape for the Agent and Manager tabs, specifically around scorecard counts grouped by scorecard template and the feasibility of reusing existing leaderboard APIs.

## Current Objective

Document the current data sources, filter coverage, and implementation constraints for adding template-grouped scorecard counts to the Agent and Manager leaderboard tabs.

## Current Scope

In scope:

- Agent leaderboard data path for scorecards grouped by template
- Manager leaderboard `Scorecards completed` data source and semantics
- API/filter compatibility for current leaderboard filters
- Feasibility of deriving manager submitted or reviewed scorecards grouped by template

Out of scope:

- Team leaderboard changes
- Final UI design
- Final backend API design

## Key Findings

- The Agent leaderboard submitted-scorecard column should use a separate `RetrieveQAScoreStats` query with `scorecardStatuses = [MANUALLY_SUBMITTED]`; the existing Performance score query cannot be reused because an empty status filter includes all matching statuses.
- `RetrieveQAScoreStats` does not expose a template dimension, so it cannot directly return scorecard counts grouped by template per agent.
- `RetrieveQAConversations` can be post-processed into submitted scorecard counts grouped by template because the transformed response is grouped by `scorecardId` and includes `scorecardTemplateId`; it should also hardcode `scorecardStatuses = [MANUALLY_SUBMITTED]` for the Agent drawer.
- The Manager leaderboard `Scorecards completed` column uses `RetrieveScorecardStats`, grouped by `ATTRIBUTE_TYPE_AGENT`, and the UI reads `averageScorecardCompletedPerUser`.
- As of the latest backend change, `RetrieveScorecardStats` now treats completed scorecards as distinct submitted scorecards attributed to `submitter_user_id`; its ClickHouse query rewrites `scorecard_time` to `scorecard_submit_time` and `agent_user_id` to `submitter_user_id`.
- The current manager scorecard stats response has no template dimension, so it cannot be directly reused for grouped-by-template reporting.
- For MVP, `ListScorecards` is the clearest raw API for Manager drawer scorecard rows grouped by template because it supports `submitterUserNames`, `templateName`, and submit-time filters.
- `RetrieveQAConversations` should not replace `ListScorecards` for the Manager drawer yet: it can return scorecard/template rows, but the traced ClickHouse path does not filter by `submitter_user_id`, does not apply `scorecardReviewerAudience`, and does not use `scorecard_submit_time` as its default time basis.
- `RetrieveQAScoreStats` should not replace `RetrieveScorecardStats` for the Manager aggregate yet: it can count submitted scorecards, but its user grouping is still `agent_user_id`, not `submitter_user_id`.
- The Manager drawer data fetching should be wrapped in a normalized provider abstraction so it can later switch to `RetrieveQAConversations` if that API gains submitter filtering and submit-time range support.
- FE should add a new clickable Agent column titled `Number of submitted scorecards`, and make the existing Manager `Scorecards completed` cell clickable instead of adding a duplicate Manager column.
- A new shared leaderboard drawer is recommended. Use parent-owned state from the Agent/Manager page components, `FullDrawer`, and Mantine `Accordion`; reuse normalized data hooks rather than reusing the Performance conversation examples drawer directly.

## Status

Active

## Source Context

- **Primary repo:** `director`
- **Repo path:** `/Users/xuanyu.wang/repos/director`
- **Active worktree:** `/Users/xuanyu.wang/repos/director`
- **Branch:** `main`

Related investigation context:

- `director`
- `cresta-proto`

## Log History

| Date | Summary |
|------|---------|
| 2026-06-01 | Created the project and documented the initial data investigation for Agent and Manager leaderboard scorecard metrics. |
| 2026-06-02 | Added a dedicated Agent-tab API analysis and clarified the recommended split between aggregate and drill-down APIs. |
| 2026-06-02 | Added FE UI investigation for table-column insertion points, drawer opening patterns, and implementation workload. |
| 2026-06-02 | Updated decisions: Agent column is submitted-only with a separate query; Manager drawer uses `ListScorecards` for MVP behind a future-proof data-provider abstraction. |
| 2026-06-03 | Updated Manager API decision after `RetrieveScorecardStats` moved from creator attribution to submitter attribution. |

## Related Artifacts

- `project.yaml`
- `log/2026-06-01.md`
- `deliverables/api-decision-table.md`
- `deliverables/agent-tab-api-analysis.md`
- `deliverables/manager-tab-api-analysis.md`
- `deliverables/fe-ui-investigation.md`
- `deliverables/`
- `decisions/`
- `sessions/`
