# CONVI-6842 Director PR Diff Cleanup

Source repo: `/Users/xuanyu.wang/repos/director-convi-6842`  
Branch: `xwang/convi-6842-conversation-volume`  
PR: https://github.com/cresta/director/pull/20153

## User Direction

The PR diff should only replace the no-template Performance Insights conversation-volume data source from `RetrieveConversationStats` to `RetrieveQAScoreStats`. Extra UI, legend, export, and memo typing changes should be removed unless required by the source swap.

## Findings

- The effective working diff is limited to `ConversationCountChart.tsx`.
- The prior CI failure in `agent-performance.spec.ts` was consistent with no-template legend behavior being changed.
- The cleanup restores the original no-template chart shape and legend behavior while feeding the existing no-template volume series from QA score stats.

## Implementation Notes

- `scoreStats` now runs for no-template and template cases.
- The no-template `useConversationStats` / `useInsightsRequestParams` path is removed.
- The first volume series uses `convCountSeriesFiltered` when no template exists, replacing the old `statsNoTemplate` series.
- The second filtered series remains present but carries no data for no-template, matching the prior no-template chart shape.
- The headline metric uses `scoreStats.data?.qaScoreResult[metricKey]`.
- Loading state uses the QA stats queries required for the active branch.

## Next Steps

## Validation

- `yarn workspace @cresta/director-app eslint --cache --cache-strategy content --cache-location ../../.eslintcache/director-app --max-warnings=0 src/components/insights/qa-insights/conversation-count-chart/ConversationCountChart.tsx` passed.
- `yarn workspace @cresta/director-app test ConversationCountChart.test.tsx` passed.
- `NODE_OPTIONS='--max-old-space-size=8192' yarn workspace @cresta/director-app tsc` passed.
- The repo pre-commit hook also passed protected-file checks, i18n extraction/lint, lint, format, and commit-message ticket prefixing.

## Result

Committed and pushed `bf1f564675` (`[CONVI-6842] Trim conversation volume data source change`) to `origin/xwang/convi-6842-conversation-volume`.
