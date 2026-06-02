# CONVI-6968 - Schwab Leaderboard Launch Support

**Created:** 2026-06-01  
**Updated:** 2026-06-02

## Overview

This project tracks the investigation and implementation planning for the Schwab leaderboard launch work on the Insights Leaderboard page.

The current focus is the data path for the Agent and Manager tabs, specifically around scorecard counts grouped by scorecard template and the feasibility of reusing existing leaderboard APIs.

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

- The Agent leaderboard QA score path uses `RetrieveQAScoreStats`, which supports the current QA filter set better than `RetrieveQAConversations`, including `filterToAgentsOnly`.
- `RetrieveQAScoreStats` does not expose a template dimension, so it cannot directly return scorecard counts grouped by template per agent.
- `RetrieveQAConversations` can be post-processed into scorecard counts grouped by template because the transformed response is grouped by `scorecardId` and includes `scorecardTemplateId`, but it does not support `filterToAgentsOnly`.
- The Manager leaderboard `Scorecards completed` column uses `RetrieveScorecardStats`, grouped by `ATTRIBUTE_TYPE_AGENT`, and the UI reads `averageScorecardCompletedPerUser`.
- The current manager scorecard stats response has no template dimension, so it cannot be directly reused for grouped-by-template reporting.
- `ListScorecards` is the clearest raw API for manager-submitted scorecards grouped by template because it supports `submitterUserNames`, `templateName`, and submit-time filters, but it does not match every leaderboard filter out of the box.

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

## Related Artifacts

- `project.yaml`
- `log/2026-06-01.md`
- `deliverables/api-decision-table.md`
- `deliverables/agent-tab-api-analysis.md`
- `deliverables/manager-tab-api-analysis.md`
- `deliverables/`
- `decisions/`
- `sessions/`
