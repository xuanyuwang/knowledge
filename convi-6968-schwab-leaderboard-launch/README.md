# CONVI-6968 - Schwab Leaderboard Launch Support

**Created:** 2026-06-01  
**Updated:** 2026-06-08

## Overview

This project tracks the investigation, backend API changes, and frontend integration planning for the Schwab leaderboard launch work on the Insights Leaderboard page.

The current focus is the data path and frontend implementation shape for the Agent and Manager tabs, specifically around scorecard counts grouped by scorecard template and the migration of Manager scorecard data to QA APIs with submitter filtering.

## Current Objective

Document the final backend API semantics for Manager submitter-attributed scorecard data and the remaining frontend migration path.

## Current Scope

In scope:

- Agent leaderboard data path for scorecards grouped by template
- Manager leaderboard `Scorecards completed` data source and semantics
- API/filter compatibility for current leaderboard filters
- Feasibility of deriving manager submitted or reviewed scorecards grouped by template

Out of scope:

- Team leaderboard changes
- Final UI design

## Key Findings

- The Agent leaderboard submitted-scorecard column should use a separate `RetrieveQAScoreStats` query with `scorecardStatuses = [MANUALLY_SUBMITTED]`; the existing Performance score query cannot be reused because an empty status filter includes all matching statuses.
- `RetrieveQAScoreStats` does not expose a template dimension, so it cannot directly return scorecard counts grouped by template per agent.
- `RetrieveQAConversations` can be post-processed into submitted scorecard counts grouped by template because the transformed response is grouped by `scorecardId` and includes `scorecardTemplateId`; it should also hardcode `scorecardStatuses = [MANUALLY_SUBMITTED]` for the Agent drawer.
- The Manager leaderboard `Scorecards completed` column uses `RetrieveScorecardStats`, grouped by `ATTRIBUTE_TYPE_AGENT`, and the UI reads `averageScorecardCompletedPerUser`.
- As of the latest backend change, `RetrieveScorecardStats` now treats completed scorecards as distinct submitted scorecards attributed to `submitter_user_id`; its ClickHouse query rewrites `scorecard_time` to `scorecard_submit_time` and `agent_user_id` to `submitter_user_id`.
- The current manager scorecard stats response has no template dimension, so it cannot be directly reused for grouped-by-template reporting.
- The backend QA API gap for Manager submitter attribution has been closed. Proto added `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER = 10`, and go-servers now supports dual agent and submitter user axes on `RetrieveQAScoreStats` and `RetrieveQAConversations`.
- `QAAttribute.users/groups` remain agent filters and apply to `agent_user_id`; `QAAttribute.scorecard_reviewer_audience` is the submitter filter and applies to `submitter_user_id`.
- `QA_ATTRIBUTE_TYPE_AGENT` groups by `agent_user_id`; `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` groups by `submitter_user_id`; `QA_ATTRIBUTE_TYPE_GROUP` remains agent-group aggregation.
- Manager aggregate can now migrate to `RetrieveQAScoreStats` with manager selection in `scorecard_reviewer_audience` and submitter grouping when needed.
- Manager drawer can now migrate to `RetrieveQAConversations` with manager selection in `scorecard_reviewer_audience` and FE grouping by scorecard template.
- The older submit-time parity requirement is no longer blocking the Manager QA API migration because the project decision is to accept the QA API time-range semantics instead of preserving the old `RetrieveScorecardStats` submit-time behavior.
- FE should add a new clickable Agent column titled `Number of submitted scorecards`, and make the existing Manager `Scorecards completed` cell clickable instead of adding a duplicate Manager column.
- A new shared leaderboard drawer is recommended. Use parent-owned state from the Agent/Manager page components, `FullDrawer`, and Mantine `Accordion`; reuse normalized data hooks rather than reusing the Performance conversation examples drawer directly.
- FE integration cost remains medium: about 1-2 focused FE days after generated client types land.

## Status

Backend merged; FE integration pending/active

## Source Context

- **Primary repo:** `director`
- **Repo path:** `/Users/xuanyu.wang/repos/director`
- **Active worktree:** `/Users/xuanyu.wang/repos/director`
- **Branch:** `main`

Related merged PRs:

- `cresta/cresta-proto#8803`: added `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER = 10`
- `cresta/go-servers#28525`: migrated `RetrieveQAConversations` user filtering
- `cresta/go-servers#28526`: added QA submitter ClickHouse filters
- `cresta/go-servers#28530`: added `RetrieveQAConversations` submitter audience support
- `cresta/go-servers#28527`: added `RetrieveQAScoreStats` submitter support

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
| 2026-06-08 | Recorded merged proto/go backend support for dual agent and submitter filter axes and updated Manager QA API migration guidance. |

## Related Artifacts

- `project.yaml`
- `log/2026-06-01.md`
- `log/2026-06-08.md`
- `deliverables/api-decision-table.md`
- `deliverables/agent-tab-api-analysis.md`
- `deliverables/manager-tab-api-analysis.md`
- `deliverables/fe-ui-investigation.md`
- `deliverables/`
- `decisions/`
- `sessions/`
