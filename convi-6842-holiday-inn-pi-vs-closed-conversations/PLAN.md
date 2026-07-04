# CONVI-6842 Performance Insights Conversation Volume Fix

## Summary
Implement the latest product decision: keep Closed Conversations strict/non-empty, but make Performance Insights “Conversation volume” consistently scorecard-based. The no-template chart should use QA score stats with `includeNaScored: true`, matching selected-template behavior and including N/A scorecards.

## Key Changes
- Save this plan in the project documentation for `CONVI-6842` before code changes.
  - Use title: `CONVI-6842 Performance Insights Conversation Volume Plan`.
  - Include the summary, implementation changes, verification plan, and assumptions from this plan.

- In `ConversationCountChart`, remove the no-template `ConversationStats` data path.
  - Stop using `useConversationStats` / `useInsightsRequestParams` for no-template volume.
  - Use `useQAScoreStatsRequestParams(filtersState, [QA_ATTRIBUTE_TYPE_TIME_RANGE])` and `useQAScoreStats` for no-template volume.
  - Rely on `PerformanceConversations` passing `filtersWithNAValues` so no-template QA stats include N/A scorecards.

- Preserve selected-template behavior.
  - Selected template continues using QA score stats with `includeNaScored: true`.
  - Keep the selected-template unfiltered and filtered series behavior.
  - Keep process templates using `totalScorecardCount`; conversation templates use `totalConversationCount`.

- Update chart state and rendering.
  - No-template chart renders a single QA-stat series from `qaScoreResult.scores[*].totalConversationCount`.
  - No-template metric number comes from `qaScoreResult.totalConversationCount`.
  - Loading state depends only on the QA stats queries needed for the active branch.
  - Remove now-unused imports and memoized request plumbing tied to conversation stats.

- Preserve existing worktree changes.
  - Do not revert the existing local modification in `PerformanceConversations.tsx` that removes `StatsGraphContainer` and widens `ConversationCountChart`.
  - Keep the `filtersWithNAValues` wrapper.

## Verification
- Do not add new tests.
- Run the existing targeted test file after implementation:
  - `yarn workspace @cresta/director-app test ConversationCountChart.test.tsx --passWithNoTests`
- Run typecheck if practical:
  - `yarn workspace @cresta/director-app tsc`
- Manually inspect the rendered Performance Insights page if a dev server is already available; otherwise rely on existing tests and typecheck.

## Assumptions
- Product decision is to make Performance Insights internally consistent and scorecard-centric, even if it differs from Closed Conversations.
- Closed Conversations behavior is intentionally strict and out of scope.
- No backend, API, protobuf, schema, or migration work is required.
- Existing tests may be adjusted only if they fail because implementation changed behavior; do not create additional test cases.
