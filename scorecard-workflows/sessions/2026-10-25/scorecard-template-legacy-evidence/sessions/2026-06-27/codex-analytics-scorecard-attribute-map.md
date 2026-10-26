# Codex Session: Analytics Scorecard Attribute Map

**Date:** 2026-06-27  
**Tool:** Codex  
**Project:** scorecard-template  
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Source revision inspected:** `eb72d7bb87a4`  
**Knowledge repo:** `/Users/xuanyu.wang/repos/knowledge`

## Prompt

Start building the scorecard/template knowledge base from analytics. The proposed method was:

- focus only on scorecards and analytics;
- for each scorecard attribute, analyze how analytics APIs use it;
- start with simple things like DB columns;
- later expand to advanced conditions such as empty scorecards and all-N/A scorecards;
- use both directions: attribute-first and API/query-first.

## Context Read

- `workflow/ai-operating-model.md`
- `scorecard-template/project.yaml`
- `scorecard-template/README.md`
- `scorecard-template/deliverables/empty-scorecards-workflow-and-api-analysis.md`
- `workspace/repos.yaml`

The project already had the domain artifact / behavioral frame model and an empty-scorecards analytics cross-section. This session created the next seed artifact for analytics attributes rather than rewriting the existing empty-scorecards work.

## Source Files Inspected

- `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`
- `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go`
- `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_scorecard_stats_clickhouse.go`
- `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go`
- `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_manual_qa_progress.go`
- `/Users/xuanyu.wang/repos/go-servers/shared/clickhouse/conversations/conversation.go`
- `/Users/xuanyu.wang/repos/go-servers/shared/clickhouse/conversations/scorecard_score.go`

## Main Findings

Analytics scorecard behavior is not one query shape. The same request filter is split across several query layers:

- common score/scorecard conditions: time range, agent, reviewer audience, usecase, template id;
- scorecard conditions: submitted/draft/auto/published status;
- score conditions: N/A exclusion, score type, score range, criterion identifiers;
- conversation conditions: conversation-level filters and end-time filtering;
- moment annotation CTEs: metadata filters;
- Postgres workflow queries: task, template, scorecard, and score tables.

`RetrieveQAScoreStats` and `RetrieveQAConversations` both dedupe latest scorecard versions from `scorecard_d`, then join score rows to those latest scorecard rows on `scorecard_id` and `scorecard_last_update_time`. This is important for all-N/A overwrites because score rows can be filtered out while scorecard metadata still identifies the latest version.

Criterion group-by uses both `criterion_id` and `scorecard_template_id`, because criterion ids are not globally unique across templates.

Template filters in the common ClickHouse QA attribute path parse template names but reduce them to `scorecard_template_id IN (...)`; revision is not part of that filter. Revision still matters for projection and PG flows that parse template structure.

## Artifact Created

- `deliverables/analytics-apis.md`

The artifact now serves as the canonical analytics API reference and records:

- the core ClickHouse/Postgres read surfaces;
- projected scorecard and score columns;
- QA score stats and QA conversations query shape;
- the filter split by query layer;
- Postgres-backed analytics surfaces;
- a starting attribute inventory;
- the attribute-by-API response impact matrix;
- edge cases for future passes.

## Next Steps

- Expand all-N/A scorecard behavior as its own focused cross-section.
- Build an API-by-API matrix for `RetrieveQAScoreStats`, `RetrieveQAConversations`, `RetrieveScorecardStats`, `RetrieveManualQAProgress`, `RetrieveQMTaskStats`, `RetrieveAppealStats`, and `RetrieveGroupCalibrationStats`.
- Build an attribute-by-attribute matrix for template id, template revision, criterion id, scorecard status, score type, N/A, auto-failed, submitter, task ids, score ranges, and score resource.
