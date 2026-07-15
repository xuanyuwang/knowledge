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

- `convi-6753-weight-zero-pi-na/`
- `convi-6808-greenix-pi-scores/`
- `convi-6842-holiday-inn-pi-vs-closed-conversations/`
- `convi-7049-clo-filter/`
- `convi-7162-holidayinn-manager-scorecards-completed/`
- `convi-7230-performance-insights-outcome-filters/`
- `qa-score-popover-fix/`

## Open Questions

- Complete the exact surface catalog with request fields, transformations, empty/N/A behavior, and validation query for every chart/table.
- Separate intentional PI/Leaderboard filter differences from accidental drift.
