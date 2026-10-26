# Codex Implementation - CONVI-7162

**Date:** 2026-07-10
**Ticket:** CONVI-7162
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree:** `convi-7162-holiday-inn-club-vacations-manager-leaderboard-scorecards` at `/Users/xuanyu.wang/repos/go-servers-convi-7162`

## Issue Summary

Holiday Inn Club Vacations Manager Leaderboard `Scorecards Completed` daily counts were redistributed or missing because the CONVI-6968 QA-backed implementation used `scorecard_time` for submitted scorecard completion metrics. For conversation scorecards, `scorecard_time` is conversation time, not scorecard submission time.

The expected metric is submit-time based: how many scorecards the manager completed/submitted each day.

## Implementation

Changed `go-servers` analytics ClickHouse query building so submitted scorecard reviewer/completion QA requests use `scorecard_submit_time` for time filtering and grouping.

Files changed:

- `insights-server/internal/analyticsimpl/common_clickhouse.go`
- `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`
- `insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go`
- `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_test.go`

Key behavior:

- Added an explicit time-column override option for QA ClickHouse filter/group-by helpers.
- Added a narrow request-shape gate for scorecard-resource requests with `scorecardReviewerAudience` and only `MANUALLY_SUBMITTED` scorecard statuses.
- Routed that request shape to `scorecard_submit_time`.
- Left ordinary QA scorecard queries on existing conversation-time semantics.
- Included `scorecard_submit_time` in the scorecard-level QA stats projection so daily grouping can use it in the outer aggregate.
- Applied the same request-shape time option to `RetrieveQAConversations`, so the Manager scorecard drawer follows the same submitted-time window.

## Validation

Passed:

```text
go test ./insights-server/internal/analyticsimpl -run 'TestRetrieveQAScoreStatsClickhouseQuery_WithSubmittedScorecardReviewerUsesSubmitTime|TestRetrieveQAScoreStatsClickhouseQuery_WithAgentAndSubmitterFilters|TestParseClickhouseGroupByForQAAttribute_WithScorecardSubmitter|TestParseScoreConditionsForQAAttribute'
```

Also ran:

```text
git diff --check
```

Attempted full package validation:

```text
go test ./insights-server/internal/analyticsimpl
```

This broad test run produced no output for several minutes and was stopped. Focused coverage for the changed query semantics passed.

## Follow-Ups

- Consider a full package or CI validation run before merge.
- If reviewers prefer an explicit public API knob over request-shape detection, this will require coordinated proto and `director` changes.

## Proto Field Investigation

The current proto already has `conversation_time_range_field` on both QA APIs:

- `RetrieveQAScoreStatsRequest.conversation_time_range_field = 11`
- `RetrieveQAConversationsRequest.conversation_time_range_field = 9`

Both fields reuse `RetrieveClosedConversationsRequest.TargetFieldForTimeRange`, whose values are currently `UNSPECIFIED`, `CONVERSATION_STARTED_AT`, and `CONVERSATION_ENDED_AT`.

Backend usage currently maps that enum through ClickHouse query helpers:

- unspecified/default -> `conversation_start_time`, table-mapped to `scorecard_time` for scorecard/score tables
- ended-at -> `conversation_end_time`, which requires a conversation-table join because scorecard tables do not have that column

Frontend usage currently passes `filtersState.dateRangeTarget` into `conversationTimeRangeField` in shared QA stats/conversations request builders. Manager Leaderboard scorecard requests do not set a date-range target, so they currently get default conversation-start semantics.

Recommendation from this pass: prefer a new QA/scorecard-scoped request field rather than extending `RetrieveClosedConversationsRequest.TargetFieldForTimeRange` with scorecard submission time. Extending the existing enum is technically smaller, but semantically leaky because a `RetrieveClosedConversationsRequest` enum would gain a non-conversation field.
