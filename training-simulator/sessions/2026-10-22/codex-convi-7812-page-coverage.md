# CONVI-7812 page coverage

Verified 2026-10-22 using Director worktree xw/convi-7812-reporting-pass-rates at db184cd717 and live GitHub PR metadata/file list. PR #23579 merged 2026-10-14T01:59:45Z; deployment was not checked.

- Modules tab reporting drawer: overall pass rate is percentage-only for both conversation and quiz modules.
- Conversation criterion rows: passedResultCount / applicableResultCount as a percentage; N/A excluded; zero denominator uses placeholder.
- Quiz question rows remain correct/answered fractions.
- Lessons tab reporting drawer: overall lesson percentage no longer includes passed/total counts. Module drilldown reuses ModuleStatsBlock, ModuleStatsSection and toModuleStatsView, so inherits the changes (with active lessons hidden).
- Training Sessions dashboard/review drawer were not changed by the ticket; already render pass rate with fmtPercentage.
- Backend aggregation and denominators were not changed.

Evidence: packages/director-app/src/features/training-simulator/tabs/lesson-configuration/{module-stats/moduleStatsView.ts,module-stats/ModuleStatsBlock.tsx,lesson-stats-drawer/LessonStatsDrawer.tsx}; tabs/training-sessions/{dashboard/SessionsDashboard.tsx,session-review-drawer/SessionReviewDrawer.tsx}.

Credential use: gh used its existing authentication; no specific AWS, Okta, Azure, or SSH credential selected or read.
