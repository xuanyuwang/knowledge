# CONVI-7941: show detailed export failures

Date: 2026-11-02
Source repo: `/Users/xuanyu.wang/repos/director`
Worktree: `/Users/xuanyu.wang/repos/director-convi-7941-export-errors`
Branch: `xw/convi-7941-export-errors`
Base: `344235d59316f03440436e05335e56c3c86abf8a` (origin/main)
Canonical work item: [CONVI-7941](../../work-items/CONVI-7941.md)

> Historical experiment: this silent-error change and its test were reverted on November 5 at user request. PR #24681 contains only the progression await-both fix and its regression suite.

## Authorization and change

User authorized creating a worktree to implement a confident fix. The original report was not reproduced; this implements only the separately proven silent export-error bug. The previous `onError` cleared notifications then constructed an error-handler callback without executing it. It now accepts the mutation error and calls Director's existing `handleError` utility with Toast handling. CSV construction, request filters, comments, pagination, successful downloads, and notification cleanup are unchanged.

## Regression evidence

Added `useExportPerformanceInsightCriteria.test.ts` next to the hook. Tests render the real hook with Director's standard providers, reject the exact `CrestaAPI.insights.retrieveQAConversations` method lazily with an HTTP 500 service error, execute `mutateAsync`, and capture notifications through the real registered UI error handler. Conversation and process template cases both failed before the fix because the notification list was empty. Both passed after the fix.

Commands run from `packages/director-app`:

- `yarn test src/hooks/insights/useExportPerformanceInsightCriteria.test.ts`: 2 passed after fix; 2 expected failures before.
- `yarn tsc`: passed.
- Changed-file Biome check: passed.
- Changed-file ESLint: passed after adding the required explicit return type to the test helper.
- Source and edited knowledge-document `git diff --check`: passed.

Dependency setup used `yarn install --immutable` and self-authenticating `gh auth token` for package registry access without printing or persisting the token. No AWS, Okta, Azure, or SSH credentials were selected for implementation. Local Node 24.11.1 meets the package engine range but differs from `.nvmrc` 24.22.1. Dependency installation reported existing peer warnings; tests reported missing third-party source maps. No lockfile changes.

## Handoff

Local, uncommitted implementation only. No external issue comments, push, PR, deployment, or database mutations. The original reported failing customer/template/date/file is still needed to establish its root cause. No source change should be described as proof that the broader reported download issue is fixed.
