# CONVI-7612: Cresta admin scorecard editing and lock explanations

**Status:** review changes requested  
**Primary domain:** `scorecard-workflows`  
**Primary subdomain:** `permissions-and-visibility`  
**Official ticket:** [CONVI-7612](https://linear.app/cresta/issue/CONVI-7612/cng-holdings-admin-cannot-change-a-non-appealed-auto-fail-criterion)  
**Last updated:** 2026-09-02

## Objective and Impact

- **Objective:** Let Cresta admins edit the affected submitted scorecard while explaining every scorecard-form lock.
- **Customer/system impact:** Restores internal support access without weakening customer-user restrictions.
- **Role:** reviewed

## Scope

**In scope**

- Director lock-reason UI and Cresta-admin exceptions.
- Matching backend submitted-editor allowlist bypass.
- Comparison with the completed CONVI-7598 implementation.

**Non-goals**

- Changing non-admin scorecard authorization.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/director`, `cresta/go-servers`
- **Worktrees:** `/Users/xuanyu.wang/repos/director`
- **Branches:** `convi-7612-scorecard-lock-reason-banner`
- **PRs/commits:** [director#22401](https://github.com/cresta/director/pull/22401) at `d7e951a2467`; [director#22149](https://github.com/cresta/director/pull/22149) at `157d826e890`; [go-servers#31889](https://github.com/cresta/go-servers/pull/31889)

## Current Understanding

Director PR #22401 covers the same lock-explanation area as completed draft PR #22149 and adds Cresta-admin authorization exceptions. It should not merge at its current head because the new `lockReason` controls disabled fields while the save mutation and submit-button rendering still use the older, narrower `readOnly` value.

## Findings and Decisions

- Major: derive the write guard from the resolved lock reason while preserving in-appeal writes. Otherwise acknowledged, answer-key, and appeal-requested locks are not represented in `useSaveScorecardMutation`.
- Coverage gap: unlike #22149, #22401 does not apply shared reason-specific messaging to `ProcessScorecardScoring`; its global hook change still affects that surface.
- The frontend admin bypass depends on go-servers PR #31889 landing first.
- Existing CI passes except the pending codeowner check; CodeRabbit independently requested changes for the write-guard mismatch.

## Blockers and Dependencies

- [go-servers#31889](https://github.com/cresta/go-servers/pull/31889) must land before the frontend admin bypass.
- #22401 needs the lock/write-guard correction and focused integration coverage.

## Validation and Rollout

- Compared complete diffs and changed-file sets for #22401 and #22149.
- Traced `readOnly`, `lockReason`, `disableScorecardForm`, autosave, and submit rendering at #22401 head.
- Verified target CI is green apart from pending codeowner approval.

## Next Actions

1. Update #22401 so one derived lock state consistently drives form controls, autosave/write guards, submit rendering, and user-facing reason copy.
2. Decide whether process-scorecard reason parity from #22149 is required in this PR.
3. Merge/deploy backend authorization support before Director.

## Timeline

- 2026-09-02 — Reviewed #22401 against completed draft #22149 and found a merge-blocking split between visual and write lock state. Evidence: `sessions/2026-09-02/codex-pr-22401-review.md`.
