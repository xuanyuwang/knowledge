# CONVI-7744 scorecard creation fix

## Context

- Date: 2026-09-24 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7744`
- Branch: `xw/convi-7744-scorecard-pagination`
- Base: `origin/main` at `0a5c0675bcb5563f643c6a8ec97c92be5e3e46e9`
- Ticket: [CONVI-7744](https://linear.app/cresta/issue/CONVI-7744/creating-a-new-scorecard-encounter-errors-say-the-scorecard-for-the)
- Investigation: `sessions/2026-09-23/codex-create-scorecard-har-errors.md`

## Implementation

- Added a cancellable all-pages `ListScorecards` helper with a page size of 500.
- Updated `useConversationCoachingDetails` to use the helper, so existing scorecards and template metadata are accumulated until `nextPageToken` is empty.
- Deduplicated templates by their versioned resource name.
- Added independent cancellation state and a post-`await` guard so cancellation after page resolution cannot process that page or start the next request.
- Initially added failed-create name recovery, then removed it from the PR in commit `05aa63c98a` after clarifying ownership: autosave should not retry scorecard creation, and complete pagination makes an existing scorecard visible without a retry.

## Tests and Checks

- `yarn test src/hooks/conversation/listScorecardsAllPages.test.ts` — 1 file, 2 tests passed after adding cancellation coverage.
- `yarn workspace @cresta/director-app lint:precommit` — passed.
- `yarn tsc` — passed.
- `git diff --check` — passed before knowledge updates.

The root `yarn lint:precommit` wrapper ran the Director lint target successfully and then exited 1 at its final `git diff --quiet`, which is expected while the fix is uncommitted. The package lint command itself exited 0.

## Current State

The pagination implementation and automated validation are complete at commit `f8da827bc0`, including the resolved-page cancellation guard. Draft Director PR [#23190](https://github.com/cresta/director/pull/23190) was updated using the repository template with all QA/reviewer checkboxes unchanged. Preview validation with a populated conversation remains pending.

## `with-fix.har` verification

- Artifact: `/Users/xuanyu.wang/repos/with-fix.har`
- Current SHA-256: `57e05f92d9d18145b14e9a30945848c25d925385714369f96c3f74eb1004bfd0`
- The file was replaced after the initial staging capture with a local-fix capture.
- The conversation-detail `ListScorecards` request at `2026-09-24T17:10:42.132Z` has `scorecardView=FULL` and `pageSize=500`, confirming the patched helper is active.
- That detail response contains zero scorecards and an empty `nextPageToken`; stopping after the first request is therefore correct.
- Conclusion: the updated HAR verifies the patched request configuration but does not exercise page traversal. A populated conversation with at most 500 matching scorecards still produces one request; proving the second-page branch from a HAR requires more than 500 matching scorecards.
