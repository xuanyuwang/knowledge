# CONVI-7667 Director fix

## Outcome

Implemented the narrow frontend fix for the confirmed Performance Insights request-construction regression and opened draft Director PR [#22772](https://github.com/cresta/director/pull/22772).

## Source context

- Worktree: `/Users/xuanyu.wang/repos/director-convi-7667`
- Branch: `convi-7667-process-scorecard-date-target`
- Base: `origin/main` at `40734741778`
- Commits: `961a44a922`, `e392f523f3`

The main Director checkout had unrelated changes and was preserved. A remote branch for INSI-4748 contained a broader email-tenant default change; this fix stayed process-specific because CONVI-7667 only requires removing a conversation-only target from standalone process-scorecard requests.

## Implementation

In `usePerformanceFilters.tsx`, process-template normalization now clears `dateRangeTarget` along with the other conversation-only filters. The normalization helper was exported so its boundary behavior can be tested directly.

The regression tests prove both sides of the contract:

- a process template carrying `CONVERSATION_ENDED_AT` normalizes to an undefined date target;
- a conversation template preserves `CONVERSATION_ENDED_AT`.

This makes generated QA analytics requests omit `conversationTimeRangeField` for process scorecards, avoiding the backend `conversation_d` inner join that removed rows with empty conversation IDs. No backend or data backfill is required.

## Validation

- `yarn test src/components/insights/hooks/performance-filters/usePerformanceFilters.test.tsx`: 1 file, 4 tests passed.
- `yarn tsc` from `packages/director-app`: passed.
- Repository commit hooks: protected-file check skipped as not applicable; i18n extraction, lint, i18next lint, formatting, and ticket-prefix checks passed.
- `git diff --check`: passed before commit.
- Draft PR verified open with head `convi-7667-process-scorecard-date-target`, base `main`, and draft state true.
- PR metadata declares `smoke-functional` QA because this changes filtering behavior. Reproduction steps and expected behavior are populated; before/after preview videos are still required before the QA metadata check can pass.
- Working-session HAR validation found two internally reconstructed process count requests still carrying `CONVERSATION_ENDED_AT` and returning zero. Commit `e392f523f3` routes those auxiliary states through the same process normalization and adds request-boundary coverage; both focused suites now pass 6 tests.

## Follow-up

After merge and deployment, validate the reported July 2026 template in Performance Insights. Expected live values are 338 scorecards and approximately 99.72%, subject to source-data changes after investigation.
