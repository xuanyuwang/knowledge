# Codex Session: Analytics Response Impact Matrix

**Date:** 2026-06-27  
**Tool:** Codex  
**Project:** scorecard-template  
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Source revision inspected:** `eb72d7bb87a4`

## Prompt

List how each scorecard, score, and template attribute impacts the response of each analytics API.

## Source Files Inspected

- `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`
- `insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go`
- `insights-server/internal/analyticsimpl/retrieve_scorecard_stats_clickhouse.go`
- `insights-server/internal/analyticsimpl/common_clickhouse.go`
- `insights-server/internal/analyticsimpl/retrieve_manual_qa_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_manual_qa_progress.go`
- `insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_scorecard_criteria_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_appeal_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go`
- `insights-server/internal/analyticsimpl/retrieve_conversation_outcome_stats.go`

## Artifact Updated

- `deliverables/analytics-apis.md`

The response-impact matrix was merged into the canonical analytics API reference instead of remaining a standalone deliverable.

## Scope Decision

The first matrix covers scorecard-related analytics APIs rather than every analytics RPC in `insights-server`. Covered APIs:

- `RetrieveQAScoreStats`
- `RetrieveQAConversations`
- `RetrieveScorecardStats`
- `RetrieveManualQAStats`
- `RetrieveManualQAProgress`
- `RetrieveQMTaskStats`
- `RetrieveScorecardCriteriaStats`
- `RetrieveAppealStats`
- `RetrieveGroupCalibrationStats`
- `RetrieveConversationOutcomeStats`

`RetrieveClosedConversations` is left as a follow-up because it is a conversation search/list response with scorecard filters, not primarily a scorecard analytics response.

## Main Model

Each attribute can affect a response in several different ways:

- filter row eligibility;
- change response grouping or response `Attribute`;
- change metric math;
- appear directly in detail rows;
- interpret score rows through template structure;
- exclude rows by default or workflow-specific state.

The matrix separates template attributes, scorecard attributes, score attributes, and request-only analytics attributes.

## Important Findings

- `ScoreResource` changes both query source and response semantics for QA score APIs: score resource uses criterion score rows and weighted percentage math, while scorecard resource uses scorecard rows and aggregate score math.
- Template revision is not generally part of ClickHouse QA template filtering, but it is critical in Postgres workflows that parse exact template structures, especially appeal and group calibration.
- Criterion id is not globally unique; criterion group-by in QA score stats must include template id.
- Scorecard aggregate score can affect averages while missing/negative scores are excluded from score averages but not always from scorecard counts.
- N/A rows are default-excluded in QA score APIs but still matter for latest-version dedup and workflow comparisons.
- Group calibration and appeal are score-row driven at the criterion level, so empty scorecards or all-filtered scorecards can disappear from evaluated/appealed facts.

## Next Refinements

- Add `RetrieveClosedConversations` as a separate scorecard-filter-to-conversation-response matrix.
- Split `RetrieveDirectorTaskStats` into QM and group calibration modes with line-level evidence.
- Add test coverage references for all-N/A, empty scorecard, criterion collision, and score resource switching behavior.
