# Performance Insights

## Purpose

Explain every Performance Insights chart, metric card, heatmap, leaderboard table, filter, drilldown, and export from visible meaning through FE request construction to backend calculation and source data.

## Current Semantics

- The page is predominantly backed by `RetrieveQAScoreStats`.
- Score trends and metric cards commonly group by time range.
- Performance progression uses criterion/time groupings; per-agent grouping is added for specific tables or exports.
- Conversation volume uses scorecard-backed QA stats and includes all-N/A scorecards when `includeNaScored` is enabled.
- Page-wide filters can have PI-specific defaults/interpretation even when the same control appears on Leaderboard.
- Outcome filters and criterion display behavior belong here when the difference is page presentation; underlying score semantics remain in QA Score.

## Source Map

- Director `qa-insights/performance-conversations/`
- Director `qa-insights/score-line-chart/`
- Director `qa-insights/conversation-count-chart/`
- Director `qa-insights/performance-progression/`
- Director `qa-insights/leaderboard-*`
- `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`

## Legacy Sources and Cases

- [CONVI-6753](../../work-items/CONVI-6753.md)
- [CONVI-6808](../../work-items/CONVI-6808.md)
- [CONVI-6842](../../work-items/CONVI-6842.md)
- [clo-filter-query-optimization-and-rollout](../../work-items/clo-filter-query-optimization-and-rollout.md)
- [CONVI-7162](../../work-items/CONVI-7162.md)
- [CONVI-7230](../../work-items/CONVI-7230.md)
- [CONVI-5805](../../work-items/CONVI-5805.md)

## Open Questions

- Complete the exact surface catalog with request fields, transformations, empty/N/A behavior, and validation query for every chart/table.
- Separate intentional PI/Leaderboard filter differences from accidental drift.

See the [October 25 historical synthesis](../../deliverables/inactive-legacy-project-consolidation-2026-10-25.md) for consolidated cases, later superseding decisions, and their evidence limits.
