# CONVI-7667 live UI reproduction follow-up

## Request

Recheck the exact Oportun process scorecard template in the user's authenticated Chrome session using the identifiers and scope claim in Linear comment `40b9e1d6-4bca-4758-9c59-682e55358768`.

## Linear context

The authenticated Linear MCP returned the Support comment. It claims the template returns no results for both current and prior years and supplies the exact template/revision plus four QM Report scorecard IDs.

Template/revision: `94e6b75d-b1eb-4e41-a477-1d070d96fa9c@c0139eb5`.

## Live observation

The Chrome tab was already authenticated to Oportun Backoffice Processes, Performance Insights, with template `QA-NOA-BO-OPS-ZEUS APP VERIFICATION V1.1` selected.

- July 1-31, 2025, weekly: Performance Score 99%; volume 353. The accessibility tree identified chart dates from 2025-06-29 through 2025-07-27 and 353 scorecards.
- July 1-31, 2026, weekly: Performance Score 100%; volume 335 in the rendered page observed before switching to the prior-year window.

The reported zero-data state therefore did not reproduce in the browser's current filter state. The exact template is not globally absent from Performance Insights, and both years render data. This observation alone does not establish why another browser/session rendered zero.

## HAR evidence

The user saved the working session as `/Users/xuanyu.wang/repos/oportun.cresta.com.har`. Analysis excluded cookies and authorization headers.

- 14 `qaScoreStats:retrieve` requests completed successfully.
- 12 omitted `conversationTimeRangeField`; populated responses included the visible July 2025 result of 353 scorecards and average score `0.9934037`.
- Two requests sent `TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT`; both returned exactly zero scorecards and average score zero.
- The two empty requests were the count chart's internally reconstructed current/delta unfiltered states. The chart still rendered because its primary filtered request omitted the target and returned 353.

This proves that `CONVERSATION_ENDED_AT` produces the predicted zero for this exact process template in the live API. It does not prove that the originally failing page used those empty results for its visible score and volume; that requires a HAR from a failing session.

The HAR also exposed an implementation gap in the initial draft: `ConversationCountChart` rebuilt auxiliary states with `getInitialFiltersState`, reintroducing the default conversation target after the outer process state was normalized. Draft PR #22772 now normalizes those reconstructed states in commit `e392f523f3`, with coverage asserting all three process count requests omit the field.

## Recent-fix audit

The HAR loaded Director assets from release `2026-09-10-58a0ca6`; Git resolves that suffix to commit `58a0ca6dcb89c8d6e0229de4cde2c0098ed2b5c0`. That deployed source still defaults Performance Insights to `CONVERSATION_ENDED_AT` and does not clear the target for process templates. No relevant file change exists between that build and current `origin/main` at `407347417788c20448b3f95c0f9ccc9da882bfc2`.

There is parallel work on remote branch `origin/sdey/insi-4748-date-target-default`: commit `8c12fa42961f2c6c09afc682f95fe95bfabbaa15`, dated 2026-09-11, defaults close time only for email tenants, drops cached targets for non-email tenants, and preserves the chosen target in count-chart comparison states. It is not an ancestor of either the HAR production build or current main, and GitHub PR search found no associated PR. Therefore the issue has not already been fixed in production, although this unmerged branch substantially overlaps the draft CONVI fix and should be reconciled before merge.

## Tool limitation

The Linear MCP was authenticated. The dedicated Chrome/browser-control MCP reported `Codex auth token is unavailable`; native Chrome accessibility remained usable for the rendered UI but did not expose request payloads or network traces. No Linear comment or customer data was changed.
