# CONVI-7744: Fix conversation scorecard pagination

**Status:** validating
**Primary domain:** `scorecard-workflows`
**Primary subdomain:** `scorecard-lifecycle`
**Official ticket:** [CONVI-7744](https://linear.app/cresta/issue/CONVI-7744/creating-a-new-scorecard-encounter-errors-say-the-scorecard-for-the)
**Last updated:** 2026-09-24

## Objective and Impact

- **Objective:** Ensure conversation scoring finds existing scorecards beyond the first API page.
- **Customer/system impact:** High-scorecard-count conversations can otherwise show a new form for an existing template, fail duplicate creation with 409, then emit a misleading autosave 404.
- **Role:** diagnosed and implemented

## Scope

**In scope**

- Follow all `ListScorecards` pages in the conversation coaching-details loader.
- Merge scorecards and unique template metadata across pages while preserving cancellation.
- Add focused pagination coverage.

**Non-goals**

- Change backend scorecard uniqueness rules or make `CreateScorecard` idempotent.
- Change scorecard creation or autosave ownership and retry behavior.
- Attribute the latent bug to an unproven recent trigger.

## Source Context

- **Repos:** `director`
- **Worktrees:** `/Users/xuanyu.wang/repos/director-convi-7744`
- **Branches:** `xw/convi-7744-scorecard-pagination`
- **PRs/commits:** commits `1ce41abee1`, `05aa63c98a`, `f8da827bc0`; [director PR #23190](https://github.com/cresta/director/pull/23190) (draft)

## Current Understanding

The HAR proves the conversation-detail request returned only the default first 100 of 197 scorecards and exposed a non-empty `nextPageToken`. The existing selected-template scorecard was item 175, so the UI entered the create path and received the backend's expected 409 duplicate rejection. The rejected generated name remained in a ref, causing the next autosave to call update and receive 404. This is a long-standing correctness gap, not a demonstrated recent regression.

The implemented fix retrieves every page with a bounded page size and combines scorecards and template metadata. It intentionally leaves scorecard creation and autosave behavior unchanged: creation belongs to its owning workflow, and autosave should not acquire responsibility for retrying creation.

## Findings and Decisions

- Follow `nextPageToken` until exhaustion; increasing only `pageSize` would retain a correctness boundary.
- Deduplicate templates by versioned resource name while accumulating pages.
- Track cancellation independently of the currently active request. After each awaited page, return before processing results or requesting another page when cancelled.
- Keep the fix at the incomplete-read boundary. If no scorecard exists, another workflow owns creation; if one exists, complete pagination makes it visible without an autosave retry.

## Blockers and Dependencies

- None.

## Validation and Rollout

- Two focused Vitests passed: all-page aggregation and cancellation immediately after resolving a page with a next token.
- `yarn workspace @cresta/director-app lint:precommit` passed.
- `yarn tsc` passed.
- `git diff --check` passed in the Director worktree.
- Commits `1ce41abee1`, `05aa63c98a`, and `f8da827bc0` passed the repository pre-commit hooks. Draft PR #23190 is open; preview/manual validation remains pending.
- The updated local `with-fix.har` confirms the patched detail request: `scorecardView=FULL` includes `pageSize=500`. The selected conversation returned zero scorecards and no `nextPageToken`, so the loop correctly stopped after page 1; the capture does not exercise a page-2 request.

## Next Actions

1. For stronger functional proof, open a populated conversation and capture the `pageSize=500` detail response; an actual page-2 request requires more than 500 matching scorecards.
2. Select the appropriate QA request and move the PR out of draft when review-ready.

## Timeline

- 2026-09-23 — Diagnosed the 409/404 request sequence from the HAR and posted verified evidence to Linear. Evidence: `sessions/2026-09-23/codex-create-scorecard-har-errors.md`, Linear comment `d35a7fa9-1d23-4011-9344-be3769ea2f94`.
- 2026-09-24 — Created the dedicated Director worktree and implemented all-page loading. Evidence: `sessions/2026-09-24/codex-convi-7744-scorecard-fix.md`.
- 2026-09-24 — Committed the validated fix as `1ce41abee1`; all repository pre-commit hooks passed.
- 2026-09-24 — Pushed the branch and opened draft Director PR #23190 using the repository PR skill and unchanged template checkboxes.
- 2026-09-24 — Removed failed-create autosave recovery from the PR in commit `05aa63c98a`; creation and autosave behavior remain unchanged, and the PR is scoped to the incomplete paginated read.
- 2026-09-24 — Inspected `with-fix.har`; it does not contain the fixed detail request or exercise a populated conversation, so pagination remains unverified in preview.
- 2026-09-24 — Re-inspected the replaced local `with-fix.har`; it confirms `pageSize=500` on the `scorecardView=FULL` detail request. The selected conversation was empty, so no page token or second request was expected.
- 2026-09-24 — Verified and fixed the resolved-page cancellation race in `f8da827bc0`; cancellation now prevents processing the resolved page or starting its successor, with focused regression coverage.
